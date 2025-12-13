package sink

import (
	"fmt"
	"strings"
)

// MultiSink broadcasts data to multiple sinks.
type MultiSink struct {
	sinks []Sink
}

// NewMultiSink creates a new MultiSink.
func NewMultiSink(sinks []Sink) *MultiSink {
	return &MultiSink{sinks: sinks}
}

func (s *MultiSink) Write(data []byte) error {
	var errs []string
	for _, sink := range s.sinks {
		if err := sink.Write(data); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("multi sink errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (s *MultiSink) Close() error {
	var errs []string
	for _, sink := range s.sinks {
		if err := sink.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("multi sink close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}
