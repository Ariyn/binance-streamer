package sink

import (
	"os"
	"sync"
)

// FileSink writes data to a file.
type FileSink struct {
	file *os.File
	mu   sync.Mutex
}

// NewFileSink creates a new FileSink.
func NewFileSink(path string) (*FileSink, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &FileSink{file: f}, nil
}

func (s *FileSink) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Append newline for readability
	_, err := s.file.Write(append(data, '\n'))
	return err
}

func (s *FileSink) Close() error {
	return s.file.Close()
}
