package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/phantom-lancer/yatui/equalizer"
)

const _SAVE_DELAY = 2 * time.Second

var Current Config

var (
	// the delayed write lives in its own goroutine, so the state is guarded
	saveMu    sync.Mutex
	saveTimer *time.Timer
	saveData  []byte
)

// Defaults returns the configuration a fresh install starts with. The doctor
// uses it when no config file exists yet.
func Defaults() Config {
	conf := defaultConfig
	if defaultConfig.Equalizer != nil {
		eq := *defaultConfig.Equalizer
		eq.Gains = append([]float64(nil), defaultConfig.Equalizer.Gains...)
		eq.GraphicGains = append([]float64(nil), defaultConfig.Equalizer.GraphicGains...)
		conf.Equalizer = &eq
	}
	return conf
}

// Dir returns the directory the config lives in.
func Dir() (string, error) {
	return getDir()
}

// LoadForDiagnostics reads and migrates the config in memory without writing
// anything. The doctor uses it: a diagnostic run must not rewrite the file.
func LoadForDiagnostics() (Config, error) {
	return load()
}

func InitialLoad() error {
	var err error

	Current, err = load()
	if err != nil {
		configDir, err := getDir()
		if err != nil {
			return err
		}

		if oldToken, err := os.ReadFile(filepath.Join(configDir, "token")); err == nil {
			Current.Token = string(oldToken)
		}

		save(Current)
	}

	return nil
}

func getDir() (string, error) {
	userDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(userDir, ".config", DirName)
	err = os.MkdirAll(configDir, 0700)
	if err != nil {
		return "", err
	}
	// An old install created the directory world readable. Tighten it on
	// every start; a failure is not fatal because Windows has no mode bits.
	_ = os.Chmod(configDir, 0700)

	return configDir, nil
}

