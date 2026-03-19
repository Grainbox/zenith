// Package main implements a load generator for the Zenith Ingestor.
// It generates realistic event streams for performance testing and dashboard visualization.
//
// Usage:
//   go run cmd/load-generator/main.go -target http://localhost:8080 -key test-api-key -rps 50 -duration 5m
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Grainbox/zenith/internal/load"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Load generator failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse configuration
	cfg := parseFlags()

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	logger.Info("Starting load generator",
		"target", cfg.TargetURL,
		"rps", cfg.RPS,
		"duration", cfg.Duration,
		"workers", cfg.Workers,
	)

	// Create generator
	gen := load.NewGenerator(cfg, logger)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start generator in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- gen.Run(ctx)
	}()

	// Wait for completion or signal
	select {
	case <-sigChan:
		logger.Info("Shutdown signal received, draining...")
		cancel()
		select {
		case err := <-errChan:
			return err
		case <-time.After(10 * time.Second):
			return fmt.Errorf("shutdown timeout exceeded")
		}
	case err := <-errChan:
		return err
	}
}

func parseFlags() *load.Config {
	cfg := &load.Config{
		Workers: 10,
	}

	flag.StringVar(&cfg.TargetURL, "target", "http://localhost:8080", "Target ingestor URL")
	flag.StringVar(&cfg.APIKey, "key", "", "API key for authentication")
	flag.IntVar(&cfg.RPS, "rps", 10, "Target requests per second")
	flag.DurationVar(&cfg.Duration, "duration", 1*time.Minute, "How long to generate events")
	flag.IntVar(&cfg.Workers, "workers", 10, "Number of concurrent workers")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose logging")

	flag.Parse()

	return cfg
}
