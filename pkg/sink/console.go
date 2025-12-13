package sink

import (
	"fmt"
	"os"
)

// ConsoleSink writes data to stdout.
type ConsoleSink struct{}

// NewConsoleSink creates a new ConsoleSink.
func NewConsoleSink() *ConsoleSink {
	return &ConsoleSink{}
}

func (s *ConsoleSink) Write(data []byte) error {
	_, err := fmt.Fprintf(os.Stdout, "%s\n", data)
	return err
}

func (s *ConsoleSink) Close() error {
	return nil
}
