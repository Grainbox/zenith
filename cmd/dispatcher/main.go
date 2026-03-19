// Package main implements the Dispatcher entry point.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Grainbox/zenith/internal/config"
	"github.com/Grainbox/zenith/internal/dispatcher"
	"github.com/Grainbox/zenith/internal/dispatcher/sinks"
	"github.com/Grainbox/zenith/internal/domain"
	"github.com/Grainbox/zenith/internal/repository/postgres"
	"github.com/Grainbox/zenith/internal/storage"
)

const (
	drainTimeout      = 30 * time.Second
	shutdownTimeout   = 15 * time.Second
	readHeaderTimeout = 10 * time.Second
	matchBufSize      = 256
)

// commit is injected at build time via -ldflags "-X main.commit=<git-sha>"
var commit = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("Dispatcher failure", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := setupLogger()

	cfg, err := config.Load("dispatcher", "8081")
	if err != nil {
		return err
	}

	db, err := initDatabase(cfg.Database, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Failed to close database connection", "error", err)
		}
	}()

	matchCh := make(chan *domain.MatchedEvent, matchBufSize)

	httpClient := &http.Client{Timeout: 10 * time.Second}

	registry := dispatcher.NewRegistry()
	registry.Register("http", sinks.NewHttpSink(httpClient))
	registry.Register("discord", sinks.NewDiscordSink(httpClient))

	auditLogRepo := postgres.NewAuditLogRepo(db)
	d := dispatcher.New(matchCh, 4, registry, auditLogRepo, logger)
	d.Start(context.Background())

	serverAddr := ":" + cfg.Port
	server := setupHTTPServer(serverAddr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("Starting Dispatcher server", "addr", serverAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server failure", "error", err)
		}
	}()

	sig := <-stop
	logger.Info("Shutting down dispatcher...", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	// Close dispatch channel and wait for workers to finish
	close(matchCh)

	drainCtx, drainCancel := context.WithTimeout(context.Background(), drainTimeout)
	defer drainCancel()

	if err := d.Stop(drainCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		logger.Warn("Dispatcher drain incomplete", "error", err)
	}

	logger.Info("Dispatcher exited properly")
	return nil
}

func setupLogger() *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	return logger
}

func initDatabase(cfg config.DatabaseConfig, logger *slog.Logger) (*sql.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := storage.NewDatabase(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	logger.Info("Database connected successfully")
	return db, nil
}

func setupHTTPServer(addr string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"online","component":"dispatcher","commit":"` + commit + `"}`))
	})

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}
}
