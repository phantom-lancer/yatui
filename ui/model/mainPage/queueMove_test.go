package mainpage

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
)

// keyMsg builds a key message with the given name.
func keyMsg(name string) tea.KeyMsg {
	switch name {
	case "ctrl+up":
		return tea.KeyMsg{Type: tea.KeyCtrlUp}
	case "ctrl+down":
		return tea.KeyMsg{Type: tea.KeyCtrlDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "delete":
		return tea.KeyMsg{Type: tea.KeyDelete}
	case "c":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	case "p":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
	}
}

// TestQueueEntriesCanBeMoved checks that the tracks the user added to the
// queue can be rearranged.
func TestQueueEntriesCanBeMoved(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.manualQueue = []api.Track{
		testTrack("m1", "First"),
		testTrack("m2", "Second"),
		testTrack("m3", "Third"),
	}
	m.queue = &queueModel{}

	// the second track moves up
	m.moveManualQueueEntry(1, -1)
	if m.manualQueue[0].Id != "m2" || m.manualQueue[1].Id != "m1" {
		t.Fatalf("the track was not moved: %v %v", m.manualQueue[0].Id, m.manualQueue[1].Id)
	}
	if m.queue.index != 0 {
		t.Errorf("the selection did not follow the track: %d", m.queue.index)
	}

	// and back
	m.moveManualQueueEntry(0, 1)
	if m.manualQueue[0].Id != "m1" || m.manualQueue[1].Id != "m2" {
		t.Fatalf("the track was not moved back: %v %v", m.manualQueue[0].Id, m.manualQueue[1].Id)
	}

	// the edges are not crossed
	m.moveManualQueueEntry(0, -1)
	if m.manualQueue[0].Id != "m1" {
		t.Fatalf("the track moved out of the queue: %v", m.manualQueue[0].Id)
	}
	m.moveManualQueueEntry(2, 1)
	if m.manualQueue[2].Id != "m3" {
		t.Fatalf("the track moved out of the queue: %v", m.manualQueue[2].Id)
	}
}

// TestQueueEntryCanBeMovedToTop checks that a track can be put at the first
// place: it will be played right after the current one.
func TestQueueEntryCanBeMovedToTop(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.manualQueue = []api.Track{
		testTrack("m1", "First"),
		testTrack("m2", "Second"),
		testTrack("m3", "Third"),
	}
	m.queue = &queueModel{}

	m.moveManualQueueToTop(2)
	if m.manualQueue[0].Id != "m3" {
		t.Fatalf("the track was not put first: %v", m.manualQueue[0].Id)
	}
	if len(m.manualQueue) != 3 {
		t.Fatalf("the queue lost a track: %d", len(m.manualQueue))
	}
	if m.manualQueue[1].Id != "m1" || m.manualQueue[2].Id != "m2" {
		t.Fatalf("the order of the queue is broken: %v %v", m.manualQueue[1].Id, m.manualQueue[2].Id)
	}
	if m.queue.index != 0 {
		t.Errorf("the selection did not follow the track: %d", m.queue.index)
	}

	// the first track is already at the top
	m.moveManualQueueToTop(0)
	if m.manualQueue[0].Id != "m3" {
		t.Fatalf("the queue changed: %v", m.manualQueue[0].Id)
	}
}

// TestQueueKeysMoveTheEntries checks the keys of the queue dialog.
func TestQueueKeysMoveTheEntries(t *testing.T) {
	m := &Model{width: 100, height: 40}
	m.manualQueue = []api.Track{
		testTrack("m1", "First"),
		testTrack("m2", "Second"),
	}
	m.queue = &queueModel{index: 1}
	m.isQueueActive = true

	m.queueControl(keyMsg("ctrl+up"))
	if m.manualQueue[0].Id != "m2" {
		t.Fatalf("ctrl+up did not move the track: %v", m.manualQueue[0].Id)
	}

	m.queueControl(keyMsg("ctrl+down"))
	if m.manualQueue[0].Id != "m1" {
		t.Fatalf("ctrl+down did not move the track: %v", m.manualQueue[0].Id)
	}

	m.queueControl(keyMsg("p"))
	if m.manualQueue[0].Id != "m2" {
		t.Fatalf("p did not move the track to the top: %v", m.manualQueue[0].Id)
	}
}

func TestToggleQueueStartsWithoutDrag(t *testing.T) {
	m := &Model{}

	m.toggleQueue()

	if m.queue == nil {
		t.Fatal("queue dialog was not opened")
	}
	if m.queue.dragFrom != -1 {
		t.Fatalf("new queue dialog has active drag source %d, want -1", m.queue.dragFrom)
	}
}

func TestQueueMouseUsesScrolledRows(t *testing.T) {
	m := &Model{width: 100, height: 18}
	for i := 0; i < 10; i++ {
		m.manualQueue = append(m.manualQueue, testTrack(fmt.Sprintf("m%d", i), fmt.Sprintf("Track %d", i)))
	}
	m.queue = &queueModel{index: 8, dragFrom: -1}

	m.queueView()

	if m.queue.offset != 5 {
		t.Fatalf("queue offset = %d, want 5", m.queue.offset)
	}
	if m.queueLayout.rows != 4 {
		t.Fatalf("visible row count = %d, want 4", m.queueLayout.rows)
	}

	x := m.queueLayout.x + 1
	y := m.queueLayout.rowY
	m.queueMouse(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if m.queue.dragFrom != 5 {
		t.Fatalf("drag started at row %d, want absolute row 5", m.queue.dragFrom)
	}

	m.queueMouse(tea.MouseMsg{X: x, Y: y + 1, Action: tea.MouseActionMotion, Button: tea.MouseButtonNone})
	if m.queue.index != 6 {
		t.Fatalf("drag selection = %d, want absolute row 6", m.queue.index)
	}

	m.queueMouse(tea.MouseMsg{X: x, Y: y + 1, Action: tea.MouseActionRelease, Button: tea.MouseButtonNone})
	if m.manualQueue[5].Id != "m6" || m.manualQueue[6].Id != "m5" {
		t.Fatalf("drop did not move the visible row: got [%s, %s], want [m6, m5]", m.manualQueue[5].Id, m.manualQueue[6].Id)
	}
}
