package mainpage

import (
	"hash/fnv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/style"
	"github.com/phantom-lancer/yatui/visualizer"
)

// vizTickMsg is the render heartbeat of the visualizer. Only one tick is
// alive at a time; the generation makes a queued tick harmless after the
// visualizer was disabled.
type vizTickMsg struct {
	generation uint64
}

const (
	// vizMinHeight is the smallest content height of the panel.
	vizMinHeight = 4
	// vizMaxHeight caps the panel so a tall terminal keeps room for the
	// track list and the player.
	vizMaxHeight = 9
	// vizMinTerminalHeight hides the panel on terminals where it would take
	// the space the player needs.
	vizMinTerminalHeight = 22
	// vizMinWidth hides the panel when the middle panel is too narrow.
	vizMinWidth = 30
)

// visualizerEnabled reports the user setting, taking the resource saving mode
// into account.
func (m *Model) visualizerEnabled() bool {
	return config.Current.VisualizerEnabled && !config.Current.ResourceSaver
}

// visualizerVisible reports whether the panel fits the current screen and no
// dialog covers it.
func (m *Model) visualizerVisible() bool {
	if !m.visualizerEnabled() ||
		m.height < vizMinTerminalHeight ||
		!m.idleScreen() ||
		m.isHelpActive ||
		m.trackSearch != nil ||
		m.queue != nil {
		return false
	}
	// the panel lives in the middle column: a wide terminal with a visible
	// sidebar can still leave too little room for it
	width, _ := m.vizContentSize()
	return width >= vizMinWidth
}

// vizContentSize returns the size of the visualizer cells, without the
// border. The width matches the middle panel of the main screen.
func (m *Model) vizContentSize() (int, int) {
	playlistWidth := 0
	if m.playlists != nil && !m.playlists.Hidden {
		playlistWidth = style.SidePanelWidth + 2
	}
	width := m.width - playlistWidth - 2
	if width < 1 {
		width = 1
	}

	height := m.height / 5
	if height < vizMinHeight {
		height = vizMinHeight
	}
	if height > vizMaxHeight {
		height = vizMaxHeight
	}
	return width, height
}

// startVisualizer starts the worker and the frame heartbeat. It is safe to
// call repeatedly: a running visualizer is not started twice.
func (m *Model) startVisualizer() tea.Cmd {
	if !m.visualizerEnabled() {
		m.stopVisualizer()
		return nil
	}
	if m.visualizer == nil {
		m.visualizer = visualizer.New()
	}
	m.visualizer.SetPalette(m.visualizerPalette())
	m.visualizer.SetEnabled(true)
	if m.tracker != nil {
		m.tracker.SetVisualizer(true)
	}
	return m.scheduleVizTick(visualizer.DefaultFramePeriod)
}

// stopVisualizer stops the worker and drops the frame: the resource saving
// mode must not leave a rendering goroutine behind.
func (m *Model) stopVisualizer() {
	if m.visualizer != nil {
		m.visualizer.SetEnabled(false)
	}
	if m.tracker != nil {
		m.tracker.SetVisualizer(false)
	}
	m.vizGeneration++
	m.vizTicking = false
	m.vizFrame = visualizer.Frame{}
	m.vizFrameValid = false
	m.idleFrameValid = false
}

func (m *Model) scheduleVizTick(period time.Duration) tea.Cmd {
	if m.vizTicking || !m.visualizerEnabled() {
		return nil
	}
	if period <= 0 {
		period = visualizer.DefaultFramePeriod
	}
	m.vizGeneration++
	m.vizTicking = true
	generation := m.vizGeneration
	return tea.Tick(period, func(time.Time) tea.Msg {
		return vizTickMsg{generation: generation}
	})
}

