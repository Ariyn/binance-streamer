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

	var sinks []sink.Sink
	for _, sCfg := range cfg.Sinks {
		switch sCfg.Type {
		case "console":
			sinks = append(sinks, sink.NewConsoleSink())
		case "file":
			if sCfg.File == nil {
				log.Fatal("File sink configuration missing")
			}
			fs, err := sink.NewFileSink(sCfg.File.Path)
			if err != nil {
				log.Fatalf("Failed to create file sink: %v", err)
			}
			sinks = append(sinks, fs)
		case "http":
			if sCfg.Http == nil {
				log.Fatal("HTTP sink configuration missing")
			}
			sinks = append(sinks, sink.NewHttpSink(sCfg.Http.URL, sCfg.Http.Method, sCfg.Http.ContentType))
		default:
			log.Fatalf("Unknown sink type: %s", sCfg.Type)
		}
	}

	dataSink := sink.NewMultiSink(sinks)
	defer dataSink.Close()

	// Handle interrupt signal to gracefully shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	client := binancews.NewClient()

	go func() {
		<-interrupt
		fmt.Println("Interrupt received, closing connection...")
		_ = client.Close()
	}()

	fmt.Printf("Using %d sinks\n", len(sinks))

	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	if err := client.Subscribe(cfg.Streams, 1); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	fmt.Println("Subscribed to", cfg.Streams)
	pipeline.Run(client.Messages(), client.Errors(), dataSink)
}
