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
	// Run server in a goroutine
	go func() {
		if err := srv.Run(cfg.GRPC.Port, ctx); err != nil && err != context.Canceled {
			app.Logger.Errorf("Server failed: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)
	addr := fmt.Sprintf(":%d", cfg.GRPC.Port)
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

	// Check for subscription errors
	select {
	case err := <-subscribeErr:
		app.Logger.Errorf("Subscription error: %v", err)
		log.Fatalf("Subscription error: %v", err)
	default:
	}

	// Publish a message
	_, err = client.Publish(ctx, &protogen.PublishRequest{Key: "test", Data: "hello"})
	if err != nil {
		app.Logger.Errorf("Publish failed: %v", err)
		log.Fatalf("Publish failed: %v", err)
	}
	app.Logger.Info("Published message: hello")

	// Main loop for handling events
	for {
		select {

		case err := <-subscribeErr:
			app.Logger.Errorf("Subscription error: %v", err)
			log.Fatalf("Subscription error: %v", err)

		case msg := <-received:
			app.Logger.Infof("Received message: %s", msg)
			if msg != "hello" {
				app.Logger.Errorf("Expected message 'hello', got '%s'", msg)
				log.Fatalf("Expected message 'hello', got '%s'", msg)
			}
			app.Logger.Info("Message verified, waiting for shutdown signal...")
			// Continue to wait for shutdown signal

		case sig := <-sigChan:
			app.Logger.Infof("Received signal: %v, stopping server...", sig)
			cancel()
			time.Sleep(1 * time.Second)
			app.Logger.Info("Server stopped")
			return

		default:
			// Non-blocking check for stream message
			event, err := stream.Recv()
			if err != nil {
				subscribeErr <- fmt.Errorf("stream receive failed: %v", err)
				continue
			}
			received <- event.Data
		}
	}
}
