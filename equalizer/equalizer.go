// Package equalizer implements a ten band graphic equalizer.
//
// The equalizer is a cascade of peaking filters (the RBJ cookbook), so the
// bands do not overlap much and the sound stays natural. The filters are
// applied to the decoded PCM of a track right before it reaches the sound
// card.
//
// The boosted bands can push the samples over the top, so the output of the
// filter is limited softly: the peaks are rounded instead of being cut.
//
// Everything in the audio path is allocation free: the filters keep their
// history in the processor itself, and the coefficients are computed only
// when the settings change.
package equalizer

import (
	"math"
	"sync"
)

const (
	// Bands is the number of the bands of the equalizer.
	Bands = 10

	// MaxGain is the biggest boost or cut of a band, in decibels.
	MaxGain = 12

	// DefaultRate is the sample rate the coefficients are computed for when
	// the real one is unknown.
	DefaultRate = 44100

	// _BAND_Q is the quality factor of the bands: it makes every band
	// about an octave wide.
	_BAND_Q = 1.41
)

// BandFrequencies are the center frequencies of the bands, in hertz.
var BandFrequencies = [Bands]float64{60, 170, 310, 600, 1000, 3000, 6000, 12000, 14000, 16000}

// BandLabels are the names of the bands, as the user sees them.
var BandLabels = [Bands]string{"60", "170", "310", "600", "1к", "3к", "6к", "12к", "14к", "16к"}

// CustomPreset is the name of the preset that is shown when the user has
// changed the bands by hand.
const CustomPreset = "Свой"

// Preset is a named set of the gains of the bands.
type Preset struct {
	Name  string
	Gains [Bands]float64
}

// _PRESETS are the ready made settings of the equalizer.
var _PRESETS = []Preset{
	{Name: "Плоский"},
	{Name: "Поп", Gains: [Bands]float64{-2, -1, 0, 2, 3, 1, -1, -1, 0, 1}},
	{Name: "Рок", Gains: [Bands]float64{4, 3, 1, -1, -2, 1, 3, 4, 4, 3}},
	{Name: "Джаз", Gains: [Bands]float64{3, 2, 0, 1, -1, -1, 0, 2, 3, 3}},
	{Name: "Классика", Gains: [Bands]float64{4, 3, 1, -1, -1, 0, 2, 3, 3, 3}},
	{Name: "Бас", Gains: [Bands]float64{6, 5, 3, 1, 0, 0, 0, 0, 0, 0}},
	{Name: "Верх", Gains: [Bands]float64{0, 0, 0, 0, 0, 1, 3, 4, 5, 6}},
	{Name: "Голос", Gains: [Bands]float64{-2, -2, -1, 2, 4, 4, 3, 1, 0, -1}},
	{Name: "Электроника", Gains: [Bands]float64{5, 4, 1, 0, -2, 1, 0, 2, 4, 5}},
}

// Presets returns the list of the ready made presets.
func Presets() []Preset {
	presets := make([]Preset, len(_PRESETS))
	copy(presets, _PRESETS)
	return presets
}

// PresetByName returns the preset with the given name. The second value
// reports whether such a preset exists.
func PresetByName(name string) (Preset, bool) {
	for _, preset := range _PRESETS {
		if preset.Name == name {
			return preset, true
		}
	}
	return Preset{}, false
}

// NormalizeGains turns a list of the gains into the gains of the bands. The
// missing ones are left alone.
func NormalizeGains(gains []float64) [Bands]float64 {
	var result [Bands]float64
	for i := 0; i < Bands && i < len(gains); i++ {
		result[i] = gains[i]
	}
	return result
}

// ClampGain limits a gain to the range the equalizer works in.
func ClampGain(gain float64) float64 {
	if gain > MaxGain {
		return MaxGain
	}
	if gain < -MaxGain {
		return -MaxGain
	}
	return gain
}

// biquad is a peaking filter in the direct form I.
type biquad struct {
	b0, b1, b2, a1, a2 float32
}

// biquadState is the history of a filter.
type biquadState struct {
	x1, x2, y1, y2 float32
}

// process runs a sample through the filter.
func (f *biquad) process(x float32, s *biquadState) float32 {
	y := f.b0*x + f.b1*s.x1 + f.b2*s.x2 - f.a1*s.y1 - f.a2*s.y2
	s.x2, s.x1 = s.x1, x
	s.y2, s.y1 = s.y1, y
	return y
}

// peaking computes the coefficients of a peaking filter.
func peaking(freq, gainDB, rate float64) biquad {
	a := math.Pow(10, gainDB/40)
	w0 := 2 * math.Pi * freq / rate
	alpha := math.Sin(w0) / (2 * _BAND_Q)
	cosw := math.Cos(w0)

	a0 := 1 + alpha/a
	return biquad{
		b0: float32((1 + alpha*a) / a0),
		b1: float32((-2 * cosw) / a0),
		b2: float32((1 - alpha*a) / a0),
		a1: float32((-2 * cosw) / a0),
		a2: float32((1 - alpha/a) / a0),
	}
}

// Processor applies the equalizer to the PCM of a track.
//
// The settings are changed by the user while the music plays, so they are
// guarded by a mutex: the audio thread takes it for the length of a buffer
// only.
//
// Output is hard-clamped to int16 range, so a strong boost never clips.
type Processor struct {
	mu      sync.Mutex
	enabled bool
	gains   [Bands]float64
	coeffs  [Bands]biquad
	// active are the indexes of the bands that change the signal.
	active []int
	state  [Bands][2]biquadState
	rate   int
}

// New creates a processor with the flat settings.
func New() *Processor {
	p := &Processor{rate: DefaultRate}
	p.recompute()
	return p
}

// Set turns the equalizer on or off and sets the gains of the bands.
func (p *Processor) Set(enabled bool, gains [Bands]float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.enabled = enabled
	p.gains = gains
	p.recompute()
}

// SetRate sets the sample rate the coefficients are computed for. The
// filters keep their history, so the change is smooth.
func (p *Processor) SetRate(rate int) {
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
func (p *Processor) Enabled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.enabled
}

// Gains returns the gains of the bands.
func (p *Processor) Gains() [Bands]float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.gains
}

// Reset forgets the history of the filters: it is called when a new track
// starts, so the first samples of it are not filtered with the tail of the
// previous one.
func (p *Processor) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = [Bands][2]biquadState{}
}

// recompute builds the coefficients of the filters. The caller holds the
// lock.
func (p *Processor) recompute() {
	p.active = p.active[:0]

	rate := float64(p.rate)
	if rate <= 0 {
		rate = DefaultRate
	}

	for i, gain := range p.gains {
		if math.Abs(gain) > 0.01 {
			p.active = append(p.active, i)
		}
		if math.Abs(gain) <= 0.01 {
			p.coeffs[i] = biquad{b0: 1}
			continue
		}
		p.coeffs[i] = peaking(BandFrequencies[i], gain, rate)
	}
}

// Apply processes the PCM of a track in place. The samples are signed 16 bit
// little endian stereo, the way the decoder gives them.
func (p *Processor) Apply(pcm []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.enabled || len(p.active) == 0 {
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

// writeSample puts a sample back into the buffer, keeping it in range.
// Samples that go over the top are clamped: no soft curve, no crackle.
func writeSample(dst []byte, value float32) {
	if value > 1 {
		value = 1
	} else if value < -1 {
		value = -1
	}

	sample := int16(value * 32767)
	dst[0] = byte(sample)
	dst[1] = byte(sample >> 8)
}
