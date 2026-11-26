package equalizer

import (
	"math"
	"sync"
)

// The 15 band equalizer follows the layout of the equalizer of yamusic:
// peaking biquad filters with Q = 1.0 on the ISO-ish frequencies below.
// The legacy ten band presets are kept in equalizer.go untouched; this file
// is the modern engine that the graphic view drives.

const (
	// GraphicBands is the number of the bands of the graphic equalizer.
	GraphicBands = 15

	// GraphicQ is the quality factor of every graphic band.
	GraphicQ = 1.0
)

// GraphicFrequencies are the centre frequencies of the graphic bands, in
// hertz, from the lowest to the highest one.
var GraphicFrequencies = [GraphicBands]float64{
	25, 40, 63, 100, 160, 250, 400, 630, 1000, 1600, 2500, 4000, 6300, 10000, 16000,
}

// GraphicLabels are the names of the bands, as the user sees them.
var GraphicLabels = [GraphicBands]string{
	"25", "40", "63", "100", "160", "250", "400", "630", "1к",
	"1.6к", "2.5к", "4к", "6.3к", "10к", "16к",
}

// GraphicPreset is a named set of the gains of the graphic equalizer.
type GraphicPreset struct {
	Name  string
	Gains [GraphicBands]float64
}

// graphicPresets are built from the legacy presets at startup, so the two
// views keep the same character. The mapping happens in the log frequency
// domain, which preserves the shape of the curve instead of dropping bands.
var graphicPresets = buildGraphicPresets()

func buildGraphicPresets() []GraphicPreset {
	presets := make([]GraphicPreset, 0, len(_PRESETS))
	for _, legacy := range _PRESETS {
		gains := ResampleGains(BandFrequencies[:], legacy.Gains[:], GraphicFrequencies[:])
		presets = append(presets, GraphicPreset{Name: legacy.Name, Gains: gains})
	}
	return presets
}

// GraphicPresets returns the list of the graphic presets.
func GraphicPresets() []GraphicPreset {
	presets := make([]GraphicPreset, len(graphicPresets))
	copy(presets, graphicPresets)
	return presets
}

// GraphicPresetByName returns the graphic preset with the given name.
func GraphicPresetByName(name string) (GraphicPreset, bool) {
	for _, preset := range graphicPresets {
		if preset.Name == name {
			return preset, true
		}
	}
	return GraphicPreset{}, false
}

// NormalizeGraphicGains turns a list of gains into the gains of the fifteen
// bands. The missing ones are left at zero.
func NormalizeGraphicGains(gains []float64) [GraphicBands]float64 {
	var result [GraphicBands]float64
	for i := 0; i < GraphicBands && i < len(gains); i++ {
		result[i] = ClampGain(gains[i])
	}
	return result
}

// ResampleGains converts the gains of the source bands to the target bands.
// Both frequency lists must be sorted from the lowest to the highest one.
// The conversion is linear in log frequency, so a smooth legacy curve stays
// smooth; frequencies outside the source range use the nearest source band.
func ResampleGains(sourceFreqs, sourceGains, targetFreqs []float64) [GraphicBands]float64 {
	var result [GraphicBands]float64
	if len(sourceFreqs) == 0 || len(sourceFreqs) != len(sourceGains) {
		return result
	}

	for i, target := range targetFreqs {
		if i >= len(result) {
			break
		}
		result[i] = ClampGain(interpolateLogGain(sourceFreqs, sourceGains, target))
	}
	return result
}

func interpolateLogGain(freqs, gains []float64, target float64) float64 {
	if target <= freqs[0] {
		return gains[0]
	}
	for i := 1; i < len(freqs); i++ {
		if target <= freqs[i] {
			low, high := freqs[i-1], freqs[i]
			if high <= low {
				return gains[i]
			}
			// log interpolation: pitch is logarithmic, a plain linear
			// interpolation would overstate the low bands
			t := (math.Log(target) - math.Log(low)) / (math.Log(high) - math.Log(low))
			return gains[i-1] + (gains[i]-gains[i-1])*t
		}
	}
	return gains[len(gains)-1]
}