// handleVizTick feeds the worker with the current audio level and palette and
// requests a repaint when a new frame is ready.
func (m *Model) handleVizTick(msg vizTickMsg) tea.Cmd {
	if msg.generation != m.vizGeneration {
		return nil
	}
	m.vizTicking = false
	if !m.visualizerEnabled() {
		m.stopVisualizer()
		return nil
	}
	if m.visualizer == nil {
		return m.startVisualizer()
	}

	playing := m.tracker != nil && m.tracker.IsPlaying()
	amplitude := 0.0
	if m.tracker != nil {
		amplitude = m.tracker.Amplitude()
	}
	m.visualizer.SetAmplitude(amplitude)
	m.visualizer.SetPlaying(playing)
	m.visualizer.SetPalette(m.visualizerPalette())

	width, height := 0, 0
	if m.visualizerVisible() && m.vizContentW > 0 {
		width, height = m.vizContentW, m.vizContentH
	}
	m.visualizer.Resize(width, height)
	if m.tracker != nil {
		// a hidden panel does not need the audio level either
		m.tracker.SetVisualizer(width > 0)
	}

	// A hidden panel does not need 20 wakeups per second: the palette and
	// the audio level are still tracked, just less often.
	period := visualizer.DefaultFramePeriod
	if width == 0 {
		period = 200 * time.Millisecond
	}

	if frame, ok := m.visualizer.Latest(); ok {
		if frame.Generation != m.vizFrame.Generation || !m.vizFrameValid {
			m.vizFrame = frame
			m.vizFrameValid = true
			m.vizFrameWidth, m.vizFrameHeight = width, height
			m.idleFrameValid = false
		}
	}

	return m.scheduleVizTick(period)
}

// visualizerPalette builds the palette from the theme accent. A stable hash
// of the track id rotates the hues, so every track tints the rings a little
// differently while the theme keeps its character. The renderer fades
// between palettes, never cuts.
func (m *Model) visualizerPalette() visualizer.Palette {
	accent, ok := visualizer.ParseHex(string(style.AccentColor))
	if !ok {
		accent = visualizer.RGB{R: 247, G: 212, B: 75}
	}
	background, ok := visualizer.ParseHex(string(style.BackgroundColor))
	if !ok {
		background = visualizer.RGB{R: 13, G: 13, B: 13}
	}
	// the rings need a quiet backdrop; the theme background is used for the
	// progress bar and can be light
	background = visualizer.Scale(background, 0.3)

	shift := 0.0
	if m.tracker != nil {
		if track := m.tracker.CurrentTrack(); track != nil && track.Id != "" {
			h := fnv.New32a()
			_, _ = h.Write([]byte(track.Id))
			shift = float64(h.Sum32()%80) - 40
		}
	}
	return visualizer.PaletteFromAccent(accent, background, shift)
}

// visualizerPanel renders the frame inside a rounded frame that matches the
// rest of the interface. The frame content is padded when the worker has not
// produced the first frame yet, so the layout never jumps.
func (m *Model) visualizerPanel(contentWidth, contentHeight int) string {
	content := m.vizFrame.Text
	lines := strings.Split(content, "\n")
	if len(lines) != contentHeight || m.vizFrameWidth != contentWidth || m.vizFrameHeight != contentHeight {
		blank := strings.Repeat(" ", contentWidth)
		padded := make([]string, contentHeight)
		for i := range padded {
			padded[i] = blank
		}
		content = strings.Join(padded, "\n")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.BorderColor).
		Render(content)
}

// applyResourceSaving starts or stops the optional visual work after the
// setting changed.
func (m *Model) applyResourceSaving() tea.Cmd {
	if config.Current.ResourceSaver {
		m.stopVisualizer()
		m.winterStop()
		m.rgbRunning = false
		m.idleFrameValid = false
		m.showToast("Экономия ресурсов: вкл")
		return nil
	}

	m.showToast("Экономия ресурсов: выкл")
	return tea.Batch(m.startVisualizer(), m.rgbStart(), m.winterStart())
}

// toggleVisualizer flips the visualizer setting and applies it immediately.
func (m *Model) toggleVisualizer() tea.Cmd {
	if config.Current.ResourceSaver {
		m.showToast("Сначала выключите экономию ресурсов")
		return nil
	}
	config.Current.VisualizerEnabled = !config.Current.VisualizerEnabled
	m.saveConfig()
	if config.Current.VisualizerEnabled {
		m.showToast("Визуализатор: вкл")
		return m.startVisualizer()
	}
	m.stopVisualizer()
	m.showToast("Визуализатор: выкл")
	return nil
}

// vizStatus is the value of the settings row.
func vizStatus() string {
	return onOff(config.Current.VisualizerEnabled && !config.Current.ResourceSaver)
}
