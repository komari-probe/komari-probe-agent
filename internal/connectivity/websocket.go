package connectivity

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketDialerOptions configures the outbound WebSocket connection policy.
type WebSocketDialerOptions struct {
	HandshakeTimeout  time.Duration
	DialTimeout       time.Duration
	PreferIPVersion   string
	IgnoreUnsafeCert  bool
	EnableCompression bool
}

// NewWebSocketDialer creates a dialer that follows the configured DNS and IP
// version preferences used by other outbound connections.
func NewWebSocketDialer(options WebSocketDialerOptions) *websocket.Dialer {
	if options.HandshakeTimeout <= 0 {
		options.HandshakeTimeout = 15 * time.Second
	}
	if options.DialTimeout <= 0 {
		options.DialTimeout = 15 * time.Second
	}

	dialer := &websocket.Dialer{
		HandshakeTimeout:  options.HandshakeTimeout,
		NetDialContext:    NewDialContextWithPreference(options.DialTimeout, options.PreferIPVersion),
		Proxy:             http.ProxyFromEnvironment,
		EnableCompression: options.EnableCompression,
	}
	if options.IgnoreUnsafeCert {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return dialer
}

// SafeConn serializes writes to a WebSocket connection. Gorilla WebSocket
// permits one concurrent reader and one concurrent writer.
type SafeConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewSafeConn(conn *websocket.Conn) *SafeConn {
	return &SafeConn{conn: conn}
}

func (conn *SafeConn) WriteMessage(messageType int, data []byte) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	return conn.conn.WriteMessage(messageType, data)
}

func (conn *SafeConn) Close() error {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	return conn.conn.Close()
}

func (conn *SafeConn) ReadMessage() (int, []byte, error) {
	return conn.conn.ReadMessage()
}
