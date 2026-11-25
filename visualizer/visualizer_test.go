package visualizer

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestSimplexIsFiniteAndBounded(t *testing.T) {
	for i := 0; i < 500; i++ {
		x := float64(i) * 0.37
		value := simplex3(vec3{x, x * 0.7, x * 0.3})
		if math.IsNaN(value) || math.IsInf(value, 0) {
			t.Fatalf("simplex3(%v) = %v", x, value)
		}
		if math.Abs(value) > 1.5 {
			t.Fatalf("simplex3(%v) = %v, outside the expected range", x, value)
		}
	}
}

func TestFrameHasExactSizeAndHalfBlocks(t *testing.T) {
	state := animState{speed: 0.8}
	for i := range state.palette {
		state.palette[i] = [3]float64{0.9, 0.4, 0.2}
	}
	frame := renderFrame(12, 5, state)
	lines := strings.Split(frame, "\n")
	if len(lines) != 5 {
		t.Fatalf("the frame has %d lines, want 5", len(lines))
	}
	for i, line := range lines {
		if got := strings.Count(line, "▀"); got != 12 {
			t.Fatalf("line %d has %d half blocks, want 12", i, got)
		}
		if !strings.Contains(line, "\x1b[38;2;") || !strings.Contains(line, ";48;2;") {
			t.Fatalf("line %d does not set both half cell colours", i)
		}
	}
}

func TestRendererLifecycleReleasesTheFrame(t *testing.T) {
	r := New()
	r.Resize(8, 3)
	r.SetAmplitude(0.8)
	r.SetEnabled(true)
	defer r.Stop()

	deadline := time.Now().Add(5 * time.Second)
	for {
		frame, ok := r.Latest()
		if ok && frame.Text != "" {
			if lines := strings.Count(frame.Text, "\n") + 1; lines != 3 {
				t.Fatalf("the frame has %d lines, want 3", lines)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the renderer produced no frame")
		}
		time.Sleep(5 * time.Millisecond)
	}

	r.SetEnabled(false)
	if _, ok := r.Latest(); ok {
		t.Fatal("a disabled renderer kept its frame")
	}
}

func TestPaletteFromAccentKeepsSixColours(t *testing.T) {
	palette := PaletteFromAccent(RGB{220, 60, 160}, RGB{13, 13, 13}, 10)
	for i, color := range palette.Colors {
		if color == (RGB{}) {
			t.Fatalf("band %d is black", i)
		}
	}
	if palette.Bg != (RGB{13, 13, 13}) {
		t.Fatalf("backdrop = %v", palette.Bg)
	}
}

func TestParseHex(t *testing.T) {
	if color, ok := ParseHex("#f7d44b"); !ok || color != (RGB{247, 212, 75}) {
		t.Fatalf("parsed %v, %v", color, ok)
	}
	if _, ok := ParseHex("nope"); ok {
		t.Fatal("a broken colour was accepted")
	}
}

func BenchmarkRenderFrame(b *testing.B) {
	state := animState{speed: 0.8, amplitude: 0.6}
	for i := range state.palette {
		state.palette[i] = [3]float64{0.9, 0.4, 0.2}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderFrame(100, 8, state)
	}
}

func TestRingsAreVisibleAboveTheBackdrop(t *testing.T) {
	state := animState{speed: 0.8, amplitude: 0.7}
	for i := range state.palette {
		state.palette[i] = [3]float64{0.95, 0.55, 0.15}
	}
	state.bg = [3]float64{0.05, 0.05, 0.05}

	// sample a grid and compare it with the backdrop: the rings must add
	// light somewhere, otherwise the panel is just a black rectangle
	brighter := 0
	total := 0
	for y := 0; y < 12; y++ {
		for x := 0; x < 24; x++ {
			uv := getUV(x, y, 1.5, 24, 12, 0.8)
			color := getPixelColor(uv, state)
			total++
			if color.X+color.Y+color.Z > 0.4 {
				brighter++
			}
		}
	}
	if brighter == 0 {
		t.Fatalf("the rings are invisible: %d of %d cells are lit", brighter, total)
	}
	if brighter == total {
		t.Fatal("every cell is lit: the rings have no shape")
	}
}
