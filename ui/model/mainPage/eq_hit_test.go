package mainpage

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/equalizer"
	"github.com/phantom-lancer/yatui/ui/components/tracker"
)

func TestEqualizerHitboxes(t *testing.T) {
	m := &Model{width: 120, height: 40}
	m.tracker = &tracker.Model{}
	m.eq = &eqModel{enabled: true, preset: "Плоский", index: -1}
	view := m.placeEqualizer()
	l := m.eq.layout

	// geometry must match the rendered text
	lines := strings.Split(view, "\n")
	if l.presetRect.Y < 0 || l.presetRect.Y >= len(lines) || !strings.Contains(lines[l.presetRect.Y], "Пресет") {
		t.Errorf("preset rect y=%d does not point at the preset row", l.presetRect.Y)
	}
	if l.applyRect.Y < 0 || l.applyRect.Y >= len(lines) || !strings.Contains(lines[l.applyRect.Y], "Применить") {
		t.Errorf("apply rect y=%d does not point at the apply row", l.applyRect.Y)
	}

	// toggle
	before := m.eq.enabled
	m.eqMouse(tea.MouseMsg{X: l.toggleRect.X, Y: l.toggleRect.Y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if m.eq.enabled == before {
		t.Errorf("toggle not hit at %d,%d", l.toggleRect.X, l.toggleRect.Y)
	}

	// preset
	prev := m.eq.preset
	m.eqMouse(tea.MouseMsg{X: l.presetRect.X + l.presetRect.Width/2, Y: l.presetRect.Y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if m.eq.preset == prev {
		t.Errorf("preset not hit at y=%d (prev=%q)", l.presetRect.Y, prev)
	}

	// apply
	m.eqMouse(tea.MouseMsg{X: l.applyRect.X + l.applyRect.Width/2, Y: l.applyRect.Y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if m.eq.index != 1+equalizer.Bands {
		t.Errorf("apply not hit at y=%d, index=%d", l.applyRect.Y, m.eq.index)
	}

	// every slider
	for i, s := range l.sliders {
		if idx := l.sliderAt(s.x, s.y+s.h/2); idx != i {
			t.Errorf("slider %d not hit at %d,%d -> %d", i, s.x, s.y+s.h/2, idx)
		}
	}
}

func TestWaveRowsHitboxes(t *testing.T) {
	m := &Model{width: 100, height: 30}
	m.wave = &waveSettingsModel{}
	m.wave.setStations(testStationList())
	m.isWaveActive = true
	m.placeWaveSettings()
	l := m.wave.layout
	n := len(m.wave.groups)

	// apply row (visual: groups + blank + reset + apply)
	applyY := l.rowY + n + 2
	m.wave.index = 0
	m.waveMouse(tea.MouseMsg{X: l.x + l.width/2, Y: applyY, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if m.isWaveActive {
		t.Errorf("apply row not hit at y=%d rowY=%d n=%d", applyY, l.rowY, n)
	}
}
