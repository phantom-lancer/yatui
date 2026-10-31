package mainpage

import (
	"testing"
	"time"

	"github.com/phantom-lancer/yatui/config"
)

// TestRgbFrameSetting checks the smoothness of the animated theme: the frame
// comes from the config, is clamped and defaults to five frames a second.
func TestRgbFrameSetting(t *testing.T) {
	old := config.Current.RgbFrameMs
	defer func() { config.Current.RgbFrameMs = old }()

	cases := []struct {
		config int
		want   int
	}{
		{0, rgbDefaultFrameMs},
		{1, rgbMinFrameMs},
		{rgbMaxFrameMs + 100, rgbMaxFrameMs},
		{100, 100},
	}
	for _, c := range cases {
		config.Current.RgbFrameMs = c.config
		if got := rgbFrameMs(); got != c.want {
			t.Errorf("rgbFrameMs with %d in the config = %d, want %d", c.config, got, c.want)
		}
	}

	config.Current.RgbFrameMs = 200
	if got := rgbFrameInterval(); got != 200*time.Millisecond {
		t.Errorf("rgbFrameInterval = %s, want 200ms", got)
	}
}

// TestRgbFrameSteps checks that the smoothness row walks the list and wraps.
func TestRgbFrameSteps(t *testing.T) {
	if got := stepRgbFrame(rgbFrameSteps[0], 1); got != rgbFrameSteps[1] {
		t.Errorf("next frame = %d, want %d", got, rgbFrameSteps[1])
	}
	if got := stepRgbFrame(rgbFrameSteps[len(rgbFrameSteps)-1], 1); got != rgbFrameSteps[0] {
		t.Errorf("the list did not wrap: %d", got)
	}
	if got := stepRgbFrame(rgbFrameSteps[0], -1); got != rgbFrameSteps[len(rgbFrameSteps)-1] {
		t.Errorf("the list did not wrap backwards: %d", got)
	}
}

// TestRgbFrameLabel checks that every frame of the list has a readable label.
func TestRgbFrameLabel(t *testing.T) {
	for _, ms := range rgbFrameSteps {
		if label := rgbFrameLabel(ms); label == "" {
			t.Errorf("frame %d has no label", ms)
		}
	}
	if got := rgbFrameLabel(200); got != "обычно · 5 к/с" {
		t.Errorf("label of the default frame = %q", got)
	}
	if got := rgbFrameLabel(400); got != "экономно · 2.5 к/с" {
		t.Errorf("label of the slowest frame = %q", got)
	}
}
