package loginpage

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/oauth"
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
	if !m.busy {
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

// TestHoverHighlightsTheButton checks that the button reacts to the mouse: it
// is lit under the pointer and is drawn in the active style.
func TestHoverHighlightsTheButton(t *testing.T) {
	m := loginModel()
	m.View()

	m.observeHover(tea.MouseMsg{Action: tea.MouseActionMotion, X: m.buttonX + 1, Y: m.buttonY})
	if !m.hover {
		t.Fatal("the button must react when the mouse is over it")
	}

	m.observeHover(tea.MouseMsg{Action: tea.MouseActionMotion, X: m.buttonX, Y: m.buttonY - 2})
	if m.hover {
		t.Fatal("the button must not react when the mouse is away")
	}
}

// TestDeviceCodeIsShown checks that the waiting screen tells what to do: the
// page and the short code are on it.
func TestDeviceCodeIsShown(t *testing.T) {
	m := loginModel()
	m.busy = true
	m.busyText = "Waiting for the browser..."
	m.device = &oauth.DeviceCode{
		UserCode:        "abcd-efgh",
		VerificationURL: "https://ya.ru/device",
	}

	view := ansi.ReplaceAllString(m.View(), "")
	if !strings.Contains(view, "abcd-efgh") {
		t.Fatal("the short code is not on the screen")
	}
	if !strings.Contains(view, "https://ya.ru/device") {
		t.Fatal("the page is not on the screen")
	}
}

// TestRejectedTokenIsNotSaved checks that a token without access to the Music
// API never replaces the working one.
func TestRejectedTokenIsNotSaved(t *testing.T) {
	m := loginModel()
	saved := config.Current.Token
	t.Cleanup(func() { config.Current.Token = saved })

	config.Current.Token = "old-token"
	m.busy = true
	m.Update(tokenCheckedMsg{
		token:  "bad-token",
		source: sourcePaste,
		err:    api.ErrTokenRejected,
	})

	if config.Current.Token != "old-token" {
		t.Fatalf("the rejected token was saved: %q", config.Current.Token)
	}
	if m.uiErr == nil {
		t.Fatal("the refusal must be shown")
	}
	if m.busy {
		t.Fatal("the check must end")
	}
}

// TestOauthAnswerResumesTheScreen checks that a finished authorization stops
// the waiting state.
func TestOauthAnswerResumesTheScreen(t *testing.T) {
	m := loginModel()
	m.busy = true

	m.Update(deviceTokenMsg{err: errTest{}})
	if m.busy {
		t.Fatal("the waiting state must end after an error")
	}
	if m.uiErr == nil {
		t.Fatal("the error must be shown")
	}
}

type errTest struct{}

func (errTest) Error() string { return "test error" }
