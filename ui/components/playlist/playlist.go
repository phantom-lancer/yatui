package playlist

import (
	"strings"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/model"
	"github.com/phantom-lancer/yatui/ui/style"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Control uint

const (
	CURSOR_UP Control = iota
	CURSOR_DOWN
	RENAME
	TOGGLE_VIEW
)

type PlaylistType = uint64

const (
	NONE PlaylistType = iota
	MYWAVE
	LIKES
	LOCAL
	ALBUMS
	// Should be the last to detect downloaded user playlists
	USER
)

var emptyItem = Item{Name: "", Kind: NONE, Active: false, Subitem: false}

func ItemEmpty() *Item {
	return &emptyItem
}

func ItemCategory(name string) *Item {
	return &Item{Name: name, Kind: NONE, Active: false, Subitem: false}
}

type Model struct {
	program       *tea.Program
	list          list.Model
	Hidden        bool
	helpMap       *helpKeyMap
	width, height int
	layout        listLayout

	// footer is an optional clickable block under the list
	footer      string
	footerHover bool
	footerRect  Rect
}

func New(p *tea.Program, title string) *Model {
	m := &Model{
		program: p,
		helpMap: newHelpMap(),
	}

	controls := config.Current.Controls
	m.list = list.New([]list.Item{}, ItemDelegate{programm: p, layout: &m.layout}, 512, 512)
	m.list.Title = title
	m.list.SetShowStatusBar(false)
	m.list.Styles.Title = m.list.Styles.Title.Foreground(style.AccentColor).UnsetBackground().Padding(0)
	m.list.KeyMap = list.KeyMap{
		CursorUp:   key.NewBinding(controls.PlaylistsUp.Binding(), controls.PlaylistsUp.Help("up")),
		CursorDown: key.NewBinding(controls.PlaylistsDown.Binding(), controls.PlaylistsDown.Help("down")),
	}
	m.list.SetShowHelp(false)

	return m
}

// RefreshStyle re-applies the colors after a theme switch.
func (m *Model) RefreshStyle() {
	m.list.Styles.Title = m.list.Styles.Title.Foreground(style.AccentColor).UnsetBackground().Padding(0)
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) View() string {
	if m.Hidden {
		m.footerRect = Rect{}
		return ""
	}

	m.helpMap.Renamable = m.SelectedItem().Kind >= USER

	// the footer is dropped when there is no room for it: the sidebar must
	// keep its height, or the layout of the whole screen would break
	footer := ""
	if m.footer != "" && m.height-lipgloss.Height(m.footer)-2 >= 1 {
		footer = m.footer
	}

	// the list takes everything the footer does not need, the rest of the
	// panel is the empty line between them
	listHeight := m.height - 1
	if footer != "" {
		listHeight -= lipgloss.Height(footer)
	}
	m.list.SetHeight(listHeight)

	m.resetLayout()
	listView := m.list.View()
	m.buildLayout()

	content := lipgloss.JoinVertical(lipgloss.Left, listView, "")
	m.footerRect = Rect{}
	if footer != "" {
		// the block is centered in the panel, so it looks like a part of it
		offset := (m.width - lipgloss.Width(footer)) / 2
		if offset < 0 {
			offset = 0
		}

		lines := strings.Split(footer, "\n")
		for i := range lines {
			lines[i] = strings.Repeat(" ", offset) + lines[i]
		}

		content = lipgloss.JoinVertical(lipgloss.Left, content, strings.Join(lines, "\n"))
		m.footerRect = Rect{
			X:      m.layout.originX + offset,
			Y:      style.SideBoxStyle.GetBorderTopSize() + style.SideBoxStyle.GetPaddingTop() + listHeight + 1,
			Width:  lipgloss.Width(footer),
			Height: lipgloss.Height(footer),
		}
	}

	return style.SideBoxStyle.Render(content)
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
		case controls.PlaylistsUp.Contains(keypress):
			m.list, cmd = m.list.Update(msg)

			for len(m.list.Items()) > 0 && m.list.Index() > 0 && !m.list.SelectedItem().(*Item).Active {
				m.list.CursorUp()
			}

			cmds = append(cmds, cmd)
			cmds = append(cmds, model.Cmd(CURSOR_UP))
		case controls.PlaylistsDown.Contains(keypress):
			m.list, cmd = m.list.Update(msg)

			for m.list.Index() < len(m.list.Items())-1 && !m.list.SelectedItem().(*Item).Active {
				m.list.CursorDown()
			}

			cmds = append(cmds, cmd)
			cmds = append(cmds, model.Cmd(CURSOR_DOWN))
		case controls.PlaylistsRename.Contains(keypress):
			cmds = append(cmds, model.Cmd(RENAME))
		case controls.PlaylistsHide.Contains(keypress):
			m.Hidden = !m.Hidden
			cmds = append(cmds, model.Cmd(TOGGLE_VIEW))
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) GetFirst(kind PlaylistType) (*Item, int) {
	var pl *Item
	items := m.list.Items()
	for i := range items {
		pl = items[i].(*Item)
		if pl.Kind == kind {
			return pl, i
		}
	}
	return nil, -1
}

func (m *Model) Items() []*Item {
	litems := m.list.Items()
	items := make([]*Item, len(litems))
	for i := range litems {
		items[i] = litems[i].(*Item)
	}
	return items
}

func (m *Model) Reset() tea.Cmd {
	return m.list.SetItems([]list.Item{})
}

func (m *Model) SetItems(items []*Item) tea.Cmd {
	newItems := make([]list.Item, len(items))
	for i := 0; i < len(items); i++ {
		newItems[i] = items[i]
	}
	return m.list.SetItems(newItems)
}

func (m *Model) InsertItem(index int, item *Item) tea.Cmd {
	if index < 0 {
		index = len(m.list.Items()) + 1
	}
	return m.list.InsertItem(index, item)
}

func (m *Model) SetItem(index int, item *Item) tea.Cmd {
	return m.list.SetItem(index, item)
}

func (m *Model) RemoveItem(index int) {
	m.list.RemoveItem(index)
}

func (m *Model) SelectedItem() *Item {
	return m.list.SelectedItem().(*Item)
}

func (m *Model) Index() int {
	return m.list.Index()
}

func (m *Model) Select(index int) {
	m.list.Select(index)
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.list.SetSize(m.width, m.height)
}

func (m *Model) SetWidth(w int) {
	m.width = w
	m.list.SetSize(m.width, m.height)
}

func (m *Model) Width() int {
	return m.width
}

func (m *Model) SetHeight(h int) {
	m.height = h
	m.list.SetSize(m.width, m.height)
}

func (m *Model) Height() int {
	return m.height
}