func load() (Config, error) {
	configDir, err := getDir()
	if err != nil {
		return defaultConfig, err
	}

	configPath := filepath.Join(configDir, "config.yaml")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return defaultConfig, err
	}
	// Older versions left the file world readable; the token inside it is
	// private. Windows ignores the mode, so the error is not fatal.
	_ = os.Chmod(configPath, 0600)

	var newConfig Config
	err = yaml.Unmarshal(configContent, &newConfig)
	if err != nil {
		return defaultConfig, err
	}

	// Missing winter keys receive defaults, while an explicit intensity of
	// zero remains the user's way to turn snowfall completely off.
	var configFields map[string]yaml.Node
	if yaml.Unmarshal(configContent, &configFields) == nil {
		if _, ok := configFields["winter-intensity"]; !ok {
			newConfig.WinterIntensity = defaultConfig.WinterIntensity
		}
		if _, ok := configFields["winter-garland"]; !ok {
			newConfig.WinterGarland = defaultConfig.WinterGarland
		}
		if _, ok := configFields["preload-next-track"]; !ok {
			newConfig.PreloadNextTrack = defaultConfig.PreloadNextTrack
		}
		if _, ok := configFields["winter-speed"]; !ok {
			newConfig.WinterSpeed = defaultConfig.WinterSpeed
		}
		if _, ok := configFields["visualizer"]; !ok {
			newConfig.VisualizerEnabled = defaultConfig.VisualizerEnabled
		}
		if _, ok := configFields["winter-density"]; !ok {
			// Preserve the old 0..3 snow intensity when migrating an existing config.
			switch newConfig.WinterIntensity {
			case 0:
				newConfig.WinterDensity = 0
			case 1:
				newConfig.WinterDensity = 20
			case 2:
				newConfig.WinterDensity = 40
			case 3:
				newConfig.WinterDensity = 75
			default:
				newConfig.WinterDensity = defaultConfig.WinterDensity
			}
		}
	}

	if newConfig.BufferSize <= 0 {
		newConfig.BufferSize = defaultConfig.BufferSize
	} else if newConfig.BufferSize < 180 {
		newConfig.BufferSize = 180
	} else if newConfig.BufferSize > 500 {
		newConfig.BufferSize = 500
	}

	if newConfig.WinterSpeed < 10 || newConfig.WinterSpeed > 100 {
		newConfig.WinterSpeed = defaultConfig.WinterSpeed
	}
	if newConfig.WinterDensity < 0 || newConfig.WinterDensity > 100 {
		newConfig.WinterDensity = defaultConfig.WinterDensity
	}

	if newConfig.VolumeStep == 0 {
		newConfig.VolumeStep = defaultConfig.VolumeStep
	}

	if newConfig.Search == nil {
		search := *defaultConfig.Search
		newConfig.Search = &search
	}

	// missing in the config file means "enabled", so an existing config
	// keeps working exactly as before this option appeared
	if newConfig.Normalization == nil {
		normalization := *defaultConfig.Normalization
		newConfig.Normalization = &normalization
	}

	if newConfig.LastFM == nil {
		lastfm := *defaultConfig.LastFM
		newConfig.LastFM = &lastfm
	}

	if newConfig.Wave == nil {
		wave := *defaultConfig.Wave
		newConfig.Wave = &wave
	}

	// A config written by the older versions carries the old Discord defaults;
	// such a config was never tuned by the user, so it gets the new defaults
	// once. Edited values are kept.
	if newConfig.Discord == nil || *newConfig.Discord == legacyDiscord {
		discord := *defaultConfig.Discord
		newConfig.Discord = &discord
	}

	if newConfig.Equalizer == nil {
		eq := *defaultConfig.Equalizer
		eq.Gains = append([]float64(nil), defaultConfig.Equalizer.Gains...)
		eq.GraphicGains = append([]float64(nil), defaultConfig.Equalizer.GraphicGains...)
		newConfig.Equalizer = &eq
	} else {
		// the gains must cover all the bands: an old config may have fewer
		if len(newConfig.Equalizer.Gains) < equalizer.Bands {
			gains := make([]float64, equalizer.Bands)
			copy(gains, newConfig.Equalizer.Gains)
			newConfig.Equalizer.Gains = gains
		}
		if newConfig.Equalizer.Preset == "" {
			newConfig.Equalizer.Preset = defaultConfig.Equalizer.Preset
		}

		// A config written before the graphic equalizer has no fifteen band
		// values. They are derived from the ten band curve once, so the
		// sound does not jump and the user's tuning is not lost. The original
		// ten band values stay untouched for the classic view.
		if len(newConfig.Equalizer.GraphicGains) != equalizer.GraphicBands {
			legacy := equalizer.NormalizeGains(newConfig.Equalizer.Gains)
			resampled := equalizer.ResampleGains(
				equalizer.BandFrequencies[:],
				legacy[:],
				equalizer.GraphicFrequencies[:],
			)
			newConfig.Equalizer.GraphicGains = append([]float64(nil), resampled[:]...)
		}
		if newConfig.Equalizer.GraphicPreset == "" {
			// A custom legacy curve has no matching modern preset.
			if newConfig.Equalizer.Preset == equalizer.CustomPreset {
				newConfig.Equalizer.GraphicPreset = equalizer.CustomPreset
			} else {
				newConfig.Equalizer.GraphicPreset = "Плоский"
			}
		}
	}

	if newConfig.Controls == nil {
		controls := *defaultConfig.Controls
		newConfig.Controls = &controls
	} else {
		fillDefault(newConfig.Controls, defaultConfig.Controls)
		if newConfig.Controls.Quit.IsEmpty() {
			newConfig.Controls.Quit = defaultConfig.Controls.Quit
		}
	}

	if newConfig.Style == nil {
		style := *defaultConfig.Style
		newConfig.Style = &style
	} else {
		if newConfig.Style.Icons == nil {
			icons := *defaultConfig.Style.Icons
			newConfig.Style.Icons = &icons
		} else {
			fillDefault(newConfig.Style.Icons, defaultConfig.Style.Icons)
		}
		if newConfig.Style.Colors == nil {
			colors := *defaultConfig.Style.Colors
			newConfig.Style.Colors = &colors
		} else {
			fillDefault(newConfig.Style.Colors, defaultConfig.Style.Colors)
		}
		if newConfig.Style.SidePanelWidth == 0 {
			newConfig.Style.SidePanelWidth = defaultConfig.Style.SidePanelWidth
		}
		// a named theme overrides the colors from the config file
		if newConfig.Style.Theme != "" {
			if colors, ok := ThemeColors(newConfig.Style.Theme); ok {
				newConfig.Style.Colors = &colors
			} else {
				newConfig.Style.Theme = ""
			}
		}
	}

	return newConfig, nil
}

