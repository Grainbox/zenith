package sinks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Grainbox/zenith/internal/domain"
)

// HttpSink POSTs matched events as JSON to the URL in rule.TargetAction.
// It is the generic fallback sink for any HTTP endpoint that speaks Zenith's event format.
type HttpSink struct {
	client *http.Client
}

// NewHttpSink creates an HttpSink with the given HTTP client.
func NewHttpSink(client *http.Client) *HttpSink {
	return &HttpSink{client: client}
}

// Name returns the sink name.
func (s *HttpSink) Name() string { return "http" }

// Send POSTs the matched event as JSON to the target URL.
func (s *HttpSink) Send(ctx context.Context, matched *domain.MatchedEvent) error {
	body, err := json.Marshal(matched)
	if err != nil {
		return fmt.Errorf("http sink: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, matched.Rule.TargetAction, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http sink: failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("http sink: request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http sink: unexpected status %d from %s", resp.StatusCode, matched.Rule.TargetAction)
	}
	return nil
}
