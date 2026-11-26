package mainpage

import (
	"fmt"
	"math"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/equalizer"
	"github.com/phantom-lancer/yatui/ui/style"
)

// _EQ_SETTINGS_WIDTH is the width of the equalizer window.
const _EQ_SETTINGS_WIDTH = 90

// _EQ_SLIDER_HEIGHT is the height of one vertical fader in cells.
const _EQ_SLIDER_HEIGHT = 11

// _EQ_COL_WIDTH is the width of one fader column in cells.
const _EQ_COL_WIDTH = 6

// _EQ_COL_GAP is the gap between fader columns.
const _EQ_COL_GAP = 2

// _EQ_GRAPH_COL is the width of one band in the graphic curve.
const _EQ_GRAPH_COL = 5

// _EQ_GRAPH_AXIS is the width of the decibel axis of the curve.
const _EQ_GRAPH_AXIS = 6

// _EQ_GRAPH_HEIGHT is the preferred height of the curve, in cells. Small
// terminals get a shorter curve, never less than _EQ_GRAPH_MIN_HEIGHT.
const _EQ_GRAPH_HEIGHT = 12

// _EQ_GRAPH_MIN_HEIGHT is the smallest readable height of the curve.
const _EQ_GRAPH_MIN_HEIGHT = 5

// eqSlider is one vertical fader (or one band column of the curve) of the
// equalizer.
type eqSlider struct {
	band int
	x    int // screen x of the fader column centre
	y    int // screen y of the first fader row
	w    int // width of the fader column
	h    int // height of the fader column
}

// eqLayout is where the rendered dialog ended up on the screen. The geometry
// is computed from fixed constants, never parsed from text, so the mouse
// hitboxes always match what the user sees.
type eqLayout struct {
	x, y          int
	width, height int
	// toggleRect is the area of the on/off switch text.
	toggleRect Rect
	// viewRect is the area of the classic/graphic view switch.
	viewRect Rect
	// presetRect is the area of the preset selector row.
	presetRect Rect
	// applyRect is the area of the apply button.
	applyRect Rect
	// sliders are the faders, left to right.
	sliders []eqSlider
	// graphTop is the first row of the curve.
	graphTop int
	// graphHeight is the height of the curve.
	graphHeight int
	valid       bool
}

// sliderAt returns the index of the fader under the given screen coordinates,
// or -1 when there is none.
func (l eqLayout) sliderAt(x, y int) int {
	for i, s := range l.sliders {
		half := s.w / 2
		if half < 2 {
			half = 2
		}
		if x >= s.x-half && x <= s.x+half && y >= s.y && y < s.y+s.h {
			return i
		}
	}
	return -1
}

// eqModel is the window with the equalizer.
//
// index is the control under the cursor keys: -1 for the on/off toggle, 0
// for the preset selector, 1+band for a band and 1+bands for the apply
// button.
//
// graphic selects the fifteen band curve; classic keeps the original ten
// faders. The two sets of gains are independent and are both persisted.
//
// dragging is the band the mouse is moving, -1 when nothing is dragged.
type eqModel struct {
	enabled       bool
	preset        string
	gains         [equalizer.Bands]float64
	graphic       bool
	graphicPreset string
	graphicGains  [equalizer.GraphicBands]float64
	index         int
	dragging      int
	graphHeight   int
	layout        eqLayout

	// The geometry follows the terminal: a narrow window uses thinner
	// columns and a smaller gap instead of overflowing the screen.
	metricsReady bool
	colW, colGap int
	graphCol     int
}

// prepareMetrics fits the equalizer into the terminal. The classic view needs
// ten columns, the graphic one fifteen, so the widths are chosen separately;
// the same values are used by the view and by the mouse hitboxes.
func (m *eqModel) prepareMetrics(screenWidth int) {
	if screenWidth <= 0 {
		screenWidth = _EQ_SETTINGS_WIDTH
	}

	labelW := labelWidthEQ()
	// border (2) + horizontal padding (4) + the two spaces after the label
	overhead := labelW + 2 + 6

	col, gap := _EQ_COL_WIDTH, _EQ_COL_GAP
	for col > 2 && equalizer.Bands*col+(equalizer.Bands-1)*gap > screenWidth-overhead {
		if gap > 0 {
			gap--
		} else {
			col--
		}
	}

	graphCol := _EQ_GRAPH_COL
	for graphCol > 2 && equalizer.GraphicBands*graphCol > screenWidth-overhead-_EQ_GRAPH_AXIS {
		graphCol--
	}

	m.colW, m.colGap, m.graphCol = col, gap, graphCol
	m.metricsReady = true
}

