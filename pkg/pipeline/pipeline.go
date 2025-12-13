package pipeline

import (
	"log"

	"binance-api/pkg/sink"
)

// Run starts the pipeline, reading from msgCh and writing to the sink.
// It blocks until errCh receives an error or msgCh is closed.
func Run(msgCh <-chan []byte, errCh <-chan error, s sink.Sink) {
	for {
		select {
		case msg, ok := <-msgCh:
			if !ok {
				return
			}
			if err := s.Write(msg); err != nil {
				log.Printf("Sink error: %v", err)
			}
		case err := <-errCh:
			log.Printf("Source error: %v", err)
			return
		}
	}
}
