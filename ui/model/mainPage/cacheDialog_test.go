package mainpage

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/phantom-lancer/yatui/api"
)

func TestHumanSize(t *testing.T) {
	cases := []struct {
		size int64
		want string
	}{
		{0, "0 Б"},
		{512, "512 Б"},
		{1024, "1.0 КБ"},
		{1536, "1.5 КБ"},
		{1024 * 1024, "1.0 МБ"},
		{3 * 1024 * 1024 * 1024, "3.0 ГБ"},
	}

	for _, c := range cases {
		if got := humanSize(c.size); got != c.want {
			t.Fatalf("humanSize(%d) = %q, want %q", c.size, got, c.want)
		}
	}
}

func TestCacheNavigation(t *testing.T) {
	c := &cacheModel{tracks: make([]api.Track, 5)}

	c.move(-1)
	if c.index != 0 {
		t.Fatalf("index below zero: %d", c.index)
	}
	for i := 0; i < 10; i++ {
		c.move(1)
	}
	if c.index != 4 {
		t.Fatalf("index above the end: %d", c.index)
	}

	c.ensureVisible(2, true)
	if c.offset != 3 {
		t.Fatalf("offset: %d", c.offset)
	}
}

func TestCacheViewKeepsLayout(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.cache = &cacheModel{
		tracks: []api.Track{
			{Id: "a", Title: "A very long title that has to be shortened to fit the dialog width", FileSize: 3 * 1024 * 1024},
			{Id: "b", Title: "Short", FileSize: 512 * 1024},
		},
		total: 3*1024*1024 + 512*1024,
	}

	plain := ansi.Strip(m.cacheView())
	if !strings.Contains(plain, "Кэш") {
		t.Fatal("title is missing")
	}
	if !strings.Contains(plain, "Файлов: 2") {
		t.Fatalf("summary is missing:\n%s", plain)
	}

	for _, line := range strings.Split(plain, "\n") {
		if w := ansi.StringWidth(line); w > 80 {
			t.Fatalf("line is too wide (%d): %q", w, line)
		}
	}
}

func TestCacheViewEmptyAndLoading(t *testing.T) {
	m := &Model{width: 100, height: 40}

	m.cache = &cacheModel{loading: true}
	if !strings.Contains(ansi.Strip(m.cacheView()), "Читаю кэш") {
		t.Fatal("loading message is missing")
	}

	m.cache = &cacheModel{}
	if !strings.Contains(ansi.Strip(m.cacheView()), "Кэш пуст") {
		t.Fatal("empty cache message is missing")
	}

	m.cache = &cacheModel{err: "boom"}
	if !strings.Contains(ansi.Strip(m.cacheView()), "boom") {
		t.Fatal("error message is missing")
	}
}
