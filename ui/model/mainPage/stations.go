package mainpage

import (
	"sort"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
)

// stationsCache holds the fetched Yandex Music stations.
// The list never changes during a session, so it is requested once.
var (
	stationsMu     sync.Mutex
	stationsCache  []api.StationDesc
	stationsLoaded bool
)

// loadStations returns the available stations, fetching them on the first call.
func loadStations(client *api.YaMusicClient) []api.StationDesc {
	stationsMu.Lock()
	defer stationsMu.Unlock()

	if stationsLoaded {
		return stationsCache
	}

	if client == nil {
		return nil
	}

	stations, err := client.Stations("ru")
	if err != nil {
		log.Print(log.LVL_ERROR, "failed to obtain the stations list: %s", err)
		return nil
	}

	// only the real stations are useful: skip the ones without a name
	// or with an id that cannot be started
	filtered := make([]api.StationDesc, 0, len(stations))
	for _, station := range stations {
		if station.Station.Name == "" || station.Station.Id.Tag == "" {
			continue
		}
		if station.Station.Id.Type == "" {
			continue
		}
		filtered = append(filtered, station)
	}

	// the main wave always first, the rest alphabetically
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i].Station, filtered[j].Station
		if a.Id.Tag == api.MyWaveId.Tag {
			return true
		}
		if b.Id.Tag == api.MyWaveId.Tag {
			return false
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	stationsCache = filtered
	stationsLoaded = true

	return stationsCache
}

// cachedStations returns the stations that are already fetched. It never
// goes to the network: the names of the settings are needed on every repaint.
func cachedStations() []api.StationDesc {
	stationsMu.Lock()
	defer stationsMu.Unlock()

	return stationsCache
}

// openStationPicker shows the window with the available stations.
func (m *Model) openStationPicker(x, y int) tea.Cmd {
	if len(loadStations(m.client)) == 0 {
		m.tracker.ShowError("stations list")
		return nil
	}

	return m.openStations()
}

// stationSessionMsg carries a started station back to the UI loop.
type stationSessionMsg struct {
	item *playlist.Item
	err  bool
}

// startStation begins a radio session for the given station.
func (m *Model) startStation(desc api.StationDesc) tea.Cmd {
	client := m.client
	if client == nil {
		return nil
	}

	m.isLoading = true

	go func() {
		defer m.Send(LOADING_DONE)

		session, err := client.RotorNewSession(desc.Station.Id)
		if err != nil {
			log.Print(log.LVL_ERROR, "failed to start the [%s] station: %s", desc.Station.Name, err)
			m.Send(stationSessionMsg{err: true})
			return
		}
		if len(session.Sequence) == 0 {
			m.Send(stationSessionMsg{err: true})
			return
		}

		item := &playlist.Item{
			Name:         desc.Station.Name,
			Kind:         playlist.MYWAVE,
			Active:       true,
			Subitem:      false,
			Rotor:        true,
			Label:        "волна",
			SessionId:    session.RadioSessionId,
			SessionBatch: session.BatchId,
			Tracks:       []api.Track{session.Sequence[0].Track},
		}
		if len(session.AcceptedSeeds) > 0 {
			item.StationId = session.AcceptedSeeds[0]
		} else {
			item.StationId = session.Id
		}

		m.Send(stationSessionMsg{item: item})
	}()

	return m.spinner.Tick
}

// applyStationSession adds the started station to the sidebar.
func (m *Model) applyStationSession(msg stationSessionMsg) tea.Cmd {
	if msg.err || msg.item == nil {
		m.tracker.ShowError("station session")
		return nil
	}
	m.insertSidebarItem(msg.item)
	return nil
}