func (m *eqModel) classicColumns() (int, int) {
	if !m.metricsReady || m.colW <= 0 {
		return _EQ_COL_WIDTH, _EQ_COL_GAP
	}
	return m.colW, m.colGap
}

func (m *eqModel) graphColumns() int {
	if !m.metricsReady || m.graphCol <= 0 {
		return _EQ_GRAPH_COL
	}
	return m.graphCol
}

// bands returns the number of the bands of the active view.
func (m *eqModel) bands() int {
	if m.graphic {
		return equalizer.GraphicBands
	}
	return equalizer.Bands
}

// applyIndex is the index of the apply button of the active view.
func (m *eqModel) applyIndex() int {
	return 1 + m.bands()
}

// openEqualizer shows the window with the equalizer. The settings come from
// the config, not from the player, so the window always shows what was saved.
func (m *Model) openEqualizer() tea.Cmd {
	eqCfg := config.Current.Equalizer
	if eqCfg == nil {
		eqCfg = &config.Equalizer{}
	}

	graphic := equalizer.NormalizeGraphicGains(eqCfg.GraphicGains)
	if len(eqCfg.GraphicGains) != equalizer.GraphicBands {
		legacy := equalizer.NormalizeGains(eqCfg.Gains)
		graphic = equalizer.ResampleGains(
			equalizer.BandFrequencies[:],
			legacy[:],
			equalizer.GraphicFrequencies[:],
		)
	}

	preset := eqCfg.GraphicPreset
	if preset == "" {
		preset = eqCfg.Preset
	}
	if preset == "" {
		preset = "Плоский"
	}

	m.eq = &eqModel{
		enabled:       eqCfg.Enabled,
		preset:        eqCfg.Preset,
		gains:         equalizer.NormalizeGains(eqCfg.Gains),
		graphic:       !eqCfg.ClassicView,
		graphicPreset: preset,
		graphicGains:  graphic,
		index:         -1,
	}
	return tea.ClearScreen
}

// applySavedEQ pushes the saved settings to the player: without it the
// equalizer was flat after every restart, and the window showed nothing.
func (m *Model) applySavedEQ() {
	eqCfg := config.Current.Equalizer
	if eqCfg == nil || m.tracker == nil {
		return
	}

	if eqCfg.ClassicView {
		m.tracker.SetEQMode(false)
		m.tracker.SetEQ(eqCfg.Enabled, equalizer.NormalizeGains(eqCfg.Gains))
		return
	}

	graphic := equalizer.NormalizeGraphicGains(eqCfg.GraphicGains)
	if len(eqCfg.GraphicGains) != equalizer.GraphicBands {
		legacy := equalizer.NormalizeGains(eqCfg.Gains)
		graphic = equalizer.ResampleGains(
			equalizer.BandFrequencies[:],
			legacy[:],
			equalizer.GraphicFrequencies[:],
		)
	}
	m.tracker.SetEQMode(true)
	m.tracker.SetGraphicEQ(eqCfg.Enabled, graphic)
}

// toggleEqualizer switches the equalizer on or off.
func (m *Model) toggleEqualizer() tea.Cmd {
	if m.eq == nil {
		if config.Current.Equalizer == nil {
			config.Current.Equalizer = &config.Equalizer{}
		}
		eqCfg := config.Current.Equalizer
		m.eq = &eqModel{
			enabled:       !eqCfg.Enabled,
			preset:        eqCfg.Preset,
			gains:         equalizer.NormalizeGains(eqCfg.Gains),
			graphic:       !eqCfg.ClassicView,
			graphicPreset: eqCfg.GraphicPreset,
			graphicGains:  equalizer.NormalizeGraphicGains(eqCfg.GraphicGains),
			index:         -1,
		}
	} else {
		m.eq.enabled = !m.eq.enabled
		m.eq.index = -1
	}
	m.applyEQ()
	return nil
}

// applyEQ pushes the current state of the window to the player immediately:
// every knob move changes the sound right away, no "Apply" click needed.
func (m *Model) applyEQ() {
	if m.eq == nil {
		return
	}
	if config.Current.Equalizer == nil {
		config.Current.Equalizer = &config.Equalizer{}
	}

	eqCfg := config.Current.Equalizer
	eqCfg.Enabled = m.eq.enabled
	eqCfg.ClassicView = !m.eq.graphic
	if m.eq.graphic {
		eqCfg.GraphicPreset = m.eq.graphicPreset
		eqCfg.GraphicGains = append([]float64(nil), m.eq.graphicGains[:]...)
	} else {
		eqCfg.Preset = m.eq.preset
		eqCfg.Gains = append([]float64(nil), m.eq.gains[:]...)
	}
	// dragging a fader produces a value per mouse event; the delayed write
	// keeps the disk quiet and the audio thread does not wait for it
	config.SaveLater()

	if m.tracker == nil {
		return
	}
	if m.eq.graphic {
		m.tracker.SetEQMode(true)
		m.tracker.SetGraphicEQ(m.eq.enabled, m.eq.graphicGains)
	} else {
		m.tracker.SetEQMode(false)
		m.tracker.SetEQ(m.eq.enabled, m.eq.gains)
	}
}

