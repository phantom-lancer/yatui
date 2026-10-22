package mainpage

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/ui/components/playlist"
	"github.com/phantom-lancer/yatui/ui/model/hotkeys"
)

// TestWaveGroupsAreBuiltFromTheStations checks that the window offers the
// activities and the options of the character, the mood and the language.
func TestWaveGroupsAreBuiltFromTheStations(t *testing.T) {
	groups := waveGroups(testStations())

	if len(groups) != 4 {
		t.Fatalf("the window has %d groups, want 4", len(groups))
	}

	activity := groups[0]
	if activity.title != "По занятию" {
		t.Errorf("the first group is %q", activity.title)
	}
	// the wave without an activity comes first, the activities follow
	if len(activity.options) != 3 {
		t.Fatalf("the activities are %d, want 3", len(activity.options))
	}
	if activity.options[0].value != "" {
		t.Errorf("the first option is %q, want the wave without an activity", activity.options[0].value)
	}
	if activity.options[1].name != "Просыпаюсь" {
		t.Errorf("the first activity is %q", activity.options[1].name)
	}
	if activity.options[1].seed != "activity:wake-up" {
		t.Errorf("the seed of the activity is %q", activity.options[1].seed)
	}

	character := groups[1]
	if len(character.options) != 4 || character.options[0].value != "favorite" {
		t.Errorf("the character options are %+v", character.options)
	}
	if character.options[0].seed != "settingDiversity:favorite" {
		t.Errorf("the seed of the character is %q", character.options[0].seed)
	}

	mood := groups[2]
	if len(mood.options) != 5 || mood.options[0].value != "active" {
		t.Errorf("the mood options are %+v", mood.options)
	}

	language := groups[3]
	if len(language.options) != 4 || language.options[3].value != "without-words" {
		t.Errorf("the language options are %+v", language.options)
	}
}

// TestWaveSettingsChangeTheValues checks the keys of the window.
func TestWaveSettingsChangeTheValues(t *testing.T) {
	wave := &waveSettingsModel{}
	wave.setStations(testStations())

	// the cursor starts at the first group
	if wave.index != 0 {
		t.Fatalf("the cursor is at %d, want 0", wave.index)
	}

	wave.change(1)
	if got := wave.option("activity").value; got != "wake-up" {
		t.Errorf("the activity is %q, want wake-up", got)
	}

	// the change wraps around
	wave.change(-1)
	if got := wave.option("activity").value; got != "" {
		t.Errorf("the activity is %q, want the empty one", got)
	}

	// the cursor moves between the groups and the apply row
	wave.down()
	if wave.index != 1 {
		t.Fatalf("the cursor is at %d, want 1", wave.index)
	}
	wave.change(1)
	// the character starts at the option without a restriction, the change
	// wraps around to the first one
	if got := wave.option("diversity").value; got != "favorite" {
		t.Errorf("the character is %q, want favorite", got)
	}

	wave.down()
	wave.down()
	wave.down()
	wave.down()
	if wave.index != wave.applyRow() {
		t.Fatalf("the cursor is at %d, want the apply row", wave.index)
	}
	wave.down()
	if wave.index != 0 {
		t.Errorf("the cursor did not wrap around: %d", wave.index)
	}
}

// TestWaveSettingsCanBeReset makes sure that the settings can be put back to
// the default ones.
func TestWaveSettingsCanBeReset(t *testing.T) {
	wave := &waveSettingsModel{}
	wave.setStations(testStations())

	// something is chosen
	wave.change(1)
	if got := wave.option("activity").value; got != "wake-up" {
		t.Fatalf("the activity is %q, want wake-up", got)
	}

	wave.reset()

	if got := wave.option("activity").value; got != "" {
		t.Errorf("the activity is %q, want the empty one", got)
	}
	if got := wave.option("diversity").value; got != "default" {
		t.Errorf("the character is %q, want the default one", got)
	}
	if got := wave.option("mood").value; got != "all" {
		t.Errorf("the mood is %q, want the default one", got)
	}
	if got := wave.option("language").value; got != "any" {
		t.Errorf("the language is %q, want the default one", got)
	}
}

// TestWaveSettingsShowTheChosenOptions makes sure that the options of the
// config are marked when the window opens.
func TestWaveSettingsShowTheChosenOptions(t *testing.T) {
	previous := config.Current.Wave
	config.Current.Wave = &config.Wave{
		Activity:   "run",
		Diversity:  "popular",
		MoodEnergy: "calm",
		Language:   "without-words",
	}
	t.Cleanup(func() { config.Current.Wave = previous })

	wave := &waveSettingsModel{}
	wave.setStations(testStations())

	for key, want := range map[string]string{
		"activity":  "run",
		"diversity": "popular",
		"mood":      "calm",
		"language":  "without-words",
	} {
		if got := wave.option(key).value; got != want {
			t.Errorf("the option %q is %q, want %q", key, got, want)
		}
	}
}

// TestWaveSeeds checks the seeds that are sent to the server.
func TestWaveSeeds(t *testing.T) {
	seeds := waveSeeds("wake-up", "favorite", "active", "russian")
	want := []string{
		"user:onyourwave",
		"activity:wake-up",
		"settingMoodEnergy:active",
		"settingDiversity:favorite",
		"settingLanguage:russian",
	}

	if len(seeds) != len(want) {
		t.Fatalf("the seeds are %v, want %v", seeds, want)
	}
	for i := range want {
		if seeds[i] != want[i] {
			t.Errorf("the seed %d is %q, want %q", i, seeds[i], want[i])
		}
	}

	// the wave without any settings has only the radio itself
	if seeds := waveSeeds("", "", "", ""); len(seeds) != 1 || seeds[0] != "user:onyourwave" {
		t.Errorf("the plain wave has the seeds %v", seeds)
	}
}

