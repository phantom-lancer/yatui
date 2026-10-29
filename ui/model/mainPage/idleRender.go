package mainpage

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/ui/components/tracker"
)

// Idle rendering: a player that only plays a track changes almost nothing on
// the screen, but every message used to trigger a full frame. The mouse is in
// all-motion mode, so moving the cursor across the window (or a trackpad's
// jitter) sent dozens of motion events per second and the UI redrew the whole
// window for each one.
//
// The frame is now coalesced: when a mouse motion or a progress update
// arrives while the screen is idle, its frame is postponed until the input
// goes quiet, and the newest state is drawn once. Keys and clicks are never
// delayed, and a dialog, a notification or a new track still draws at once.

const (
	// idleRenderPeriod is the longest a frame may wait for the input to
	// settle. Every newer event restarts the wait, so a steady stream of
	// motion events draws at this rate instead of once per event.
	idleRenderPeriod = 80 * time.Millisecond
	// idleRenderSettle is how long the input has to stay quiet before the
	// postponed frame is drawn. A fast sweep of the mouse still redraws a
	// few times a second, so the highlight does not feel stuck.
	idleRenderSettle = 16 * time.Millisecond
)

// idleRenderMsg is the postponed frame of an idle screen.
type idleRenderMsg struct{}

// idleScreen reports whether the model is in the plain playback state: no
// dialog, no menu, nothing that a motion event can change quickly.
func (m *Model) idleScreen() bool {
	return !m.isLoading &&
		!m.isSettingsActive &&
		!m.isQueueActive &&
		!m.isSearchActive &&
		!m.isAddPlaylistActive &&
		!m.isRenamePlaylistActive &&
		!m.isStationsActive &&
		!m.isCacheActive &&
		m.eq == nil &&
		m.discordSettings == nil &&
		m.wave == nil &&
		m.contextMenu == nil
}

// frameDecision tells whether the message gets its own frame right away.
func (m *Model) frameDecision(message tea.Msg) (immediate bool, schedule bool) {
	if !m.idleScreen() {
		return true, false
	}

	switch message.(type) {
	case tea.MouseMsg:
		return false, true
	case tracker.ProgressControl:
		// an idle screen only needs a few frames per second to move the bar
		return false, time.Since(m.progressDrawn) >= idleRenderPeriod
	}

	// keys, dialogs, notifications and the like react at once
	return true, false
}

// idleRenderTick is a variable, so the tests can drive the wait themselves.
var idleRenderTick = defaultIdleRenderTick

func defaultIdleRenderTick() tea.Cmd {
	return tea.Tick(idleRenderSettle, func(time.Time) tea.Msg {
		return idleRenderMsg{}
	})
}

// settleIdle draws the postponed frame. While the input is still coming, the
// wait starts over instead, so a moving pointer does not redraw the screen
// dozens of times a second.
func (m *Model) settleIdle() tea.Cmd {
	defer func() { m.progressDrawn = time.Now() }()

	if m.idlePending && time.Since(m.idleSince) < idleRenderPeriod {
		m.idleSince = time.Now()
		return idleRenderTick()
	}

	m.idlePending = false
	return nil
}