// toggleEQView switches between the graphic curve and the classic faders.
// The gains of both views stay in memory and in the config, so switching
// never resets the equalizer.
func (m *Model) toggleEQView() {
	if m.eq == nil {
		return
	}
	m.eq.graphic = !m.eq.graphic
	m.eq.index = -1
	m.eq.dragging = -1
	m.applyEQ()
}

// presetList returns the names of the items the classic preset selector shows:
//
// "Своя настройка" means the gains were edited by hand, the rest are the
// ready made presets.
func presetList() []string {
	names := make([]string, 0, len(equalizer.Presets())+1)
	names = append(names, equalizer.CustomPreset)
	for _, preset := range equalizer.Presets() {
		names = append(names, preset.Name)
	}
	return names
}

// presetIndex returns the index of the preset with the given name in
// presetList.
func presetIndex(name string) int {
	for i, n := range presetList() {
		if n == name {
			return i
		}
	}
	return 0
}

// graphicPresetList returns the names of the preset selector of the graphic
// view.
func graphicPresetList() []string {
	names := make([]string, 0, len(equalizer.GraphicPresets())+1)
	names = append(names, equalizer.CustomPreset)
	for _, preset := range equalizer.GraphicPresets() {
		names = append(names, preset.Name)
	}
	return names
}

func graphicPresetIndex(name string) int {
	for i, n := range graphicPresetList() {
		if n == name {
			return i
		}
	}
	return 0
}

// sliderValue converts a gain in decibels to the row of its fader handle.
func sliderValue(gain float64) int {
	row := int(math.Round((12 - gain) / 24 * float64(_EQ_SLIDER_HEIGHT-1)))
	if row < 0 {
		row = 0
	}
	if row > _EQ_SLIDER_HEIGHT-1 {
		row = _EQ_SLIDER_HEIGHT - 1
	}
	return row
}

// sliderGain converts the row of a fader handle back to a gain in decibels.
func sliderGain(row int) float64 {
	gain := 12 - float64(row)*24/float64(_EQ_SLIDER_HEIGHT-1)
	return equalizer.ClampGain(gain)
}

// graphGain converts the row of the curve to a gain in decibels.
func graphGain(row, height int) float64 {
	if height <= 1 {
		return 0
	}
	gain := 12 - float64(row)*24/float64(height-1)
	return equalizer.ClampGain(gain)
}

// graphRow converts a gain in decibels to the row of the curve.
func graphRow(gain float64, height int) int {
	if height <= 1 {
		return 0
	}
	row := int(math.Round((12 - gain) / 24 * float64(height-1)))
	if row < 0 {
		row = 0
	}
	if row > height-1 {
		row = height - 1
	}
	return row
}

// sliderColumn renders one vertical fader.
//
// The column is _EQ_COL_WIDTH cells wide so the mouse has a big hitbox: the
// track is "│", the middle is "─", the handle is "●". When active the fader
// is painted in the accent color.
func sliderColumn(gain float64, active bool, colWidth int) string {
	knob := sliderValue(gain)

	lines := make([]string, _EQ_SLIDER_HEIGHT)
	for row := 0; row < _EQ_SLIDER_HEIGHT; row++ {
		var c string
		switch {
		case row == knob:
			c = "●"
		case row == _EQ_SLIDER_HEIGHT/2:
			c = "─"
		default:
			c = "│"
		}

		stl := lipgloss.NewStyle().Width(1).Align(lipgloss.Center)
		if active {
			stl = stl.Foreground(style.AccentColor)
		} else {
			stl = stl.Foreground(style.InactiveTextColor)
		}
		lines[row] = stl.Render(c)
	}

	// centre the single-cell column inside _EQ_COL_WIDTH cells
	out := make([]string, _EQ_SLIDER_HEIGHT)
	for row := 0; row < _EQ_SLIDER_HEIGHT; row++ {
		out[row] = padCenterEQ(lines[row], colWidth)
	}
	return lipgloss.JoinVertical(lipgloss.Center, out...)
}

// padCenterEQ centers a string in the given width.
func padCenterEQ(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return s
	}
	pad := width - lipgloss.Width(s)
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// labelWidthEQ is the width of the column with the names of the controls.
func labelWidthEQ() int {
	width := lipgloss.Width("Эквалайзер")
	if w := lipgloss.Width("Пресет"); w > width {
		width = w
	}
	return width
}

