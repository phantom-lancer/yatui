package tracker

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/style"
)

func strip(s string) string {
	return ansi.Strip(s)
}

// cell returns the rune rendered at the given visual column.
func cell(t *testing.T, line string, col int) string {
	t.Helper()
	line = strip(line)
	x := 0
	for _, r := range line {
		w := runewidth.RuneWidth(r)
		if x <= col && col < x+w {
			return string(r)
		}
		x += w
	}
	return ""
}

func lineOf(t *testing.T, view string, y int) string {
	t.Helper()
	lines := strings.Split(view, "\n")
	if y < 0 || y >= len(lines) {
		t.Fatalf("line %d is out of range (%d lines)", y, len(lines))
	}
	return lines[y]
}

func newTestModel() *Model {
	m := &Model{
		progress:    progress.New(),
		volumeBar:   progress.New(),
		help:        help.New(),
		helpMap:     newHelpMap(),
		likesMap:    &map[string]bool{},
		dislikesMap: &map[string]bool{},
		volume:      0.5,
		track: api.Track{
			Id:         "1",
			Title:      "test title",
			Artists:    []api.Artist{{Id: 1, Name: "artist"}},
			DurationMs: 200000,
			Available:  true,
		},
	}

	m.progress.ShowPercentage = false
	m.progress.Empty = m.progress.Full
	m.progress.FullColor = string(style.AccentColor)
	m.progress.EmptyColor = string(style.BackgroundColor)
	m.progress.SetSpringOptions(60, 1)

	m.volumeBar.FullColor = string(style.AccentColor)
	m.volumeBar.EmptyColor = string(style.BackgroundColor)
	m.volumeBar.Width = style.VolumeIndicatorWidth
	m.help.Ellipsis = "…"
	m.SetWidth(80)

	return m
}

