package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vtodo2vevent/internal/caldav"
	"vtodo2vevent/internal/config"
	syncer "vtodo2vevent/internal/sync"
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

	sourceClient := caldav.NewClient(cfg.Source.BaseURL, cfg.Source.Username, cfg.Source.Password, location)
	targetClient := caldav.NewClient(cfg.Target.BaseURL, cfg.Target.Username, cfg.Target.Password, location)
	reconciler := syncer.NewReconciler(sourceClient, targetClient, cfg)
	service := syncer.NewService(reconciler, cfg.Sync.PollIntervalDuration())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := service.Run(ctx); err != nil {
		log.Printf("sync service failed: %v", err)
		os.Exit(1)
	}

	if cfg.Sync.PollIntervalDuration() > 0 {
		log.Printf("sync service stopped")
		return
	}
	log.Printf("sync completed")
}