// gainAtColumn interpolates the curve between the band centres, in the log
// frequency domain, so the drawn line follows the filters instead of
// stepping between the bands.
func (m *eqModel) gainAtColumn(column int) float64 {
	colWidth := m.graphColumns()
	centres := equalizer.GraphicBands * colWidth
	position := (float64(column) + 0.5) / float64(centres) * float64(equalizer.GraphicBands-1)
	low := int(math.Floor(position))
	if low < 0 {
		low = 0
	}
	if low >= equalizer.GraphicBands-1 {
		return m.graphicGains[equalizer.GraphicBands-1]
	}
	// the column position is already linear in the log frequency domain, so
	// the curve follows the filters instead of stepping between them
	t := position - float64(low)
	return m.graphicGains[low] + (m.graphicGains[low+1]-m.graphicGains[low])*t
}

// graphicCurve renders the fifteen band curve: the line, the band points,
// the 0 dB reference and the decibel axis.
func (m *eqModel) graphicCurve(height int) string {
	if height < _EQ_GRAPH_MIN_HEIGHT {
		height = _EQ_GRAPH_MIN_HEIGHT
	}

	colWidth := m.graphColumns()
	width := equalizer.GraphicBands * colWidth
	zeroRow := graphRow(0, height)

	// precompute the row of the curve for every column
	curveRows := make([]int, width)
	for column := 0; column < width; column++ {
		curveRows[column] = graphRow(m.gainAtColumn(column), height)
	}

	axis := make([]string, height)
	for row := 0; row < height; row++ {
		gain := 12 - float64(row)*24/float64(height-1)
		label := ""
		switch {
		case row == 0:
			label = "+12"
		case row == height-1:
			label = "-12"
		case row == zeroRow:
			label = "  0"
		case height >= 9 && row == height/4:
			label = " +6"
		case height >= 9 && row == height*3/4:
			label = " -6"
		}
		axis[row] = lipgloss.NewStyle().
			Width(_EQ_GRAPH_AXIS-2).
			Align(lipgloss.Right).
			Foreground(style.InactiveTextColor).
			Render(label) + " " + lipgloss.NewStyle().Foreground(style.InactiveTextColor).Render("┤")
		_ = gain
	}

	lines := make([]string, height)
	for row := 0; row < height; row++ {
		var b strings.Builder
		b.WriteString(axis[row])
		for column := 0; column < width; column++ {
			band := column / colWidth
			centre := column == band*colWidth+colWidth/2
			onCurve := curveRows[column] == row
			active := m.index == 1+band

			switch {
			case centre && onCurve:
				glyph := "●"
				if active {
					b.WriteString(lipgloss.NewStyle().Foreground(style.AccentColor).Bold(true).Render(glyph))
				} else {
					b.WriteString(lipgloss.NewStyle().Foreground(style.ActiveTextColor).Render(glyph))
				}
			case centre && row == zeroRow:
				b.WriteString(lipgloss.NewStyle().Foreground(style.InactiveTextColor).Render("┼"))
			case centre:
				b.WriteString(lipgloss.NewStyle().Foreground(style.InactiveTextColor).Render("┊"))
			case onCurve:
				glyph := "─"
				if active {
					b.WriteString(lipgloss.NewStyle().Foreground(style.AccentColor).Render(glyph))
				} else {
					b.WriteString(lipgloss.NewStyle().Foreground(style.ActiveTextColor).Render(glyph))
				}
			case row == zeroRow:
				b.WriteString(lipgloss.NewStyle().Foreground(style.InactiveTextColor).Render("·"))
			default:
				b.WriteString(" ")
			}
		}
		lines[row] = b.String()
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// eqView renders the window with the equalizer.
func (m *Model) eqView() string {
	m.eq.prepareMetrics(m.width)
	mode := "графический"
	if !m.eq.graphic {
		mode = "классический"
	}
	title := style.DialogTitleStyle.Render("Эквалайзер · " + mode)

	labelWidth := labelWidthEQ()

	// the toggle row: label on the left, the switch on the right
	toggle := "выкл"
	if m.eq.enabled {
		toggle = "вкл"
	}
	toggleStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(style.NormalTextColor)
	if m.eq.index == -1 {
		toggleStyle = toggleStyle.Foreground(style.InactiveTextColor).Background(style.AccentColor)
	}
	toggleRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Width(labelWidth).Render("Эквалайзер"),
		"  ",
		toggleStyle.Render(toggle),
	)

	// the view row: the same switch is repeated in the settings, here it is
	// one click away, next to the thing it changes
	viewStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(style.NormalTextColor)
	viewHint := "v — переключить"
	if m.eq.layout.valid {
		viewHint = "v — переключить"
	}
	viewRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Width(labelWidth).Render("Вид"),
		"  ",
		viewStyle.Render(mode),
		"  ",
		style.DialogHelpStyle.Padding(0).Render(viewHint),
	)

	// the preset row
	preset := m.eq.preset
	presets := presetList()
	if m.eq.graphic {
		preset = m.eq.graphicPreset
		presets = graphicPresetList()
	}
	if preset == "" {
		preset = "Плоский"
	}
	presetStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(style.NormalTextColor)
	if m.eq.index == 0 {
		presetStyle = presetStyle.Foreground(style.InactiveTextColor).Background(style.AccentColor)
	}
	presetNumber := 1
	for i, n := range presets {
		if n == preset {
			presetNumber = i + 1
			break
		}
	}
	presetRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Width(labelWidth).Render("Пресет"),
		"  ",
		presetStyle.Render(preset),
		"  ",
		style.DialogHelpStyle.Padding(0).Render(fmt.Sprintf("%d/%d", presetNumber, len(presets))),
	)

	if m.eq.graphic {
		return m.graphicEQView(title, labelWidth, toggleRow, viewRow, presetRow)
	}
	return m.classicEQView(title, labelWidth, toggleRow, viewRow, presetRow)
}

