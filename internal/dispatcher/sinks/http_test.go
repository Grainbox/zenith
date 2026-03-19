package sinks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/google/uuid"
)

func TestHttpSink_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		var payload interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sink := NewHttpSink(server.Client())
	matched := &domain.MatchedEvent{
		Event: &domain.Event{
			ID:     "test-event-1",
			Type:   "test",
			Source: "test-source",
		},
		Rule: &domain.Rule{
			ID:           uuid.New(),
			TargetAction: server.URL,
			SinkType:     "http",
		},
	}

	err := sink.Send(context.Background(), matched)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestHttpSink_Send_NonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	sink := NewHttpSink(server.Client())
	matched := &domain.MatchedEvent{
		Event: &domain.Event{
			ID:     "test-event-1",
			Type:   "test",
			Source: "test-source",
		},
		Rule: &domain.Rule{
			ID:           uuid.New(),
			TargetAction: server.URL,
			SinkType:     "http",
		},
	}

	err := sink.Send(context.Background(), matched)
	if err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
}

func TestHttpSink_Send_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	sink := NewHttpSink(client)
	matched := &domain.MatchedEvent{
		Event: &domain.Event{
			ID:     "test-event-1",
			Type:   "test",
			Source: "test-source",
		},
		Rule: &domain.Rule{
			ID:           uuid.New(),
			TargetAction: server.URL,
			SinkType:     "http",
		},
	}

	err := sink.Send(context.Background(), matched)
	if err == nil {
		t.Fatal("expected error for timeout, got nil")
	}
}
