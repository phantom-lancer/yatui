package tracklist

import (
	"strings"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/model"
	"github.com/phantom-lancer/yatui/ui/style"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Control uint

const (
	PLAY Control = iota
	CURSOR_UP
	CURSOR_DOWN
	PAGE_UP
	PAGE_DOWN
	SEARCH
	SHUFFLE
	SHARE
	LIKE
	SHOW_ARTIST
	ADD_TO_PLAYLIST
	REMOVE_FROM_PLAYLIST
	BACK
	TOGGLE_VIEW
)

type Model struct {
	program       *tea.Program
	list          list.Model
	help          help.Model
	helpMap       *helpKeyMap
	width, height int
	Hidden        bool
	Title         string
	Shufflable    bool
	layout        listLayout
	// dragIndex is the row the mouse took hold of, -1 when nothing is
	// dragged. dragStart is the row the drag began at.
	dragIndex int
	dragStart int
}

func New(p *tea.Program, likesMap *map[string]bool, dislikesMap *map[string]bool, cacheMap *map[string]bool) *Model {
	m := &Model{
		program:   p,
		help:      help.New(),
		helpMap:   newHelpMap(),
		Title:     "Tracks",
		dragIndex: -1,
		dragStart: -1,
	}

	controls := config.Current.Controls

	m.list = list.New([]list.Item{}, ItemDelegate{likesMap: likesMap, dislikesMap: dislikesMap, cacheMap: cacheMap, layout: &m.layout}, 512, 512)
	m.list.Styles.Title = style.TrackListTitleStyle
	m.list.KeyMap = list.KeyMap{
		CursorUp:   key.NewBinding(controls.CursorUp.Binding(), controls.CursorUp.Help("up")),
		CursorDown: key.NewBinding(controls.CursorDown.Binding(), controls.CursorDown.Help("down")),
		NextPage:   key.NewBinding(controls.TracksPrevPage.Binding(), controls.TracksPrevPage.Help("next page")),
		PrevPage:   key.NewBinding(controls.TracksNextPage.Binding(), controls.TracksNextPage.Help("prev page")),
	}
	m.list.Paginator.KeyMap.NextPage.SetEnabled(false)
	m.list.Paginator.KeyMap.PrevPage.SetEnabled(false)
	m.list.SetShowHelp(false)

	m.help.Ellipsis = "…"
	m.help.Styles.FullDesc = m.help.Styles.FullDesc.PaddingRight(1)

	return m
}

// RefreshStyle re-applies the colors after a theme switch.
func (m *Model) RefreshStyle() {
	m.list.Styles.Title = style.TrackListTitleStyle
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) View() string {
	if m.Hidden {
		return ""
	}

	titleLen := lipgloss.Width(m.Title)
	if titleLen > m.width-8 {
		m.list.Title = lipgloss.NewStyle().MaxWidth(m.width-9).Render(m.Title) + "…"
	} else {
		m.list.Title = m.Title
	}

	track := m.list.SelectedItem()
	if track != nil {
		m.helpMap.Shafflable = m.Shufflable
		m.helpMap.HasArtist = false
		if track.(Item).Track != nil {
			artists := track.(Item).Track.Artists
			for i := range artists {
				if artists[i].Id > 0 {
					m.helpMap.HasArtist = true
					break
				}
			}
		}
	} else {
		m.helpMap.Shafflable = false
		m.helpMap.HasArtist = false
	}

	m.list.SetHeight(m.height - 4)

	m.resetLayout()
	listView := m.list.View()
	if lipgloss.Height(listView) <= m.list.Height() {
		lastLine := strings.LastIndex(listView[:len(listView)-1], "\n")
		listView = listView[:lastLine] + "\n" + listView[lastLine:]
	}

	m.buildLayout()
	return style.TrackBoxStyle.Width(m.width).Render(lipgloss.JoinVertical(lipgloss.Left, listView, ""))
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

		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

		switch {
		case controls.Apply.Contains(keypress):
			cmds = append(cmds, model.Cmd(PLAY))
		case controls.CursorUp.Contains(keypress):
			cmds = append(cmds, model.Cmd(CURSOR_UP))
		case controls.CursorDown.Contains(keypress):
			cmds = append(cmds, model.Cmd(CURSOR_DOWN))
		case controls.TracksNextPage.Contains(keypress):
			cmds = append(cmds, model.Cmd(PAGE_UP))
		case controls.TracksPrevPage.Contains(keypress):
			cmds = append(cmds, model.Cmd(PAGE_DOWN))
		case controls.TracksSearch.Contains(keypress):
			cmds = append(cmds, model.Cmd(SEARCH))
		case controls.TracksShuffle.Contains(keypress):
			cmds = append(cmds, model.Cmd(SHUFFLE))
		case controls.TracksShare.Contains(keypress):
			cmds = append(cmds, model.Cmd(SHARE))
		case controls.TracksLike.Contains(keypress):
			cmds = append(cmds, model.Cmd(LIKE))
		case controls.TracksArtist.Contains(keypress):
			cmds = append(cmds, model.Cmd(SHOW_ARTIST))
		case controls.TracksAddToPlaylist.Contains(keypress):
			cmds = append(cmds, model.Cmd(ADD_TO_PLAYLIST))
		case controls.TracksRemoveFromPlaylist.Contains(keypress):
			cmds = append(cmds, model.Cmd(REMOVE_FROM_PLAYLIST))
		case controls.TracksBack.Contains(keypress):
			cmds = append(cmds, model.Cmd(BACK))
		case controls.TracksHide.Contains(keypress):
			m.Hidden = !m.Hidden
			cmds = append(cmds, model.Cmd(TOGGLE_VIEW))
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) Items() []Item {
	litems := m.list.Items()
	items := make([]Item, len(litems))
	for i := range litems {
		items[i] = litems[i].(Item)
	}
	return items
}

func (m *Model) SetItems(items []Item) tea.Cmd {
	newItems := make([]list.Item, len(items))
	for i := 0; i < len(items); i++ {
		newItems[i] = items[i]
	}
	return m.list.SetItems(newItems)
}

func (m *Model) InsertItem(index int, item Item) tea.Cmd {
	if index < 0 {
		index = len(m.list.Items()) + 1
	}
	return m.list.InsertItem(index, item)
}

func (m *Model) RemoveItem(index int) {
	m.list.RemoveItem(index)
}

func (m *Model) RemoveItemAfter(index int) tea.Cmd {
	items := m.list.Items()[:index+1]
	return m.list.SetItems(items)
}

func (m *Model) SetItem(index int, item Item) tea.Cmd {
	return m.list.SetItem(index, item)
}

func (m *Model) SelectedItem() Item {
	return m.list.SelectedItem().(Item)
}

func (m *Model) Index() int {
	return m.list.Index()
}

func (m *Model) Select(index int) {
	m.list.Select(index)
}

// BeginDrag takes hold of the track under the cursor: it is moved with the
// mouse from now on.
func (m *Model) BeginDrag(index int) bool {
	if index < 0 || index >= len(m.list.Items()) {
		return false
	}

	m.dragIndex = index
	m.dragStart = index
	return true
}

// Dragging reports whether a track is being dragged right now.
func (m *Model) Dragging() bool {
	return m.dragIndex >= 0
}

// DragTo moves the dragged track to the row under the cursor: the track
// follows the mouse and the others shift.
func (m *Model) DragTo(index int) {
	if m.dragIndex < 0 || index < 0 || index == m.dragIndex {
		return
	}

	items := m.list.Items()
	if index >= len(items) {
		return
	}

	dragged := items[m.dragIndex]

	rest := make([]list.Item, 0, len(items))
	rest = append(rest, items[:m.dragIndex]...)
	rest = append(rest, items[m.dragIndex+1:]...)

	moved := make([]list.Item, 0, len(items))
	moved = append(moved, rest[:index]...)
	moved = append(moved, dragged)
	moved = append(moved, rest[index:]...)

	m.list.SetItems(moved)
	m.dragIndex = index
}

// EndDrag finishes the drag and reports whether the tracks were moved.
func (m *Model) EndDrag() bool {
	moved := m.dragIndex >= 0 && m.dragIndex != m.dragStart

	m.dragIndex = -1
	m.dragStart = -1

	return moved
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.help.Width = m.width - 4
	m.list.SetWidth(m.width - 6)
	m.SetHeight(h)
}

func (m *Model) SetWidth(w int) {
	m.width = w
	m.help.Width = m.width - 4
	m.list.SetWidth(m.width - 6)
}

func (m *Model) Width() int {
	return m.width
}

func (m *Model) SetHeight(h int) {
	m.height = h
}

func (m *Model) Height() int {
	return m.height
}
