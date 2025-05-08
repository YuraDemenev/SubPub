package main

import (
	"log"
	"subpun/internal/app"
	"subpun/internal/config"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	app := app.NewApp(cfg)
	//Log info
	app.Logger.Info("Application started")
	app.Logger.Infof("gRPC Port: %d", cfg.GRPC.Port)
	app.Logger.Infof("Logging Level: %s", cfg.Logging.Level)
}
