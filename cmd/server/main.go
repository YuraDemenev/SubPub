package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"subpun/internal/app"
	"subpun/internal/config"
	"subpun/internal/server"
	"subpun/proto/protogen"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	addr := fmt.Sprintf(":%d", cfg.GRPC.Port)
	// Run server in a goroutine
	go func() {
		app.Logger.Infof("Starting gRPC server on %s", addr)
		if err := srv.Run(cfg.GRPC.Port, ctx); err != nil && err != context.Canceled {
			app.Logger.Errorf("Server failed: %v", err)
		}
	}()

	// Create gRPC client
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		app.Logger.Errorf("Failed to dial server: %v", err)
		log.Fatalf("Failed to dial server: %v", err)
	}
	defer conn.Close()

	client := protogen.NewPubSubClient(conn)
	app.Logger.Info("gRPC client connected")

	// Subscribe to topic
	received := make(chan string, 1)
	subscribeErr := make(chan error, 1)

	stream, err := client.Subscribe(ctx, &protogen.SubscribeRequest{Key: "test"})
	if err != nil {
		return
	}
	app.Logger.Info("Subscribed to topic: test")

	go func() {
		for {
			event, err := stream.Recv()
			if err != nil {
				subscribeErr <- fmt.Errorf("subscribe failed: %v", err)
				return
			}
			received <- event.Data
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
