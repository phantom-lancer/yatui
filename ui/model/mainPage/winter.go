package mainpage

import (
	"math"
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

type winterParticle struct {
	x, y   float64
	vx, vy float64
	life   int
}

const winterMaxInteractionParticles = 64

// handleWinterMouse emits a short burst on clicks and a sparse particle trail
// while the left button is held. It reuses the existing winter frame timer.
func (m *Model) handleWinterMouse(msg tea.MouseMsg) {
	if !winterActive() {
		m.winterPointerDown = false
		return
	}
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft {
			return
		}
		m.winterPointerDown = true
		m.addWinterBurst(msg.X, msg.Y)
	case tea.MouseActionMotion:
		if !m.winterPointerDown || m.winterFrame%2 != 0 {
			return
		}
		m.addWinterTrailParticle(msg.X, msg.Y)
	case tea.MouseActionRelease:
		if msg.Button == tea.MouseButtonLeft || msg.Button == tea.MouseButtonNone {
			m.winterPointerDown = false
		}
	}
	m.idleFrameValid = false
	if cmd := m.winterStart(); cmd != nil && m.program != nil {
		program := m.program
		go func() {
			program.Send(cmd())
		}()
	}
}

func (m *Model) addWinterBurst(x, y int) {
	for i := 0; i < 12 && len(m.winterParticles) < winterMaxInteractionParticles; i++ {
		angle := 2 * math.Pi * float64(i) / 12
		speed := 0.22 + float64(i%3)*0.07
		m.winterParticles = append(m.winterParticles, winterParticle{
			x: float64(x), y: float64(y),
			vx:   math.Cos(angle) * speed,
			vy:   math.Sin(angle)*speed - 0.05,
			life: 8 + i%5,
		})
	}
}

func (m *Model) addWinterTrailParticle(x, y int) {
	if len(m.winterParticles) >= winterMaxInteractionParticles {
		m.winterParticles = m.winterParticles[1:]
	}
	m.winterParticles = append(m.winterParticles, winterParticle{
		x: float64(x), y: float64(y),
		vx: -0.08, vy: -0.12, life: 9,
	})
}

func (m *Model) advanceWinterParticles() {
	live := m.winterParticles[:0]
	for _, particle := range m.winterParticles {
		particle.x += particle.vx
		particle.y += particle.vy
		particle.vx *= 0.92
		particle.vy = particle.vy*0.92 + 0.012
		particle.life--
		if particle.life > 0 && particle.x >= 0 && particle.x < float64(m.width) &&
			particle.y >= 0 && particle.y < float64(m.height) {
			live = append(live, particle)
		}
	}
	m.winterParticles = live
}

func winterActive() bool {
	return config.Current.Style != nil && config.Current.Style.Theme == "winter"
}

func winterDensity() int {
	return max(0, min(100, config.Current.WinterDensity))
}

func winterSpeed() int {
	return max(10, min(100, config.Current.WinterSpeed))
}

func winterIntensity() int {
	return winterDensity()
}

func winterMotionEnabled() bool {
	if config.Current.ResourceSaver {
		return false
	}
	return winterActive() && (winterDensity() > 0 || config.Current.WinterGarland)
}

func winterTick(generation uint64) tea.Cmd {
	return tea.Tick(winterFrameInterval, func(time.Time) tea.Msg {
		return winterTickMsg{generation: generation}
	})
}

// winterStart arms one low-frequency frame timer only while the winter theme
// has visible effects enabled. There is no animation goroutine or idle polling.
func (m *Model) winterStart() tea.Cmd {
	if !winterMotionEnabled() && len(m.winterParticles) == 0 {
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
	if !winterMotionEnabled() && len(m.winterParticles) == 0 {
		m.winterStop()
		return nil
	}
	m.winterFrame++
	m.advanceWinterParticles()
	m.idleFrameValid = false
	if !winterMotionEnabled() && len(m.winterParticles) == 0 {
		m.winterStop()
		return nil
	}
	return winterTick(m.winterGeneration)
}

func (m *Model) applyWinterIntensity(value int) tea.Cmd {
	value = max(0, min(100, value))
	config.Current.WinterDensity = value
	config.SaveLater()
	m.winterFrame = 0
	m.idleFrameValid = false
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
	return drawWinterOverlayWithParticles(view, frame, nil)
}

func drawWinterOverlayWithParticles(view string, frame uint64, particles []winterParticle) string {
	if !winterActive() || (!winterMotionEnabled() && len(particles) == 0) {
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

	snow := winterSnowPositionsAtSpeed(width, len(lines), frame, winterDensity(), winterSpeed())
	sparks := make(map[int]struct{}, len(particles))
	for _, particle := range particles {
		x, y := int(math.Round(particle.x)), int(math.Round(particle.y))
		if x >= 0 && x < width && y >= 0 && y < len(lines) && particle.life > 0 {
			sparks[y*width+x] = struct{}{}
		}
	}
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
		lines[row] = decorateWinterLineWithSparks(lines[row], row, width, snow, sparks, row == garlandRow, frame)
	}
	return strings.Join(lines, "\n")
}

func winterSnowPositions(width, height int, frame uint64, density int) map[int]struct{} {
	return winterSnowPositionsAtSpeed(width, height, frame, density, 50)
}

func winterSnowPositionsAtSpeed(width, height int, frame uint64, density, speed int) map[int]struct{} {
	positions := make(map[int]struct{})
	if width <= 0 || height <= 0 || density <= 0 {
		return positions
	}

	count := width * height * density / 3000
	if count < 8 {
		count = 8
	}
	if count > 180 {
		count = 180
	}

	// Every flake has its own fall speed and horizontal drift. A shared
	// vertical offset made all particles line up into a visible horizontal
	// band when the terminal was tall or fullscreen.
	phase := int(frame) * max(1, min(100, speed)) / 50
	for i := 0; i < count; i++ {
		seedY := (i*37 + i*i*11) % height
		flakeSpeed := 1 + i%4
		y := (seedY + phase*flakeSpeed) % height
		seedX := (i*53 + i*i*7) % width
		sway := (phase/3+i*5)%7 - 3
		x := (seedX + sway + width) % width
		positions[y*width+x] = struct{}{}
	}
	return positions
}

func decorateWinterLine(line string, row, gridWidth int, snow map[int]struct{}, garland bool, frame uint64) string {
	return decorateWinterLineWithSparks(line, row, gridWidth, snow, nil, garland, frame)
}

func decorateWinterLineWithSparks(line string, row, gridWidth int, snow, sparks map[int]struct{}, garland bool, frame uint64) string {
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
				key := row*max(1, gridWidth) + column
				if _, ok := sparks[key]; ok {
					out.WriteString("\x1b[1;38;2;225;241;255m✧\x1b[0m" + activeSGR)
				} else if _, ok := snow[key]; ok {
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
