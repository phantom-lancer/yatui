package tracker

import (
	"math"
	"testing"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
)

// helper: builds a stereo PCM buffer from int16 samples (little endian)
func pcmBuffer(samples ...int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		buf[i*2] = byte(s)
		buf[i*2+1] = byte(s >> 8)
	}
	return buf
}

func readSamples(buf []byte) []int16 {
	samples := make([]int16, len(buf)/2)
	for i := range samples {
		samples[i] = int16(uint16(buf[i*2]) | uint16(buf[i*2+1])<<8)
	}
	return samples
}

func setNormalization(t *testing.T, enabled bool) {
	t.Helper()
	previous := config.Current.Normalization
	config.Current.Normalization = &config.Normalization{Enabled: enabled}
	t.Cleanup(func() { config.Current.Normalization = previous })
}

func TestNormalizationDisabled(t *testing.T) {
	setNormalization(t, false)

	track := &api.Track{}
	track.Normalization.Gain = -6
	track.Normalization.Peak = 0.9

	n, _ := newNormalization(track)
	if n.scale() {
		t.Fatalf("disabled normalization must not scale, got %+v", n)
	}

	buf := pcmBuffer(1000, -1000, 32000)
	before := readSamples(buf)
	n.apply(buf)
	after := readSamples(buf)
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("sample %d changed: %d -> %d", i, before[i], after[i])
		}
	}
}

func TestNormalizationNoInfoIsMeasured(t *testing.T) {
	setNormalization(t, true)

	// the API does not send the gain anymore: such tracks have to be measured
	n, measure := newNormalization(&api.Track{})
	if n.scale() {
		t.Fatalf("track without gain info must not scale on its own, got %+v", n)
	}
	if !measure {
		t.Fatal("track without gain info has to be measured")
	}

	// a disabled normalization must never measure anything
	setNormalization(t, false)
	if _, measure := newNormalization(&api.Track{}); measure {
		t.Fatal("disabled normalization must not measure the track")
	}
}

func TestNormalizationAppliesGain(t *testing.T) {
	setNormalization(t, true)

	track := &api.Track{}
	track.Normalization.Gain = -6.0206 // ~ -1/2 in linear scale
	track.Normalization.Peak = 0

	n, _ := newNormalization(track)
	if !n.scale() {
		t.Fatal("expected scaling to be enabled")
	}

	buf := pcmBuffer(1000, -1000)
	n.apply(buf)

	for i, sample := range readSamples(buf) {
		want := int16(-500)
		if i == 0 {
			want = 500
		}
		if diff := int(sample) - int(want); diff > 2 || diff < -2 {
			t.Fatalf("sample %d: got %d, want ~%d", i, sample, want)
		}
	}
}

func TestNormalizationPeakLimiting(t *testing.T) {
	setNormalization(t, true)

	track := &api.Track{}
	track.Normalization.Gain = 6 // +6 dB => linear ~2.0
	track.Normalization.Peak = 0.8

	n, _ := newNormalization(track)
	if !n.scale() {
		t.Fatal("expected scaling to be enabled")
	}

	// the limiter must bring the declared peak of the track exactly to the
	// full scale: 2.0 * 0.8 == 1.6, so the result is divided by 1.6
	total := float64(n.linear) * float64(n.limit)
	if diff := total*0.8 - 1; diff > 0.001 || diff < -0.001 {
		t.Fatalf("declared peak is not at full scale: %f", total*0.8)
	}

	// a sample above the declared peak has to be clamped, not wrapped around
	buf := pcmBuffer(32000, -32000)
	n.apply(buf)

	samples := readSamples(buf)
	if samples[0] <= 0 || samples[1] >= 0 {
		t.Fatalf("samples wrapped around instead of being clamped: %v", samples)
	}
}

func TestNormalizationClampsInvalidGain(t *testing.T) {
	setNormalization(t, true)

	// a broken value from the API must not be able to destroy the audio
	track := &api.Track{}
	track.Normalization.Gain = 200

	n, _ := newNormalization(track)
	if n.linear > 10 {
		t.Fatalf("gain was not clamped: %f", n.linear)
	}
}

func TestNormalizationPeakGuard(t *testing.T) {
	// a track measured on a quiet head asks for a boost, but the loud part of
	// it arrives later: the guard has to lower the gain instead of clipping
	n := newGainNormalization(12, 0.1)
	before := n.gainDB()

	buf := pcmBuffer(32767, -32767, 32767, -32767)
	if guarded := n.apply(buf); !guarded {
		t.Fatal("the peak guard did not react to a clipping buffer")
	}

	for i, sample := range readSamples(buf) {
		if sample == math.MaxInt16 || sample == math.MinInt16 {
			t.Fatalf("sample %d was clipped: %d", i, sample)
		}
	}
	if after := n.gainDB(); after >= before {
		t.Fatalf("the gain was not lowered: %.2f -> %.2f", before, after)
	}

	// and a quiet buffer must not be raised back up
	guarded := n.gainDB()
	quiet := pcmBuffer(1000, -1000)
	n.apply(quiet)
	if again := n.gainDB(); again != guarded {
		t.Fatalf("the guard is not sticky: %.2f -> %.2f", guarded, again)
	}
	for _, sample := range readSamples(quiet) {
		if math.Abs(float64(sample)) > 1000.0001 {
			t.Fatalf("the quiet buffer was not left below the guarded gain: %d", sample)
		}
	}
}
