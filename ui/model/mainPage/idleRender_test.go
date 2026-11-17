package mainpage

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/ui/components/tracker"
)

func idleModel() *Model {
	return &Model{}
}

func TestMotionOnIdleScreenIsCoalescedUntilTheNextFrame(t *testing.T) {
	m := idleModel()
	m.progressDrawn = time.Now()

	immediate, schedule := m.frameDecision(tea.MouseMsg{Action: tea.MouseActionMotion})
	if immediate || !schedule {
		t.Fatalf("recent motion must be postponed: immediate=%v schedule=%v", immediate, schedule)
	}

	m.progressDrawn = time.Now().Add(-idleRenderPeriod)
	immediate, schedule = m.frameDecision(tea.MouseMsg{Action: tea.MouseActionMotion})
	if !immediate || schedule {
		t.Fatalf("motion after the frame interval must draw now: immediate=%v schedule=%v", immediate, schedule)
	}
}

func TestMouseButtonsAreNeverPostponed(t *testing.T) {
	m := idleModel()
	m.progressDrawn = time.Now()

	for _, action := range []tea.MouseAction{tea.MouseActionPress, tea.MouseActionRelease} {
		immediate, schedule := m.frameDecision(tea.MouseMsg{Action: action})
		if !immediate || schedule {
			t.Fatalf("mouse action %v must draw immediately: immediate=%v schedule=%v", action, immediate, schedule)
		}
	}
}

func TestIdleFrameCacheIsUsedWhileMotionIsPending(t *testing.T) {
	m := &Model{idlePending: true, idleFrame: "last rendered frame", idleFrameValid: true}
	if got := m.View(); got != "last rendered frame" {
		t.Fatalf("View() = %q, want cached frame", got)
	}
}

func TestSettlingIdleInvalidatesTheCachedFrameOnce(t *testing.T) {
	m := &Model{idlePending: true, idleFrame: "old", idleFrameValid: true, idleRenderGeneration: 4}
	if cmd := m.settleIdle(); cmd != nil {
		t.Fatal("settling the pending frame must not schedule another timer")
	}
	if m.idlePending || m.idleFrameValid {
		t.Fatal("pending state or stale frame cache was not cleared")
	}
	if m.idleRenderGeneration != 5 {
		t.Fatalf("generation = %d, want 5", m.idleRenderGeneration)
	}
}

func TestStaleIdleRenderTickDoesNotClearCurrentPendingFrame(t *testing.T) {
	m := &Model{idlePending: true, idleFrame: "current", idleFrameValid: true, idleRenderGeneration: 8}
	if immediate, schedule := m.frameDecision(idleRenderMsg{generation: 7}); immediate || schedule {
		t.Fatalf("stale timer decision = (%v, %v), want (false, false)", immediate, schedule)
	}
	_, cmd := m.Update(idleRenderMsg{generation: 7})
	if cmd != nil || !m.idlePending || !m.idleFrameValid || m.idleRenderGeneration != 8 {
		t.Fatal("stale timer changed the current pending frame")
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
	if immediate, schedule := m.frameDecision(tea.MouseMsg{Action: tea.MouseActionMotion}); !immediate || schedule {
		t.Fatalf("a dialog must draw at once: immediate=%v schedule=%v", immediate, schedule)
	}
}

func TestProgressIsThrottledOnIdleScreen(t *testing.T) {
	m := idleModel()
	m.progressDrawn = time.Now()

	immediate, schedule := m.frameDecision(tracker.ProgressControl(0.5))
	if immediate || !schedule {
		t.Fatalf("a recent progress sample must be coalesced: immediate=%v schedule=%v", immediate, schedule)
	}

	m.progressDrawn = time.Now().Add(-idleRenderPeriod)
	immediate, schedule = m.frameDecision(tracker.ProgressControl(0.5))
	if !immediate || schedule {
		t.Fatalf("a progress sample after the interval must draw now: immediate=%v schedule=%v", immediate, schedule)
	}
}

func TestNothingPendingDrawsNothing(t *testing.T) {
	m := idleModel()
	if cmd := m.settleIdle(); cmd != nil {
		t.Fatal("nothing is pending: no extra frame")
	}
}
