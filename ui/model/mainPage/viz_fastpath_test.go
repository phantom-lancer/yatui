package mainpage

import (
	"strings"
	"sync"
	"testing"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
	"github.com/phantom-lancer/yatui/visualizer"
)

// newVizModel builds a main screen with a visible visualizer.
func newVizModel(t testing.TB, width, height int, mode string) *Model {
	t.Helper()
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	func() {
		config.Current.VisualizerEnabled = mode != "off"
		config.Current.VisualizerMode = mode
		config.Current.ResourceSaver = false
	}()

	// The audio context of the player is global: only one model can be built
	// per test binary, so the tests share it and reset the screen.
	vizModelOnce.Do(func() { vizModel = New(nil) })
	m := vizModel
	m.width, m.height = width, height
	m.resize(width, height)
	m.playlists.Reset()
	m.playlists.Hidden = false
	m.playlists.InsertItem(-1, &playlist.Item{Name: "local", Kind: playlist.LOCAL, Active: true})
	m.playlists.Select(0)
	m.isLoading = false
	m.isSettingsActive = false
	m.isQueueActive = false
	m.isSearchActive = false
	m.isAddPlaylistActive = false
	m.isRenamePlaylistActive = false
	m.isStationsActive = false
	m.isCacheActive = false
	m.eq = nil
	m.contextMenu = nil
	m.trackSearch = nil
	m.queue = nil
	m.wave = nil
	m.toasts = nil
	m.idleFrameValid = false
	m.vizLayout.valid = false
	return m
}

var (
	vizModelOnce sync.Once
	vizModel     *Model
)

// fakeFrame builds a frame of the given size with a recognisable colour.
func fakeFrame(width, height int, generation uint64) visualizer.Frame {
	lines := make([]string, height)
	cells := make([]visualizer.RGB, width*height)
	for y := 0; y < height; y++ {
		var b strings.Builder
		for x := 0; x < width; x++ {
			b.WriteString("\x1b[38;2;10;20;30;48;2;30;20;10m▀")
			cells[y*width+x] = visualizer.RGB{R: 200, G: 100, B: 50}
		}
		b.WriteString("\x1b[0m")
		lines[y] = b.String()
	}
	return visualizer.Frame{
		Text:       strings.Join(lines, "\n"),
		Cells:      cells,
		Width:      width,
		Height:     height,
		Generation: generation,
	}
}

