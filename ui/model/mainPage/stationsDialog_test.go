package mainpage

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/phantom-lancer/yatui/api"
)

// testStationList builds a short list of stations like the API returns it.
func testStationList() []api.StationDesc {
	var list []api.StationDesc

	add := func(kind, tag, name string) {
		var desc api.StationDesc
		desc.Station.Id = api.StationId{Type: kind, Tag: tag}
		desc.Station.Name = name
		list = append(list, desc)
	}

	add("genre", "pop", "Поп")
	add("genre", "rock", "Рок")
	add("mood", "sad", "Грустное")
	add("activity", "run", "Бег")
	add("local-language", "russian", "Русский")

	return list
}

// TestStationsDialogFilters checks the search of the stations.
func TestStationsDialogFilters(t *testing.T) {
	dialog := &stationsDialogModel{}
	dialog.setStations(testStationList())

	if len(dialog.items) != 5 {
		t.Fatalf("the window has %d stations, want 5", len(dialog.items))
	}

	dialog.input.SetValue("ро")
	dialog.applyFilter()

	// only "Рок" matches
	if len(dialog.items) != 1 {
		t.Fatalf("the filter left %d stations, want 1", len(dialog.items))
	}

	dialog.input.SetValue("русский")
	dialog.applyFilter()
	if len(dialog.items) != 1 || dialog.items[0].Station.Name != "Русский" {
		t.Fatalf("the filter left %d stations, want the Russian one", len(dialog.items))
	}

	dialog.input.SetValue("нет такого")
	dialog.applyFilter()
	if len(dialog.items) != 0 {
		t.Fatalf("the filter left %d stations, want none", len(dialog.items))
	}
}

// TestStationsDialogSelection checks that the cursor walks over the stations
// and skips the names of the groups.
func TestStationsDialogSelection(t *testing.T) {
	dialog := &stationsDialogModel{}
	dialog.setStations(testStationList())

	station, ok := dialog.selected()
	if !ok || station.Station.Name != "Поп" {
		t.Fatalf("the first station is %+v", station.Station.Name)
	}

	dialog.down()
	station, _ = dialog.selected()
	if station.Station.Name != "Рок" {
		t.Fatalf("the second station is %q", station.Station.Name)
	}

	// the name of the group is not a station
	dialog.down()
	station, _ = dialog.selected()
	if station.Station.Name != "Грустное" {
		t.Fatalf("the station after the group is %q", station.Station.Name)
	}

	dialog.up()
	station, _ = dialog.selected()
	if station.Station.Name != "Рок" {
		t.Fatalf("the cursor did not skip the group: %q", station.Station.Name)
	}
}

// TestStationsDialogMouseLayout checks that the rows of the window are found
// where they are drawn.
func TestStationsDialogMouseLayout(t *testing.T) {
	m := &Model{width: 100, height: 30}
	m.stations = &stationsDialogModel{}
	m.stations.setStations(testStationList())
	m.isStationsActive = true

	view := ansi.Strip(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.stationsView()))
	lines := strings.Split(view, "\n")

	// the layout is measured while the window is drawn
	layout := m.stationsLayout
	if !layout.valid {
		t.Fatal("the layout is not measured")
	}

	// the first station is under the cursor
	row, ok := layout.rowAt(layout.rowX+2, layout.rowY)
	if !ok {
		t.Fatalf("the row %d is not found", layout.rowY)
	}

	name := ""
	if row < len(m.stations.lines) {
		name = m.stations.lines[row].text
	}
	if !strings.Contains(lines[layout.rowY], name) {
		t.Errorf("the row %d shows %q, want %q", layout.rowY, lines[layout.rowY], name)
	}
}

// TestStationsDialogOpensAndCloses checks the keys of the window.
func TestStationsDialogOpensAndCloses(t *testing.T) {
	m := &Model{width: 100, height: 30}

	m.openStations()
	if !m.isStationsActive {
		t.Fatal("the window did not open")
	}

	// the stations are loaded in the background
	m.stations.setStations(testStationList())

	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.isStationsActive {
		t.Error("the window did not close on esc")
	}
}
