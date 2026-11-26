package mainpage

import (
	"hash/fnv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

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
	// bgMinRun is the shortest run of empty cells that the background layer
	// paints. Shorter gaps are the spaces between words; painting them would
	// make the text hard to read and add noise to the frame.
	bgMinRun = 4
)

// visualizerMode reports the configured mode, with the resource saving mode
// forcing it off.
func visualizerMode() visualizer.Mode {
	if config.Current.ResourceSaver {
		return visualizer.ModeOff
	}
	switch config.Current.VisualizerMode {
	case "background":
		return visualizer.ModeBackground
	case "panel":
		return visualizer.ModePanel
	case "off":
		return visualizer.ModeOff
	}
	// an older config only has the boolean
	if config.Current.VisualizerEnabled {
		return visualizer.ModePanel
	}
	return visualizer.ModeOff
}

// visualizerEnabled reports whether the worker has anything to do.
func (m *Model) visualizerEnabled() bool {
	return visualizerMode() != visualizer.ModeOff
}

// visualizerVisible reports whether the frame belongs on the screen: the
// panel needs room, the background layer needs the plain main screen.
func (m *Model) visualizerVisible() bool {
	mode := visualizerMode()
	if mode == visualizer.ModeOff || m.height < vizMinTerminalHeight {
		return false
	}
	if !m.idleScreen() || m.isHelpActive || m.trackSearch != nil || m.queue != nil {
		return false
	}
	if mode == visualizer.ModeBackground {
		return m.width >= 40
	}
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

// vizDetail picks how many cells share one noise evaluation. A bigger area
// gets a coarser grid: the rings are soft, the difference is invisible, and
// the CPU cost stays inside the budget.
func vizDetail(width, height int) int {
	cells := width * height
	switch {
	case cells > 900:
		return 4
	case cells > 350:
		return 3
	default:
		return 2
	}
}

// startVisualizer starts the worker and the frame heartbeat. It is safe to
// call repeatedly: a running visualizer is not started twice.
func (m *Model) startVisualizer() tea.Cmd {
	mode := visualizerMode()
	if mode == visualizer.ModeOff {
		m.stopVisualizer()
		return nil
	}
	if m.visualizer == nil {
		m.visualizer = visualizer.New()
	}
	m.visualizer.SetMode(mode)
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
	m.vizDirty = false
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
// marks the view dirty when a new frame is ready.
func (m *Model) handleVizTick(msg vizTickMsg) tea.Cmd {
	if msg.generation != m.vizGeneration {
		return nil
	}
	m.vizTicking = false
	// Until a new frame is ready the screen stays as it is: a tick without a
	// frame must not rebuild the interface.
	m.vizNoop = true
	mode := visualizerMode()
	if mode == visualizer.ModeOff {
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
	if m.visualizerVisible() {
		if mode == visualizer.ModeBackground {
			width, height = m.width, m.height
		} else if m.vizContentW > 0 {
			width, height = m.vizContentW, m.vizContentH
		}
	}
	m.visualizer.SetMode(mode)
	m.visualizer.SetDetail(vizDetail(width, height))
	m.visualizer.Resize(width, height)
	if m.tracker != nil {
		// a hidden visualizer does not need the audio level either
		m.tracker.SetVisualizer(width > 0)
	}

	// A hidden frame is not drawn; the panel keeps the last one instead of
	// repainting the screen while a dialog is open.
	if width == 0 {
		return m.scheduleVizTick(200 * time.Millisecond)
	}

	if frame, ok := m.visualizer.Latest(); ok {
		if frame.Generation != m.vizFrame.Generation {
			m.vizFrame = frame
			m.vizFrameValid = true
			m.vizFrameWidth, m.vizFrameHeight = width, height
			m.vizDirty = true
			m.vizNoop = false
		}
	}

	// A frame that took longer than the budget already slowed the worker
	// down; the tick follows it so the queue never builds up.
	period := visualizer.DefaultFramePeriod
	if m.vizFrame.RenderTime > 0 {
		if wanted := time.Duration(float64(m.vizFrame.RenderTime) / 0.015); wanted > period {
			period = wanted
		}
		if period > 250*time.Millisecond {
			period = 250 * time.Millisecond
		}
	}
	if !m.tracker.IsPlaying() {
		// a paused picture moves slowly; fewer frames are invisible
		if period < 200*time.Millisecond {
			period = 200 * time.Millisecond
		}
	}
	if mode == visualizer.ModeBackground && period < 100*time.Millisecond {
		// every background frame is a full screen of escapes: the layer is
		// dim and slow, so ten frames per second look the same
		period = 100 * time.Millisecond
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

// cycleVisualizer switches off → panel → background → off.
func (m *Model) cycleVisualizer() tea.Cmd {
	if config.Current.ResourceSaver {
		m.showToast("Сначала выключите экономию ресурсов")
		return nil
	}

	next := "panel"
	switch visualizerMode() {
	case visualizer.ModePanel:
		next = "background"
	case visualizer.ModeBackground:
		next = "off"
	}
	m.setVisualizerMode(next)
	return nil
}

// setVisualizerMode stores the mode and applies it at once.
func (m *Model) setVisualizerMode(mode string) {
	config.Current.VisualizerMode = mode
	config.Current.VisualizerEnabled = mode != "off"
	m.saveConfig()

	m.vizLayout.valid = false
	m.idleFrameValid = false

	switch mode {
	case "off":
		m.stopVisualizer()
		m.showToast("Визуализатор: выкл")
	case "background":
		m.showToast("Визуализатор: фон")
		m.startVisualizer()
	default:
		m.showToast("Визуализатор: панель")
		m.startVisualizer()
	}
}

// vizStatus is the value of the settings row.
func vizStatus() string {
	if config.Current.ResourceSaver {
		return "выкл (экономия)"
	}
	switch visualizerMode() {
	case visualizer.ModeBackground:
		return "фон"
	case visualizer.ModePanel:
		return "панель"
	}
	return "выкл"
}

//
// background layer
//

// vizLayoutCache keeps the rendered main screen and the geometry of the
// animated block. Rebuilding the whole interface twenty times per second was
// the real cost of the visualizer: the noise is cheap compared to laying out
// every row and measuring every string again.
type vizLayoutCache struct {
	valid bool

	mode visualizer.Mode

	// lines is the rendered screen split into lines, with the version label
	// already painted: the animation never touches that corner.
	lines []string
	// width is the screen width in cells.
	width int

	// panel block: the first line of the border and the byte offsets of the
	// block inside every affected line
	vizTop, vizHeight int
	vizLeft           []int
	vizRight          []int
	vizW              int

	// background layer: runs of empty cells per line
	bgRuns [][]bgRun

	// the borders of the panel block, rendered once
	panelTop, panelBottom, panelSide string
	// out is the reusable frame buffer of the fast path
	out []byte
}

// bgRun is a run of empty cells of the base view that the background layer
// paints.
type bgRun struct {
	startByte, endByte int
	startCell, cells   int
}

// layoutCacheRequest carries the rendered main screen from viewBody to View,
// where the version label is painted before the cache is stored.
type layoutCacheRequest struct {
	mode visualizer.Mode
	vizW int
	vizH int
	// vizTop is the screen row of the panel's top border, vizCell the first
	// cell of the panel. They come from the layout, not from a search: the
	// track list has the same width as the panel, so searching for a border
	// would find the wrong box.
	vizTop  int
	vizCell int
}

// invalidateLayout drops the cache. Every message except the visualizer tick
// may change the screen, so the caller resets it before such a message is
// applied.
func (m *Model) invalidateLayout() {
	m.vizLayout.valid = false
}

// cacheLayout remembers the rendered main screen and finds the animated
// block inside it. It runs after a full render, never per frame.
func (m *Model) cacheLayout(versioned string, req *layoutCacheRequest) {
	c := &m.vizLayout
	c.valid = true
	c.mode = req.mode
	c.lines = strings.Split(versioned, "\n")
	c.width = m.width
	c.vizTop, c.vizHeight, c.vizW = 0, 0, 0
	c.vizLeft, c.vizRight = nil, nil
	c.bgRuns = nil

	switch req.mode {
	case visualizer.ModeBackground:
		c.buildBackground()
	case visualizer.ModePanel:
		if req.vizW > 0 && req.vizH > 0 && req.vizTop >= 0 && req.vizTop+req.vizH+2 <= len(c.lines) {
			c.vizTop = req.vizTop
			c.vizHeight = req.vizH
			c.vizW = req.vizW
			horizontal := strings.Repeat("─", req.vizW)
			c.panelTop = style.BorderStyle.Render("╭" + horizontal + "╮")
			c.panelBottom = style.BorderStyle.Render("╰" + horizontal + "╯")
			c.panelSide = style.BorderStyle.Render("│")
			c.vizLeft = make([]int, req.vizH+2)
			c.vizRight = make([]int, req.vizH+2)
			for j := 0; j <= req.vizH+1; j++ {
				line := c.lines[req.vizTop+j]
				c.vizLeft[j] = byteOffsetAtCell(line, req.vizCell)
				c.vizRight[j] = len(line)
			}
		}
	}
}

// visibleCells counts the cells of a styled string without the escape
// sequences. Emoji and other wide runes take two cells, so the count matches
// the terminal, not the number of runes.
func visibleCells(s string) int {
	cells := 0
	for pos := 0; pos < len(s); {
		if s[pos] == 0x1b {
			pos = skipEscape(s, pos)
			continue
		}
		r, size := utf8.DecodeRuneInString(s[pos:])
		cells += runeCells(r)
		pos += size
	}
	return cells
}

// skipEscape returns the position after the escape sequence at pos.
func skipEscape(s string, pos int) int {
	pos++
	for pos < len(s) && s[pos] != 'm' && s[pos] != 'K' && s[pos] != 'J' {
		pos++
	}
	if pos < len(s) {
		pos++
	}
	return pos
}

// runeCells is the width of a rune in terminal cells. The icons of the
// interface are emoji and take two cells.
func runeCells(r rune) int {
	if r < 0x80 {
		return 1
	}
	return ansi.StringWidth(string(r))
}

// byteOffsetAtCell returns the byte offset of the given cell of a styled
// line, or the end of the line when it is shorter.
func byteOffsetAtCell(line string, cell int) int {
	if cell <= 0 {
		return 0
	}
	cells := 0
	for pos := 0; pos < len(line); {
		if cells >= cell {
			return pos
		}
		if line[pos] == 0x1b {
			pos = skipEscape(line, pos)
			continue
		}
		r, size := utf8.DecodeRuneInString(line[pos:])
		cells += runeCells(r)
		pos += size
	}
	return len(line)
}

// buildBackground finds the runs of empty cells of the base view. Text cells
// are never painted: the layer only shows through the gaps, which keeps the
// interface readable.
func (c *vizLayoutCache) buildBackground() {
	c.bgRuns = make([][]bgRun, len(c.lines))

	for i, line := range c.lines {
		var runs []bgRun
		cell := 0
		runStart := -1
		runCells := 0

		for pos := 0; pos < len(line); {
			if line[pos] == 0x1b {
				pos = skipEscape(line, pos)
				continue
			}
			r, size := utf8.DecodeRuneInString(line[pos:])
			width := runeCells(r)
			if r == ' ' {
				if runStart < 0 {
					runStart = pos
					runCells = 0
				}
				runCells++
				cell++
				pos += size
				continue
			}
			if runStart >= 0 && runCells >= bgMinRun {
				runs = append(runs, bgRun{startByte: runStart, endByte: pos, startCell: cell - runCells, cells: runCells})
			}
			runStart = -1
			cell += width
			pos += size
		}
		if runStart >= 0 && runCells >= bgMinRun {
			runs = append(runs, bgRun{startByte: runStart, endByte: len(line), startCell: cell - runCells, cells: runCells})
		}
		c.bgRuns[i] = runs
	}
}

// composeBackground paints the frame into the empty cells of the cached
// screen. Only the gaps are touched, so the layout and the text stay exactly
// as they were.
func (c *vizLayoutCache) composeBackground(frame visualizer.Frame) string {
	if frame.Width != c.width || len(frame.Cells) != frame.Width*frame.Height {
		return ""
	}
	// The rendered screen may have one more line than the terminal height
	// (a trailing empty line); the frame covers the visible rows only.
	rows := frame.Height
	if rows > len(c.lines) {
		return ""
	}

	out := c.out[:0]
	if cap(out) < len(c.lines)*c.width*2 {
		out = make([]byte, 0, len(c.lines)*c.width*2)
	}

	for i, line := range c.lines {
		if i >= rows {
			out = append(out, line...)
			if i != len(c.lines)-1 {
				out = append(out, '\n')
			}
			continue
		}
		runs := c.bgRuns[i]
		if len(runs) == 0 {
			out = append(out, line...)
		} else {
			pos := 0
			for _, run := range runs {
				out = append(out, line[pos:run.startByte]...)
				for cell := 0; cell < run.cells; cell++ {
					color := frame.Cells[i*frame.Width+run.startCell+cell]
					if dim := backgroundDim(color); dim != (visualizer.RGB{}) {
						out = append(out, "\x1b[48;2;"...)
						out = appendUint8(out, dim.R)
						out = append(out, ';')
						out = appendUint8(out, dim.G)
						out = append(out, ';')
						out = appendUint8(out, dim.B)
						out = append(out, "m \x1b[0m"...)
					} else {
						out = append(out, ' ')
					}
				}
				pos = run.endByte
			}
			out = append(out, line[pos:]...)
		}
		if i != len(c.lines)-1 {
			out = append(out, '\n')
		}
	}
	c.out = out
	return string(out)
}

// backgroundDim keeps the layer quiet: the rings must be visible but the
// text above them must stay the brightest thing on the screen.
func backgroundDim(color visualizer.RGB) visualizer.RGB {
	dim := visualizer.Scale(color, 0.5)
	// Only the glow of the rings is painted: the dark backdrop stays the
	// terminal's own background. The threshold is high on purpose — a dense
	// field of dim cells looks like noise in a terminal, while the sparse
	// glow of the rings reads as a wave behind the text.
	if int(dim.R)+int(dim.G)+int(dim.B) < 150 {
		return visualizer.RGB{}
	}
	// The layer is dim: an eight step quantization is invisible, and it lets
	// the flat parts of the picture share one escape sequence.
	quantize := func(v uint8) uint8 {
		q := (v + 4) &^ 7
		if q > 248 {
			q = 248
		}
		return q
	}
	return visualizer.RGB{R: quantize(dim.R), G: quantize(dim.G), B: quantize(dim.B)}
}

// composePanel writes the bordered panel into the cached screen. The frame
// lines are placed between the cached borders; no text is measured.
func (c *vizLayoutCache) composePanel(frame visualizer.Frame) string {
	if frame.Text == "" || frame.Width != c.vizW || frame.Height != c.vizHeight || len(c.vizLeft) != c.vizHeight+2 {
		return ""
	}
	lines := strings.Split(frame.Text, "\n")
	if len(lines) != c.vizHeight {
		return ""
	}

	out := c.out[:0]
	if cap(out) < len(c.lines)*c.width*2 {
		out = make([]byte, 0, len(c.lines)*c.width*2)
	}

	writeRow := func(row int, block string) {
		if row < 0 || row >= len(c.lines) {
			return
		}
		out = append(out, c.lines[row][:c.vizLeft[row-c.vizTop]]...)
		out = append(out, block...)
	}

	for row := 0; row < len(c.lines); row++ {
		switch {
		case row == c.vizTop:
			writeRow(row, c.panelTop)
		case row > c.vizTop && row <= c.vizTop+c.vizHeight:
			line := lines[row-c.vizTop-1]
			out = append(out, c.lines[row][:c.vizLeft[row-c.vizTop]]...)
			out = append(out, c.panelSide...)
			out = append(out, line...)
			out = append(out, c.panelSide...)
		case row == c.vizTop+c.vizHeight+1:
			writeRow(row, c.panelBottom)
		default:
			out = append(out, c.lines[row]...)
		}
		if row != len(c.lines)-1 {
			out = append(out, '\n')
		}
	}
	c.out = out
	return string(out)
}

// composeVizOnly rebuilds the screen from the cached lines. It is the fast
// path of the visualizer tick: every line that the animation does not touch
// is reused as is.
func (m *Model) composeVizOnly() (string, bool) {
	c := &m.vizLayout
	if !c.valid || !m.vizFrameValid || m.contextMenu != nil {
		return "", false
	}

	switch c.mode {
	case visualizer.ModeBackground:
		view := c.composeBackground(m.vizFrame)
		return view, view != ""
	case visualizer.ModePanel:
		if c.vizHeight == 0 {
			return "", false
		}
		view := c.composePanel(m.vizFrame)
		return view, view != ""
	}
	return "", false
}

// appendUint8 writes a decimal byte without allocations.
func appendUint8(dst []byte, v uint8) []byte {
	if v < 10 {
		return append(dst, '0'+v)
	}
	if v < 100 {
		return append(dst, '0'+v/10, '0'+v%10)
	}
	return append(dst, '0'+v/100, '0'+(v/10)%10, '0'+v%10)
}

// visualizerAnimating reports whether frames are being produced for a
// visible block. It is the condition under which a repaint of the progress
// bar can wait for the next animation frame.
func (m *Model) visualizerAnimating() bool {
	return m.visualizerVisible() && m.vizFrameValid && m.visualizerEnabled()
}
