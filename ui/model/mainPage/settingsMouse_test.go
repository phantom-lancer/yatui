package mainpage

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func mouseAt(action tea.MouseAction, button tea.MouseButton, x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: action, Button: button}
}

// settingsTestModel builds a dialog with rows that have recognizable labels.
func settingsTestModel() *Model {
	m := &Model{width: 100, height: 30}
	m.settings = &settingsModel{
		width: 44,
		items: []settingItem{
			{label: "Громкость", kind: settingNumber, value: func() string { return "50%" }},
			{label: "Нормализация", kind: settingBool, value: func() string { return "вкл" }},
			{label: "Тема", kind: settingChoice, value: func() string { return "nord" }},
			{label: "Конфиг", kind: settingInfo, value: func() string { return `C:\config.yaml` }},
		},
	}
	m.isSettingsActive = true
	return m
}

func TestSettingsRowsMatchTheRenderedView(t *testing.T) {
	m := settingsTestModel()
	view := m.placeSettings()

	layout := m.settings.layout
	if !layout.valid {
		t.Fatal("the dialog was not measured")
	}

	for i, item := range m.settings.items {
		y := findLine(t, view, item.label)
		row, ok := layout.rowAt(layout.x+2, y)
		if !ok || row != i {
			t.Fatalf("row of %q: got %d, %v, want %d", item.label, row, ok, i)
		}
	}

	// the whole dialog is a hit area for its own rows
	for i := range m.settings.items {
		y := layout.rowY + i
		if row, ok := layout.rowAt(layout.x+layout.width-1, y); !ok || row != i {
			t.Fatalf("row %d is not reachable from the right edge: %d, %v", i, row, ok)
		}
	}

	// and nothing outside of it is
	if _, ok := layout.rowAt(layout.x-1, layout.rowY); ok {
		t.Error("a point left of the dialog must not hit a row")
	}
	if _, ok := layout.rowAt(layout.x+2, layout.rowY-1); ok {
		t.Error("a point above the first row must not hit a row")
	}
	if _, ok := layout.rowAt(layout.x+2, layout.rowY+len(m.settings.items)); ok {
		t.Error("a point below the last row must not hit a row")
	}
}

func TestSettingsValueIsClickable(t *testing.T) {
	m := settingsTestModel()
	m.placeSettings()

	layout := m.settings.layout
	valueX := layout.rowX + layout.labelWidth + 2
	valueWidth := layout.rowWidth - layout.labelWidth - 2
	if valueWidth < 4 {
		t.Fatalf("the value area is too narrow: %d", valueWidth)
	}

	if dir := layout.valueDir(valueX, 0); dir != -1 {
		t.Errorf("the left half of the value must go down, got %d", dir)
	}
	if dir := layout.valueDir(valueX+valueWidth-1, 0); dir != 1 {
		t.Errorf("the right half of the value must go up, got %d", dir)
	}
	if dir := layout.valueDir(valueX-1, 0); dir != 0 {
		t.Errorf("the label must not change the value, got %d", dir)
	}
	if dir := layout.valueDir(valueX+valueWidth, 0); dir != 0 {
		t.Errorf("the padding must not change the value, got %d", dir)
	}
}

// the arrows of the selected row have to sit inside the value area
func TestSettingsSelectionArrowsAreInTheValueArea(t *testing.T) {
	m := settingsTestModel()
	view := m.placeSettings()

	layout := m.settings.layout
	y := layout.rowY
	line := []rune(lineOf(t, view, y))

	if got := string(line[layout.rowX+layout.labelWidth+2]); got != "‹" {
		t.Fatalf("the left arrow is not at the start of the value area: %q", got)
	}
	if got := string(line[layout.rowX+layout.rowWidth-1]); got != "›" {
		t.Fatalf("the right arrow is not at the end of the value area: %q", got)
	}
}

func TestSettingsMouseSelectsAndChanges(t *testing.T) {
	m := settingsTestModel()

	view := m.placeSettings()
	_ = view

	layout := m.settings.layout
	valueX := layout.rowX + layout.labelWidth + 2
	valueWidth := layout.rowWidth - layout.labelWidth - 2

	// hovering a row selects it
	m.settings.index = 0
	m.settingsMouse(mouseAt(tea.MouseActionMotion, tea.MouseButtonNone, layout.x+2, layout.rowY+2))
	if m.settings.index != 2 {
		t.Fatalf("the hovered row was not selected: %d", m.settings.index)
	}

	// a click on the right half of the value goes up
	m.settingsMouse(mouseAt(tea.MouseActionPress, tea.MouseButtonLeft, valueX+valueWidth-1, layout.rowY+1))
	if m.settings.index != 1 {
		t.Fatalf("the clicked row was not selected: %d", m.settings.index)
	}

	// a click outside of the dialog closes it
	m.settingsMouse(mouseAt(tea.MouseActionPress, tea.MouseButtonLeft, 0, 0))
	if m.isSettingsActive || m.settings != nil {
		t.Fatal("a click outside of the dialog must close it")
	}
}
