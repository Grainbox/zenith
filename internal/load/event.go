package load

// Event represents a synthetic event for load testing
type Event struct {
	EventID   string                 `json:"event_id"`
	EventType string                 `json:"event_type"`
	Source    string                 `json:"source"`
	Payload   map[string]interface{} `json:"payload"`
}
