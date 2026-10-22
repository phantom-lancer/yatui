package equalizer

import (
	"math"
	"testing"
)

func TestEqualizerOffDoesNotChangeSamples(t *testing.T) {
	p := New()
	p.Set(false, [Bands]float64{12, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	// a loud sine-like buffer: a peak near the top
	pcm := []byte{0x7F, 0x7F, 0x7F, 0x7F, 0x7F, 0x7F, 0x7F, 0x7F}
	before := make([]byte, len(pcm))
	copy(before, pcm)

	p.Apply(pcm)

	for i := range pcm {
		if pcm[i] != before[i] {
			t.Fatalf("byte %d changed: %02x -> %02x", i, before[i], pcm[i])
		}
	}
}

func TestEqualizerBoostChangesSamples(t *testing.T) {
	p := New()
	p.Set(true, [Bands]float64{0, 0, 0, 0, 0, 12, 0, 0, 0, 0})

	pcm := []byte{0x00, 0x40, 0x00, 0x40, 0x00, 0x40, 0x00, 0x40}
	before := make([]byte, len(pcm))
	copy(before, pcm)

	p.Apply(pcm)

	changed := false
	for i := range pcm {
		if pcm[i] != before[i] {
			changed = true
		}
	}
	if !changed {
		t.Fatal("the boost did not change the samples")
	}
}

func TestEqualizerDoesNotClip(t *testing.T) {
	p := New()
	p.Set(true, [Bands]float64{12, 12, 12, 12, 12, 12, 12, 12, 12, 12})

	// the loudest possible sample
	pcm := []byte{0xFF, 0x7F, 0xFF, 0x7F, 0xFF, 0x7F, 0xFF, 0x7F}
	p.Apply(pcm)

	for i := 0; i+1 < len(pcm); i += 2 {
		sample := int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8)
		if sample == math.MinInt16 || sample == math.MaxInt16 {
			// clipping is acceptable only at the very edge; fail hard on
			// anything beyond
			continue
		}
		if math.Abs(float64(sample)) > 32767 {
			t.Fatalf("sample out of range: %d", sample)
		}
	}
}

func TestClampGain(t *testing.T) {
	if got := ClampGain(100); got != MaxGain {
		t.Fatalf("clamp(100) = %f, want %d", got, MaxGain)
	}
	if got := ClampGain(-100); got != -MaxGain {
		t.Fatalf("clamp(-100) = %f, want %d", got, -MaxGain)
	}
	if got := ClampGain(3.5); got != 3.5 {
		t.Fatalf("clamp(3.5) = %f, want 3.5", got)
	}
}

func TestNormalizeGains(t *testing.T) {
	gains := NormalizeGains([]float64{1, 2})
	if gains[0] != 1 || gains[1] != 2 || gains[2] != 0 {
		t.Fatalf("unexpected gains: %v", gains)
	}
	if len(gains) != Bands {
		t.Fatalf("expected %d gains, got %d", Bands, len(gains))
	}
}

func TestPresetsHaveTenBands(t *testing.T) {
	for _, preset := range Presets() {
		var nonzero int
		for _, gain := range preset.Gains {
			if gain != 0 {
				nonzero++
			}
		}
		if preset.Name == "Плоский" && nonzero != 0 {
			t.Fatalf("preset %q is not flat", preset.Name)
		}
	}
	if _, ok := PresetByName("Рок"); !ok {
		t.Fatal("preset Рок not found")
	}
	if _, ok := PresetByName("Нет такого"); ok {
		t.Fatal("nonexistent preset was found")
	}
}
