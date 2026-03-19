package dispatcher

import "fmt"

// Registry maps sink type identifiers to Sink implementations.
// It is built once at startup and is read-only during operation.
type Registry struct {
	sinks map[string]Sink
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{sinks: make(map[string]Sink)}
}

// Register adds a Sink to the registry under the given type key.
// Panics if the type key is already registered — wiring errors must be caught at startup.
func (r *Registry) Register(sinkType string, sink Sink) {
	if _, exists := r.sinks[sinkType]; exists {
		panic(fmt.Sprintf("dispatcher: sink type %q already registered", sinkType))
	}
	r.sinks[sinkType] = sink
}

// Resolve returns the Sink registered for the given type, or false if not found.
func (r *Registry) Resolve(sinkType string) (Sink, bool) {
	s, ok := r.sinks[sinkType]
	return s, ok
}
