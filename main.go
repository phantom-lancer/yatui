package main

import (
	"crypto/x509"
	"fmt"
	"os"

	"github.com/phantom-lancer/yatui/api"
	"github.com/phantom-lancer/yatui/config"
	"github.com/phantom-lancer/yatui/log"
	"github.com/phantom-lancer/yatui/media"
	"github.com/phantom-lancer/yatui/ui/model"
	loginpage "github.com/phantom-lancer/yatui/ui/model/loginPage"
	mainpage "github.com/phantom-lancer/yatui/ui/model/mainPage"
	"github.com/phantom-lancer/yatui/ui/style"
)

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--version", "-v", "version":
			fmt.Println("yatui " + mainpage.Version)
			return
		}
	}

	log.Start()
	defer log.Stop()

	err := config.InitialLoad()
	if err != nil {
		log.Print(log.LVL_WARNIGN, "config load error: %s", err.Error())
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

	if config.Current.Token == "" {
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
