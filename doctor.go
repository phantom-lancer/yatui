package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/equalizer"
	mainpage "github.com/phantom-lancer/yatui/ui/model/mainPage"
)

// doctorReport is the sanitized result of the self check. It never contains
// the token, the Last.fm keys, the proxy address or any other secret: the
// values are only reported as present or missing.
type doctorReport struct {
	Version   string `json:"version"`
	GoVersion string `json:"go"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Timestamp string `json:"timestamp"`

	Config doctorConfig `json:"config"`
	Cache  doctorCache  `json:"cache"`
	Log    doctorLog    `json:"log"`
	Player doctorPlayer `json:"player"`
	Issues []string     `json:"issues"`
	OK     bool         `json:"ok"`
}

type doctorConfig struct {
	Path        string `json:"path"`
	Exists      bool   `json:"exists"`
	Readable    bool   `json:"readable"`
	Parsed      bool   `json:"parsed"`
	Permissions string `json:"permissions,omitempty"`
	TokenSaved  bool   `json:"token_saved"`
	Error       string `json:"error,omitempty"`
}

type doctorCache struct {
	Path       string `json:"path"`
	Writable   bool   `json:"writable"`
	Tracks     int    `json:"tracks"`
	TotalBytes int64  `json:"total_bytes"`
	Partial    int    `json:"partial_files"`
}

type doctorLog struct {
	Path     string `json:"path"`
	Writable bool   `json:"writable"`
	Bytes    int64  `json:"bytes"`
}

type doctorPlayer struct {
	Theme            string `json:"theme"`
	Visualizer       bool   `json:"visualizer"`
	ResourceSaver    bool   `json:"resource_saver"`
	Offline          bool   `json:"offline"`
	EqualizerEnabled bool   `json:"equalizer_enabled"`
	EqualizerView    string `json:"equalizer_view"`
	EqualizerBands   int    `json:"equalizer_bands"`
	LastFM           bool   `json:"lastfm"`
	Discord          bool   `json:"discord"`
}

// runDoctor performs the self check and prints the report. It never changes
// the config, never deletes the cache and never touches the network session:
// the report is meant to be pasted into an issue.
func runDoctor(asJSON bool) int {
	report := buildDoctorReport()

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, "не удалось напечатать отчёт:", err)
			return 1
		}
	} else {
		printDoctorReport(report)
	}

	if report.OK {
		return 0
	}
	return 1
}

func buildDoctorReport() doctorReport {
	report := doctorReport{
		Version:   mainpage.Version,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Timestamp: time.Now().Format(time.RFC3339),
		Issues:    []string{},
	}

	// The report describes the configured cache, not the default one: a user
	// who moved the cache to another disk must see the real path.
	conf, err := config.LoadForDiagnostics()
	if err != nil {
		conf = config.Defaults()
	}
	cache.SetDir(conf.CacheDir)

	report.Config = inspectConfig(&report)
	report.Cache = inspectCache(&report)
	report.Log = inspectLog(&report)
	report.Player = inspectPlayer(conf)
	report.OK = len(report.Issues) == 0
	return report
}

func inspectConfig(report *doctorReport) doctorConfig {
	info := doctorConfig{Path: config.Path()}

	stat, err := os.Stat(info.Path)
	if err != nil {
		if os.IsNotExist(err) {
			report.Issues = append(report.Issues,
				"конфиг не найден: при первом запуске он будет создан со значениями по умолчанию")
			return info
		}
		info.Error = err.Error()
		report.Issues = append(report.Issues, "конфиг недоступен: "+err.Error())
		return info
	}

	info.Exists = true
	info.Permissions = fmt.Sprintf("%04o", stat.Mode().Perm())
	if stat.Mode().Perm()&0o077 != 0 {
		report.Issues = append(report.Issues,
			"файл конфига доступен другим пользователям; права будут исправлены при следующем запуске")
	}

	// load reads the file without writing anything: the doctor must not
	// migrate or rewrite the config.
	if _, err := os.ReadFile(info.Path); err != nil {
		info.Error = err.Error()
		report.Issues = append(report.Issues, "конфиг не читается: "+err.Error())
		return info
	}
	info.Readable = true

	// config.LoadForDiagnostics parses and migrates in memory only.
	conf, err := config.LoadForDiagnostics()
	if err != nil {
		info.Error = err.Error()
		report.Issues = append(report.Issues, "конфиг не разбирается: "+err.Error())
		return info
	}
	info.Parsed = true
	info.TokenSaved = conf.Token != ""

	if conf.Token == "" {
		report.Issues = append(report.Issues,
			"токен не сохранён: запустите yatui --login")
	}
	return info
}

func inspectCache(report *doctorReport) doctorCache {
	info := doctorCache{}
	dir, err := cache.Dir()
	if err != nil {
		report.Issues = append(report.Issues, "каталог кэша недоступен: "+err.Error())
		return info
	}
	info.Path = dir

	// the probe file is created next to the cached tracks and removed right
	// away: the cached tracks themselves are never touched.
	probe, err := os.CreateTemp(dir, "doctor-*.probe")
	if err != nil {
		report.Issues = append(report.Issues, "кэш недоступен для записи: "+err.Error())
	} else {
		info.Writable = true
		name := probe.Name()
		probe.Close()
		os.Remove(name)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		report.Issues = append(report.Issues, "кэш не читается: "+err.Error())
		return info
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch filepath.Ext(entry.Name()) {
		case ".mp3":
			if stat, err := entry.Info(); err == nil && stat.Size() > 0 {
				info.Tracks++
				info.TotalBytes += stat.Size()
			}
		case ".part":
			info.Partial++
		}
	}
	if info.Partial > 0 {
		report.Issues = append(report.Issues,
			fmt.Sprintf("незавершённых загрузок: %d; они будут убраны при следующем запуске", info.Partial))
	}
	return info
}

func inspectLog(report *doctorReport) doctorLog {
	info := doctorLog{}
	path := filepath.Join(os.TempDir(), config.DirName, config.DirName+".log")
	info.Path = path

	if stat, err := os.Stat(path); err == nil {
		info.Bytes = stat.Size()
	}

	probe, err := os.CreateTemp(filepath.Dir(path), "doctor-*.probe")
	if err != nil {
		report.Issues = append(report.Issues, "каталог логов недоступен для записи: "+err.Error())
		return info
	}
	info.Writable = true
	name := probe.Name()
	probe.Close()
	os.Remove(name)
	return info
}

func inspectPlayer(conf config.Config) doctorPlayer {

	view := "графический"
	bands := equalizer.GraphicBands
	if conf.Equalizer != nil && conf.Equalizer.ClassicView {
		view = "классический"
		bands = equalizer.Bands
	}

	player := doctorPlayer{
		Theme:          conf.Style.Theme,
		Visualizer:     conf.VisualizerEnabled,
		ResourceSaver:  conf.ResourceSaver,
		Offline:        conf.Offline,
		EqualizerView:  view,
		EqualizerBands: bands,
		LastFM:         conf.LastFM != nil && conf.LastFM.Enabled,
		Discord:        conf.Discord != nil && conf.Discord.Enabled,
	}
	if player.Theme == "" {
		player.Theme = "по конфигу"
	}
	if conf.Equalizer != nil {
		player.EqualizerEnabled = conf.Equalizer.Enabled
	}
	return player
}

func printDoctorReport(report doctorReport) {
	fmt.Printf("yatui %s — диагностика\n", report.Version)
	fmt.Printf("  сборка:      %s %s/%s\n", report.GoVersion, report.OS, report.Arch)
	fmt.Printf("  время:       %s\n", report.Timestamp)
	fmt.Println()

	fmt.Println("Конфиг:")
	fmt.Printf("  путь:        %s\n", report.Config.Path)
	fmt.Printf("  найден:      %s\n", yesNo(report.Config.Exists))
	fmt.Printf("  читается:    %s\n", yesNo(report.Config.Readable))
	fmt.Printf("  разбирается: %s\n", yesNo(report.Config.Parsed))
	if report.Config.Permissions != "" {
		fmt.Printf("  права:       %s\n", report.Config.Permissions)
	}
	fmt.Printf("  токен:       %s\n", presentAbsent(report.Config.TokenSaved))
	if report.Config.Error != "" {
		fmt.Printf("  ошибка:      %s\n", report.Config.Error)
	}
	fmt.Println()

	fmt.Println("Кэш:")
	fmt.Printf("  путь:        %s\n", report.Cache.Path)
	fmt.Printf("  запись:      %s\n", yesNo(report.Cache.Writable))
	fmt.Printf("  треков:      %d (%.1f МиБ)\n", report.Cache.Tracks, float64(report.Cache.TotalBytes)/(1<<20))
	fmt.Printf("  недокачано:  %d\n", report.Cache.Partial)
	fmt.Println()

	fmt.Println("Лог:")
	fmt.Printf("  путь:        %s\n", report.Log.Path)
	fmt.Printf("  запись:      %s\n", yesNo(report.Log.Writable))
	fmt.Printf("  размер:      %.1f КиБ\n", float64(report.Log.Bytes)/1024)
	fmt.Println()

	fmt.Println("Плеер:")
	fmt.Printf("  тема:        %s\n", report.Player.Theme)
	fmt.Printf("  визуализатор: %s\n", onOffLabel(report.Player.Visualizer))
	fmt.Printf("  экономия:    %s\n", onOffLabel(report.Player.ResourceSaver))
	fmt.Printf("  офлайн:      %s\n", onOffLabel(report.Player.Offline))
	fmt.Printf("  эквалайзер:  %s, %d полос, %s\n",
		onOffLabel(report.Player.EqualizerEnabled), report.Player.EqualizerBands, report.Player.EqualizerView)
	fmt.Printf("  Last.fm:     %s\n", onOffLabel(report.Player.LastFM))
	fmt.Printf("  Discord:     %s\n", onOffLabel(report.Player.Discord))
	fmt.Println()

	if report.OK {
		fmt.Println("Проблем не найдено.")
		return
	}
	fmt.Println("Найдено:")
	for _, issue := range report.Issues {
		fmt.Println("  • " + issue)
	}
}

func yesNo(value bool) string {
	if value {
		return "да"
	}
	return "нет"
}

func presentAbsent(value bool) string {
	if value {
		return "сохранён"
	}
	return "нет"
}

func onOffLabel(value bool) string {
	if value {
		return "вкл"
	}
	return "выкл"
}
