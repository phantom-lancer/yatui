package loginpage

import (
	"context"
	"strings"
	"time"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/oauth"
	"github.com/phantom-lancer/yatui/ui/model/hotkeys"
	"github.com/phantom-lancer/yatui/ui/style"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	err           error
	program       *tea.Program
	width, height int
	hotkeys       *hotkeys.Tracker

	input   textinput.Model
	help    help.Model
	helpMap *helpKeyMap

	// oauthRunning is set while the browser authorization is in progress:
	// the input and the button are locked until the answer arrives.
	oauthRunning bool
	oauthErr     error

	// the button is placed by the view, so a click can be matched to it
	buttonX, buttonY, buttonW int
}

// oauthTimeout is how long the browser window may be open before the login
// gives up and shows the error.
const oauthTimeout = 5 * time.Minute

// buttonLabel is what the sign-in button says; the view finds it to place the
// click area.
const buttonLabel = "Sign in via browser"

// loginpage.Model constructor.
func New() *Model {
	m := &Model{
		input:   textinput.New(),
		help:    help.New(),
		helpMap: newHelpMap(),
		hotkeys: hotkeys.NewTracker(),
	}

	m.input.Width = 64
	// a Yandex token is a long string: the old limit cut longer ones
	m.input.CharLimit = 512
	m.input.Placeholder = "paste your token here"
	m.input.Focus()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p
	m.hotkeys.Start(p)

	return m
}

//
// model.Model interface implementation
//

func (m *Model) Run() error {
	_, err := m.program.Run()
	if err != nil {
		return err
	}
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *Model) Send(msg tea.Msg) {
	go m.program.Send(msg)
}

//
// tea.Model interface implementation
//

func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, tea.ClearScreen

	case oauthDoneMsg:
		m.oauthRunning = false
		if msg.err != nil {
			m.oauthErr = msg.err
			return m, nil
		}
		m.oauthErr = nil
		config.Current.Token = msg.token
		m.err = config.Save()
		return m, tea.Quit

	case hotkeys.HotkeyMsg:
		// player hotkeys confirm the entered token as well
		return m, m.confirm()

	case tea.KeyMsg:
		m.hotkeys.Observe(msg)
		controls := config.Current.Controls
		keypress := msg.String()

		switch {
		case controls.Quit.Contains(keypress):
			return m, tea.Quit
		case m.oauthRunning:
			// the browser window has the focus: the input waits
		case controls.Apply.Contains(keypress),
			controls.PlayerRewindBackward.Contains(keypress),
			controls.PlayerStop.Contains(keypress),
			controls.PlayerRewindForward.Contains(keypress):
			return m, m.confirm()
		default:
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.MouseMsg:
		if !m.oauthRunning && m.clickedButton(msg) {
			cmds = append(cmds, m.startOAuth())
		}

	default:
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() string {
	title := style.DialogTitleStyle.Render("Enter your token")
	inputView := m.input.View()
	button := style.ButtonStyle.Padding(0, 3).Render(buttonLabel)
	hint := style.DialogHelpStyle.Render(
		"or press the button: the browser opens oauth.yandex.ru")

	var content string
	switch {
	case m.oauthRunning:
		m.buttonW = 0
		content = lipgloss.JoinVertical(lipgloss.Left,
			title,
			style.AccentTextStyle.Render("Waiting for the browser..."),
			"Allow the access in oauth.yandex.ru, the token is saved automatically.",
		)
	case m.oauthErr != nil:
		content = lipgloss.JoinVertical(lipgloss.Left,
			title,
			inputView,
			button,
			style.ErrorTextStyle.Render("Authorization failed: "+m.oauthErr.Error()),
			"Try again, or paste a token manually.",
		)
	default:
		content = lipgloss.JoinVertical(lipgloss.Left, title, inputView, button)
	}

	box := style.DialogBoxStyle.Render(content)
	helpView := style.DialogHelpStyle.Render(m.help.View(m.helpMap))

	var block string
	if m.oauthErr != nil || !m.oauthRunning {
		block = lipgloss.JoinVertical(lipgloss.Left, box, hint, helpView)
	} else {
		block = lipgloss.JoinVertical(lipgloss.Left, box, helpView)
	}

	// the button is placed here, so a click in the view finds it
	m.placeButton(block)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		block,
	)
}

// placeButton remembers where the button ended up in the centered dialog.
// The line of the block is where the button label is drawn, so the click area
// cannot drift from what is on the screen, whatever the style does.
func (m *Model) placeButton(block string) {
	m.buttonW = 0
	if m.oauthRunning {
		return
	}

	left := (m.width - lipgloss.Width(block)) / 2
	top := (m.height - lipgloss.Height(block)) / 2
	if left < 0 {
		left = 0
	}
	if top < 0 {
		top = 0
	}

	for i, line := range strings.Split(block, "\n") {
		index := strings.Index(line, buttonLabel)
		if index < 0 {
			continue
		}
		m.buttonX = left + lipgloss.Width(line[:index])
		m.buttonY = top + i
		m.buttonW = lipgloss.Width(buttonLabel)
		return
	}
}

// clickedButton reports whether the mouse event is a click on the button.
func (m *Model) clickedButton(msg tea.MouseMsg) bool {
	if m.buttonW == 0 || msg.Action != tea.MouseActionPress ||
		msg.Button != tea.MouseButtonLeft {
		return false
	}
	return msg.X >= m.buttonX && msg.X < m.buttonX+m.buttonW &&
		msg.Y == m.buttonY
}

// confirm saves the pasted token, or starts the browser authorization when
// the field is empty.
func (m *Model) confirm() tea.Cmd {
	if m.oauthRunning {
		return nil
	}
	token := strings.TrimSpace(m.input.Value())
	if token == "" {
		return m.startOAuth()
	}
	config.Current.Token = token
	m.err = config.Save()
	return tea.Quit
}

// startOAuth runs the browser authorization in the background: the model is
// only touched by the message the command sends back.
func (m *Model) startOAuth() tea.Cmd {
	m.oauthRunning = true
	m.oauthErr = nil
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), oauthTimeout)
		defer cancel()

		token, err := oauth.GetToken(ctx)
		return oauthDoneMsg{token: token, err: err}
	}
}

// oauthDoneMsg is the result of the browser authorization.
type oauthDoneMsg struct {
	token string
	err   error
}

//
// private methods
//

func (m *Model) resize(width, height int) {
	m.width, m.height = width, height
}
