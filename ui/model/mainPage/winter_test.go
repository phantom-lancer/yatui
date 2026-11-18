package mainpage

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	previousDensity := config.Current.WinterDensity
	previousGarland := config.Current.WinterGarland
	t.Cleanup(func() {
		config.Current.Style = previousStyle
		config.Current.WinterIntensity = previousIntensity
		config.Current.WinterDensity = previousDensity
		config.Current.WinterGarland = previousGarland
	})

	config.Current.Style = &config.Style{Theme: "winter"}
	config.Current.WinterIntensity = 0
	config.Current.WinterDensity = 0
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
	low := len(winterSnowPositions(120, 40, 12, 10))
	high := len(winterSnowPositions(120, 40, 12, 80))
	if low == 0 || high <= low || high > 180 {
		t.Fatalf("unexpected particle counts: low=%d high=%d", low, high)
	}
}

func TestWinterSpeedChangesSnowPosition(t *testing.T) {
	slow := winterSnowPositionsAtSpeed(100, 40, 1, 45, 10)
	fast := winterSnowPositionsAtSpeed(100, 40, 1, 45, 100)
	if len(slow) == 0 || len(fast) == 0 {
		t.Fatal("both speeds must produce snow")
	}
	if equalSnowPositions(slow, fast) {
		t.Fatal("snow speed slider does not change particle positions")
	}
}

func equalSnowPositions(a, b map[int]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if _, ok := b[key]; !ok {
			return false
		}
	}
	return true
}

func TestWinterClickBurstAndDragTrailAreBounded(t *testing.T) {
	previousStyle := config.Current.Style
	previousDensity := config.Current.WinterDensity
	previousGarland := config.Current.WinterGarland
	t.Cleanup(func() {
		config.Current.Style = previousStyle
		config.Current.WinterDensity = previousDensity
		config.Current.WinterGarland = previousGarland
	})
	config.Current.Style = &config.Style{Theme: "winter"}
	config.Current.WinterDensity = 0
	config.Current.WinterGarland = false

	m := &Model{width: 100, height: 40}
	m.handleWinterMouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 50, Y: 20})
	if len(m.winterParticles) != 12 || !m.winterRunning {
		t.Fatalf("click burst: particles=%d running=%v", len(m.winterParticles), m.winterRunning)
	}
	m.handleWinterMouse(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 52, Y: 20})
	if len(m.winterParticles) != 13 {
		t.Fatalf("drag trail added no particle: %d", len(m.winterParticles))
	}
	m.handleWinterMouse(tea.MouseMsg{Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	if m.winterPointerDown {
		t.Fatal("mouse release did not stop the trail")
	}
	for i := 0; i < 24; i++ {
		m.advanceWinterParticles()
	}
	if len(m.winterParticles) != 0 {
		t.Fatalf("expired interaction particles remain: %d", len(m.winterParticles))
	}
}

func TestWinterInteractionOverlayPreservesGeometry(t *testing.T) {
	previousStyle := config.Current.Style
	previousDensity := config.Current.WinterDensity
	previousGarland := config.Current.WinterGarland
	t.Cleanup(func() {
		config.Current.Style = previousStyle
		config.Current.WinterDensity = previousDensity
		config.Current.WinterGarland = previousGarland
	})
	config.Current.Style = &config.Style{Theme: "winter"}
	config.Current.WinterDensity = 0
	config.Current.WinterGarland = false

	view := "          \nTEXT"
	particles := []winterParticle{{x: 3, y: 0, life: 2}}
	got := drawWinterOverlayWithParticles(view, 1, particles)
	if ansi.StringWidth(strings.Split(got, "\n")[0]) != 10 || !strings.Contains(got, "✧") || !strings.Contains(got, "TEXT") {
		t.Fatalf("particle overlay changed geometry or missed spark: %q", got)
	}
}

func TestWinterGarlandFindsFreeRowAfterResize(t *testing.T) {
	previousStyle := config.Current.Style
	previousIntensity := config.Current.WinterIntensity
	previousDensity := config.Current.WinterDensity
	previousGarland := config.Current.WinterGarland
	t.Cleanup(func() {
		config.Current.Style = previousStyle
		config.Current.WinterIntensity = previousIntensity
		config.Current.WinterDensity = previousDensity
		config.Current.WinterGarland = previousGarland
	})

	config.Current.Style = &config.Style{Theme: "winter"}
	config.Current.WinterIntensity = 0
	config.Current.WinterDensity = 0
	config.Current.WinterGarland = true

	view := "TITLE\n                \nBODY"
	got := drawWinterOverlay(view, 4)
	if count := strings.Count(got, "●"); count < 2 {
		t.Fatalf("garland has %d bulbs after resize, want at least 2: %q", count, got)
	}
	if !strings.Contains(got, "TITLE") || !strings.Contains(got, "BODY") {
		t.Fatalf("garland damaged visible text: %q", got)
	}
}
