package tracker

import "math"

// The Yandex API stopped sending the R128 gain along with the tracks, so the
// only way to keep the volume of the tracks on the same level is to measure
// it locally. This file implements a small ITU-R BS.1770 (EBU R128) loudness
// meter: K-weighting filters, 100 ms gating blocks and an absolute gate.
const (
	// _LOUDNESS_TARGET is the loudness every track is brought to. -14 LUFS is
	// the value the streaming services (Yandex included) normalize to.
	_LOUDNESS_TARGET = -14.0
	// _LOUDNESS_SILENCE is the absolute gate of the measurement: blocks
	// quieter than this are not a part of the integrated loudness.
	_LOUDNESS_SILENCE = -70.0
	// _LOUDNESS_RELATIVE_GATE is the second, relative gate: blocks that are
	// this many LU quieter than the track itself are dropped as well.
	_LOUDNESS_RELATIVE_GATE = 10.0
	// _LOUDNESS_MAX_GAIN clamps the correction so that a bad measurement can
	// neither make a track scream nor silence it.
	_LOUDNESS_MAX_GAIN = 12.0
	// _LOUDNESS_BLOCK_MS is the length of one gating block.
	_LOUDNESS_BLOCK_MS = 100
	// _LOUDNESS_MIN_BLOCKS is how many blocks a stream has to have to be
	// measured at all: the filters and the average need some material.
	_LOUDNESS_MIN_BLOCKS = 5
)

// biquad is a second order IIR filter in the transposed direct form II.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	z1, z2             float64
}

func (f *biquad) process(x float64) float64 {
	y := f.b0*x + f.z1
	f.z1 = f.b1*x - f.a1*y + f.z2
	f.z2 = f.b2*x - f.a2*y
	return y
}

// kWeighting is the frequency weighting of the loudness measurement: a high
// shelf followed by a high pass. The coefficients are the ones from BS.1770.
type kWeighting struct {
	pre biquad
	rlb biquad
}

func newKWeighting() kWeighting {
	return kWeighting{
		pre: biquad{
			b0: 1.53512485958697, b1: -2.69169618940638, b2: 1.19839281085285,
			a1: -1.69065929318241, a2: 0.73248077421585,
		},
		rlb: biquad{
			b0: 1.0, b1: -2.0, b2: 1.0,
			a1: -1.99004745483398, a2: 0.99007225036621,
		},
	}
}

// loudnessMeter accumulates the loudness of the decoded PCM stream. Every
// sample of a block costs a handful of multiplications, so the meter can run
// on the audio thread.
type loudnessMeter struct {
	sampleRate int
	blockSize  int
	filters    [2]kWeighting
	sum        float64
	count      int
	blocks     []float64
	peak       float64
	rest       []byte
}

func newLoudnessMeter(sampleRate int) *loudnessMeter {
	if sampleRate <= 0 {
		sampleRate = 44100
	}

	return &loudnessMeter{
		sampleRate: sampleRate,
		blockSize:  sampleRate * _LOUDNESS_BLOCK_MS / 1000,
		filters:    [2]kWeighting{newKWeighting(), newKWeighting()},
	}
}

// add feeds the next chunk of the decoded stream: signed 16 bit little endian
// stereo PCM, as the mp3 decoder always produces.
func (m *loudnessMeter) add(pcm []byte) {
	if len(m.rest) > 0 {
		joined := make([]byte, 0, len(m.rest)+len(pcm))
		joined = append(joined, m.rest...)
		joined = append(joined, pcm...)
		pcm = joined
		m.rest = nil
	}

	// a chunk may end in the middle of a stereo sample: keep it for later
	if extra := len(pcm) % 4; extra != 0 {
		m.rest = append(m.rest, pcm[len(pcm)-extra:]...)
		pcm = pcm[:len(pcm)-extra]
	}

	for i := 0; i+3 < len(pcm); i += 4 {
		left := float64(int16(uint16(pcm[i])|uint16(pcm[i+1])<<8)) / 32768
		right := float64(int16(uint16(pcm[i+2])|uint16(pcm[i+3])<<8)) / 32768

		if abs := math.Abs(left); abs > m.peak {
			m.peak = abs
		}
		if abs := math.Abs(right); abs > m.peak {
			m.peak = abs
		}

		l := m.filters[0].pre.process(left)
		l = m.filters[0].rlb.process(l)
		r := m.filters[1].pre.process(right)
		r = m.filters[1].rlb.process(r)

		m.sum += l*l + r*r
		m.count++
		if m.count >= m.blockSize {
			m.blocks = append(m.blocks, m.sum/float64(2*m.count))
			m.sum = 0
			m.count = 0
		}
	}
}

// integratedLoudness returns the gated loudness of everything that was fed to
// the meter, or -inf when the stream was silent. The measurement is gated
// twice, the way BS.1770 says: the absolute gate drops the silence, and the
// relative one drops the parts that are much quieter than the track itself, so
// that a quiet intro does not make the whole track louder.
func (m *loudnessMeter) integratedLoudness() float64 {
	absolute := m.gatedLoudness(_LOUDNESS_SILENCE)
	if math.IsInf(absolute, -1) {
		return absolute
	}

	return m.gatedLoudness(absolute - _LOUDNESS_RELATIVE_GATE)
}

// gatedLoudness averages the blocks that are louder than the given threshold
// (in LUFS).
func (m *loudnessMeter) gatedLoudness(threshold float64) float64 {
	var (
		sum   float64
		count int
	)

	for _, block := range m.blocks {
		if block <= 0 {
			continue
		}
		if loudness := -0.691 + 10*math.Log10(block); loudness <= threshold {
			continue
		}
		sum += block
		count++
	}

	if count == 0 {
		return math.Inf(-1)
	}

	return -0.691 + 10*math.Log10(sum/float64(count))
}

// normalization turns the measurement into the gain of the track. Streams that
// are too short to be measured (and silent ones) are played as is.
func (m *loudnessMeter) normalization() normalization {
	// the filters need some time to settle, and a stream that is shorter than
	// half a second is not a track anyway
	if len(m.blocks) < _LOUDNESS_MIN_BLOCKS {
		return noNormalization
	}

	loudness := m.integratedLoudness()
	if math.IsInf(loudness, -1) {
		return noNormalization
	}

	gain := _LOUDNESS_TARGET - loudness
	if gain > _LOUDNESS_MAX_GAIN {
		gain = _LOUDNESS_MAX_GAIN
	} else if gain < -_LOUDNESS_MAX_GAIN {
		gain = -_LOUDNESS_MAX_GAIN
	}

	n := normalization{
		linear: float32(math.Pow(10, gain/20)),
		limit:  1,
	}

	// the peak is limited as well: the gain is computed for the average
	// loudness of the track, but a loud peak can still go over the top
	if m.peak > 0 {
		if loudest := m.peak * float64(n.linear); loudest > 1 {
			n.limit = float32(1 / loudest)
		}
	}

	return n
}
