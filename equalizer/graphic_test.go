package equalizer

import (
	"math"
	"testing"
)

func TestGraphicHasFifteenSortedBands(t *testing.T) {
	if len(GraphicFrequencies) != GraphicBands {
		t.Fatalf("%d frequencies for %d bands", len(GraphicFrequencies), GraphicBands)
	}
	for i := 1; i < len(GraphicFrequencies); i++ {
		if GraphicFrequencies[i] <= GraphicFrequencies[i-1] {
			t.Fatalf("the frequencies are not sorted at %d: %v", i, GraphicFrequencies)
		}
	}
}

func TestResampleKeepsAFlatCurveFlat(t *testing.T) {
	legacy := [Bands]float64{}
	got := ResampleGains(BandFrequencies[:], legacy[:], GraphicFrequencies[:])
	for i, gain := range got {
		if math.Abs(gain) > 0.001 {
			t.Fatalf("band %d is %.2f dB instead of flat", i, gain)
		}
	}
}

func TestResampleKeepsTheShapeOfABoost(t *testing.T) {
	legacy := [Bands]float64{0, 0, 0, 0, 0, 12, 0, 0, 0, 0}
	got := ResampleGains(BandFrequencies[:], legacy[:], GraphicFrequencies[:])

	peakBand := 0
	for i, gain := range got {
		if gain > got[peakBand] {
			peakBand = i
		}
	}
	// the boosted legacy band is 3 kHz, which lands between the 2.5 and
	// 4 kHz graphic bands
	if peakBand < 9 || peakBand > 11 {
		t.Fatalf("the peak moved to band %d (%v), want around 2.5-4 kHz", peakBand, got)
	}
	if got[peakBand] < 6 {
		t.Fatalf("the peak is only %.1f dB: %v", got[peakBand], got)
	}
	if math.Abs(got[0]) > 0.5 || math.Abs(got[GraphicBands-1]) > 0.5 {
		t.Fatalf("the edges were lifted: %v", got)
	}
}

func TestGraphicOffDoesNotChangeSamples(t *testing.T) {
	p := NewGraphic()
	var gains [GraphicBands]float64
	gains[8] = 12
	p.Set(false, gains)

	pcm := []byte{0x10, 0x20, 0x30, 0x40}
	before := append([]byte(nil), pcm...)
	p.Apply(pcm)
	for i := range pcm {
		if pcm[i] != before[i] {
			t.Fatalf("byte %d changed: %02x -> %02x", i, before[i], pcm[i])
		}
	}
}

func TestGraphicBoostChangesSamples(t *testing.T) {
	p := NewGraphic()
	var gains [GraphicBands]float64
	gains[8] = 12 // 1 kHz
	p.Set(true, gains)

	// a 1 kHz sine at 44.1 kHz: the band must amplify it
	pcm := sinePCM(1000, 44100, 0.25, 2048)
	before := append([]byte(nil), pcm...)
	p.Apply(pcm)

	changed := false
	for i := range pcm {
		if pcm[i] != before[i] {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("the boost did not change the samples")
	}
}

func TestGraphicDoesNotClip(t *testing.T) {
	p := NewGraphic()
	var gains [GraphicBands]float64
	for i := range gains {
		gains[i] = 12
	}
	p.Set(true, gains)

	pcm := []byte{0xFF, 0x7F, 0xFF, 0x7F, 0xFF, 0x7F, 0xFF, 0x7F}
	p.Apply(pcm)

	// a wrap-around would flip the sign of the loudest positive sample
	sample := int16(uint16(pcm[0]) | uint16(pcm[1])<<8)
	if sample < 0 {
		t.Fatalf("the loudest sample wrapped around: %d", sample)
	}
}

func TestGraphicBandsAboveNyquistAreClamped(t *testing.T) {
	p := NewGraphic()
	p.SetRate(22050)
	var gains [GraphicBands]float64
	gains[GraphicBands-1] = 12 // 16 kHz, above the 11 kHz Nyquist limit
	p.Set(true, gains)

	pcm := sinePCM(4000, 22050, 0.1, 1024)
	p.Apply(pcm)
	for i := 0; i+1 < len(pcm); i += 2 {
		sample := int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8)
		if sample > 32000 || sample < -32000 {
			t.Fatalf("the filter is unstable at the band edge: %d", sample)
		}
	}
}

func TestGraphicPresetsCoverTheBands(t *testing.T) {
	presets := GraphicPresets()
	if len(presets) == 0 {
		t.Fatal("no graphic presets")
	}
	for _, preset := range presets {
		if preset.Name == "" {
			t.Fatal("a preset without a name")
		}
		for i, gain := range preset.Gains {
			if math.IsNaN(gain) || gain < -MaxGain || gain > MaxGain {
				t.Fatalf("preset %q band %d is %v", preset.Name, i, gain)
			}
		}
	}
	if _, ok := GraphicPresetByName(presets[0].Name); !ok {
		t.Fatal("a preset cannot be found by name")
	}
}

func sinePCM(freq, rate int, amplitude float64, samples int) []byte {
	pcm := make([]byte, samples*4)
	for i := 0; i < samples; i++ {
		value := int16(amplitude * 32767 * math.Sin(2*math.Pi*float64(freq)*float64(i)/float64(rate)))
		for ch := 0; ch < 2; ch++ {
			pcm[i*4+ch*2] = byte(value)
			pcm[i*4+ch*2+1] = byte(uint16(value) >> 8)
		}
	}
	return pcm
}

func TestGraphicRampsTowardsTheTarget(t *testing.T) {
	p := NewGraphic()
	var flat [GraphicBands]float64
	p.Set(true, flat)

	var target [GraphicBands]float64
	target[8] = 12
	p.Set(true, target)

	if !p.ramping {
		t.Fatal("a large change must ramp, not jump")
	}
	if p.current[8] != 0 {
		t.Fatalf("the applied gain jumped to %.2f dB", p.current[8])
	}

	buf := sinePCM(1000, 44100, 0.1, 512)
	for i := 0; i < 200 && p.ramping; i++ {
		p.Apply(buf)
	}
	if p.ramping {
		t.Fatal("the ramp never finished")
	}
	if math.Abs(p.current[8]-12) > 0.01 {
		t.Fatalf("the ramp stopped at %.2f dB", p.current[8])
	}
}
