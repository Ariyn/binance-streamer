package main

import (
	"binance-api/pkg/binancews"
	"binance-api/pkg/pipeline"
	"binance-api/pkg/sink"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
)

func main() {
	sinkType := flag.String("sink", "console", "Type of sink: console, file, http")
	filePath := flag.String("out", "output.json", "Output file path (if sink=file)")
	httpURL := flag.String("url", "http://localhost:8080", "HTTP endpoint URL (if sink=http)")
	httpMethod := flag.String("method", "POST", "HTTP method (if sink=http)")
	httpContentType := flag.String("content-type", "application/json", "HTTP Content-Type (if sink=http)")
	flag.Parse()

	var dataSink sink.Sink
	var err error

	switch *sinkType {
	case "console":
		dataSink = sink.NewConsoleSink()
	case "file":
		dataSink, err = sink.NewFileSink(*filePath)
		if err != nil {
			log.Fatalf("Failed to create file sink: %v", err)
		}
	case "http":
		dataSink = sink.NewHttpSink(*httpURL, *httpMethod, *httpContentType)
	default:
		log.Fatalf("Unknown sink type: %s", *sinkType)
	}
	defer dataSink.Close()

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
	fmt.Printf("Using sink: %s\n", *sinkType)

	// Handle interrupt signal to gracefully shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		pipeline.Run(client.Messages(), client.Errors(), dataSink)
	}()

	<-interrupt
	fmt.Println("Interrupt received, closing connection...")
}
