package sinks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Grainbox/zenith/internal/domain"
	"github.com/google/uuid"
)

func TestDiscordSink_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}

		var payload discordPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}

		if len(payload.Embeds) == 0 {
			t.Fatal("expected at least one embed")
		}

		embed := payload.Embeds[0]
		if embed.Title == "" {
			t.Fatal("expected embed title to be set")
		}

		if len(embed.Fields) == 0 {
			t.Fatal("expected at least one field in embed")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sink := NewDiscordSink(server.Client())
	matched := &domain.MatchedEvent{
		Event: &domain.Event{
			ID:     "test-event-1",
			Type:   "test",
			Source: "test-source",
		},
		Rule: &domain.Rule{
			ID:           uuid.New(),
			Name:         "Test Rule",
			TargetAction: server.URL,
			SinkType:     "discord",
		},
	}

	err := sink.Send(context.Background(), matched)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestDiscordSink_Send_NonNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Discord expects 204 No Content on success
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sink := NewDiscordSink(server.Client())
	matched := &domain.MatchedEvent{
		Event: &domain.Event{
			ID:     "test-event-1",
			Type:   "test",
			Source: "test-source",
		},
		Rule: &domain.Rule{
			ID:           uuid.New(),
			Name:         "Test Rule",
			TargetAction: server.URL,
			SinkType:     "discord",
		},
	}

	err := sink.Send(context.Background(), matched)
	if err == nil {
		t.Fatal("expected error for non-204 status, got nil")
	}
}