// TestPanelFastPathRebuildsOnlyTheBlock checks that an animation frame does
// not lay out the whole interface again: the cached screen must be reused and
// only the block lines must change.
func TestPanelFastPathRebuildsOnlyTheBlock(t *testing.T) {
	m := newVizModel(t, 100, 30, "panel")
	view := m.View()
	if !m.vizLayout.valid {
		t.Fatal("the layout was not cached")
	}
	if m.vizLayout.vizHeight == 0 {
		t.Fatalf("the panel block was not found in:\n%s", view)
	}
	// the block must be the visualizer panel, not the track list above it
	blockLine := stripANSI(m.vizLayout.lines[m.vizLayout.vizTop])
	if !strings.Contains(blockLine, "╭") {
		t.Fatalf("the block does not start on a border line: %q", blockLine)
	}
	above := m.vizLayout.lines[m.vizLayout.vizTop-1]
	if strings.Contains(stripANSI(above), "▀") {
		t.Fatal("the block starts inside the previous panel")
	}

	w, h := m.vizContentSize()
	m.vizFrame = fakeFrame(w, h, 1)
	m.vizFrameValid = true
	m.vizDirty = true

	animated, ok := m.composeVizOnly()
	if !ok {
		t.Fatal("the fast path refused a ready frame")
	}
	if !strings.Contains(animated, "▀") {
		t.Fatal("the animated frame is missing")
	}

	// only the block lines may differ from the cached screen
	baseLines := strings.Split(view, "\n")
	animLines := strings.Split(animated, "\n")
	if len(baseLines) != len(animLines) {
		t.Fatalf("line count changed: %d -> %d", len(baseLines), len(animLines))
	}
	changed := 0
	for i := range baseLines {
		if baseLines[i] != animLines[i] {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("nothing changed: the frame was not inserted")
	}
	if changed > m.vizLayout.vizHeight+2 {
		t.Fatalf("%d lines changed, only the %d block lines may change", changed, m.vizLayout.vizHeight+2)
	}
}

// TestBackgroundFastPathKeepsTheLayout checks that the background layer does
// not move a single character of the interface.
func TestBackgroundFastPathKeepsTheLayout(t *testing.T) {
	m := newVizModel(t, 100, 30, "background")
	base := m.View()
	if m.vizLayout.mode != visualizer.ModeBackground {
		t.Fatal("the background mode was not cached")
	}

	m.vizFrame = fakeFrame(m.width, m.height, 1)
	m.vizFrameValid = true
	m.vizDirty = true

	animated, ok := m.composeVizOnly()
	if !ok {
		t.Fatal("the background fast path refused a ready frame")
	}
	if !strings.Contains(animated, "48;2;") {
		t.Fatal("the background colour is missing")
	}

	// the visible text of every line must be exactly the same
	baseLines := strings.Split(base, "\n")
	animLines := strings.Split(animated, "\n")
	if len(baseLines) != len(animLines) {
		t.Fatalf("line count changed: %d -> %d", len(baseLines), len(animLines))
	}
	for i := range baseLines {
		if visibleCells(baseLines[i]) != visibleCells(animLines[i]) {
			t.Fatalf("line %d changed width: %d -> %d", i, visibleCells(baseLines[i]), visibleCells(animLines[i]))
		}
		if stripANSI(baseLines[i]) != stripANSI(animLines[i]) {
			t.Fatalf("line %d text changed:\n%q\n%q", i, stripANSI(baseLines[i]), stripANSI(animLines[i]))
		}
	}
}

func BenchmarkVizPanelCompose(b *testing.B) {
	m := newVizModel(b, 86, 27, "panel")
	m.View()
	w, h := m.vizContentSize()
	m.vizFrame = fakeFrame(w, h, 1)
	m.vizFrameValid = true
	m.vizDirty = true

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.vizDirty = true
		if _, ok := m.composeVizOnly(); !ok {
			b.Fatal("no fast path")
		}
	}
}

func BenchmarkVizBackgroundCompose(b *testing.B) {
	m := newVizModel(b, 86, 27, "background")
	m.View()
	m.vizFrame = fakeFrame(m.width, m.height, 1)
	m.vizFrameValid = true
	m.vizDirty = true

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.vizDirty = true
		if _, ok := m.composeVizOnly(); !ok {
			b.Fatal("no fast path")
		}
	}
}

func BenchmarkFullView(b *testing.B) {
	m := newVizModel(b, 86, 27, "panel")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.vizLayout.valid = false
		_ = m.viewBody()
	}
}

// TestBackgroundPaintsTheGaps checks that the background layer really paints
// the empty cells of a real main screen, not just one cell.
func TestBackgroundPaintsTheGaps(t *testing.T) {
	m := newVizModel(t, 86, 27, "background")
	m.View()
	if !m.vizLayout.valid || m.vizLayout.mode != visualizer.ModeBackground {
		t.Fatal("the background layout was not cached")
	}
	runs := 0
	cells := 0
	for _, lineRuns := range m.vizLayout.bgRuns {
		runs += len(lineRuns)
		for _, run := range lineRuns {
			cells += run.cells
		}
	}
	if runs < 20 {
		t.Fatalf("only %d runs of empty cells found", runs)
	}
	if cells < 300 {
		t.Fatalf("only %d empty cells found", cells)
	}

	m.vizFrame = fakeFrame(m.width, m.height, 1)
	m.vizFrameValid = true
	m.vizDirty = true
	view, ok := m.composeVizOnly()
	if !ok {
		t.Fatal("the background fast path refused the frame")
	}
	if painted := strings.Count(view, "\x1b[48;2;"); painted < 300 {
		t.Fatalf("only %d cells were painted", painted)
	}

	// and the whole View path must return the painted screen, not a cached
	// frame of the previous layout
	m.vizLayout.valid = true
	m.vizFrame = fakeFrame(m.width, m.height, 2)
	m.vizDirty = true
	m.idleFrameValid = false
	m.vizNoop = false
	rendered := m.View()
	if painted := strings.Count(rendered, "\x1b[48;2;"); painted < 300 {
		t.Fatalf("View returned only %d painted cells", painted)
	}
}
