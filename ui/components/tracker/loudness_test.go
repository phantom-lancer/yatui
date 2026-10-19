package tracker

import (
	"math"
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

func TestLoudnessMeterMeasuresTone(t *testing.T) {
	meter := newLoudnessMeter(_TEST_SAMPLE_RATE)
	meter.add(sineWave(5, 1000, 0.5))

	// a 1 kHz tone at half scale is about -9 LUFS: -0.691 + 10*log10(0.125)
	// plus the high shelf of the weighting
	loudness := meter.integratedLoudness()
	if loudness < -11 || loudness > -8 {
		t.Fatalf("unexpected loudness of a 1 kHz tone: %.2f LUFS", loudness)
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
	// three seconds of a very quiet intro followed by five loud ones: the
	// correction has to follow the music, not the intro
	pcm := append(sineWave(3, 1000, 0.02), sineWave(5, 1000, 0.8)...)

	loud := newLoudnessMeter(_TEST_SAMPLE_RATE)
	loud.add(sineWave(5, 1000, 0.8))

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
