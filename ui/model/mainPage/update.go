package mainpage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/updater"
)

type updateCheckMsg struct {
	release   *updater.Release
	available bool
	err       error
}

type updateDownloadMsg struct {
	path string
	err  error
}

func (m *Model) updateStatusText() string {
	if m.updateStatus == "" {
		return "не проверено"
	}
	return m.updateStatus
}

func (m *Model) refreshSettingsItems() {
	if m.settings == nil {
		return
	}
	index := m.settings.index
	m.settings.items = m.buildSettings()
	if index >= len(m.settings.items) {
		index = len(m.settings.items) - 1
	}
	if index < 0 {
		index = 0
	}
	m.settings.index = index
}

// checkUpdatesCmd performs a single request after an explicit user action.
// No updater goroutine, timer or HTTP request exists before this command runs.
func (m *Model) checkUpdatesCmd() tea.Cmd {
	if m.updateChecking || m.updateDownloading {
		return nil
	}
	m.updateChecking = true
	m.updateStatus = "проверка..."
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 14*time.Second)
		defer cancel()
		release, available, err := updater.CheckLatest(ctx, Version, config.Current.Proxy)
		return updateCheckMsg{release: release, available: available, err: err}
	}
}

func (m *Model) handleUpdateCheck(msg updateCheckMsg) tea.Cmd {
	m.updateChecking = false
	switch {
	case msg.err != nil:
		m.updateRelease = nil
		m.updateStatus = "ошибка проверки"
		log.Print(log.LVL_WARNIGN, "update check failed: %s", msg.err)
		m.showToast("Не удалось проверить обновления: " + shortUpdateError(msg.err))
	case msg.available && msg.release != nil:
		m.updateRelease = msg.release
		m.updateStatus = "доступна " + msg.release.TagName
		m.showToast("Доступно обновление " + msg.release.TagName)
	default:
		m.updateRelease = nil
		m.updateStatus = "актуально"
		m.showToast("YATUI обновлён до последней версии")
	}
	m.refreshSettingsItems()
	return nil
}

func (m *Model) downloadUpdateCmd() tea.Cmd {
	if m.updateRelease == nil || m.updateDownloading || m.updateChecking {
		return nil
	}
	release := m.updateRelease
	m.updateDownloading = true
	m.updateStatus = "скачивание..."
	return func() tea.Msg {
		executable, err := os.Executable()
		if err != nil {
			return updateDownloadMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cancel()
		path, err := updater.DownloadVerified(ctx, release, filepath.Dir(executable), config.Current.Proxy)
		return updateDownloadMsg{path: path, err: err}
	}
}

func (m *Model) handleUpdateDownload(msg updateDownloadMsg) tea.Cmd {
	m.updateDownloading = false
	if msg.err != nil {
		m.updateStatus = "ошибка загрузки"
		log.Print(log.LVL_WARNIGN, "update download failed: %s", msg.err)
		m.showToast("Не удалось скачать обновление: " + shortUpdateError(msg.err))
		m.refreshSettingsItems()
		return nil
	}

	if err := updater.InstallCurrentExecutable(msg.path, os.Args[1:]); err != nil {
		_ = os.Remove(msg.path)
		m.updateStatus = "ошибка установки"
		log.Print(log.LVL_WARNIGN, "update installer could not start: %s", err)
		m.showToast("Не удалось запустить установку: " + shortUpdateError(err))
		m.refreshSettingsItems()
		return nil
	}

	m.updateStatus = "перезапуск..."
	m.showToast("Обновление установлено. Перезапускаю YATUI…")
	return tea.Quit
}

func shortUpdateError(err error) string {
	if err == nil {
		return ""
	}
	value := []rune(strings.TrimSpace(err.Error()))
	if len(value) > 90 {
		value = append(value[:87], '.', '.', '.')
	}
	return string(value)
}
