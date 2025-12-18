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
	"sync"
	"time"
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

	var client *binancews.Client
	var clientMu sync.Mutex
	shutdown := false

	go func() {
		<-interrupt
		fmt.Println("Interrupt received, closing connection...")
		clientMu.Lock()
		shutdown = true
		if client != nil {
			client.Close()
		}
		clientMu.Unlock()
	}()

	fmt.Printf("Using %d sinks\n", len(sinks))

	for {
		clientMu.Lock()
		if shutdown {
			clientMu.Unlock()
			break
		}
		client = binancews.NewClient()
		clientMu.Unlock()

		if err := client.Connect(); err != nil {
			log.Printf("Failed to connect: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if err := client.Subscribe(cfg.Streams, 1); err != nil {
			log.Printf("Failed to subscribe: %v. Retrying in 5s...", err)
			client.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		fmt.Println("Subscribed to", cfg.Streams)

		pipeline.Run(client.Messages(), client.Errors(), dataSink)

		clientMu.Lock()
		if shutdown {
			clientMu.Unlock()
			break
		}
		clientMu.Unlock()

		log.Println("Connection lost. Reconnecting in 1s...")
		time.Sleep(1 * time.Second)
	}
}
