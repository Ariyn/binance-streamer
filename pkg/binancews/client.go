package binancews

import (
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
	conn *websocket.Conn
	mu   sync.Mutex

	done      chan struct{}
	closeOnce sync.Once

	read  chan []byte
	errCh chan error

	started bool

	// active stream subscriptions, used for resubscribing after reconnect
	subs map[string]struct{}

	reconnectMin time.Duration
	reconnectMax time.Duration
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
		done:         make(chan struct{}),
		read:         make(chan []byte, 100),
		errCh:        make(chan error, 10),
		subs:         make(map[string]struct{}),
		reconnectMin: 1 * time.Second,
		reconnectMax: 30 * time.Second,
	}
}

// Connect establishes a connection to the Binance WebSocket API
func (c *Client) Connect() error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = true
	c.mu.Unlock()

	conn, err := c.dial()
	if err != nil {
		c.mu.Lock()
		c.started = false
		c.mu.Unlock()
		return err
	}
	c.setConn(conn)

	go c.run()
	return nil
}

func (c *Client) dial() (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(BaseURL, nil)
	if err != nil {
		return nil, err
	}

	// Set initial read deadline
	conn.SetReadDeadline(time.Now().Add(ReadTimeout))

	// Set Ping Handler to respond to server Pings and extend deadline
	conn.SetPingHandler(func(appData string) error {
		log.Println("Received ping, extending deadline")
		conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})

	return conn, nil
}

func (c *Client) setConn(conn *websocket.Conn) {
	c.mu.Lock()
	old := c.conn
	c.conn = conn
	c.mu.Unlock()

	if old != nil {
		_ = old.Close()
	}
}

func (c *Client) getConn() *websocket.Conn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn
}

// IsConnected reports whether the client currently has an active websocket connection.
func (c *Client) IsConnected() bool {
	return c.getConn() != nil
}

func (c *Client) clearConnIfSame(conn *websocket.Conn) {
	c.mu.Lock()
	if c.conn == conn {
		c.conn = nil
	}
	c.mu.Unlock()
}

func (c *Client) snapshotSubs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.subs) == 0 {
		return nil
	}
	streams := make([]string, 0, len(c.subs))
	for s := range c.subs {
		streams = append(streams, s)
	}
	return streams
}

// Subscribe sends a subscription request for the specified streams
func (c *Client) Subscribe(streams []string, id int) error {
	req := SubscribeRequest{
		Method: "SUBSCRIBE",
		Params: streams,
		ID:     id,
	}

	c.mu.Lock()
	for _, s := range streams {
		c.subs[s] = struct{}{}
	}
	conn := c.conn
	c.mu.Unlock()

	// If we're currently disconnected, queue the subscription for the next reconnect.
	if conn == nil {
		return nil
	}

	if err := conn.WriteJSON(req); err != nil {
		c.clearConnIfSame(conn)
		_ = conn.Close()
		return err
	}
	return nil
}

// Unsubscribe sends an unsubscription request for the specified streams
func (c *Client) Unsubscribe(streams []string, id int) error {
	req := SubscribeRequest{
		Method: "UNSUBSCRIBE",
		Params: streams,
		ID:     id,
	}

	c.mu.Lock()
	for _, s := range streams {
		delete(c.subs, s)
	}
	conn := c.conn
	c.mu.Unlock()

	// If we're currently disconnected, treat as success; we'll simply not resubscribe.
	if conn == nil {
		return nil
	}

	if err := conn.WriteJSON(req); err != nil {
		c.clearConnIfSame(conn)
		_ = conn.Close()
		return err
	}
	return nil
}

func (c *Client) run() {
	defer func() {
		c.mu.Lock()
		conn := c.conn
		c.conn = nil
		c.mu.Unlock()
		if conn != nil {
			_ = conn.Close()
		}
		close(c.read)
	}()

	backoff := c.reconnectMin
	resubscribeID := 1

	for {
		select {
		case <-c.done:
			return
		default:
		}

		conn := c.getConn()
		if conn == nil {
			newConn, err := c.dial()
			if err != nil {
				sleep := backoff
				if sleep > c.reconnectMax {
					sleep = c.reconnectMax
				}
				log.Printf("WebSocket dial failed: %v. Retrying in %s...", err, sleep)
				select {
				case <-time.After(sleep):
				case <-c.done:
					return
				}
				if backoff < c.reconnectMax {
					backoff *= 2
					if backoff > c.reconnectMax {
						backoff = c.reconnectMax
					}
				}
				continue
			}

			c.setConn(newConn)
			backoff = c.reconnectMin
			conn = newConn

			streams := c.snapshotSubs()
			if len(streams) > 0 {
				req := SubscribeRequest{Method: "SUBSCRIBE", Params: streams, ID: resubscribeID}
				resubscribeID++
				if err := conn.WriteJSON(req); err != nil {
					log.Printf("Resubscribe failed: %v", err)
					c.clearConnIfSame(conn)
					_ = conn.Close()
					continue
				}
				log.Printf("Reconnected and resubscribed to %d streams", len(streams))
			} else {
				log.Printf("Reconnected")
			}
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			// If we're shutting down, just exit.
			select {
			case <-c.done:
				return
			default:
			}

			// Transient disconnect (e.g. close 1001): clear the conn and retry.
			log.Printf("WebSocket read error: %v. Reconnecting...", err)
			c.clearConnIfSame(conn)
			_ = conn.Close()
			continue
		}

		conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		select {
		case c.read <- message:
		case <-c.done:
			return
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
	var conn *websocket.Conn
	c.closeOnce.Do(func() {
		close(c.done)
		c.mu.Lock()
		conn = c.conn
		c.conn = nil
		c.mu.Unlock()
	})
	if conn != nil {
		return conn.Close()
	}
	return nil
}
