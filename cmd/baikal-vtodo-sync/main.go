package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"baikal-vtodo-sync/internal/caldav"
	"baikal-vtodo-sync/internal/config"
	syncer "baikal-vtodo-sync/internal/sync"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("load config failed: %v", err)
		os.Exit(1)
	}

	location, err := time.LoadLocation(cfg.Sync.Timezone)
	if err != nil {
		log.Printf("load timezone failed: %v", err)
		os.Exit(1)
	}

	client := caldav.NewClient(cfg.Server.BaseURL, cfg.Server.Username, cfg.Server.Password, location)
	reconciler := syncer.NewReconciler(client, cfg)

	if err := reconciler.Run(context.Background()); err != nil {
		log.Printf("sync failed: %v", err)
		os.Exit(1)
	}

	log.Printf("sync completed")
}
