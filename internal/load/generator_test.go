package load

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				TargetURL: "http://localhost:8080",
				APIKey:    "test-key",
				RPS:       10,
				Duration:  1 * time.Minute,
				Workers:   5,
			},
			wantErr: false,
		},
		{
			name: "missing target URL",
			cfg: &Config{
				APIKey:   "test-key",
				RPS:      10,
				Duration: 1 * time.Minute,
				Workers:  5,
			},
			wantErr: true,
		},
		{
			name: "missing API key",
			cfg: &Config{
				TargetURL: "http://localhost:8080",
				RPS:       10,
				Duration:  1 * time.Minute,
				Workers:   5,
			},
			wantErr: true,
		},
		{
			name: "invalid RPS",
			cfg: &Config{
				TargetURL: "http://localhost:8080",
				APIKey:    "test-key",
				RPS:       0,
				Duration:  1 * time.Minute,
				Workers:   5,
			},
			wantErr: true,
		},
		{
			name: "invalid duration",
			cfg: &Config{
				TargetURL: "http://localhost:8080",
				APIKey:    "test-key",
				RPS:       10,
				Duration:  0,
				Workers:   5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGenerator_Run(t *testing.T) {
	// Create mock server
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/v1/events", r.URL.Path)
		require.Equal(t, "test-key", r.Header.Get("X-Api-Key"))

		// Read and discard body
		_, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()

		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := &Config{
		TargetURL: server.URL,
		APIKey:    "test-key",
		RPS:       100,
		Duration:  100 * time.Millisecond,
		Workers:   5,
		Verbose:   false,
	}

	gen := NewGenerator(cfg, logger)
	ctx := context.Background()

	err := gen.Run(ctx)
	require.NoError(t, err)
	require.Greater(t, gen.metrics.Sent, int64(0))
	require.Equal(t, int64(0), gen.metrics.Failed)
}

func TestGenerator_Run_WithCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cfg := &Config{
		TargetURL: server.URL,
		APIKey:    "test-key",
		RPS:       100,
		Duration:  5 * time.Second, // Long duration
		Workers:   5,
	}

	gen := NewGenerator(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel after short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := gen.Run(ctx)
	require.NoError(t, err)
	require.Greater(t, gen.metrics.Sent, int64(0))
	require.Less(t, gen.metrics.Sent, int64(500)) // Should stop early
}

func TestGenerator_GenerateEvent(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	gen := NewGenerator(&Config{}, logger)

	event := gen.generateEvent()

	require.NotEmpty(t, event.EventID)
	require.NotEmpty(t, event.EventType)
	require.NotEmpty(t, event.Source)
	require.NotEmpty(t, event.Payload)

	// Verify payload structure
	require.Contains(t, event.Payload, "severity")
	require.Contains(t, event.Payload, "value")
	require.Contains(t, event.Payload, "duration")
}
