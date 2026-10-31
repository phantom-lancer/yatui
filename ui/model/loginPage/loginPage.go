package loginpage

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/phantom-lancer/yatui/api"
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

	// busy is set while the browser authorization runs or the token is being
	// checked: the input and the button are locked until the answer arrives.
	busy     bool
	busyText string
	uiErr    error

	// hover is set while the mouse is over the button, so it reacts like a
	// button of the main page
	hover bool

	// the button is placed by the view, so a click and a hover can be matched
	// to it
	buttonX, buttonY, buttonW int
}

// oauthTimeout is how long the browser window may be open before the login
// gives up and shows the error.
const oauthTimeout = 5 * time.Minute

// buttonLabel is what the sign-in button says; the view finds it to place the
// click area.
const buttonLabel = "Sign in via browser"

// tokenSource tells where a token came from, because the advice for a token
// without Music API access depends on it.
type tokenSource int

const (
	sourcePaste tokenSource = iota
	sourceOAuth
)

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

	// the hover of the button needs the motion events, not only the clicks
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
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
		if msg.err != nil {
			m.busy = false
			m.uiErr = msg.err
			return m, nil
		}
		return m, m.checkToken(msg.token, sourceOAuth)

	case tokenCheckedMsg:
		m.busy = false
		if msg.err == nil {
			return m, m.accept(msg.token)
		}
		if !errors.Is(msg.err, api.ErrTokenRejected) {
			// the network hid the answer: the token cannot be blamed, so it is
			// saved and the API will tell about it later
			return m, m.accept(msg.token)
		}
		if msg.source == sourceOAuth {
			m.uiErr = errors.New("Yandex does not let this app use the Music API. " +
				"Take a token from the 'Yandex Music Token' extension: " +
				"open music.yandex.ru, copy the token and paste it here. " +
				"The saved token was not changed.")
		} else {
			m.uiErr = errors.New("this token cannot open the Music API. " +
				"Copy the token from the 'Yandex Music Token' extension on music.yandex.ru. " +
				"The saved token was not changed.")
		}
		return m, nil

	case hotkeys.HotkeyMsg:
		// player hotkeys confirm the entered token, or start the browser
		// authorization when the field is empty
		return m, m.confirm()

	case tea.KeyMsg:
		m.hotkeys.Observe(msg)
		controls := config.Current.Controls
		keypress := msg.String()

		switch {
		case controls.Quit.Contains(keypress):
			return m, tea.Quit
		case m.busy:
			// the browser window or the check has the focus
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
		m.observeHover(msg)
		if !m.busy && m.clickedButton(msg) {
			cmds = append(cmds, m.confirm())
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

	buttonStyle := style.ButtonStyle.Padding(0, 3)
	if m.hover {
		buttonStyle = style.ActiveButtonStyle.Padding(0, 3)
	}
	button := buttonStyle.Render(buttonLabel)

	var content string
	switch {
	case m.busy:
		m.buttonW = 0
		content = lipgloss.JoinVertical(lipgloss.Left,
			title,
			style.AccentTextStyle.Render(m.busyText),
			"Keep this window open, the token is saved automatically.",
		)
	default:
		content = lipgloss.JoinVertical(lipgloss.Left, title, inputView, button)
		if m.uiErr != nil {
			content = lipgloss.JoinVertical(lipgloss.Left, content,
				style.ErrorTextStyle.Width(64).Render(m.uiErr.Error()))
		}
	}

	box := style.DialogBoxStyle.Render(content)
	helpView := style.DialogHelpStyle.Render(m.help.View(m.helpMap))

	// the hint stays outside the box, where it does not move the button
	var hint string
	if !m.busy {
		hint = style.DialogHelpStyle.Render(
			"Token: music.yandex.ru -> the 'Yandex Music Token' extension -> copy")
	}

	block := lipgloss.JoinVertical(lipgloss.Left, box, hint, helpView)

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
	if m.busy {
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
	return m.overButton(msg.X, msg.Y)
}

// observeHover lights the button while the mouse is over it.
func (m *Model) observeHover(msg tea.MouseMsg) {
	if m.buttonW == 0 || m.busy {
		m.hover = false
		return
	}
	m.hover = m.overButton(msg.X, msg.Y)
}

func (m *Model) overButton(x, y int) bool {
	return x >= m.buttonX && x < m.buttonX+m.buttonW && y == m.buttonY
}

// confirm starts the browser authorization when the field is empty, and
// checks the pasted token otherwise.
func (m *Model) confirm() tea.Cmd {
	if m.busy {
		return nil
	}
	token := strings.TrimSpace(m.input.Value())
	if token == "" {
		return m.startOAuth()
	}
	return m.checkToken(token, sourcePaste)
}

// accept saves the token that the API has confirmed and ends the login.
func (m *Model) accept(token string) tea.Cmd {
	config.Current.Token = token
	m.err = config.Save()
	return tea.Quit
}

// checkToken verifies the token in the background: a token without access to
// the Music API must not replace the saved one.
func (m *Model) checkToken(token string, source tokenSource) tea.Cmd {
	m.busy = true
	m.busyText = "Checking the token..."
	m.uiErr = nil
	return func() tea.Msg {
		return tokenCheckedMsg{token: token, source: source, err: api.CheckToken(token)}
	}
}

// startOAuth runs the browser authorization in the background: the model is
// only touched by the message the command sends back.
func (m *Model) startOAuth() tea.Cmd {
	m.busy = true
	m.busyText = "Waiting for the browser..."
	m.uiErr = nil
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

// tokenCheckedMsg is the answer of the Music API about the token.
type tokenCheckedMsg struct {
	token  string
	source tokenSource
	err    error
}

//
// private methods
//

func (m *Model) resize(width, height int) {
	m.width, m.height = width, height
}
