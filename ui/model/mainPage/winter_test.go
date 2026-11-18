package mainpage

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/phantom-lancer/yatui/config"
)

func TestDecorateWinterLineKeepsLayoutAndText(t *testing.T) {
	line := "A B  "
	snow := map[int]struct{}{3: {}}
	got := decorateWinterLine(line, 0, len(line), snow, false, 0)

	if width := ansi.StringWidth(got); width != len(line) {
		t.Fatalf("decorated width = %d, want %d", width, len(line))
	}
	if !strings.Contains(got, "A") || !strings.Contains(got, "B") || !strings.Contains(got, "·") {
		t.Fatalf("unexpected decorated line: %q", got)
	}
}

func TestWinterGarlandUsesExistingBlankCells(t *testing.T) {
	line := " A  "
	got := decorateWinterLine(line, 0, len(line), nil, true, 0)

	if width := ansi.StringWidth(got); width != len(line) {
		t.Fatalf("garland width = %d, want %d", width, len(line))
	}
	if !strings.Contains(got, "●") || !strings.Contains(got, "A") {
		t.Fatalf("garland did not preserve text and decorate a blank: %q", got)
	}
}

func TestWinterAnimationStopsWhenEffectsAreOff(t *testing.T) {
	previousStyle := config.Current.Style
	previousIntensity := config.Current.WinterIntensity
	previousGarland := config.Current.WinterGarland
	t.Cleanup(func() {
		config.Current.Style = previousStyle
		config.Current.WinterIntensity = previousIntensity
		config.Current.WinterGarland = previousGarland
	})

	config.Current.Style = &config.Style{Theme: "winter"}
	config.Current.WinterIntensity = 0
	config.Current.WinterGarland = false

	m := &Model{}
	if cmd := m.winterStart(); cmd != nil || m.winterRunning {
		t.Fatal("winter animation must stay stopped when both effects are disabled")
	}

	config.Current.WinterGarland = true
	if cmd := m.winterStart(); cmd == nil || !m.winterRunning {
		t.Fatal("garland must start the animation timer")
	}
	m.winterStop()
	if m.winterRunning {
		t.Fatal("winterStop must invalidate the active timer")
	}
}

func TestWinterSnowIntensityControlsParticleCount(t *testing.T) {
	if got := len(winterSnowPositions(120, 40, 12, 0)); got != 0 {
		t.Fatalf("disabled snow produced %d particles", got)
	}
	low := len(winterSnowPositions(120, 40, 12, 1))
	high := len(winterSnowPositions(120, 40, 12, 3))
	if low == 0 || high <= low || high > 72 {
		t.Fatalf("unexpected particle counts: low=%d high=%d", low, high)
	}
}
