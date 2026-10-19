package mainpage

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestDrawToastsKeepsLayout(t *testing.T) {
	m := &Model{width: 80, height: 10}
	view := strings.Join([]string{
		"line one",
		"line two",
		"",
		"",
	}, "\n")

	// no toasts: the view is returned untouched
	if got := m.drawToasts(view); got != view {
		t.Fatal("view changed without toasts")
	}

	m.toasts = []toast{
		{text: "short", expires: time.Now().Add(time.Second)},
		{text: "a much longer toast message", expires: time.Now().Add(time.Second)},
	}

	got := m.drawToasts(view)
	if lines := strings.Split(got, "\n"); len(lines) != len(strings.Split(view, "\n")) {
		t.Fatalf("line count changed: %d", len(lines))
	}

	for _, line := range strings.Split(got, "\n") {
		if w := ansi.StringWidth(line); w > 80 {
			t.Fatalf("line is wider than the screen: %d", w)
		}
	}

	if !strings.Contains(ansi.Strip(got), "short") || !strings.Contains(ansi.Strip(got), "a much longer toast message") {
		t.Fatalf("toasts are not rendered:\n%s", ansi.Strip(got))
	}
}

func TestExpireToasts(t *testing.T) {
	m := &Model{toasts: []toast{
		{text: "old", expires: time.Now().Add(-time.Second)},
		{text: "fresh", expires: time.Now().Add(time.Minute)},
	}}

	m.expireToasts()

	if len(m.toasts) != 1 || m.toasts[0].text != "fresh" {
		t.Fatalf("unexpected toasts: %+v", m.toasts)
	}
}
