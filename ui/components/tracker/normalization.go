package tracker

import (
	"math"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
)

// _NORMALIZATION_CEILING is the loudest sample the peak guard allows: a
// correction may miss the loudest part of a track, and a gain that would send
// the samples over the top has to be lowered instead of clipping them.
const _NORMALIZATION_CEILING = 0.999

// normalization describes how the PCM stream of a single track should be
// scaled. Everything is precomputed once per track, so the audio callback
// only has to do a multiply per sample.
type normalization struct {
	// linear is the R128 gain converted from dB to a linear multiplier.
	linear float32
	// limit is an extra multiplier that keeps the samples below 1.0 (peak
	// limiting). It is 1 when clipping is impossible.
	limit float32
}

var noNormalization = normalization{linear: 1, limit: 1}

// normalizationEnabled reports whether the config asks for the normalized
// playback. The key is optional: without it the normalization is on.
func normalizationEnabled() bool {
	cfg := config.Current.Normalization
	return cfg != nil && cfg.Enabled
}

// newGainNormalization turns a correction in decibels and the peak the track
// was measured with into the multipliers of the audio callback.
func newGainNormalization(gainDB, peak float64) normalization {
	n := noNormalization
	if gainDB == 0 {
		return n
	}

	linear := float32(math.Pow(10, gainDB/20))
	n.linear = linear

	// peak limiting: the gain is computed for the average loudness of the
	// track, so a loud peak can still go over the top
	if peak > 0 {
		if loudest := peak * float64(linear); loudest > 1 {
			n.limit = float32(1 / loudest)
		}
	}

	return n
}

// newNormalization reads the track gain out of the API response and turns it
// into multipliers. The second return value reports that the track has no gain
// info (which is the case for every track nowadays) and that the loudness of
// the track has to be measured locally.
func newNormalization(track *api.Track) (normalization, bool) {
	n := noNormalization
	if !normalizationEnabled() {
		return n, false
	}
	if track == nil {
		return n, false
	}

	gain := float64(track.Normalization.Gain)
	if gain == 0 {
		return n, true
	}

	// sanity clamp: a wrong value must never blow the speakers up
	if gain > 20 {
		gain = 20
	} else if gain < -60 {
		gain = -60
	}

	return newGainNormalization(gain, float64(track.Normalization.Peak)), false
}

// gainDB is the correction, in decibels, that the normalization applies to a
// track.
func (n normalization) gainDB() float64 {
	if n.linear <= 0 {
		return 0
	}
	return 20 * math.Log10(float64(n.linear*n.limit))
}

// scale returns true when the buffer has to be rewritten at all.
func (n normalization) scale() bool {
	return n.linear != 1 || n.limit != 1
}

// apply scales the signed 16 bit little endian stereo samples in place.
// The buffer is never reallocated: this runs in the audio callback.
// The returned value reports that the peak guard had to lower the gain.
func (n *normalization) apply(pcm []byte) bool {
	if !n.scale() {
		return false
	}

	gain := n.linear * n.limit
	guarded := false

	// the measurement covers the head of the track only, and the loudest part
	// of it may still be ahead: a buffer that would be driven over the top
	// lowers the gain of the whole track. A drop of the volume is much easier
	// to survive than the distortion of a clipped peak.
	if peak := pcmPeak(pcm); peak > 0 {
		if ceiling := float32(_NORMALIZATION_CEILING / peak); gain > ceiling {
			gain = ceiling
			n.limit = ceiling / n.linear
			guarded = true
		}
	}

	for i := 0; i+1 < len(pcm); i += 2 {
		sample := int32(int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8))
		scaled := int32(float32(sample) * gain)
		if scaled > math.MaxInt16 {
			scaled = math.MaxInt16
		} else if scaled < math.MinInt16 {
			scaled = math.MinInt16
		}
		pcm[i] = byte(scaled)
		pcm[i+1] = byte(scaled >> 8)
	}

	return guarded
}

// pcmPeak returns the loudest sample of a buffer, as a fraction of the full
// scale.
func pcmPeak(pcm []byte) float64 {
	var peak float64
	for i := 0; i+1 < len(pcm); i += 2 {
		sample := int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8)
		if sample == math.MinInt16 {
			return 1
		}
		if abs := math.Abs(float64(sample)) / 32768; abs > peak {
			peak = abs
		}
	}
	return peak
}
