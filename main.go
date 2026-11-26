package main

import (
	"crypto/x509"
	"fmt"
	"os"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/cache"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/media"
	"github.com/phantom-lancer/yatui/ui/model"
	loginpage "github.com/phantom-lancer/yatui/ui/model/loginPage"
	mainpage "github.com/phantom-lancer/yatui/ui/model/mainPage"
	"github.com/phantom-lancer/yatui/ui/style"
)

func main() {
	args := os.Args[1:]
	options := parseArgs(args)
	if hasHelpArg(args) {
		printUsage()
		return
	}
	if options.showVersion {
		fmt.Println("yatui " + mainpage.Version)
		return
	}
	if options.doctor {
		os.Exit(runDoctor(options.doctorJSON))
	}
	forceLogin := options.forceLogin

	// the sign stays in the console when the player is closed
	defer printGoodbye()

	log.Start()
	defer log.Stop()

	err := config.InitialLoad()
	if err != nil {
		log.Print(log.LVL_WARNIGN, "config load error: %s", err.Error())
	}
	// the delayed writes must reach the disk before the app exits
	defer config.Flush()

	// The cache directory is resolved once and then shared by the download
	// workers, the audio thread and the UI. Passing it explicitly removes the
	// reads of the mutable config from those goroutines.
	cache.SetDir(config.Current.CacheDir)
	if removed := cache.PrunePartials(); removed > 0 {
		log.Print(log.LVL_INFO, "removed %d interrupted downloads", removed)
	}

	// The command line flag applies to this run only; the saved setting is
	// left as the user set it in the settings window.
	if options.offline {
		config.Current.Offline = true
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		log.Print(log.LVL_WARNIGN, "failed to obtain system certificate pool: %s", err.Error())
	}

	if certPool != nil && len(config.Current.SSLCerts) > 0 {
		for _, certPath := range config.Current.SSLCerts {
			certBytes, err := os.ReadFile(certPath)
			if err != nil {
				log.Print(log.LVL_WARNIGN, "failed to load certificate at '%s': %s", certPath, err.Error())
				continue
			}
			if !certPool.AppendCertsFromPEM(certBytes) {
				log.Print(log.LVL_WARNIGN, "failed to parse certificate at '%s'", certPath)
				continue
			}
		}
	}

	style.Apply(config.Current.Style)
	api.SetupClient(config.Current.Proxy, certPool)

	// --login always opens the sign-in page, even when the offline mode is
	// saved: the user explicitly asked for it. A normal offline start with a
	// saved token goes straight to the player.
	if forceLogin || (!config.Current.Offline && config.Current.Token == "") {
		if forceLogin {
			log.Print(log.LVL_INFO, "login page requested from the command line")
		}
		err = loginpage.New().Run()
		if err != nil {
			log.Print(log.LVL_PANIC, err.Error())
			model.PrettyExit(err, 4)
		}
	}

	mediaHandler := media.NewHandler(config.DirName, "Yandex music terminal client")
	page := mainpage.New(mediaHandler)
	err = mediaHandler.Start(page.Run)
	if err != nil {
		log.Print(log.LVL_PANIC, err.Error())
		model.PrettyExit(err, 6)
	}
}

// cliOptions is the parsed command line.
type cliOptions struct {
	// forceLogin shows the login page even when a token is already saved, so
	// the browser sign-in can be used without losing the token.
	forceLogin bool
	// showVersion prints the version and exits.
	showVersion bool
	// doctor runs the self check and exits.
	doctor bool
	// doctorJSON selects the machine readable report.
	doctorJSON bool
	// offline starts the player without the Yandex API: only cached tracks
	// are listed and played.
	offline bool
}

// parseArgs reads the command line. Unknown flags are ignored, so a typo in
// an optional flag never stops the player from starting.
func parseArgs(args []string) cliOptions {
	var options cliOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version", "-v", "version":
			options.showVersion = true
		case "--login", "-login":
			options.forceLogin = true
		case "--doctor", "doctor":
			options.doctor = true
		case "--json":
			options.doctorJSON = true
		case "--offline", "offline":
			options.offline = true
		}
	}
	return options
}

// hasHelpArg reports whether the usage was asked for.
func hasHelpArg(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--help", "-help", "-h", "help":
			return true
		}
	}
	return false
}

// printUsage prints the command line help.
func printUsage() {
	fmt.Println(`yatui — клиент Яндекс.Музыки в терминале

Использование:
  yatui [флаги]

Флаги:
  --login        показать экран входа, даже если токен уже сохранён
  --offline      работать только с сохранёнными треками, без сети
  --doctor       проверить конфиг, кэш, права и настройки
  --doctor --json  тот же отчёт в JSON (без секретов)
  --version      показать версию и выйти
  --help         показать эту справку`)
}
