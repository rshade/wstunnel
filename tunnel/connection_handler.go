package tunnel

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/inconshreveable/log15.v2"
)

// ConnectionHandler handles the websocket connection lifecycle
type ConnectionHandler struct {
	client *WSTunnelClient
	conn   *WSConnection
	log    log15.Logger
}

// NewConnectionHandler creates a new ConnectionHandler
func NewConnectionHandler(client *WSTunnelClient) *ConnectionHandler {
	return &ConnectionHandler{
		client: client,
		log:    client.Log.New("component", "connection_handler"),
	}
}

// Connect establishes a new websocket connection
func (ch *ConnectionHandler) Connect() error {
	ch.client.connManager.SetState(ConnectionStateConnecting)

	dialer := websocket.Dialer{
		HandshakeTimeout: ch.client.Timeout,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
	}

	// Set up proxy if configured
	if ch.client.Proxy != nil {
		dialer.Proxy = http.ProxyURL(ch.client.Proxy)
	}

	// Set up TLS if using secure connection
	if ch.client.Tunnel.Scheme == "wss" {
		dialer.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: ch.client.Insecure,
			MinVersion:         tls.VersionTLS12, // Enforce minimum TLS 1.2
		}
		if ch.client.Cert != "" {
			// Load CA certificate for server verification
			caCert, err := os.ReadFile(ch.client.Cert)
			if err != nil {
				return fmt.Errorf("failed to read certificate file: %v", err)
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return fmt.Errorf("failed to parse certificate")
			}
			dialer.TLSClientConfig.RootCAs = caCertPool
		}
	}

	// Set up local port binding if configured
	if len(ch.client.ClientPorts) > 0 {
		dialer.NetDial = func(network, addr string) (net.Conn, error) {
			return ch.client.wsDialerLocalPort(network, addr, ch.client.ClientPorts)
		}
	}

	// Add authentication headers if configured
	header := http.Header{}
	if ch.client.Token != "" {
		// Set token in Origin header (server expects it there)
		header.Set("Origin", ch.client.Token)

		// If password is provided, use HTTP Basic Auth
		if ch.client.Password != "" {
			auth := ch.client.Token + ":" + ch.client.Password
			encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
			header.Set("Authorization", "Basic "+encodedAuth)
		}
	}

	// Connect to the websocket server
	ws, _, err := dialer.Dial(ch.client.Tunnel.String(), header)
	if err != nil {
		ch.client.connManager.RecordError(err)
		return fmt.Errorf("failed to connect to websocket server: %v", err)
	}

	// Create new connection
	ch.conn = &WSConnection{
		Log: ch.log.New("ws", fmt.Sprintf("%p", ws)),
		ws:  ws,
		tun: ch.client,
	}

	ch.client.conn = ch.conn
	ch.client.connManager.RecordSuccess()
	ch.client.setConnected(true)

	// Start connection handlers
	go ch.conn.handleRequests()
	go ch.conn.pinger()

	return nil
}

// Disconnect closes the current websocket connection
func (ch *ConnectionHandler) Disconnect() error {
	if ch.conn != nil && ch.conn.ws != nil {
		ch.log.Info("Closing websocket connection")
		err := ch.conn.ws.Close()
		ch.conn = nil
		ch.client.conn = nil
		ch.client.setConnected(false)
		return err
	}
	return nil
}

// Reconnect attempts to establish a new connection with retry logic
func (ch *ConnectionHandler) Reconnect() error {
	// Disconnect existing connection if any
	if err := ch.Disconnect(); err != nil {
		ch.log.Warn("Error disconnecting existing connection", "err", err)
	}

	// Try to connect with retry logic
	for {
		err := ch.Connect()
		if err == nil {
			return nil
		}

		ch.log.Error("Connection failed", "err", err)

		// Check if we should retry (error was already recorded in Connect)
		if !ch.client.connManager.ShouldRetry() {
			return fmt.Errorf("max retries exceeded: %v", err)
		}

		// Calculate delay before next attempt
		delay := ch.client.connManager.GetRetryDelay()
		ch.log.Info("Retrying connection", "delay", delay)

		// Wait for delay or exit signal
		select {
		case <-time.After(delay):
			continue
		case <-ch.client.exitChan:
			return fmt.Errorf("connection attempt cancelled")
		}
	}
}

// IsConnected returns true if there is an active connection
func (ch *ConnectionHandler) IsConnected() bool {
	return ch.conn != nil && ch.conn.ws != nil && ch.client.IsConnected()
}

// GetStats returns the current connection statistics
func (ch *ConnectionHandler) GetStats() ClientStats {
	return ch.client.connManager.GetStats()
}

// GetLastError returns the last connection error
func (ch *ConnectionHandler) GetLastError() error {
	return ch.client.connManager.GetLastError()
}
