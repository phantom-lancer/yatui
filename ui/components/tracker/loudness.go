package tracker

import "math"

// The Yandex API stopped sending the R128 gain along with the tracks, so the
// only way to keep the volume of the tracks on the same level is to measure
// it locally. This file implements a small ITU-R BS.1770 (EBU R128) loudness
// meter: K-weighting filters, 400 ms gating windows that overlap by 75
// percents and the absolute and the relative gates.
const (
	// _LOUDNESS_TARGET is the loudness every track is brought to. -14 LUFS is
	// the value the streaming services (Yandex included) normalize to.
	_LOUDNESS_TARGET = -14.0
	// _LOUDNESS_SILENCE is the absolute gate of the measurement: windows
	// quieter than this are not a part of the integrated loudness.
	_LOUDNESS_SILENCE = -70.0
	// _LOUDNESS_RELATIVE_GATE is the second, relative gate: windows that are
	// this many LU quieter than the track itself are dropped as well.
	_LOUDNESS_RELATIVE_GATE = 10.0
	// _LOUDNESS_MAX_GAIN clamps the correction so that a bad measurement can
	// neither make a track scream nor silence it.
	_LOUDNESS_MAX_GAIN = 12.0
	// _LOUDNESS_BLOCK_MS is the length of one gating block.
	_LOUDNESS_BLOCK_MS = 100
	// _LOUDNESS_WINDOW_BLOCKS is how many blocks a gating window covers:
	// BS.1770 measures 400 ms with a 75 percents overlap, which is one block
	// of a new material per window.
	_LOUDNESS_WINDOW_BLOCKS = 4
	// _LOUDNESS_MIN_WINDOWS is how many windows a stream has to have to be
	// measured at all: the filters and the average need some material.
	_LOUDNESS_MIN_WINDOWS = 5
	// _LOUDNESS_OFFSET is the calibration offset of BS.1770.
	_LOUDNESS_OFFSET = -0.691
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
// shelf followed by a high pass.
type kWeighting struct {
	pre biquad
	rlb biquad
}

// newKWeighting designs the filters of the K-weighting for the given sample
// rate. The coefficients the recommendation lists are the ones for 48 kHz, so
// they are recomputed here: a stream of another rate filtered with them would
// be weighted differently and the measurement would be off.
func newKWeighting(sampleRate int) kWeighting {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	fs := float64(sampleRate)

	// the high shelf: +4 dB above 1.7 kHz
	{
		const (
			f0   = 1681.974450955533
			q    = 0.7071752369554196
			gain = 3.999843853973347
		)
		k := math.Tan(math.Pi * f0 / fs)
		vh := math.Pow(10, gain/20)
		vb := math.Pow(vh, 0.4996667741545416)
		a0 := 1 + k/q + k*k
		pre := kWeighting{}.pre
		pre.b0 = (vh + vb*k/q + k*k) / a0
		pre.b1 = 2 * (k*k - vh) / a0
		pre.b2 = (vh - vb*k/q + k*k) / a0
		pre.a1 = 2 * (k*k - 1) / a0
		pre.a2 = (1 - k/q + k*k) / a0

		// the high pass: the "RLB" curve of the recommendation
		const (
			hpF0 = 38.13547087602444
			hpQ  = 0.5003270373238773
		)
		hk := math.Tan(math.Pi * hpF0 / fs)
		ha0 := 1 + hk/hpQ + hk*hk
		rlb := kWeighting{}.rlb
		// the recommendation does not normalize the numerator of the high
		// pass: its gain at the top of the band is 1.0043, not 1
		rlb.b0 = 1
		rlb.b1 = -2
		rlb.b2 = 1
		rlb.a1 = 2 * (hk*hk - 1) / ha0
		rlb.a2 = (1 - hk/hpQ + hk*hk) / ha0

		return kWeighting{pre: pre, rlb: rlb}
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
	windows    []float64
	ring       [_LOUDNESS_WINDOW_BLOCKS]float64
	ringIndex  int
	ringFull   bool
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
		filters:    [2]kWeighting{newKWeighting(sampleRate), newKWeighting(sampleRate)},
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
			m.pushBlock(m.sum / float64(m.count))
			m.sum = 0
			m.count = 0
		}
	}
}

// pushBlock closes a gating block of the meter. The energy of a block is the
// sum of the mean squares of all channels, the way BS.1770 defines it.
func (m *loudnessMeter) pushBlock(energy float64) {
	m.blocks = append(m.blocks, energy)

	m.ring[m.ringIndex] = energy
	m.ringIndex = (m.ringIndex + 1) % _LOUDNESS_WINDOW_BLOCKS
	if m.ringIndex == 0 {
		m.ringFull = true
	}
	if !m.ringFull {
		return
	}

	var sum float64
	for _, block := range m.ring {
		sum += block
	}
	m.windows = append(m.windows, sum/_LOUDNESS_WINDOW_BLOCKS)
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

// gatedLoudness averages the windows that are louder than the given threshold
// (in LUFS).
func (m *loudnessMeter) gatedLoudness(threshold float64) float64 {
	var (
		sum   float64
		count int
	)

	for _, window := range m.windows {
		if window <= 0 {
			continue
		}
		if loudness := _LOUDNESS_OFFSET + 10*math.Log10(window); loudness <= threshold {
			continue
		}
		sum += window
		count++
	}

	if count == 0 {
		return math.Inf(-1)
	}

	return _LOUDNESS_OFFSET + 10*math.Log10(sum/float64(count))
}

// normalization turns the measurement into the gain of the track. Streams that
// are too short to be measured (and silent ones) are played as is.
func (m *loudnessMeter) normalization() normalization {
	// the filters need some time to settle, and a stream that is shorter than
	// half a second is not a track anyway
	if len(m.windows) < _LOUDNESS_MIN_WINDOWS {
		return noNormalization
	}

	return normalizationFor(m.integratedLoudness(), m.peak)
}

// normalizationFor builds the correction of a track with a known loudness.
func normalizationFor(loudness, peak float64) normalization {
	if math.IsInf(loudness, 0) || math.IsNaN(loudness) {
		return noNormalization
	}

	gain := _LOUDNESS_TARGET - loudness
	if gain > _LOUDNESS_MAX_GAIN {
		gain = _LOUDNESS_MAX_GAIN
	} else if gain < -_LOUDNESS_MAX_GAIN {
		gain = -_LOUDNESS_MAX_GAIN
	}

	return newGainNormalization(gain, peak)
}
