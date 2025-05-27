package tunnel

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ClientImpl provides the implementation of the WSTunnelClient
type ClientImpl struct {
	client         *WSTunnelClient
	statusWriterWg sync.WaitGroup
}

// NewClientImpl creates a new ClientImpl
func NewClientImpl(client *WSTunnelClient) *ClientImpl {
	return &ClientImpl{
		client: client,
	}
}

// Start starts the client
func (ci *ClientImpl) Start() error {
	ci.client.Log.Info("Starting wstunnel client")

	// validate -server
	if ci.client.InternalServer != nil {
		ci.client.Server = ""
	} else if ci.client.Server != "" {
		if !strings.HasPrefix(ci.client.Server, "http://") && !strings.HasPrefix(ci.client.Server, "https://") {
			return fmt.Errorf("local server (-server option) must begin with http:// or https://")
		}
		ci.client.Server = strings.TrimSuffix(ci.client.Server, "/")
	}

	// Create connection handler
	handler := NewConnectionHandler(ci.client)

	// Start connection with retry logic
	err := handler.Reconnect()
	if err != nil {
		return fmt.Errorf("failed to establish connection: %v", err)
	}

	// Start status writer if configured
	if ci.client.StatusFd != nil {
		ci.statusWriterWg.Add(1)
		go ci.startStatusWriter(handler)
	}

	return nil
}

// Stop stops the client
func (ci *ClientImpl) Stop() {
	close(ci.client.exitChan)

	// Wait for status writer to exit before closing StatusFd
	ci.statusWriterWg.Wait()

	if ci.client.conn != nil {
		if err := ci.client.conn.ws.Close(); err != nil {
			ci.client.Log.Error("Failed to close websocket", "err", err)
		}
	}
	if ci.client.StatusFd != nil {
		if err := ci.client.StatusFd.Close(); err != nil {
			ci.client.Log.Error("Failed to close status file", "err", err)
		}
	}
}

// startStatusWriter writes periodic status updates
func (ci *ClientImpl) startStatusWriter(handler *ConnectionHandler) {
	defer ci.statusWriterWg.Done()

	for {
		select {
		case <-ci.client.exitChan:
			return
		case <-time.After(time.Second):
			stats := handler.GetStats()
			if _, err := fmt.Fprintf(ci.client.StatusFd, "Connected: %v, Total Connections: %d, Failed Connections: %d, Last Error: %v\n",
				handler.IsConnected(), stats.TotalConnections, stats.FailedConnections, stats.LastError); err != nil {
				ci.client.Log.Error("Failed to write to status file", "err", err)
			}
		}
	}
}

// GetStats returns the current client statistics
func (ci *ClientImpl) GetStats() ClientStats {
	if ci == nil || ci.client == nil || ci.client.connManager == nil {
		return ClientStats{} // Return zero value
	}
	return ci.client.connManager.GetStats()
}

// IsConnected returns true if the client is connected
func (ci *ClientImpl) IsConnected() bool {
	if ci == nil || ci.client == nil {
		return false
	}
	return ci.client.IsConnected() && ci.client.conn != nil && ci.client.conn.ws != nil
}

// GetLastError returns the last error encountered
func (ci *ClientImpl) GetLastError() error {
	if ci == nil || ci.client == nil || ci.client.connManager == nil {
		return nil
	}
	return ci.client.connManager.GetLastError()
}