// classicEQView renders the original ten faders.
func (m *Model) classicEQView(title string, labelWidth int, toggleRow, viewRow, presetRow string) string {
	colWidth, gap := m.eq.classicColumns()

	// the level labels above the faders
	topLine := strings.Repeat(" ", labelWidth+2)
	for i := 0; i < equalizer.Bands; i++ {
		topLine += padCenterEQ("+12", colWidth) + strings.Repeat(" ", gap)
	}
	topLine = strings.TrimRight(topLine, " ")

	// the fader columns: every column is rendered to its own set of lines,
	// then the rows are joined side by side
	cols := make([][]string, equalizer.Bands)
	for i := 0; i < equalizer.Bands; i++ {
		cols[i] = strings.Split(sliderColumn(m.eq.gains[i], m.eq.index == 1+i, colWidth), "\n")
	}
	midLines := make([]string, _EQ_SLIDER_HEIGHT)
	for row := 0; row < _EQ_SLIDER_HEIGHT; row++ {
		line := strings.Repeat(" ", labelWidth+2)
		for i := 0; i < equalizer.Bands; i++ {
			line += cols[i][row] + strings.Repeat(" ", gap)
		}
		midLines[row] = strings.TrimRight(line, " ")
	}

	// the frequency labels below the faders
	botLine := strings.Repeat(" ", labelWidth+2)
	for i := 0; i < equalizer.Bands; i++ {
		botLine += padCenterEQ(equalizer.BandLabels[i], colWidth) + strings.Repeat(" ", gap)
	}
	botLine = strings.TrimRight(botLine, " ")

	// the level labels at the bottom
	lvlLine := strings.Repeat(" ", labelWidth+2)
	for i := 0; i < equalizer.Bands; i++ {
		if i == 0 {
			lvlLine += padCenterEQ("-12дБ", colWidth)
		} else if i == equalizer.Bands/2 {
			lvlLine += padCenterEQ("0дБ", colWidth)
		} else if i == equalizer.Bands-1 {
			lvlLine += padCenterEQ("+12дБ", colWidth)
		} else {
			lvlLine += strings.Repeat(" ", colWidth)
		}
		lvlLine += strings.Repeat(" ", gap)
	}
	lvlLine = strings.TrimRight(lvlLine, " ")

	faders := lipgloss.JoinVertical(lipgloss.Left,
		topLine,
		lipgloss.JoinVertical(lipgloss.Left, midLines...),
		botLine,
		lvlLine,
	)
	faderBlock := lipgloss.NewStyle().Padding(0, 1).Render(faders)

	rows := make([]string, 0, 5+equalizer.Bands)
	rows = append(rows, toggleRow, viewRow, "", presetRow, faderBlock, "")

	// the apply button
	rows = append(rows, m.eqApplyButton())

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)

	box := style.DialogBoxStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		body,
	))

	return lipgloss.JoinVertical(
		lipgloss.Center,
		box,
		style.DialogHelpStyle.Render("↑/↓ — выбор, ←/→ — уровень, v — вид, мышь — крутить ползунок, изменения применяются сразу, esc — выход"),
	)
}