// GraphicProcessor applies the fifteen band equalizer to the PCM of a
// track. It mirrors Processor: the settings are guarded by a mutex, the
// audio thread takes it for a buffer only, and nothing allocates in Apply.
type GraphicProcessor struct {
	mu      sync.Mutex
	enabled bool
	// gains is the target the user set; current follows it smoothly, so a
	// preset or a dragged point does not click.
	gains   [GraphicBands]float64
	current [GraphicBands]float64
	ramping bool
	coeffs  [GraphicBands]biquad
	active  []int
	state   [GraphicBands][2]biquadState
	rate    int
}

// graphicRampStep is how many decibels one audio buffer may move towards the
// target. At about fifty buffers per second a full preset change takes half a
// second, which is audible as a slide instead of a click.
const graphicRampStep = 0.75

// NewGraphic creates a graphic processor with flat settings.
func NewGraphic() *GraphicProcessor {
	p := &GraphicProcessor{rate: DefaultRate}
	p.recompute()
	return p
}

// Set turns the equalizer on or off and sets the gains of the bands.
func (p *GraphicProcessor) Set(enabled bool, gains [GraphicBands]float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	wasEnabled := p.enabled
	p.enabled = enabled
	p.gains = gains
	if !enabled || !wasEnabled {
		// switching the equalizer on starts from the saved curve, not from
		// a slow slide out of the flat one
		p.current = gains
		p.ramping = false
		p.recompute()
		return
	}
	p.ramping = p.current != gains
}

// SetRate sets the sample rate the coefficients are computed for.
func (p *GraphicProcessor) SetRate(rate int) {
	if rate <= 0 {
		rate = DefaultRate
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.rate == rate {
		return
	}
	p.rate = rate
	p.recompute()
}

// Enabled reports whether the equalizer changes the sound.
func (p *GraphicProcessor) Enabled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.enabled
}

// Gains returns the gains of the bands.
func (p *GraphicProcessor) Gains() [GraphicBands]float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.gains
}

// Reset forgets the history of the filters.
func (p *GraphicProcessor) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = [GraphicBands][2]biquadState{}
}

func (p *GraphicProcessor) recompute() {
	p.active = p.active[:0]

	rate := float64(p.rate)
	if rate <= 0 {
		rate = DefaultRate
	}

	for i, gain := range p.current {
		if math.Abs(gain) > 0.01 {
			p.active = append(p.active, i)
		}
		if math.Abs(gain) <= 0.01 {
			p.coeffs[i] = biquad{b0: 1}
			continue
		}
		// A filter above Nyquist is meaningless and can even be unstable:
		// clamp it just below the Nyquist frequency.
		freq := math.Min(GraphicFrequencies[i], rate*0.49)
		p.coeffs[i] = peakingQ(freq, gain, rate, GraphicQ)
	}
}

// ramp moves the applied gains one step towards the target and recomputes the
// coefficients when anything moved.
func (p *GraphicProcessor) ramp() {
	moved := false
	done := true
	for i := range p.current {
		delta := p.gains[i] - p.current[i]
		if math.Abs(delta) <= 0.01 {
			continue
		}
		done = false
		if delta > graphicRampStep {
			delta = graphicRampStep
		} else if delta < -graphicRampStep {
			delta = -graphicRampStep
		}
		p.current[i] += delta
		moved = true
	}
	if moved {
		p.recompute()
	}
	if done {
		p.current = p.gains
		p.ramping = false
		p.recompute()
	}
}

// Apply processes stereo 16 bit little endian PCM in place.
func (p *GraphicProcessor) Apply(pcm []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.enabled {
		return
	}
	if p.ramping {
		p.ramp()
	}
	if len(p.active) == 0 {
		return
	}

	for i := 0; i+3 < len(pcm); i += 4 {
		left := float32(int16(uint16(pcm[i])|uint16(pcm[i+1])<<8)) / 32768
		right := float32(int16(uint16(pcm[i+2])|uint16(pcm[i+3])<<8)) / 32768

		for _, band := range p.active {
			coeff := &p.coeffs[band]
			left = coeff.process(left, &p.state[band][0])
			right = coeff.process(right, &p.state[band][1])
		}

		writeSample(pcm[i:i+2], left)
		writeSample(pcm[i+2:i+4], right)
	}
}
