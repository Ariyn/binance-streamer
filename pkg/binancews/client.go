package binancews

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	BaseURL     = "wss://stream.binance.com:9443/ws"
	ReadTimeout = 5 * time.Minute
)

// Client represents a Binance WebSocket client
type Client struct {
	conn  *websocket.Conn
	mu    sync.Mutex
	done  chan struct{}
	read  chan []byte
	errCh chan error
}

// SubscribeRequest represents the payload for subscribing to streams
type SubscribeRequest struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int      `json:"id"`
}

// NewClient creates a new Binance WebSocket client
func NewClient() *Client {
	return &Client{
		done:  make(chan struct{}),
		read:  make(chan []byte, 100),
		errCh: make(chan error, 10),
	}
}

// Connect establishes a connection to the Binance WebSocket API
func (c *Client) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(BaseURL, nil)
	if err != nil {
		return err
	}
	c.conn = conn

	// Set initial read deadline
	conn.SetReadDeadline(time.Now().Add(ReadTimeout))

	// Set Ping Handler to respond to server Pings and extend deadline
	conn.SetPingHandler(func(appData string) error {
		log.Println("Received ping, extending deadline")
		conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})

	go c.readLoop()
	return nil
}

// Subscribe sends a subscription request for the specified streams
func (c *Client) Subscribe(streams []string, id int) error {
	req := SubscribeRequest{
		Method: "SUBSCRIBE",
		Params: streams,
		ID:     id,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection not established")
	}

	return c.conn.WriteJSON(req)
}

// Unsubscribe sends an unsubscription request for the specified streams
func (c *Client) Unsubscribe(streams []string, id int) error {
	req := SubscribeRequest{
		Method: "UNSUBSCRIBE",
		Params: streams,
		ID:     id,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection not established")
	}

	return c.conn.WriteJSON(req)
}

func (c *Client) readLoop() {
	defer func() {
		if c.conn != nil {
			c.conn.Close()
		}
		close(c.read)
	}()

	for {
		select {
		case <-c.done:
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				// Only send error if not closed intentionally
				select {
				case <-c.done:
					return
				default:
					c.errCh <- err
					return
				}
			}
			c.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
			select {
			case c.read <- message:
			case <-c.done:
				return
			}
		}
	}
}

// Messages returns a channel to receive messages from the WebSocket
func (c *Client) Messages() <-chan []byte {
	return c.read
}

// Errors returns a channel to receive errors
func (c *Client) Errors() <-chan error {
	return c.errCh
}

// Close closes the WebSocket connection
func (c *Client) Close() error {
	close(c.done)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