// graphicEQView renders the fifteen band curve.
func (m *Model) graphicEQView(title string, labelWidth int, toggleRow, viewRow, presetRow string) string {
	height := _EQ_GRAPH_HEIGHT
	if m.height > 0 {
		// title, help, paddings and the rows around the curve take about 12
		// lines; a small terminal gets a shorter curve instead of a dialog
		// that does not fit
		available := m.height - 14
		if available < height {
			height = available
		}
	}
	if height < _EQ_GRAPH_MIN_HEIGHT {
		height = _EQ_GRAPH_MIN_HEIGHT
	}
	m.eq.graphHeight = height

	band := m.eq.index - 1
	if band < 0 || band >= equalizer.GraphicBands {
		band = 0
	}
	readout := fmt.Sprintf("Полоса %s Гц: %+.1f дБ (Q %.1f)", equalizer.GraphicLabels[band], m.eq.graphicGains[band], equalizer.GraphicQ)
	if max := m.width - 8; max > 8 && ansi.StringWidth(readout) > max {
		readout = ansi.Truncate(readout, max, "…")
	}
	readoutRow := style.DialogHelpStyle.Padding(0).Render(readout)

	curve := m.eq.graphicCurve(height)
	curveBlock := lipgloss.NewStyle().Padding(0, 1).Render(curve)

	graphCol := m.eq.graphColumns()
	labels := strings.Repeat(" ", labelWidth+2) + strings.Repeat(" ", _EQ_GRAPH_AXIS)
	for i := 0; i < equalizer.GraphicBands; i++ {
		labels += padCenterEQ(equalizer.GraphicLabels[i], graphCol)
	}
	labels = strings.TrimRight(labels, " ")

	rows := make([]string, 0, 9)
	rows = append(rows, toggleRow, viewRow, presetRow, readoutRow, "")
	rows = append(rows, curveBlock, labels, "")
	rows = append(rows, m.eqApplyButton())

	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	box := style.DialogBoxStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		body,
	))

	return lipgloss.JoinVertical(
		lipgloss.Center,
		box,
		style.DialogHelpStyle.Render("←/→ — полоса, ↑/↓ — уровень, v — вид, мышь — тянуть точку, изменения применяются сразу, esc — выход"),
	)
}

func (m *Model) eqApplyButton() string {
	applyStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(style.NormalTextColor).
		Background(style.InactiveTextColor).
		Width(16).
		Align(lipgloss.Center)
	if m.eq.index == m.eq.applyIndex() {
		applyStyle = applyStyle.Foreground(style.InactiveTextColor).Background(style.AccentColor)
	}
	return lipgloss.NewStyle().AlignHorizontal(lipgloss.Center).Render(applyStyle.Render("Применить"))
}

// placeEqualizer draws the window in the middle of the screen and computes
// the geometry of every control from fixed constants, so the mouse hitboxes
// always match the rendered view.
func (m *Model) placeEqualizer() string {
	view := m.eqView()
	placed := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, view)

	width := lipgloss.Width(view)
	height := lipgloss.Height(view)
	x := (m.width - width) / 2
	y := (m.height - height) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	labelW := labelWidthEQ()

	// horizontal geometry is fixed and well tested
	const contentPad = 3 // border left(1) + padding left(2)
	toggleX := x + contentPad + labelW + 2
	// the fader block has its own left padding of 1 cell
	sliderStartX := x + contentPad + 1 + labelW + 2

	// vertical geometry is measured from the rendered text, so it follows the
	// real layout (title margins, spacer rows) automatically.
	lines := strings.Split(view, "\n")
	toggleY := -1
	viewY := -1
	presetY := -1
	applyY := -1
	freqY := -1
	graphY := -1
	for i, line := range lines {
		strip := ansi.Strip(line)
		if toggleY < 0 {
			if idx := strings.Index(strip, "выкл"); idx >= 0 {
				toggleX = x + lipgloss.Width(strip[:idx])
				toggleY = y + i
			} else if idx := strings.Index(strip, "вкл"); idx >= 0 {
				toggleX = x + lipgloss.Width(strip[:idx])
				toggleY = y + i
			}
		}
		if viewY < 0 && strings.Contains(strip, "Вид") {
			viewY = y + i
		}
		if presetY < 0 && strings.Contains(strip, "Пресет") {
			presetY = y + i
		}
		if freqY < 0 && strings.Contains(strip, "16к") {
			freqY = y + i
		}
		if !m.eq.graphic && freqY < 0 && strings.Contains(strip, "60") {
			freqY = y + i
		}
		if graphY < 0 && strings.Contains(strip, "+12") {
			graphY = y + i
		}
		if applyY < 0 && strings.Contains(strip, "Применить") {
			applyY = y + i
		}
	}

	// fallbacks for the unlikely case the text is not found
	if toggleY < 0 {
		toggleY = y + 4
	}
	if viewY < 0 {
		viewY = toggleY + 1
	}
	if presetY < 0 {
		presetY = viewY + 1
	}
	if applyY < 0 {
		applyY = y + height - 3
	}

	var sliders []eqSlider
	graphTop := 0
	graphHeight := 0
	if m.eq.graphic {
		graphHeight = m.eq.graphHeight
		if graphHeight <= 0 {
			graphHeight = _EQ_GRAPH_HEIGHT
		}
		if graphY < 0 {
			graphY = presetY + 3
		}
		graphTop = graphY
		graphCol := m.eq.graphColumns()
		for i := 0; i < equalizer.GraphicBands; i++ {
			centre := x + contentPad + 1 + labelW + 2 + _EQ_GRAPH_AXIS + i*graphCol + graphCol/2
			sliders = append(sliders, eqSlider{
				band: i,
				x:    centre,
				y:    graphTop,
				w:    graphCol,
				h:    graphHeight,
			})
		}
	} else {
		colWidth, gap := m.eq.classicColumns()
		sliderY := y + 8
		if freqY >= 0 {
			sliderY = freqY - _EQ_SLIDER_HEIGHT
		}
		for i := 0; i < equalizer.Bands; i++ {
			centre := sliderStartX + i*(colWidth+gap) + colWidth/2
			sliders = append(sliders, eqSlider{
				band: i,
				x:    centre,
				y:    sliderY,
				w:    colWidth,
				h:    _EQ_SLIDER_HEIGHT,
			})
		}
	}

	m.eq.layout = eqLayout{
		x:           x,
		y:           y,
		width:       width,
		height:      height,
		toggleRect:  Rect{X: toggleX, Y: toggleY, Width: 16, Height: 1},
		viewRect:    Rect{X: x + contentPad + labelW + 2, Y: viewY, Width: 24, Height: 1},
		presetRect:  Rect{X: x, Y: presetY, Width: width, Height: 1},
		applyRect:   Rect{X: x, Y: applyY, Width: width, Height: 1},
		sliders:     sliders,
		graphTop:    graphTop,
		graphHeight: graphHeight,
		valid:       true,
	}

	return placed
}

