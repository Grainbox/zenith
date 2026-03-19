package sinks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Grainbox/zenith/internal/domain"
)

// discordPayload is the JSON body expected by Discord Incoming Webhooks.
type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title  string         `json:"title"`
	Color  int            `json:"color"`  // decimal RGB
	Fields []discordField `json:"fields"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// DiscordSink sends matched events as Discord embeds to the webhook URL in rule.TargetAction.
type DiscordSink struct {
	client *http.Client
}

// NewDiscordSink creates a DiscordSink with the given HTTP client.
func NewDiscordSink(client *http.Client) *DiscordSink {
	return &DiscordSink{client: client}
}

// Name returns the sink name.
func (s *DiscordSink) Name() string { return "discord" }

// Send formats the matched event as a Discord embed and POSTs it to the webhook URL.
func (s *DiscordSink) Send(ctx context.Context, matched *domain.MatchedEvent) error {
	payload := discordPayload{
		Embeds: []discordEmbed{
			{
				Title: fmt.Sprintf("Rule matched: %s", matched.Rule.Name),
				Color: 0xE74C3C, // red — signals an alert
				Fields: []discordField{
					{Name: "Event ID", Value: matched.Event.ID, Inline: true},
					{Name: "Source", Value: matched.Event.Source, Inline: true},
					{Name: "Rule", Value: matched.Rule.Name, Inline: false},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("discord sink: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, matched.Rule.TargetAction, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord sink: failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord sink: request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Discord returns 204 No Content on success
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("discord sink: unexpected status %d", resp.StatusCode)
	}
	return nil
}
