package tunnel

import (
	"sync"
	"time"
)

// ConnectionState represents the current state of a connection
type ConnectionState int

const (
	// ConnectionStateDisconnected indicates no active connection
	ConnectionStateDisconnected ConnectionState = iota
	// ConnectionStateConnecting indicates a connection attempt is in progress
	ConnectionStateConnecting
	// ConnectionStateConnected indicates an active connection
	ConnectionStateConnected
	// ConnectionStateFailed indicates a failed connection
	ConnectionStateFailed
)

// ConnectionManager handles connection lifecycle and retry logic
type ConnectionManager struct {
	ReconnectDelay time.Duration
	MaxRetries     int
	retryCount     int
	lastError      error
	errorChan      chan error
	stats          *ClientStats
	state          ConnectionState
	mu             sync.RWMutex
}

// NewConnectionManager creates a new ConnectionManager instance
func NewConnectionManager(reconnectDelay time.Duration, maxRetries int) *ConnectionManager {
	return &ConnectionManager{
		ReconnectDelay: reconnectDelay,
		MaxRetries:     maxRetries,
		errorChan:      make(chan error, 100),
		stats:          NewClientStats(),
		state:          ConnectionStateDisconnected,
	}
}

// RecordError records a connection error and determines if retry is possible
func (cm *ConnectionManager) RecordError(err error) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.lastError = err
	cm.stats.RecordError(err)
	cm.stats.RecordConnection(false)
	cm.state = ConnectionStateFailed

	// Check if we should retry
	if cm.MaxRetries > 0 && cm.retryCount >= cm.MaxRetries {
		return false
	}

	cm.retryCount++
	return true
}

// RecordSuccess records a successful connection
func (cm *ConnectionManager) RecordSuccess() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.retryCount = 0
	cm.lastError = nil
	cm.stats.RecordConnection(true)
	cm.state = ConnectionStateConnected
}

// SetState sets the current connection state
func (cm *ConnectionManager) SetState(state ConnectionState) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.state = state
}

// GetState returns the current connection state
func (cm *ConnectionManager) GetState() ConnectionState {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.state
}

// GetRetryDelay returns the delay before the next retry attempt
func (cm *ConnectionManager) GetRetryDelay() time.Duration {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.retryCount == 0 {
		return 0
	}

	// Exponential backoff with jitter
	delay := cm.ReconnectDelay * time.Duration(cm.retryCount)
	jitter := time.Duration(float64(delay) * 0.1) // 10% jitter
	return delay + jitter
}

// GetStats returns the current connection statistics
func (cm *ConnectionManager) GetStats() ClientStats {
	return cm.stats.GetStats()
}

// GetLastError returns the last recorded error
func (cm *ConnectionManager) GetLastError() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.lastError
}

// Reset resets the connection manager state
func (cm *ConnectionManager) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.retryCount = 0
	cm.lastError = nil
	cm.state = ConnectionStateDisconnected
}

// IsConnected returns true if the connection is in a connected state
func (cm *ConnectionManager) IsConnected() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.state == ConnectionStateConnected
}

// ShouldRetry checks if retry is possible without recording an error
func (cm *ConnectionManager) ShouldRetry() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Check if we should retry
	if cm.MaxRetries > 0 && cm.retryCount >= cm.MaxRetries {
		return false
	}

	return true
}

// IsConnecting returns true if a connection attempt is in progress
func (cm *ConnectionManager) IsConnecting() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.state == ConnectionStateConnecting
}

// IsFailed returns true if the connection is in a failed state
func (cm *ConnectionManager) IsFailed() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.state == ConnectionStateFailed
}
