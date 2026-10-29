package mainpage

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/ui/components/tracker"
)

// The idle rendering must never hold back a key or collapse a burst of mouse
// motion into a stutter: this test drives the wait exactly like Update does.

func idleModel() *Model {
	return &Model{}
}

func TestMotionOnIdleScreenIsCoalesced(t *testing.T) {
	m := idleModel()
	m.progressDrawn = time.Now()

	immediate, schedule := m.frameDecision(tea.MouseMsg{Action: tea.MouseActionMotion})
	if immediate || !schedule {
		t.Fatalf("motion must be postponed: immediate=%v schedule=%v", immediate, schedule)
	}

	// every motion while the wait is running restarts it
	var wait time.Duration
	idleRenderTick = func() tea.Cmd {
		wait = idleRenderSettle
		return func() tea.Msg { return nil }
	}
	defer func() { idleRenderTick = defaultIdleRenderTick }()

	m.idlePending = true
	m.idleSince = time.Now()
	if cmd := m.settleIdle(); cmd == nil {
		t.Fatal("the wait must continue while there is no pause")
	}
	if !m.idlePending {
		t.Fatal("the frame must stay postponed")
	}
	if wait != idleRenderSettle {
		t.Fatalf("unexpected wait %v", wait)
	}
}

func TestIdleFrameIsDrawnAfterThePause(t *testing.T) {
	m := idleModel()
	idleRenderTick = func() tea.Cmd { return nil }
	defer func() { idleRenderTick = defaultIdleRenderTick }()

	m.idlePending = true
	m.idleSince = time.Now().Add(-idleRenderPeriod - time.Millisecond)
	if cmd := m.settleIdle(); cmd != nil {
		t.Fatal("the frame must be drawn, not postponed again")
	}
	if m.idlePending {
		t.Fatal("the pending frame must be cleared once drawn")
	}
}

func TestKeysAreNeverPostponed(t *testing.T) {
	m := idleModel()
	immediate, schedule := m.frameDecision(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !immediate || schedule {
		t.Fatalf("a key must draw at once: immediate=%v schedule=%v", immediate, schedule)
	}
}

func TestDialogsDrawAtOnce(t *testing.T) {
	m := idleModel()
	m.isSettingsActive = true
	if immediate, schedule := m.frameDecision(tea.MouseMsg{}); !immediate || schedule {
		t.Fatalf("a dialog must draw at once: immediate=%v schedule=%v", immediate, schedule)
	}
}

func TestProgressIsThrottledOnIdleScreen(t *testing.T) {
	m := idleModel()
	m.progressDrawn = time.Now()

	if _, schedule := m.frameDecision(tracker.ProgressControl(0.5)); schedule {
		t.Fatal("a progress sample right after a frame must not be drawn")
	}

	m.progressDrawn = time.Now().Add(-idleRenderPeriod)
	if _, schedule := m.frameDecision(tracker.ProgressControl(0.5)); !schedule {
		t.Fatal("a progress sample must be drawn at the idle rate")
	}
}

// A frame that is no longer needed must not arm another timer.
func TestNothingPendingDrawsNothing(t *testing.T) {
	m := idleModel()
	ticks := 0
	idleRenderTick = func() tea.Cmd {
		ticks++
		return nil
	}
	defer func() { idleRenderTick = defaultIdleRenderTick }()

	if cmd := m.settleIdle(); cmd != nil {
		t.Fatal("nothing is pending: no extra frame")
	}
	if ticks != 0 {
		t.Fatal("nothing is pending: no new timer")
	}
}
