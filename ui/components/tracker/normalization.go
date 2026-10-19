package tracker

import (
	"math"

	"github.com/dece2183/yamusic-tui/api"
	"github.com/dece2183/yamusic-tui/config"
)

// normalization describes how the PCM stream of a single track should be
// scaled. Everything is precomputed once per track, so the audio callback
// only has to do a multiply per sample.
type normalization struct {
	// linear is the R128 gain converted from dB to a linear multiplier.
	linear float32
	// limit is an extra multiplier that keeps the loudest sample of the
	// track below 1.0 (peak limiting). It is 1 when clipping is impossible.
	limit float32
}

var noNormalization = normalization{linear: 1, limit: 1}

// normalizationEnabled reports whether the config asks for the normalized
// playback. The key is optional: without it the normalization is on.
func normalizationEnabled() bool {
	cfg := config.Current.Normalization
	return cfg != nil && cfg.Enabled
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
	linear := float32(math.Pow(10, gain/20))
	n.linear = linear

	// peak limiting: the R128 gain is computed for the average loudness of
	// the track, so a loud peak can still go over the top
	if peak := float64(track.Normalization.Peak); peak > 0 {
		if loudest := peak * float64(linear); loudest > 1 {
			n.limit = float32(1 / loudest)
		}
	}

	return n, false
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
func (n normalization) apply(pcm []byte) {
	if !n.scale() {
		return
	}

	gain := n.linear * n.limit
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
}
