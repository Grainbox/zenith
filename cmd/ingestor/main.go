// Package main implements the main entry point.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Grainbox/zenith/internal/ingestor"
	"github.com/Grainbox/zenith/pkg/pb/proto/v1/protov1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	serverPort         = ":50051"
	readHeaderTimeout  = 10 * time.Second
	shutdownTimeout    = 15 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("Application failure", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	srv := ingestor.NewServer(logger)

	path, handler := protov1connect.NewIngestorServiceHandler(srv)

	mux := http.NewServeMux()
	mux.Handle(path, handler)

	server := &http.Server{
		Addr:              serverPort,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	// Listen for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		logger.Info("Starting Zenith Ingestor Server", "addr", serverPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server failure", "error", err)
		}
	}()

	// Wait for signal
	sig := <-stop
	logger.Info("Shutting down server...", "signal", sig.String())

	// Shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return err
	}

	logger.Info("Server exited properly")
	return nil
}
