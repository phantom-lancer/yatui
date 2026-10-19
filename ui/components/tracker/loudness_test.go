package tracker

import (
	"math"
	"math/cmplx"
	"testing"
)

const _TEST_SAMPLE_RATE = 44100

// sineWave builds the stereo PCM of a sine tone, like the mp3 decoder gives it.
func sineWave(seconds int, freq, amplitude float64) []byte {
	frames := _TEST_SAMPLE_RATE * seconds
	buf := make([]byte, frames*4)
	for i := 0; i < frames; i++ {
		sample := int16(amplitude * 32767 * math.Sin(2*math.Pi*freq*float64(i)/float64(_TEST_SAMPLE_RATE)))
		buf[i*4] = byte(sample)
		buf[i*4+1] = byte(uint16(sample) >> 8)
		buf[i*4+2] = byte(sample)
		buf[i*4+3] = byte(uint16(sample) >> 8)
	}
	return buf
}

// kWeightingGain returns the gain the K-weighting gives to a sine of the given
// frequency: |H(e^jw)| of the two biquads the meter is built of.
func kWeightingGain(sampleRate int, freq float64) float64 {
	w := 2 * math.Pi * freq / float64(sampleRate)
	z1, z2 := complex(math.Cos(-w), math.Sin(-w)), complex(math.Cos(-2*w), math.Sin(-2*w))

	filter := newKWeighting(sampleRate)
	response := func(b biquad) complex128 {
		numerator := complex(b.b0, 0) + complex(b.b1, 0)*z1 + complex(b.b2, 0)*z2
		denominator := complex(1, 0) + complex(b.a1, 0)*z1 + complex(b.a2, 0)*z2
		return numerator / denominator
	}

	h := response(filter.pre) * response(filter.rlb)
	return 20 * math.Log10(cmplx.Abs(h))
}

// loudnessOfTone is what the meter has to show for a stereo sine tone of the
// given amplitude: the energy of the two channels plus the weighting.
func loudnessOfTone(sampleRate int, freq, amplitude float64) float64 {
	amplitude = amplitude * 32767 / 32768
	// the mean square of a sine is half of its squared amplitude, summed over
	// the two channels
	energy := 2 * (amplitude * amplitude / 2)
	return _LOUDNESS_OFFSET + 10*math.Log10(energy) + kWeightingGain(sampleRate, freq)
}

func TestKWeightingMatchesTheRecommendation(t *testing.T) {
	// the coefficients BS.1770 lists for 48 kHz, the ones the reference
	// implementations are built around
	reference := kWeighting{
		pre: biquad{
			b0: 1.53512485958697, b1: -2.69169618940638, b2: 1.19839281085285,
			a1: -1.69065929318241, a2: 0.73248077421585,
		},
		rlb: biquad{
			b0: 1.0, b1: -2.0, b2: 1.0,
			a1: -1.99004745483398, a2: 0.99007225036621,
		},
	}

	got := newKWeighting(48000)
	for _, pair := range [][2]float64{
		{got.pre.b0, reference.pre.b0}, {got.pre.b1, reference.pre.b1},
		{got.pre.b2, reference.pre.b2}, {got.pre.a1, reference.pre.a1},
		{got.pre.a2, reference.pre.a2}, {got.rlb.b0, reference.rlb.b0},
		{got.rlb.b1, reference.rlb.b1}, {got.rlb.b2, reference.rlb.b2},
		{got.rlb.a1, reference.rlb.a1}, {got.rlb.a2, reference.rlb.a2},
	} {
		if diff := math.Abs(pair[0] - pair[1]); diff > 1e-9 {
			t.Fatalf("the filter designed for 48 kHz differs from the reference: %.2e (%f vs %f)",
				diff, pair[0], pair[1])
		}
	}

	// and the design has to be rate dependent: filtering a 44.1 kHz stream
	// with the 48 kHz coefficients weights it differently
	if diff := math.Abs(newKWeighting(44100).pre.b0 - newKWeighting(48000).pre.b0); diff < 1e-6 {
		t.Fatalf("the weighting does not depend on the sample rate: %.4e", diff)
	}
	if diff := math.Abs(newKWeighting(44100).rlb.a1 - newKWeighting(48000).rlb.a1); diff < 1e-6 {
		t.Fatalf("the weighting does not depend on the sample rate: %.4e", diff)
	}
}

func TestLoudnessMeterMeasuresTone(t *testing.T) {
	const (
		freq      = 1000.0
		amplitude = 0.5
	)

	meter := newLoudnessMeter(_TEST_SAMPLE_RATE)
	meter.add(sineWave(5, freq, amplitude))

	want := loudnessOfTone(_TEST_SAMPLE_RATE, freq, amplitude)
	if diff := math.Abs(meter.integratedLoudness() - want); diff > 0.1 {
		t.Fatalf("a 1 kHz tone measured %.2f LUFS, want %.2f", meter.integratedLoudness(), want)
	}
}

func TestLoudnessMeterSumsTheChannels(t *testing.T) {
	// BS.1770 measures the energy of all channels summed, not averaged: the
	// same tone in both channels is 3 dB louder than it is in one of them
	stereo := newLoudnessMeter(_TEST_SAMPLE_RATE)
	stereo.add(sineWave(5, 1000, 0.5))

	mono := newLoudnessMeter(_TEST_SAMPLE_RATE)
	silentRight := sineWave(5, 1000, 0.5)
	for i := 0; i+3 < len(silentRight); i += 4 {
		silentRight[i+2], silentRight[i+3] = 0, 0
	}
	mono.add(silentRight)

	diff := stereo.integratedLoudness() - mono.integratedLoudness()
	if math.Abs(diff-3.0103) > 0.05 {
		t.Fatalf("the channels are not summed: a stereo tone is %.3f dB louder than a mono one", diff)
	}
}

