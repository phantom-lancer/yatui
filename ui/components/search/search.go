package search

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/model"
	"github.com/phantom-lancer/yatui/ui/style"
)

type Control uint

const (
	SELECT Control = iota
	CANCEL
	CURSOR_UP
	CURSOR_DOWN
	TYPING
	UPDATE_SUGGESTIONS
)

const (
	_UPDATE_SUGGESTIONS_PERIOD = time.Millisecond * 4
)

type Model struct {
	list                 list.Model
	input                textinput.Model
	width, height        int
	value                string
	updated              bool
	lastUpdateTime       time.Time
	additionalKeyBindigs []key.Binding

	// rendered are the rows of the list as they were drawn, and rows are the
	// places they took on the screen.
	rendered []renderedItem
	rows     []searchRow

	Title  string
	Action string
}

// renderedItem is one item of the list and the place it takes.
type renderedItem struct {
	index  int
	height int
	// contentY is the offset of the text inside the rendered block
	contentY int
}

// searchRow is a rendered item placed inside the view.
type searchRow struct {
	index    int
	y        int
	height   int
	contentY int
}

func New() *Model {
	m := &Model{
		additionalKeyBindigs: []key.Binding{
			key.NewBinding(config.Current.Controls.Apply.Binding(), config.Current.Controls.Apply.Help("search")),
			key.NewBinding(config.Current.Controls.Cancel.Binding(), config.Current.Controls.Cancel.Help("cancel")),
		},
		Title:  "Search",
		Action: "search",
	}

	controls := config.Current.Controls

	m.list = list.New([]list.Item{}, ItemDelegate{rendered: &m.rendered}, 512, 512)
	m.list.SetShowTitle(false)
	m.list.DisableQuitKeybindings()
	m.list.KeyMap = list.KeyMap{
		CursorUp:   key.NewBinding(controls.CursorUp.Binding(), controls.CursorUp.Help("up")),
		CursorDown: key.NewBinding(controls.CursorDown.Binding(), controls.CursorDown.Help("down")),
	}
	m.list.AdditionalShortHelpKeys = m.keymap

	m.input = textinput.New()
	m.input.Focus()

	return m
}

func (m *Model) keymap() []key.Binding {
	return m.additionalKeyBindigs
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) View() string {
	m.additionalKeyBindigs[0].SetHelp(m.additionalKeyBindigs[0].Help().Key, m.Action)
	listHeight := m.height - 7

	titleView := style.DialogTitleStyle.Render(m.Title)
	inputView := style.DialogBoxStyle.MaxWidth(m.width).Render(m.input.View())

	m.rendered = nil
	listView := m.list.View()
	if lipgloss.Height(listView) <= listHeight {
		lastLine := strings.LastIndex(listView[:len(listView)-1], "\n")
		listView = listView[:lastLine] + "\n" + listView[lastLine:]
	}

	// the rows of the list are placed under the title, the search field and
	// the status bar of the list
	m.rows = m.rows[:0]
	y := lipgloss.Height(titleView) + lipgloss.Height(inputView)
	if m.list.ShowTitle() {
		y += lipgloss.Height(m.list.Styles.TitleBar.Render("x"))
	}
	if m.list.ShowStatusBar() {
		y += lipgloss.Height(m.list.Styles.StatusBar.Render("x"))
	}
	for _, item := range m.rendered {
		m.rows = append(m.rows, searchRow{
			index:    item.index,
			y:        y,
			height:   item.height,
			contentY: item.contentY,
		})
		y += item.height
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		titleView,
		inputView,
		lipgloss.NewStyle().MaxWidth(m.width).Render(listView),
	)
}

// ItemAt returns the index of the suggestion under the given coordinates of
// the view, or -1 when there is none.
func (m *Model) ItemAt(x, y int) int {
	for _, row := range m.rows {
		if y >= row.y+row.contentY && y < row.y+row.height {
			return row.index
		}
	}
	return -1
}

// Hover marks the suggestion under the cursor as the chosen one.
func (m *Model) Hover(index int) {
	if index < 0 || index >= len(m.list.Items()) {
		return
	}
	m.list.Select(index)
}

// Pick takes the suggestion under the cursor. It reports whether there was
// one.
func (m *Model) Pick(index int) bool {
	items := m.list.Items()
	if index < 0 || index >= len(items) {
		return false
	}

	item, ok := items[index].(Item)
	if !ok {
		return false
	}

	m.value = string(item)
	m.list.SetItems([]list.Item{})
	m.list.Select(0)
	m.input.Reset()

	return true
}

func (m *Model) Update(message tea.Msg) (*Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := message.(type) {
	case tea.KeyMsg:
		controls := config.Current.Controls
		keypress := msg.String()

		switch {
		case controls.Apply.Contains(keypress):
			cmds = append(cmds, model.Cmd(SELECT))

			if len(m.list.Items()) == 0 {
				m.value = ""
				break
			}

			suggest, ok := m.list.SelectedItem().(Item)
			if !ok {
				m.value = ""
				break
			}

			m.value = string(suggest)
			m.list.SetItems([]list.Item{})
			m.list.Select(0)
			m.input.Reset()
		case controls.Cancel.Contains(keypress):
			cmds = append(cmds, model.Cmd(CANCEL))
			m.list.SetItems([]list.Item{})
			m.list.Select(0)
			m.input.Reset()
			m.value = ""
		case controls.CursorUp.Contains(keypress):
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
			cmds = append(cmds, model.Cmd(CURSOR_UP))
		case controls.CursorDown.Contains(keypress):
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
			cmds = append(cmds, model.Cmd(CURSOR_DOWN))
		default:
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)
			cmds = append(cmds, model.Cmd(TYPING))
			m.lastUpdateTime = time.Now()
			m.updated = false
		}

	default:
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)

		if !m.updated && time.Now().After(m.lastUpdateTime.Add(_UPDATE_SUGGESTIONS_PERIOD)) {
			cmds = append(cmds, model.Cmd(UPDATE_SUGGESTIONS))
			m.updated = true
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.input.Width = w - 9
	m.list.SetSize(m.width, m.height-7)
}

func (m *Model) SetSuggestions(suggestions []string) {
	items := make([]list.Item, 0, len(suggestions)+1)

	if len(suggestions) == 0 || m.input.Value() != suggestions[0] {
		if len(m.input.Value()) > 0 {
			items = append(items, Item(m.input.Value()))
		}
	}

	for _, sug := range suggestions {
		items = append(items, Item(sug))
	}

	m.list.SetItems(items)
	m.list.Select(0)
}

func (m *Model) InputValue() string {
	return m.input.Value()
}

func (m *Model) SuggestionValue() (string, bool) {
	return m.value, len(m.value) > 0
}
