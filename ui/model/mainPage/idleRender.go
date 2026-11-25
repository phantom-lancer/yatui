package mainpage

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/components/tracker"
)

// Idle rendering limits full-frame work during high-frequency mouse motion
// and playback progress updates. Clicks, keys, dialogs, and notifications
// remain immediate. The most recently rendered frame is reused while a new
// idle frame is waiting, so intermediate events do not rebuild the layout.

const (
	// idleRenderPeriod is the minimum interval between idle frames.
	idleRenderPeriod = 80 * time.Millisecond

	// saverIdleRenderPeriod is the same interval in the resource saving
	// mode: half of the repaints, with no visual difference for the static
	// parts of the screen.
	saverIdleRenderPeriod = 200 * time.Millisecond
)

// frameInterval returns the idle cadence for the current settings.
func frameInterval() time.Duration {
	if config.Current.ResourceSaver {
		return saverIdleRenderPeriod
	}
	return idleRenderPeriod
}

// idleRenderMsg is the postponed frame of an idle screen. Its generation
// makes a stale timer harmless after a key/click or a newer frame is queued.
type idleRenderMsg struct {
	generation uint64
}

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
		m.rgbSettings == nil &&
		m.wave == nil &&
		m.contextMenu == nil
}

// frameDecision tells whether the message gets its own frame right away.
func (m *Model) frameDecision(message tea.Msg) (immediate bool, schedule bool) {
	if _, ok := message.(idleRenderMsg); ok {
		return false, false
	}
	if !m.idleScreen() {
		return true, false
	}

	switch msg := message.(type) {
	case tea.MouseMsg:
		// Presses and releases must stay immediate. Only pointer motion is
		// coalesced, and only when an idle frame was drawn recently.
		if msg.Action != tea.MouseActionMotion {
			return true, false
		}
	case tracker.ProgressControl:
		// Progress samples are drawn at the same bounded idle cadence.
	default:
		// keys, dialogs, notifications and the like react at once
		return true, false
	}

	if time.Since(m.progressDrawn) >= frameInterval() {
		return true, false
	}
	return false, true
}

func idleRenderTick(generation uint64) tea.Cmd {
	return tea.Tick(frameInterval(), func(time.Time) tea.Msg {
		return idleRenderMsg{generation: generation}
	})
}

// settleIdle releases the cached frame exactly once. The next View call
// computes the latest state after the coalescing interval has elapsed.
func (m *Model) settleIdle() tea.Cmd {
	if !m.idlePending {
		return nil
	}
	m.idlePending = false
	m.idleFrameValid = false
	m.idleRenderGeneration++
	return nil
}
