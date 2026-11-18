package mainpage

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/phantom-lancer/yatui/config"
)

const winterFrameInterval = 33 * time.Millisecond

type winterTickMsg struct {
	generation uint64
}

func winterActive() bool {
	return config.Current.Style != nil && config.Current.Style.Theme == "winter"
}

func winterIntensity() int {
	switch {
	case config.Current.WinterIntensity < 0:
		return 0
	case config.Current.WinterIntensity > 3:
		return 3
	default:
		return config.Current.WinterIntensity
	}
}

func winterMotionEnabled() bool {
	return winterActive() && (winterIntensity() > 0 || config.Current.WinterGarland)
}

func winterTick(generation uint64) tea.Cmd {
	return tea.Tick(winterFrameInterval, func(time.Time) tea.Msg {
		return winterTickMsg{generation: generation}
	})
}

// winterStart arms one low-frequency frame timer only while the winter theme
// has visible effects enabled. There is no animation goroutine or idle polling.
func (m *Model) winterStart() tea.Cmd {
	if !winterMotionEnabled() {
		m.winterStop()
		return nil
	}
	if m.winterRunning {
		return nil
	}
	m.winterGeneration++
	m.winterRunning = true
	return winterTick(m.winterGeneration)
}

func (m *Model) winterStop() {
	m.winterGeneration++
	m.winterRunning = false
}

func (m *Model) handleWinterTick(msg winterTickMsg) tea.Cmd {
	if msg.generation != m.winterGeneration || !m.winterRunning {
		return nil
	}
	if !winterMotionEnabled() {
		m.winterStop()
		return nil
	}
	m.winterFrame++
	m.idleFrameValid = false
	return winterTick(m.winterGeneration)
}

func (m *Model) applyWinterIntensity(value int) tea.Cmd {
	if value < 0 {
		value = 0
	}
	if value > 3 {
		value = 3
	}
	config.Current.WinterIntensity = value
	m.saveConfig()
	m.winterFrame = 0
	m.idleFrameValid = false
	m.showToast("Снег: " + winterIntensityLabel(value))
	return m.winterStart()
}

func winterIntensityLabel(value int) string {
	switch value {
	case 0:
		return "выкл"
	case 1:
		return "слабо"
	case 2:
		return "средне"
	default:
		return "сильно"
	}
}

func (m *Model) toggleWinterGarland() tea.Cmd {
	config.Current.WinterGarland = !config.Current.WinterGarland
	m.saveConfig()
	m.idleFrameValid = false
	m.showToast("Гирлянда: " + onOff(config.Current.WinterGarland))
	return m.winterStart()
}

// drawWinterOverlay decorates only existing blank cells. Text and layout are
// never shifted, and the original terminal style is restored after each flake.
func drawWinterOverlay(view string, frame uint64) string {
	if !winterActive() || (!winterMotionEnabled()) {
		return view
	}

	lines := strings.Split(view, "\n")
	width := 0
	for _, line := range lines {
		if w := ansi.StringWidth(line); w > width {
			width = w
		}
	}
	if width == 0 || len(lines) == 0 {
		return view
	}

	snow := winterSnowPositions(width, len(lines), frame, winterIntensity())
	garlandRow := -1
	if config.Current.WinterGarland {
		// Lipgloss may add vertical padding before the visible content. Find
		// the first row with the most free cells instead of assuming row zero
		// is the top edge after every terminal resize.
		bestSpaces := 0
		for row, line := range lines {
			spaces := strings.Count(ansi.Strip(line), " ")
			if spaces > bestSpaces {
				bestSpaces = spaces
				garlandRow = row
			}
		}
	}
	for row := range lines {
		lines[row] = decorateWinterLine(lines[row], row, width, snow, row == garlandRow, frame)
	}
	return strings.Join(lines, "\n")
}

func winterSnowPositions(width, height int, frame uint64, intensity int) map[int]struct{} {
	positions := make(map[int]struct{})
	if width <= 0 || height <= 0 || intensity <= 0 {
		return positions
	}

	count := width * height * intensity / 150
	if count < 8 {
		count = 8
	}
	if count > 180 {
		count = 180
	}

	// Every flake has its own fall speed and horizontal drift. A shared
	// vertical offset made all particles line up into a visible horizontal
	// band when the terminal was tall or fullscreen.
	for i := 0; i < count; i++ {
		seedY := (i*37 + i*i*11) % height
		speed := 1 + i%4
		y := (seedY + int(frame)*speed) % height
		seedX := (i*53 + i*i*7) % width
		sway := (int(frame/3)+i*5)%7 - 3
		x := (seedX + sway + width) % width
		positions[y*width+x] = struct{}{}
	}
	return positions
}

func decorateWinterLine(line string, row, gridWidth int, snow map[int]struct{}, garland bool, frame uint64) string {
	parser := ansi.NewParser()
	var out strings.Builder
	out.Grow(len(line) + 64)

	var state byte
	var activeSGR string
	column := 0
	input := []byte(line)

	for len(input) > 0 {
		sequence, width, consumed, nextState := ansi.DecodeSequenceWc(input, state, parser)
		if consumed <= 0 || consumed > len(input) {
			sequence = input[:1]
			consumed = 1
			width = 1
		}
		input = input[consumed:]
		state = nextState
		token := string(sequence)

		if strings.HasPrefix(token, "\x1b[") && strings.HasSuffix(token, "m") {
			if token == "\x1b[m" || token == "\x1b[0m" {
				activeSGR = ""
			} else {
				activeSGR += token
			}
		}

		if width > 0 {
			if token == " " && garland && column%7 == 0 {
				color := winterBulbColor(column/7, frame)
				out.WriteString(color + "●\x1b[0m" + activeSGR)
			} else if token == " " {
				if _, ok := snow[row*max(1, gridWidth)+column]; ok {
					out.WriteString("\x1b[38;2;195;225;242m·\x1b[0m" + activeSGR)
				} else {
					out.WriteString(token)
				}
			} else {
				out.WriteString(token)
			}
			column += width
		} else {
			out.WriteString(token)
		}
	}
	return out.String()
}

func winterBulbColor(index int, frame uint64) string {
	colors := [...][3]int{
		{255, 105, 125},
		{120, 220, 165},
		{125, 190, 255},
		{255, 215, 125},
	}
	// A small stepped breathing curve keeps the garland alive without
	// introducing another animation system or any expensive effects.
	levels := [...]int{168, 188, 214, 238, 255, 238, 214, 188}
	level := levels[(frame/2+uint64(index*2))%uint64(len(levels))]
	color := colors[index%len(colors)]
	r := color[0] * level / 255
	g := color[1] * level / 255
	b := color[2] * level / 255
	return "\x1b[1;38;2;" + strconv.Itoa(r) + ";" + strconv.Itoa(g) + ";" + strconv.Itoa(b) + "m"
}
