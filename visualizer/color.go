package visualizer

import (
	"math"
	"strconv"
	"strings"
)

// ParseHex reads a #rrggbb colour. The leading # is optional.
func ParseHex(value string) (RGB, bool) {
	s := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(s) < 6 {
		return RGB{}, false
	}
	r, errR := strconv.ParseUint(s[0:2], 16, 8)
	g, errG := strconv.ParseUint(s[2:4], 16, 8)
	b, errB := strconv.ParseUint(s[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return RGB{}, false
	}
	return RGB{R: uint8(r), G: uint8(g), B: uint8(b)}, true
}

// RotateHue turns a colour by the given number of degrees. It uses the same
// HSL conversion as the upstream widget, so the palette of a theme accent is
// recognizably the same in both players.
func RotateHue(color RGB, degrees float64) RGB {
	r := float64(color.R) / 255
	g := float64(color.G) / 255
	b := float64(color.B) / 255

	maxV := math.Max(r, math.Max(g, b))
	minV := math.Min(r, math.Min(g, b))
	l := (maxV + minV) / 2

	var h, s float64
	if maxV == minV {
		h, s = 0, 0
	} else {
		d := maxV - minV
		if l > 0.5 {
			s = d / (2 - maxV - minV)
		} else {
			s = d / (maxV + minV)
		}
		switch maxV {
		case r:
			h = (g - b) / d
			if g < b {
				h += 6
			}
		case g:
			h = (b-r)/d + 2
		default:
			h = (r-g)/d + 4
		}
		h /= 6
	}

	h = math.Mod(h+degrees/360, 1)
	if h < 0 {
		h += 1
	}

	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q

	hue := func(t float64) float64 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 1.0/2:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		default:
			return p
		}
	}

	return RGB{
		R: uint8(clamp(hue(h+1.0/3)*255, 0, 255)),
		G: uint8(clamp(hue(h)*255, 0, 255)),
		B: uint8(clamp(hue(h-1.0/3)*255, 0, 255)),
	}
}

// PaletteFromAccent builds the six ring colours from one accent, following
// the fallback palette of the upstream visualizer: the accent, its neighbours
// and complementary hues. shift rotates the whole palette, which lets a track
// tint the theme accent without losing the theme's character.
func PaletteFromAccent(accent RGB, background RGB, shift float64) Palette {
	return Palette{
		Bg: background,
		Colors: [6]RGB{
			RotateHue(accent, shift),
			RotateHue(accent, shift+40),
			RotateHue(accent, shift-40),
			RotateHue(accent, shift+180),
			RotateHue(accent, shift+20),
			RotateHue(accent, shift-20),
		},
	}
}

// Scale multiplies the channels of a colour. The visualizer uses it to keep
// the backdrop darker than the interface: the rings need a quiet background
// even when the theme background is a light grey.
func Scale(color RGB, factor float64) RGB {
	scale := func(value uint8) uint8 {
		return uint8(clamp(float64(value)*factor, 0, 255))
	}
	return RGB{R: scale(color.R), G: scale(color.G), B: scale(color.B)}
}