// TestWaveSettingsAreStored checks that the chosen options end up in the
// config.
func TestWaveSettingsAreStored(t *testing.T) {
	previous := config.Current.Wave
	t.Cleanup(func() { config.Current.Wave = previous })

	// no settings are chosen yet: the defaults are used
	config.Current.Wave = nil

	m := &Model{}
	m.wave = &waveSettingsModel{}
	m.wave.setStations(testStations())
	m.wave.change(1)
	m.wave.down()
	m.wave.change(1)

	activity, diversity, mood, language := m.waveSelection()
	m.storeWaveSettings(activity, diversity, mood, language)

	if config.Current.Wave.Activity != "wake-up" {
		t.Errorf("the activity is %q", config.Current.Wave.Activity)
	}
	if config.Current.Wave.Diversity != "favorite" {
		t.Errorf("the character is %q", config.Current.Wave.Diversity)
	}
	if config.Current.Wave.MoodEnergy != "all" {
		t.Errorf("the mood is %q", config.Current.Wave.MoodEnergy)
	}
	if config.Current.Wave.Language != "any" {
		t.Errorf("the language is %q", config.Current.Wave.Language)
	}
}

// TestWaveWindowOpensAndCloses checks the hotkey of the window.
func TestWaveWindowOpensAndCloses(t *testing.T) {
	m := &Model{width: 100, height: 30}

	// the window opens with the hotkey
	m.Update(hotkeys.HotkeyMsg{Key: tea.KeyF10})
	if !m.isWaveActive {
		t.Fatal("the wave window did not open")
	}

	// the escape key closes it
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.isWaveActive {
		t.Error("the wave window did not close")
	}
}

// TestWaveWindowShowsTheOptions makes sure that the window shows every group
// and the apply row.
func TestWaveWindowShowsTheOptions(t *testing.T) {
	m := &Model{width: 100, height: 30}
	m.wave = &waveSettingsModel{}
	m.wave.setStations(testStations())
	m.isWaveActive = true

	view := strip(m.waveView())
	for _, want := range []string{"Настроить Мою волну", "По занятию", "По характеру", "По настроению", "По языку", "Применить"} {
		if !strings.Contains(view, want) {
			t.Errorf("the window does not show %q", want)
		}
	}
}

// TestWaveContextMenuHasTheItem makes sure that the main wave can be tuned
// with the mouse.
func TestWaveContextMenuHasTheItem(t *testing.T) {
	m := &Model{width: 100, height: 30}

	item := &playlist.Item{Name: "my wave", Kind: playlist.MYWAVE, Active: true, Rotor: true}
	m.openPlaylistMenu(item, 10, 10)

	if m.contextMenu == nil {
		t.Fatal("the context menu is missing")
	}

	found := false
	for _, entry := range m.contextMenu.items {
		if entry.label == "Настроить Мою волну" {
			found = true
		}
	}
	if !found {
		t.Errorf("the menu has no wave item: %+v", m.contextMenu.items)
	}
}

// testStations builds a short list of stations like the API returns it.
func testStations() []api.StationDesc {
	var stations []api.StationDesc

	activity := func(tag, name string) api.StationDesc {
		var desc api.StationDesc
		desc.Station.Id = api.StationId{Type: "activity", Tag: tag}
		desc.Station.Name = name
		return desc
	}

	stations = append(stations, activity("wake-up", "Просыпаюсь"), activity("run", "Бег"))

	var desc api.StationDesc
	desc.Station.Id = api.StationId{Type: "genre", Tag: "pop"}
	desc.Station.Name = "Поп"

	add := func(list *[]api.StationOption, value, name, seed string, unspecified bool) {
		*list = append(*list, api.StationOption{
			Value:          value,
			Name:           name,
			SerializedSeed: seed,
			Unspecified:    unspecified,
		})
	}
	add(&desc.Station.Restrictions2.Diversity.PossibleValues, "favorite", "Любимое", "settingDiversity:favorite", false)
	add(&desc.Station.Restrictions2.Diversity.PossibleValues, "discover", "Незнакомое", "settingDiversity:discover", false)
	add(&desc.Station.Restrictions2.Diversity.PossibleValues, "popular", "Популярное", "settingDiversity:popular", false)
	add(&desc.Station.Restrictions2.Diversity.PossibleValues, "default", "Любое", "settingDiversity:default", true)
	add(&desc.Station.Restrictions2.MoodEnergy.PossibleValues, "active", "Бодрое", "settingMoodEnergy:active", false)
	add(&desc.Station.Restrictions2.MoodEnergy.PossibleValues, "fun", "Весёлое", "settingMoodEnergy:fun", false)
	add(&desc.Station.Restrictions2.MoodEnergy.PossibleValues, "calm", "Спокойное", "settingMoodEnergy:calm", false)
	add(&desc.Station.Restrictions2.MoodEnergy.PossibleValues, "sad", "Грустное", "settingMoodEnergy:sad", false)
	add(&desc.Station.Restrictions2.MoodEnergy.PossibleValues, "all", "Любое", "settingMoodEnergy:all", true)
	add(&desc.Station.Restrictions2.Language.PossibleValues, "russian", "Русский", "settingLanguage:russian", false)
	add(&desc.Station.Restrictions2.Language.PossibleValues, "not-russian", "Иностранный", "settingLanguage:not-russian", false)
	add(&desc.Station.Restrictions2.Language.PossibleValues, "any", "Любой", "settingLanguage:any", true)
	add(&desc.Station.Restrictions2.Language.PossibleValues, "without-words", "Без слов", "settingLanguage:without-words", false)

	return append(stations, desc)
}