func TestRegionsMatchRenderedView(t *testing.T) {
	m := newTestModel()
	view, regions := m.view()
	if regions == nil {
		t.Fatal("no regions returned")
	}

	// transport buttons: prev, play, next
	bar := lineOf(t, view, regions.PlayButton.Y)
	if got := cell(t, bar, regions.PrevButton.X+1); got != style.IconPrev {
		t.Errorf("prev button cell: got %q want %q", got, style.IconPrev)
	}
	if got := cell(t, bar, regions.NextButton.X+1); got != style.IconNext {
		t.Errorf("next button cell: got %q want %q", got, style.IconNext)
	}

	if got := cell(t, bar, regions.PlayButton.X+1); got != style.IconStop {
		t.Errorf("play button cell: got %q want %q", got, style.IconStop)
	}
	if got := cell(t, lineOf(t, view, regions.PlayButton.Y), regions.PlayButton.X); got != " " {
		t.Errorf("play button left padding: got %q", got)
	}
	if got := cell(t, lineOf(t, view, regions.PlayButton.Y), regions.PlayButton.X+regions.PlayButton.Width-1); got != " " {
		t.Errorf("play button right padding: got %q", got)
	}

	// progress bar
	barRune := string(m.progress.Full)
	progressLine := lineOf(t, view, regions.Progress.Y)
	if got := cell(t, progressLine, regions.Progress.X); got != barRune {
		t.Errorf("progress start cell: got %q want %q", got, barRune)
	}
	if got := cell(t, progressLine, regions.Progress.X+regions.Progress.Width-1); got != barRune {
		t.Errorf("progress end cell: got %q want %q", got, barRune)
	}
	// one cell right of the bar must not be a bar rune
	if got := cell(t, progressLine, regions.Progress.X+regions.Progress.Width); got == barRune {
		t.Errorf("progress bar is %d cells wide, expected %d", regions.Progress.Width, m.progress.Width)
	}

	// volume indicator
	volumeLine := lineOf(t, view, regions.Volume.Y)
	if regions.Volume.IsEmpty() {
		t.Fatal("volume region is empty")
	}
	if got := cell(t, volumeLine, regions.Volume.X); got != " " {
		t.Errorf("volume starts with %q, want a space", got)
	}
	if got := cell(t, volumeLine, regions.Volume.X+1); got != style.IconVolumeHigh {
		t.Errorf("volume icon: got %q want %q", got, style.IconVolumeHigh)
	}
	if got := cell(t, volumeLine, regions.VolumeBar.X); got != string(m.volumeBar.Full) {
		t.Errorf("volume bar start: got %q want %q", got, string(m.volumeBar.Full))
	}
	if regions.VolumeBar.X < regions.Volume.X || regions.VolumeBar.X+regions.VolumeBar.Width > regions.Volume.X+regions.Volume.Width {
		t.Errorf("volume bar %+v is outside of the indicator %+v", regions.VolumeBar, regions.Volume)
	}

	// like indicator
	if regions.Like.IsEmpty() {
		t.Fatal("like region is empty")
	}
	if got := cell(t, lineOf(t, view, regions.Like.Y), regions.Like.X); got != style.IconNotLiked {
		t.Errorf("like cell: got %q want %q", got, style.IconNotLiked)
	}

	// dislike indicator sits on the track title row, next to the like one
	if regions.Dislike.IsEmpty() {
		t.Fatal("dislike region is empty")
	}
	if got := cell(t, lineOf(t, view, regions.Dislike.Y), regions.Dislike.X); got != style.IconDisliked {
		t.Errorf("dislike cell: got %q want %q", got, style.IconDisliked)
	}
	if regions.Dislike.Y != regions.Like.Y {
		t.Errorf("like and dislike must be on the same row: %d vs %d", regions.Like.Y, regions.Dislike.Y)
	}
	if regions.Dislike.X < regions.Like.X+regions.Like.Width {
		t.Errorf("dislike overlaps like: %d < %d", regions.Dislike.X, regions.Like.X+regions.Like.Width)
	}

	// the transport controls must not overlap
	if regions.PrevButton.X+regions.PrevButton.Width > regions.PlayButton.X {
		t.Errorf("prev button overlaps play button")
	}
	if regions.PlayButton.X+regions.PlayButton.Width > regions.NextButton.X {
		t.Errorf("play button overlaps next button")
	}
	if regions.NextButton.X+regions.NextButton.Width > regions.Progress.X {
		t.Errorf("next button overlaps progress bar")
	}
	if regions.Progress.X+regions.Progress.Width > regions.Volume.X {
		t.Errorf("progress bar overlaps volume indicator")
	}
	if regions.Volume.X+regions.Volume.Width > m.width {
		t.Errorf("volume indicator does not fit: right edge %d, width %d", regions.Volume.X+regions.Volume.Width, m.width)
	}
}

func TestRegionsWithLyrics(t *testing.T) {
	m := newTestModel()
	m.showLyrics = true
	m.lyrics = []api.LyricPair{{Line: "some lyrics", Timestamp: 0}}

	view, regions := m.view()
	if got := cell(t, lineOf(t, view, regions.PlayButton.Y), regions.PlayButton.X+1); got != style.IconStop {
		t.Errorf("with lyrics: play button cell: got %q want %q", got, style.IconStop)
	}
	if regions.PlayButton.Y <= 2 {
		t.Errorf("with lyrics: progress row must be shifted down, got Y=%d", regions.PlayButton.Y)
	}
	if got := cell(t, lineOf(t, view, regions.Like.Y), regions.Like.X); got != style.IconNotLiked {
		t.Errorf("with lyrics: like cell: got %q want %q", got, style.IconNotLiked)
	}
	if got := cell(t, lineOf(t, view, regions.Volume.Y), regions.Volume.X+1); got != style.IconVolumeHigh {
		t.Errorf("with lyrics: volume icon: got %q", got)
	}
}

func TestRegionsNoVolumeAutohide(t *testing.T) {
	m := newTestModel()
	m.SetWidth(style.VolumeIndicatorAutohide)
	view, regions := m.view()
	if !regions.Volume.IsEmpty() {
		t.Errorf("volume region should be hidden, got %+v", regions.Volume)
	}
	if got := cell(t, lineOf(t, view, regions.Progress.Y), regions.Progress.X); got != string(m.progress.Full) {
		t.Errorf("autohide: progress start cell: got %q", got)
	}
}

func init() {
	if err := config.InitialLoad(); err != nil {
		panic(err)
	}
	style.Apply(config.Current.Style)
}
