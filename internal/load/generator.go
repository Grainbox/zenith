package load

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds load generator configuration
type Config struct {
	TargetURL string
	APIKey    string
	RPS       int           // Requests per second
	Duration  time.Duration // How long to generate events
	Workers   int           // Number of concurrent workers
	Verbose   bool          // Enable verbose logging
}

// Validate checks configuration validity
func (c *Config) Validate() error {
	if c.TargetURL == "" {
		return errors.New("target URL is required")
	}
	if c.APIKey == "" {
		return errors.New("API key is required")
	}
	if c.RPS <= 0 {
		return fmt.Errorf("RPS must be positive, got %d", c.RPS)
	}
	if c.Duration <= 0 {
		return fmt.Errorf("duration must be positive, got %v", c.Duration)
	}
	if c.Workers <= 0 {
		return fmt.Errorf("workers must be positive, got %d", c.Workers)
	}
	return nil
}

// Metrics tracks generator statistics
type Metrics struct {
	Sent      int64
	Failed    int64
	Retried   int64
	TotalTime time.Duration
}

// Generator creates synthetic load for testing
type Generator struct {
	cfg     *Config
	logger  *slog.Logger
	client  *http.Client
	metrics *Metrics
	eventID atomic.Int64
}

// NewGenerator creates a new load generator
func NewGenerator(cfg *Config, logger *slog.Logger) *Generator {
	return &Generator{
		cfg:    cfg,
		logger: logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		metrics: &Metrics{},
	}
}

// Run executes the load generation
func (g *Generator) Run(ctx context.Context) error {
	start := time.Now()

	// Create rate limiter
	ticker := time.NewTicker(time.Second / time.Duration(g.cfg.RPS))
	defer ticker.Stop()

	// Create worker pool
	eventChan := make(chan *Event, g.cfg.Workers*2)
	errChan := make(chan error, g.cfg.Workers)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < g.cfg.Workers; i++ {
		wg.Add(1)
		go g.worker(ctx, i, eventChan, errChan, &wg)
	}

	// Create events with rate limiting
	go func() {
		deadline := time.After(g.cfg.Duration)
		for {
			select {
			case <-ctx.Done():
				close(eventChan)
				return
			case <-deadline:
				close(eventChan)
				return
			case <-ticker.C:
				eventChan <- g.generateEvent()
			}
		}
	}()

	// Wait for all workers to finish
	wg.Wait()
	close(errChan)

	// Collect any errors, ignoring context cancellation (normal shutdown)
	var lastErr error
	for err := range errChan {
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			lastErr = err
			g.logger.Error("Worker error", "error", err)
		}
	}

	g.metrics.TotalTime = time.Since(start)
	g.printMetrics()

	return lastErr
}

// worker processes events from the channel
func (g *Generator) worker(ctx context.Context, id int, eventChan <-chan *Event, errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}

			if err := g.sendWithRetry(ctx, event); err != nil {
				errChan <- err
				if g.cfg.Verbose {
					g.logger.Error("Failed to send event", "worker_id", id, "error", err)
				}
			}
		}
	}
}

// sendWithRetry attempts to send an event with exponential backoff
func (g *Generator) sendWithRetry(ctx context.Context, event *Event) error {
	const maxRetries = 3
	backoff := 10 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := g.sendEvent(ctx, event); err == nil {
			atomic.AddInt64(&g.metrics.Sent, 1)
			return nil
		} else if attempt < maxRetries-1 {
			atomic.AddInt64(&g.metrics.Retried, 1)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	atomic.AddInt64(&g.metrics.Failed, 1)
	return fmt.Errorf("max retries exceeded for event %s", event.EventID)
}

// sendEvent sends a single event to the ingestor
func (g *Generator) sendEvent(ctx context.Context, event *Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.TargetURL+"/v1/events", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Api-Key", g.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

// generateEvent creates a random synthetic event
// #nosec G404: Using math/rand is acceptable for test event generation; cryptographic randomness not needed
func (g *Generator) generateEvent() *Event {
	sources := []string{"zenith-demo"}
	eventTypes := []string{"purchase", "refund", "error", "audit", "inventory"}
	severities := []string{"info", "warning", "error", "critical"}

	id := g.eventID.Add(1)

	return &Event{
		EventID:   fmt.Sprintf("evt_%d_%d", time.Now().UnixNano(), id),
		EventType: eventTypes[rand.IntN(len(eventTypes))],
		Source:    sources[rand.IntN(len(sources))],
		Payload: map[string]interface{}{
			"severity": severities[rand.IntN(len(severities))],
			"value":    rand.Float32() * 100,
			"duration": rand.IntN(5000),
		},
	}
}

// printMetrics logs the final statistics
func (g *Generator) printMetrics() {
	successRate := float64(0)
	if totalRequests := g.metrics.Sent + g.metrics.Failed; totalRequests > 0 {
		successRate = float64(g.metrics.Sent) / float64(totalRequests) * 100
	}

	rps := float64(g.metrics.Sent) / g.metrics.TotalTime.Seconds()

	g.logger.Info("Load generation complete",
		"sent", g.metrics.Sent,
		"failed", g.metrics.Failed,
		"retried", g.metrics.Retried,
		"success_rate", fmt.Sprintf("%.1f%%", successRate),
		"actual_rps", fmt.Sprintf("%.1f", rps),
		"duration", g.metrics.TotalTime.String(),
	)
}
