package main

import (
	"fmt"
	"runtime/debug"
	"time"

	mainpage "github.com/phantom-lancer/yatui/ui/model/mainPage"
	"github.com/phantom-lancer/yatui/ui/style"
)

// splash is the "yaTUI" sign shown in the console after the player closes.
const splash = `
██╗   ██╗ █████╗ ████████╗██╗   ██╗██╗
╚██╗ ██╔╝██╔══██╗╚══██╔══╝██║   ██║██║
 ╚████╔╝ ███████║   ██║   ██║   ██║██║
  ╚██╔╝  ██╔══██║   ██║   ██║   ██║██║
   ██║   ██║  ██║   ██║   ╚██████╔╝██║
   ╚═╝   ╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚═╝
`

// printGoodbye leaves a nice sign in the console instead of the last frame of
// the player, with the version and the commit the build came from.
func printGoodbye() {
	fmt.Println(style.AccentTextStyle.Render(splash))
	fmt.Println("  " + style.DialogHelpStyle.Render("Яндекс.Музыка в терминале"))
	fmt.Println("  " + style.DialogHelpStyle.Render(buildLabel()))
	fmt.Println()
}

// buildLabel is the version plus the commit and the date of the build, so a
// build from the middle of a fix series can be told apart from a release.
func buildLabel() string {
	label := mainpage.Version

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return label
	}

	var revision, date string
	var dirty bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			date = setting.Value
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}

	if len(revision) > 7 {
		revision = revision[:7]
	}
	if revision != "" {
		label += " · " + revision
	}
	if dirty {
		label += "+dirty"
	}
	if date != "" {
		if built, err := time.Parse(time.RFC3339, date); err == nil {
			label += " · " + built.Format("2006-01-02")
		}
	}

	return label
}
