package mainpage

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

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
		{label: "bool", kind: settingBool, apply: func(dir int) { got = append(got, dir) }},
		{label: "number", kind: settingNumber, apply: func(dir int) { got = append(got, dir) }},
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
