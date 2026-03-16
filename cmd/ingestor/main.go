// Package main implements the main entry point.
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

	"connectrpc.com/grpcreflect"
	"github.com/Grainbox/zenith/internal/config"
	"github.com/Grainbox/zenith/internal/engine"
	"github.com/Grainbox/zenith/internal/ingestor"
	"github.com/Grainbox/zenith/internal/repository/postgres"
	"github.com/Grainbox/zenith/internal/storage"
	"github.com/Grainbox/zenith/pkg/pb/proto/v1/protov1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 15 * time.Second
	drainTimeout      = 30 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("Application failure", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := setupLogger()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	db, err := initDatabase(cfg.Database, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Failed to close database connection", "error", err)
		} else {
			logger.Info("Database connection closed")
		}
	}()

	pipeline := setupPipeline(cfg, db, logger)
	pipeline.Start(context.Background())

	serverAddr, server := setupHTTPServer(cfg, logger, pipeline)

	// Listen for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		logger.Info("Starting Zenith Ingestor Server", "addr", serverAddr)
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
		logger.Error("HTTP server shutdown error", "error", err)
	}

	// Drain event pipeline and wait for workers to finish
	drainCtx, drainCancel := context.WithTimeout(context.Background(), drainTimeout)
	defer drainCancel()
	if err := pipeline.Stop(drainCtx); err != nil {
		logger.Warn("Pipeline did not drain within timeout", "error", err)
	}

	logger.Info("Server exited properly")
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

func setupPipeline(cfg *config.Config, db *sql.DB, logger *slog.Logger) *engine.Pipeline {
	ruleRepo := postgres.NewRuleRepo(db)
	sourceRepo := postgres.NewSourceRepo(db)
	evaluator := engine.NewEvaluator(ruleRepo, sourceRepo, logger)
	return engine.New(cfg.Engine.WorkerCount, cfg.Engine.EventBufferSize, evaluator, logger)
}

func setupHTTPServer(cfg *config.Config, logger *slog.Logger, pipeline *engine.Pipeline) (string, *http.Server) {
	srv := ingestor.NewServer(logger, pipeline)

	path, handler := protov1connect.NewIngestorServiceHandler(srv)
	reflector := grpcreflect.NewStaticReflector(
		protov1connect.IngestorServiceName,
	)

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	// Health check endpoint for Cloud Run probes
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status": "online", "version": "v1.0.0"}`))
	})

	serverAddr := ":" + cfg.Port
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	return serverAddr, server
}
