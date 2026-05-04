package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"oldbeggar-refactor/internal/admin"
	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/runner"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	mode := flag.String("mode", "run", "run, once, refresh, or check")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	app, err := runner.New(cfg)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Admin.Enabled {
		adminServer, err := admin.New(cfg, app)
		if err != nil {
			log.Fatalf("init admin: %v", err)
		}
		if err := adminServer.Start(ctx); err != nil {
			log.Fatalf("start admin: %v", err)
		}
	}

	switch *mode {
	case "run":
		if err := app.Run(ctx); err != nil {
			log.Fatalf("run: %v", err)
		}
	case "once":
		if err := app.Bootstrap(ctx); err != nil {
			log.Fatalf("once: %v", err)
		}
	case "refresh":
		if err := app.RefreshSessions(ctx); err != nil {
			log.Fatalf("refresh: %v", err)
		}
	case "check":
		if err := app.CheckShop(ctx); err != nil {
			log.Fatalf("check: %v", err)
		}
	default:
		log.Fatalf("unknown mode %q", *mode)
	}
}
