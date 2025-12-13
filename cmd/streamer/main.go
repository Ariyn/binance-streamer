package main

import (
	"binance-api/pkg/binancews"
	"binance-api/pkg/config"
	"binance-api/pkg/pipeline"
	"binance-api/pkg/sink"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var dataSink sink.Sink

	switch cfg.Sink.Type {
	case "console":
		dataSink = sink.NewConsoleSink()
	case "file":
		if cfg.Sink.File == nil {
			log.Fatal("File sink configuration missing")
		}
		dataSink, err = sink.NewFileSink(cfg.Sink.File.Path)
		if err != nil {
			log.Fatalf("Failed to create file sink: %v", err)
		}
	case "http":
		if cfg.Sink.Http == nil {
			log.Fatal("HTTP sink configuration missing")
		}
		dataSink = sink.NewHttpSink(cfg.Sink.Http.URL, cfg.Sink.Http.Method, cfg.Sink.Http.ContentType)
	default:
		log.Fatalf("Unknown sink type: %s", cfg.Sink.Type)
	}
	defer dataSink.Close()

	client := binancews.NewClient()

	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	if err := client.Subscribe(cfg.Streams, 1); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	fmt.Println("Subscribed to", cfg.Streams)
	fmt.Printf("Using sink: %s\n", cfg.Sink.Type)

	// Handle interrupt signal to gracefully shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		pipeline.Run(client.Messages(), client.Errors(), dataSink)
	}()

	<-interrupt
	fmt.Println("Interrupt received, closing connection...")
}
