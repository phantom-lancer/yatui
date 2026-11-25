package mainpage

import (
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/log"
)

// applyOfflineMode switches the whole player between the online and the
// offline mode. The mode is applied immediately: the sidebar is rebuilt from
// the cache or from the API, and the client is dropped in the offline mode so
// no background request can slip through.
func (m *Model) applyOfflineMode() tea.Cmd {
	if m.isLoading {
		// the running load owns the client and the sidebar
		m.showToast("Дождитесь загрузки")
		return nil
	}
	config.Current.Offline = !config.Current.Offline
	m.saveConfig()

	if config.Current.Offline {
		m.client = nil
		// a playlist download and the next-track prefetch would keep using
		// the network; the offline mode must not
		atomic.AddInt64(&m.offlineDownloadGen, 1)
		m.clearPreload()
		m.stopDiscordTick()
		m.showToast("Офлайн-режим: вкл")
	} else {
		m.startClient()
		m.showToast("Офлайн-режим: выкл")
	}

	m.isLoading = true
	go m.initialLoad()
	return tea.Batch(m.spinner.Tick, tea.ClearScreen)
}

// startClient creates the API client from the saved token. The offline mode
// and a missing token leave the client empty, which every caller already
// handles as "nothing to request".
func (m *Model) startClient() {
	if config.Current.Offline || config.Current.Token == "" {
		m.client = nil
		return
	}

	client, err := api.NewClient(config.DirName, config.Current.Token)
	if err != nil {
		log.Print(log.LVL_ERROR, "client init error: %s", err)
		m.client = nil
		return
	}
	m.client = client
}

// offlineStatus is the value of the settings row.
func offlineStatus() string {
	return onOff(config.Current.Offline)
}
