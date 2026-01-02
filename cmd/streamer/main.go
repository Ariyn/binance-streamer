package main

import (
	"binance-api/pkg/binancews"
	"binance-api/pkg/config"
	"binance-api/pkg/pipeline"
	"binance-api/pkg/sink"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

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

	client := binancews.NewClient()

	healthAddr := ":8081"
	if cfg.Health != nil && cfg.Health.Addr != "" {
		healthAddr = cfg.Health.Addr
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if client.IsConnected() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready"))
	})

	srv := &http.Server{
		Addr:              healthAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("Health endpoints listening on %s (/livez, /readyz)", healthAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Health server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		fmt.Println("Interrupt received, shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
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
