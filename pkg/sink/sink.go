package sink

// Sink defines the interface for data destinations.
type Sink interface {
	// Write handles a single message from the source.
	Write(data []byte) error
	// Close cleans up resources (e.g., closing files).
	Close() error
}
