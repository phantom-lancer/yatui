package loginpage

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/config"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

// loginModel is the login screen without its own tea.Program.
func loginModel() *Model {
	if config.Current.Controls == nil {
		// the key names of the help line come from the config
		_ = config.InitialLoad()
	}
	m := &Model{
		help:    help.New(),
		helpMap: newHelpMap(),
		input:   textinput.New(),
		width:   80,
		height:  24,
	}
	m.input.Focus()
	return m
}

// TestEmptyFieldStartsTheBrowser checks that the empty form does not save an
// empty token but starts the browser authorization instead.
func TestEmptyFieldStartsTheBrowser(t *testing.T) {
	m := loginModel()

	cmd := m.confirm()
	if cmd == nil {
		t.Fatal("the empty form must start the browser authorization")
	}
	if !m.oauthRunning {
		t.Fatal("the screen must show that the browser is waiting")
	}
	// the command itself is not run: it would open a browser
}

// TestButtonIsClickable checks that the click area matches the button that is
// drawn on the screen.
func TestButtonIsClickable(t *testing.T) {
	m := loginModel()

	lines := strings.Split(m.View(), "\n")
	if m.buttonW == 0 {
		t.Fatal("the button has no click area")
	}
	if m.buttonY < 0 || m.buttonY >= len(lines) {
		t.Fatalf("the button is outside the view: y=%d, lines=%d", m.buttonY, len(lines))
	}

	line := []rune(ansi.ReplaceAllString(lines[m.buttonY], ""))
	if m.buttonX < 0 || m.buttonX+m.buttonW > len(line) {
		t.Fatalf("the button is outside the line: x=%d w=%d, line=%d",
			m.buttonX, m.buttonW, len(line))
	}
	if got := string(line[m.buttonX : m.buttonX+m.buttonW]); !strings.Contains(got, buttonLabel) {
		t.Fatalf("the click area points at %q", got)
	}

	click := tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		X:      m.buttonX + 1,
		Y:      m.buttonY,
	}
	if !m.clickedButton(click) {
		t.Fatal("a click on the button is not recognized")
	}
	if m.clickedButton(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
		X: m.buttonX, Y: m.buttonY - 1}) {
		t.Fatal("a click above the button must be ignored")
	}
}

// TestOauthAnswerResumesTheScreen checks that a finished authorization stops
// the waiting state.
func TestOauthAnswerResumesTheScreen(t *testing.T) {
	m := loginModel()
	m.oauthRunning = true

	m.Update(oauthDoneMsg{err: errTest{}})
	if m.oauthRunning {
		t.Fatal("the waiting state must end after an error")
	}
	if m.oauthErr == nil {
		t.Fatal("the error must be shown")
	}
}

type errTest struct{}

func (errTest) Error() string { return "test error" }
