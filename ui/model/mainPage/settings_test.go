package mainpage

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/phantom-lancer/yatui/config"
)

func TestWinterSlidersAreOnlyInAppearanceWhenWinterThemeIsActive(t *testing.T) {
	previousStyle := config.Current.Style
	t.Cleanup(func() { config.Current.Style = previousStyle })

	m := &Model{}
	config.Current.Style = &config.Style{Theme: "classic"}
	for _, item := range m.buildSettings() {
		if item.label == "Скорость снега" || item.label == "Плотность снега" {
			t.Fatalf("winter slider %q is visible outside winter theme", item.label)
		}
	}

	config.Current.Style = &config.Style{Theme: "winter"}
	items := m.buildSettings()
	themeIndex, speedIndex, densityIndex := -1, -1, -1
	for i, item := range items {
		switch item.label {
		case "Тема":
			themeIndex = i
		case "Скорость снега":
			speedIndex = i
		case "Плотность снега":
			densityIndex = i
		}
	}
	if themeIndex < 0 || speedIndex != themeIndex+1 || densityIndex != speedIndex+1 {
		t.Fatalf("winter sliders are not grouped under appearance theme: theme=%d speed=%d density=%d", themeIndex, speedIndex, densityIndex)
	}
}

func TestSettingsSliderMapsMouseRange(t *testing.T) {
	got := -1
	m := &Model{settings: &settingsModel{
		items: []settingItem{{
			kind: settingSlider, sliderMin: 0, sliderMax: 100, sliderStep: 5,
			sliderSet: func(value int) tea.Cmd { got = value; return nil },
		}},
		layout: settingsLayout{rowX: 0, rowWidth: 20, labelWidth: 5},
	}}
	m.setSettingsSliderAt(0, 7)
	if got != 0 {
		t.Fatalf("left edge mapped to %d, want 0", got)
	}
	m.setSettingsSliderAt(0, 19)
	if got != 100 {
		t.Fatalf("right edge mapped to %d, want 100", got)
	}
	if width := ansi.StringWidth(renderSettingsSlider(50, 0, 100, 32)); width > 32 {
		t.Fatalf("slider exceeds its cell width: %d", width)
	}
}

func TestSettingsNavigation(t *testing.T) {
	s := &settingsModel{
		width: 40,
		items: []settingItem{
			{label: "one", kind: settingBool},
			{label: "two", kind: settingNumber},
		},
	}

	s.down()
	if s.index != 1 {
		t.Fatalf("down: index %d", s.index)
	}
	s.down()
	if s.index != 0 {
		t.Fatalf("down must wrap: index %d", s.index)
	}
	s.up()
	if s.index != 1 {
		t.Fatalf("up must wrap: index %d", s.index)
	}
}

func TestSettingsChangeBoolsAndChoices(t *testing.T) {
	var got []int

	s := &settingsModel{items: []settingItem{
		{label: "bool", kind: settingBool, apply: func(dir int) tea.Cmd { got = append(got, dir); return nil }},
		{label: "number", kind: settingNumber, apply: func(dir int) tea.Cmd { got = append(got, dir); return nil }},
		{label: "info", kind: settingInfo},
	}}

	// booleans and choices toggle on activation
	s.index = 0
	s.change(0)

	// numbers keep the direction
	s.index = 1
	s.change(-1)

	// info rows do nothing
	s.index = 2
	s.change(1)

	if len(got) != 2 || got[0] != 1 || got[1] != -1 {
		t.Fatalf("unexpected apply calls: %v", got)
	}
}

func TestSettingsViewKeepsSingleLineRows(t *testing.T) {
	s := &settingsModel{
		width: 44,
		items: []settingItem{
			{label: "Громкость", kind: settingBool, value: func() string { return "вкл" }},
			{label: "Нормализация", kind: settingNumber, value: func() string { return "123" }},
			{label: "Конфиг", kind: settingInfo, value: func() string { return `C:\Users\user\.config\yamusic-tui\config.yaml` }},
		},
	}

	view := s.view()
	plain := ansi.Strip(view)
	lines := strings.Split(plain, "\n")

	for _, line := range lines {
		if strings.ContainsRune(line, '\t') {
			t.Fatal("tabs must not appear in the dialog")
		}
		if w := ansi.StringWidth(line); w > s.width+8 {
			t.Fatalf("line is too wide (%d): %q", w, line)
		}
	}

	// every row is a single line: a wrapped value would duplicate its label
	for _, item := range s.items {
		count := 0
		for _, line := range lines {
			if strings.Contains(line, item.label) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("row %q is rendered on %d lines", item.label, count)
		}
	}
}
func TestSettingsNavigationSkipsSectionHeaders(t *testing.T) {
	s := &settingsModel{
		items: []settingItem{
			{label: "ВОСПРОИЗВЕДЕНИЕ", kind: settingSection},
			{label: "Громкость", kind: settingNumber},
			{label: "ВНЕШНИЙ ВИД", kind: settingSection},
			{label: "Тема", kind: settingChoice},
			{label: "Конфиг", kind: settingInfo},
		},
	}
	s.index = 0
	s.down()
	if s.index != 1 {
		t.Fatalf("down from section header = %d, want setting 1", s.index)
	}
	s.down()
	if s.index != 3 {
		t.Fatalf("down skipped to %d, want setting 3", s.index)
	}
	s.down()
	if s.index != 4 {
		t.Fatalf("actionable info row was skipped: index = %d, want 4", s.index)
	}
	s.up()
	if s.index != 3 {
		t.Fatalf("up from info row = %d, want setting 3", s.index)
	}
	s.down()
	if s.index != 4 {
		t.Fatalf("down to info row = %d, want 4", s.index)
	}
}
