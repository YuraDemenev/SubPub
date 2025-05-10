package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"subpun/internal/app"
	"subpun/internal/config"
	"subpun/internal/server"
	"syscall"
	"time"
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

	srv := server.New(app)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run server in a goroutine
	go func() {
		if err := srv.Run(cfg.GRPC.Port, ctx); err != nil && err != context.Canceled {
			app.Logger.Errorf("Server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	app.Logger.Info("Received shutdown signal, stopping server")

	// Trigger graceful shutdown
	cancel()

	// Wait for server to stop
	time.Sleep(1 * time.Second)
	app.Logger.Info("Server stopped")
}