func TestLoudnessMeterNormalizationHitsTarget(t *testing.T) {
	pcm := sineWave(5, 1000, 0.5)

	meter := newLoudnessMeter(_TEST_SAMPLE_RATE)
	meter.add(pcm)

	n := meter.normalization()
	if !n.scale() {
		t.Fatal("a tone has to be normalized")
	}

	// after the correction the measured loudness has to be exactly the target
	n.apply(pcm)

	after := newLoudnessMeter(_TEST_SAMPLE_RATE)
	after.add(pcm)

	if diff := math.Abs(after.integratedLoudness() - _LOUDNESS_TARGET); diff > 0.5 {
		t.Fatalf("loudness after the normalization is %.2f LUFS, want %.2f",
			after.integratedLoudness(), _LOUDNESS_TARGET)
	}
}

func TestLoudnessMeterQuietTrackIsBoosted(t *testing.T) {
	meter := newLoudnessMeter(_TEST_SAMPLE_RATE)
	meter.add(sineWave(5, 1000, 0.05))

	if gain := meter.normalization().gainDB(); gain < 5 {
		t.Fatalf("a quiet track has to be boosted, got %.2f dB", gain)
	}
}

func TestLoudnessMeterLoudTrackIsTonedDown(t *testing.T) {
	meter := newLoudnessMeter(_TEST_SAMPLE_RATE)
	meter.add(sineWave(5, 1000, 1))

	if gain := meter.normalization().gainDB(); gain > -3 {
		t.Fatalf("a loud track has to be toned down, got %.2f dB", gain)
	}
}

func TestLoudnessMeterGainIsClamped(t *testing.T) {
	meter := newLoudnessMeter(_TEST_SAMPLE_RATE)
	meter.add(sineWave(5, 1000, 0.0005))

	if gain := meter.normalization().gainDB(); gain > _LOUDNESS_MAX_GAIN+0.01 {
		t.Fatalf("the correction was not clamped: %.2f dB", gain)
	}
}

func TestLoudnessMeterSilenceIsNoop(t *testing.T) {
	meter := newLoudnessMeter(_TEST_SAMPLE_RATE)
	meter.add(make([]byte, _TEST_SAMPLE_RATE*4*2))

	if n := meter.normalization(); n.scale() {
		t.Fatalf("silence must not be normalized, got %+v", n)
	}
}

func TestLoudnessMeterShortTrackIsNoop(t *testing.T) {
	meter := newLoudnessMeter(_TEST_SAMPLE_RATE)
	// a fifth of a second is too short to be measured
	meter.add(sineWave(1, 1000, 0.5)[:_TEST_SAMPLE_RATE/5*4])

	if n := meter.normalization(); n.scale() {
		t.Fatalf("a short track must not be normalized, got %+v", n)
	}
}

func TestLoudnessMeterSplitsSamples(t *testing.T) {
	pcm := sineWave(3, 1000, 0.5)

	whole := newLoudnessMeter(_TEST_SAMPLE_RATE)
	whole.add(pcm)

	// the decoder can hand over a chunk that ends in the middle of a sample
	split := newLoudnessMeter(_TEST_SAMPLE_RATE)
	for i := 0; i < len(pcm); i += 999 {
		end := i + 999
		if end > len(pcm) {
			end = len(pcm)
		}
		split.add(pcm[i:end])
	}

	if whole.peak != split.peak {
		t.Fatalf("peak differs: %.6f vs %.6f", whole.peak, split.peak)
	}
	if diff := math.Abs(whole.integratedLoudness() - split.integratedLoudness()); diff > 0.001 {
		t.Fatalf("loudness differs: %.6f vs %.6f", whole.integratedLoudness(), split.integratedLoudness())
	}
}

func TestLoudnessMeterQuietIntroIsIgnored(t *testing.T) {
	// three seconds of a very quiet intro followed by a long loud part: the
	// correction has to follow the music, not the intro
	pcm := append(sineWave(3, 1000, 0.002), sineWave(12, 1000, 0.8)...)

	loud := newLoudnessMeter(_TEST_SAMPLE_RATE)
	loud.add(sineWave(12, 1000, 0.8))

	withIntro := newLoudnessMeter(_TEST_SAMPLE_RATE)
	withIntro.add(pcm)

	diff := math.Abs(loud.normalization().gainDB() - withIntro.normalization().gainDB())
	if diff > 0.5 {
		t.Fatalf("the quiet intro changed the correction by %.2f dB", diff)
	}
}

func TestLoudnessMeterPeakLimiting(t *testing.T) {
	// a quiet signal with a single loud spike: the average loudness asks for a
	// boost, but the spike must not clip
	pcm := sineWave(5, 1000, 0.15)
	// one full scale sample at the very beginning
	copy(pcm, []byte{0xff, 0x7f, 0xff, 0x7f})

	meter := newLoudnessMeter(_TEST_SAMPLE_RATE)
	meter.add(pcm)

	n := meter.normalization()
	if n.linear < 1 {
		t.Fatalf("the track has to be boosted, got %.2f dB", n.gainDB())
	}
	if n.limit >= 1 {
		t.Fatalf("the peak was not limited, got %+v", n)
	}
	if loudest := meter.peak * float64(n.linear) * float64(n.limit); loudest > 1.0001 {
		t.Fatalf("the signal can clip: %f", loudest)
	}
}
