package sink

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

// HttpSink sends data to an HTTP endpoint.
type HttpSink struct {
	url         string
	method      string
	contentType string
	client      *http.Client
}

// NewHttpSink creates a new HttpSink.
func NewHttpSink(url, method, contentType string) *HttpSink {
	return &HttpSink{
		url:         url,
		method:      method,
		contentType: contentType,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *HttpSink) Write(data []byte) error {
	req, err := http.NewRequest(s.method, s.url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", s.contentType)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http sink received status code: %d", resp.StatusCode)
	}

	return nil
}

func (s *HttpSink) Close() error {
	// HTTP client doesn't need explicit closing of connections usually,
	// but we implement the interface.
	return nil
}