// eqControl handles the keys and the mouse while the window is open. The
// second return value says whether the window swallowed the event.
func (m *Model) eqControl(message tea.Msg) (tea.Cmd, bool) {
	if m.eq == nil {
		return nil, false
	}

	switch msg := message.(type) {
	case tea.KeyMsg:
		keypress := msg.String()

		switch keypress {
		case "esc":
			m.eq = nil
			return tea.ClearScreen, true
		case "v", "V":
			m.toggleEQView()
			return tea.ClearScreen, true
		case "t", "T":
			m.eq.enabled = !m.eq.enabled
			m.applyEQ()
			return nil, true
		case "up":
			m.eq.index--
			if m.eq.index < -1 {
				m.eq.index = m.eq.applyIndex()
			}
			return nil, true
		case "down":
			m.eq.index++
			if m.eq.index > m.eq.applyIndex() {
				m.eq.index = -1
			}
			return nil, true
		case "left", "right":
			return m.eqChange(keypress == "right"), true
		case "enter", " ":
			if m.eq.index == -1 {
				m.eq.enabled = !m.eq.enabled
				m.applyEQ()
				return nil, true
			}
			if m.eq.index == m.eq.applyIndex() {
				m.applyEQ()
				return nil, true
			}
			return nil, true
		}

		// the window swallows every other key
		return nil, true

	case tea.MouseMsg:
		return m.eqMouse(msg), true
	}

	return nil, false
}

// eqChange changes the value of the selected control: dir is 1 for "right",
// -1 for "left".
func (m *Model) eqChange(right bool) tea.Cmd {
	dir := 1
	if !right {
		dir = -1
	}

	switch m.eq.index {
	case -1:
		// the toggle
		m.eq.enabled = !m.eq.enabled
		m.applyEQ()
		return nil

	case 0:
		// the preset
		if m.eq.graphic {
			presets := graphicPresetList()
			index := graphicPresetIndex(m.eq.graphicPreset)
			index = (index + dir + len(presets)) % len(presets)
			name := presets[index]
			m.eq.graphicPreset = name
			m.eq.enabled = true
			if name != equalizer.CustomPreset {
				if preset, ok := equalizer.GraphicPresetByName(name); ok {
					m.eq.graphicGains = preset.Gains
				}
			}
			m.applyEQ()
			return nil
		}

		presets := presetList()
		index := presetIndex(m.eq.preset)
		index = (index + dir + len(presets)) % len(presets)
		name := presets[index]
		m.eq.preset = name
		m.eq.enabled = true
		if name != equalizer.CustomPreset {
			if preset, ok := equalizer.PresetByName(name); ok {
				m.eq.gains = preset.Gains
			}
		}
		m.applyEQ()
		return nil

	default:
		// a band
		band := m.eq.index - 1
		if band < 0 || band >= m.eq.bands() {
			return nil
		}
		m.eq.enabled = true
		if m.eq.graphic {
			m.eq.graphicPreset = equalizer.CustomPreset
			m.eq.graphicGains[band] = equalizer.ClampGain(m.eq.graphicGains[band] + float64(dir))
		} else {
			m.eq.preset = equalizer.CustomPreset
			m.eq.gains[band] = equalizer.ClampGain(m.eq.gains[band] + float64(dir))
		}
		m.applyEQ()
		return nil
	}
}

