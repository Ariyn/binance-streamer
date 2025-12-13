package main

import (
	"binance-api/pkg/binancews"
	"fmt"
	"log"
	"os"
	"os/signal"
)

func main() {
	client := binancews.NewClient()

	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Subscribe to btcusdt@aggTrade
	streams := []string{"btcusdt@aggTrade", "btcusdt@depth"}
	if err := client.Subscribe(streams, 1); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	fmt.Println("Subscribed to", streams)

	// Handle interrupt signal to gracefully shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		for {
			select {
			case msg := <-client.Messages():
				fmt.Printf("Received: %s\n", msg)
			case err := <-client.Errors():
				log.Printf("Error: %v", err)
				return
			}
		}
	}()

	<-interrupt
	fmt.Println("Interrupt received, closing connection...")
}
