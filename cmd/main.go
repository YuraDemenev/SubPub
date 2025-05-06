package main

import (
	"context"
	"fmt"
	"subpun/subpub"
	"time"
)

func main() {
	sp := subpub.NewSubPub()

	// Subscriber 1: Fast
	_, err := sp.Subscribe("user_registered", func(msg interface{}) {
		fmt.Println("Subscriber 1: Received:", msg)
	})
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Subscriber 2: Slow
	_, err = sp.Subscribe("user_registered", func(msg interface{}) {
		time.Sleep(1 * time.Second)
		fmt.Println("Subscriber 2: Received:", msg)
	})
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Publish messages
	sp.Publish("user_registered", "user123")
	sp.Publish("user_registered", "user456")

	// Allow some time for processing
	time.Sleep(2 * time.Second)

	// Close with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := sp.Close(ctx); err != nil {
		fmt.Println("Close error:", err)
	}

	fmt.Println("Program finished")
}