func fillDefault(target, values any) {
	targetStruct := reflect.ValueOf(target).Elem()
	defaultStruct := reflect.ValueOf(values).Elem()
	for i := 0; i < targetStruct.NumField(); i++ {
		field := targetStruct.Field(i)
		if field.IsZero() || field.String() == "" {
			field.Set(defaultStruct.Field(i))
		}
	}
}

func save(conf Config) error {
	data, err := encode(conf)
	if err != nil {
		return err
	}
	return writeFile(data)
}

// encode turns the config into the YAML bytes. It runs on the calling
// goroutine, so a delayed write never touches a config that is being changed.
func encode(conf Config) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(4)
	if err := enc.Encode(conf); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeFile writes the config to a temporary file next to the real one and
// renames it. A crash in the middle leaves the old config intact instead of a
// truncated file with no token.
func writeFile(data []byte) error {
	configDir, err := getDir()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(configDir, "config-*.yaml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	// the data must be on the disk before the rename, otherwise a power loss
	// can still leave an empty file in place of the config
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(configDir, "config.yaml")); err != nil {
		return err
	}

	tmpName = ""
	return nil
}

// DefaultDiscord returns the Discord settings of a fresh config.
func DefaultDiscord() Discord {
	return *defaultConfig.Discord
}

func Path() string {
	configDir, err := getDir()
	if err != nil {
		return ""
	}

	return filepath.Join(configDir, "config.yaml")
}

// Save writes the config right away. It is used where the answer matters
// (the login page) and clears any delayed write that is still pending.
func Save() error {
	discardPending()
	return save(Current)
}

// SaveLater writes the config after a short pause. The volume and the lyrics
// toggle change the config on every key press, and rewriting the file for
// each press is slow and risky; the newest state simply replaces the pending
// one. The app calls Flush on exit.
func SaveLater() {
	data, err := encode(Current)
	if err != nil {
		return
	}

	saveMu.Lock()
	defer saveMu.Unlock()

	saveData = data
	if saveTimer == nil {
		saveTimer = time.AfterFunc(_SAVE_DELAY, flushPending)
	} else {
		saveTimer.Reset(_SAVE_DELAY)
	}
}

// Flush writes the pending config right now. It is called on exit, so a
// volume change made a second before closing is not lost.
func Flush() {
	saveMu.Lock()
	if saveTimer != nil {
		saveTimer.Stop()
		saveTimer = nil
	}
	data := saveData
	saveData = nil
	saveMu.Unlock()

	if data != nil {
		_ = writeFile(data)
	}
}

func discardPending() {
	saveMu.Lock()
	if saveTimer != nil {
		saveTimer.Stop()
		saveTimer = nil
	}
	saveData = nil
	saveMu.Unlock()
}

func flushPending() {
	saveMu.Lock()
	data := saveData
	saveData = nil
	saveTimer = nil
	saveMu.Unlock()

	if data != nil {
		_ = writeFile(data)
	}
}

func Reset() error {
	discardPending()
	var err error
	Current, err = load()
	return err
}
