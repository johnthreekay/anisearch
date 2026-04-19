package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/john3k/anisearch/internal/api"
	"github.com/john3k/anisearch/internal/config"
	"github.com/john3k/anisearch/internal/qbit"
	"github.com/john3k/anisearch/internal/sonarr"
	"github.com/john3k/anisearch/internal/watcher"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	configPath := ""
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("AniSearch starting on port %d", cfg.Port)
	log.Printf("API Key: %s", cfg.APIKey)
	log.Printf("qBittorrent: %s", cfg.QBitURL)
	log.Printf("Sonarr: %s", cfg.SonarrURL)

	if cfg.NeedsSetup() {
		log.Printf("*** No password configured — visit the web UI to complete first-time setup ***")
	} else {
		log.Printf("Auth user: %s", cfg.Username)
	}

	qbitClient := qbit.NewClient(cfg.QBitURL, cfg.QBitUser, cfg.QBitPass, cfg.QBitCategory)
	sonarrClient := sonarr.NewClient(cfg.SonarrURL, cfg.SonarrAPIKey)

	w := watcher.New(qbitClient, sonarrClient, 30*time.Second)
	w.Start()
	defer w.Stop()

	server := api.NewServer(cfg)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Listening on %s", addr)
	log.Printf("Download watcher active (polling every 30s)")
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