// eqSetBand sets the gain of one band from a zero based row of the curve.
func (m *Model) eqSetBand(band, row int) {
	if band < 0 || band >= m.eq.bands() {
		return
	}
	height := m.eq.graphHeight
	if height <= 0 {
		height = _EQ_GRAPH_HEIGHT
	}
	if row < 0 {
		row = 0
	}
	if row > height-1 {
		row = height - 1
	}

	m.eq.enabled = true
	if m.eq.graphic {
		m.eq.graphicPreset = equalizer.CustomPreset
		m.eq.graphicGains[band] = graphGain(row, height)
	} else {
		m.eq.preset = equalizer.CustomPreset
		if row > _EQ_SLIDER_HEIGHT-1 {
			row = _EQ_SLIDER_HEIGHT - 1
		}
		m.eq.gains[band] = sliderGain(row)
	}
	m.applyEQ()
}

// eqMouse handles the mouse while the window is open.
func (m *Model) eqMouse(msg tea.MouseMsg) tea.Cmd {
	if m.eq == nil || !m.eq.layout.valid {
		return nil
	}
	layout := m.eq.layout

	// the view switch is not part of the index navigation; it is a plain
	// click target in both views
	if msg.Action == tea.MouseActionPress &&
		(msg.Button == tea.MouseButtonLeft || msg.Button == tea.MouseButtonMiddle) &&
		layout.viewRect.Contains(msg.X, msg.Y) {
		m.toggleEQView()
		return tea.ClearScreen
	}

	switch msg.Action {
	case tea.MouseActionMotion:
		// dragging a band: once the press started on a band, keep dragging.
		// Some terminals report Button == None for motion, so we don't check it.
		if m.eq.dragging >= 0 && m.eq.dragging < m.eq.bands() {
			band := m.eq.dragging
			if idx := layout.sliderAt(msg.X, msg.Y); idx >= 0 {
				band = idx
				m.eq.dragging = band
			}
			s := layout.sliders[band]
			m.eqSetBand(band, msg.Y-s.y)
			return nil
		}
		// hover: highlight the control under the cursor
		if idx := layout.sliderAt(msg.X, msg.Y); idx >= 0 {
			if m.eq.index != 1+idx {
				m.eq.index = 1 + idx
			}
		} else if layout.toggleRect.Contains(msg.X, msg.Y) {
			if m.eq.index != -1 {
				m.eq.index = -1
			}
		} else if layout.presetRect.Contains(msg.X, msg.Y) {
			if m.eq.index != 0 {
				m.eq.index = 0
			}
		} else if layout.applyRect.Contains(msg.X, msg.Y) {
			if m.eq.index != m.eq.applyIndex() {
				m.eq.index = m.eq.applyIndex()
			}
		}

	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft && msg.Button != tea.MouseButtonMiddle {
			break
		}

		// the toggle
		if layout.toggleRect.Contains(msg.X, msg.Y) {
			m.eq.enabled = !m.eq.enabled
			m.eq.index = -1
			m.applyEQ()
			return nil
		}

		// a band: click sets the level, press-and-hold drags it
		if idx := layout.sliderAt(msg.X, msg.Y); idx >= 0 {
			m.eq.dragging = idx
			m.eq.index = 1 + idx
			s := layout.sliders[idx]
			m.eqSetBand(idx, msg.Y-s.y)
			return nil
		}

		// the preset row
		if layout.presetRect.Contains(msg.X, msg.Y) {
			m.eq.index = 0
			m.eqChange(msg.X > layout.x+layout.width/2)
			return nil
		}

		// the apply button
		if layout.applyRect.Contains(msg.X, msg.Y) {
			m.eq.index = m.eq.applyIndex()
			m.applyEQ()
			return nil
		}

		// a click outside of the window closes it
		if msg.X < layout.x || msg.X >= layout.x+layout.width || msg.Y < layout.y || msg.Y >= layout.y+layout.height {
			m.eq = nil
			return tea.ClearScreen
		}

	case tea.MouseActionRelease:
		m.eq.dragging = -1
	}

	return nil
}
