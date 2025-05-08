package main

import (
	"fmt"
	"log"
	"subpun/internal/config"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("gRPC Port: %d\n", cfg.GRPC.Port)
	fmt.Printf("Logging Level: %s\n", cfg.Logging.Level)
	fmt.Printf("Message Channel Size: %d\n", cfg.SubPub.MessageChannelSize)
}
