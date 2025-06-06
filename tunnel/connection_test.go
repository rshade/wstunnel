package tunnel

import (
	"errors"
	"testing"
	"time"
)

func TestNewConnectionManager(t *testing.T) {
	reconnectDelay := 5 * time.Second
	maxRetries := 3

	cm := NewConnectionManager(reconnectDelay, maxRetries)

	if cm == nil {
		t.Fatal("NewConnectionManager returned nil")
	}
	if cm.ReconnectDelay != reconnectDelay {
		t.Errorf("Expected ReconnectDelay to be %v, got %v", reconnectDelay, cm.ReconnectDelay)
	}
	if cm.MaxRetries != maxRetries {
		t.Errorf("Expected MaxRetries to be %d, got %d", maxRetries, cm.MaxRetries)
	}
	if cm.errorChan == nil {
		t.Error("Expected errorChan to be initialized")
	}
	if cm.stats == nil {
		t.Error("Expected stats to be initialized")
	}
	if cm.state != ConnectionStateDisconnected {
		t.Errorf("Expected initial state to be ConnectionStateDisconnected, got %v", cm.state)
	}
}

func TestConnectionManager_RecordError(t *testing.T) {
	tests := []struct {
		name           string
		maxRetries     int
		currentRetries int
		expectedRetry  bool
		expectedState  ConnectionState
	}{
		{
			name:           "first error with retries allowed",
			maxRetries:     3,
			currentRetries: 0,
			expectedRetry:  true,
			expectedState:  ConnectionStateFailed,
		},
		{
			name:           "max retries reached",
			maxRetries:     2,
			currentRetries: 2, // At the limit, should not retry
			expectedRetry:  false,
			expectedState:  ConnectionStateFailed,
		},
		{
			name:           "unlimited retries",
			maxRetries:     0,
			currentRetries: 10,
			expectedRetry:  true,
			expectedState:  ConnectionStateFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewConnectionManager(time.Second, tt.maxRetries)
			cm.retryCount = tt.currentRetries

			testError := errors.New("test error")
			shouldRetry := cm.RecordError(testError)

			if shouldRetry != tt.expectedRetry {
				t.Errorf("Expected shouldRetry to be %v, got %v", tt.expectedRetry, shouldRetry)
			}
			if cm.state != tt.expectedState {
				t.Errorf("Expected state to be %v, got %v", tt.expectedState, cm.state)
			}
			if cm.lastError != testError {
				t.Errorf("Expected lastError to be %v, got %v", testError, cm.lastError)
			}
			// RetryCount only increments if shouldRetry was true
			expectedRetryCount := tt.currentRetries
			if tt.expectedRetry {
				expectedRetryCount++
			}
			if cm.retryCount != expectedRetryCount {
				t.Errorf("Expected retryCount to be %d, got %d", expectedRetryCount, cm.retryCount)
			}
		})
	}
}

func TestConnectionManager_RecordSuccess(t *testing.T) {
	cm := NewConnectionManager(time.Second, 3)
	cm.retryCount = 5
	cm.lastError = errors.New("previous error")
	cm.state = ConnectionStateFailed

	cm.RecordSuccess()

	if cm.retryCount != 0 {
		t.Errorf("Expected retryCount to be reset to 0, got %d", cm.retryCount)
	}
	if cm.lastError != nil {
		t.Errorf("Expected lastError to be nil, got %v", cm.lastError)
	}
	if cm.state != ConnectionStateConnected {
		t.Errorf("Expected state to be ConnectionStateConnected, got %v", cm.state)
	}
}

func TestConnectionManager_SetState(t *testing.T) {
	cm := NewConnectionManager(time.Second, 3)

	states := []ConnectionState{
		ConnectionStateConnecting,
		ConnectionStateConnected,
		ConnectionStateFailed,
		ConnectionStateDisconnected,
	}

	for _, state := range states {
		cm.SetState(state)
		if cm.state != state {
			t.Errorf("Expected state to be %v, got %v", state, cm.state)
		}
	}
}

func TestConnectionManager_GetState(t *testing.T) {
	cm := NewConnectionManager(time.Second, 3)

	states := []ConnectionState{
		ConnectionStateConnecting,
		ConnectionStateConnected,
		ConnectionStateFailed,
		ConnectionStateDisconnected,
	}

	for _, expectedState := range states {
		cm.state = expectedState
		actualState := cm.GetState()
		if actualState != expectedState {
			t.Errorf("Expected GetState to return %v, got %v", expectedState, actualState)
		}
	}
}

func TestConnectionManager_GetRetryDelay(t *testing.T) {
	reconnectDelay := 2 * time.Second
	cm := NewConnectionManager(reconnectDelay, 3)

	tests := []struct {
		name        string
		retryCount  int
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{
			name:        "no retries",
			retryCount:  0,
			expectedMin: 0,
			expectedMax: 0,
		},
		{
			name:        "first retry",
			retryCount:  1,
			expectedMin: reconnectDelay,
			expectedMax: reconnectDelay + (reconnectDelay / 10), // with 10% jitter
		},
		{
			name:        "second retry",
			retryCount:  2,
			expectedMin: 2 * reconnectDelay,
			expectedMax: 2*reconnectDelay + (2 * reconnectDelay / 10), // with 10% jitter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm.retryCount = tt.retryCount
			delay := cm.GetRetryDelay()

			if tt.retryCount == 0 {
				if delay != 0 {
					t.Errorf("Expected delay to be 0 for no retries, got %v", delay)
				}
			} else {
				if delay < tt.expectedMin {
					t.Errorf("Expected delay to be at least %v, got %v", tt.expectedMin, delay)
				}
				if delay > tt.expectedMax {
					t.Errorf("Expected delay to be at most %v, got %v", tt.expectedMax, delay)
				}
			}
		})
	}
}

func TestConnectionManager_GetStats(t *testing.T) {
	cm := NewConnectionManager(time.Second, 3)

	// Verify that GetStats returns the stats from the internal ClientStats
	stats := cm.GetStats()

	// Should return initialized ClientStats with zero values
	if stats.TotalConnections != 0 {
		t.Errorf("Expected TotalConnections to be 0, got %d", stats.TotalConnections)
	}
	if stats.FailedConnections != 0 {
		t.Errorf("Expected FailedConnections to be 0, got %d", stats.FailedConnections)
	}
}

func TestConnectionManager_GetLastError(t *testing.T) {
	cm := NewConnectionManager(time.Second, 3)

	// Initially should be nil
	if err := cm.GetLastError(); err != nil {
		t.Errorf("Expected GetLastError to return nil initially, got %v", err)
	}

	// Set an error and verify it's returned
	testError := errors.New("test error")
	cm.RecordError(testError)

	if err := cm.GetLastError(); err != testError {
		t.Errorf("Expected GetLastError to return %v, got %v", testError, err)
	}
}

func TestConnectionManager_Reset(t *testing.T) {
	cm := NewConnectionManager(time.Second, 3)

	// Set some state
	cm.retryCount = 5
	cm.lastError = errors.New("test error")
	cm.state = ConnectionStateFailed

	// Reset
	cm.Reset()

	// Verify reset state
	if cm.retryCount != 0 {
		t.Errorf("Expected retryCount to be 0 after reset, got %d", cm.retryCount)
	}
	if cm.lastError != nil {
		t.Errorf("Expected lastError to be nil after reset, got %v", cm.lastError)
	}
	if cm.state != ConnectionStateDisconnected {
		t.Errorf("Expected state to be ConnectionStateDisconnected after reset, got %v", cm.state)
	}
}

func TestConnectionManager_IsConnected(t *testing.T) {
	cm := NewConnectionManager(time.Second, 3)

	tests := []struct {
		name     string
		state    ConnectionState
		expected bool
	}{
		{
			name:     "disconnected state",
			state:    ConnectionStateDisconnected,
			expected: false,
		},
		{
			name:     "connecting state",
			state:    ConnectionStateConnecting,
			expected: false,
		},
		{
			name:     "connected state",
			state:    ConnectionStateConnected,
			expected: true,
		},
		{
			name:     "failed state",
			state:    ConnectionStateFailed,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm.state = tt.state
			result := cm.IsConnected()
			if result != tt.expected {
				t.Errorf("Expected IsConnected to return %v for state %v, got %v", tt.expected, tt.state, result)
			}
		})
	}
}

func TestConnectionManager_ShouldRetry(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		retryCount int
		expected   bool
	}{
		{
			name:       "unlimited retries",
			maxRetries: 0,
			retryCount: 100,
			expected:   true,
		},
		{
			name:       "within retry limit",
			maxRetries: 5,
			retryCount: 3,
			expected:   true,
		},
		{
			name:       "at retry limit",
			maxRetries: 5,
			retryCount: 5,
			expected:   false,
		},
		{
			name:       "exceeded retry limit",
			maxRetries: 3,
			retryCount: 4,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewConnectionManager(time.Second, tt.maxRetries)
			cm.retryCount = tt.retryCount

			result := cm.ShouldRetry()
			if result != tt.expected {
				t.Errorf("Expected ShouldRetry to return %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConnectionManager_IsConnecting(t *testing.T) {
	cm := NewConnectionManager(time.Second, 3)

	tests := []struct {
		name     string
		state    ConnectionState
		expected bool
	}{
		{
			name:     "disconnected state",
			state:    ConnectionStateDisconnected,
			expected: false,
		},
		{
			name:     "connecting state",
			state:    ConnectionStateConnecting,
			expected: true,
		},
		{
			name:     "connected state",
			state:    ConnectionStateConnected,
			expected: false,
		},
		{
			name:     "failed state",
			state:    ConnectionStateFailed,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm.state = tt.state
			result := cm.IsConnecting()
			if result != tt.expected {
				t.Errorf("Expected IsConnecting to return %v for state %v, got %v", tt.expected, tt.state, result)
			}
		})
	}
}

func TestConnectionManager_IsFailed(t *testing.T) {
	cm := NewConnectionManager(time.Second, 3)

	tests := []struct {
		name     string
		state    ConnectionState
		expected bool
	}{
		{
			name:     "disconnected state",
			state:    ConnectionStateDisconnected,
			expected: false,
		},
		{
			name:     "connecting state",
			state:    ConnectionStateConnecting,
			expected: false,
		},
		{
			name:     "connected state",
			state:    ConnectionStateConnected,
			expected: false,
		},
		{
			name:     "failed state",
			state:    ConnectionStateFailed,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm.state = tt.state
			result := cm.IsFailed()
			if result != tt.expected {
				t.Errorf("Expected IsFailed to return %v for state %v, got %v", tt.expected, tt.state, result)
			}
		})
	}
}

func TestConnectionManager_ConcurrentAccess(t *testing.T) {
	cm := NewConnectionManager(time.Second, 10)
	done := make(chan struct{})

	// Start multiple goroutines that access the connection manager concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()

			for j := 0; j < 50; j++ {
				// Test concurrent state changes
				cm.SetState(ConnectionStateConnecting)
				_ = cm.GetState()
				_ = cm.IsConnected()
				_ = cm.IsConnecting()
				_ = cm.IsFailed()

				// Test concurrent error recording
				if j%2 == 0 {
					cm.RecordError(errors.New("concurrent error"))
				} else {
					cm.RecordSuccess()
				}

				_ = cm.GetLastError()
				_ = cm.ShouldRetry()
				_ = cm.GetRetryDelay()
				_ = cm.GetStats()

				if j%10 == 0 {
					cm.Reset()
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Just verify the connection manager is still functional
	cm.SetState(ConnectionStateConnected)
	if !cm.IsConnected() {
		t.Error("Connection manager should be functional after concurrent access")
	}
}

func TestConnectionState_Values(t *testing.T) {
	// Test that the connection state constants have expected values
	if ConnectionStateDisconnected != 0 {
		t.Errorf("Expected ConnectionStateDisconnected to be 0, got %d", ConnectionStateDisconnected)
	}
	if ConnectionStateConnecting != 1 {
		t.Errorf("Expected ConnectionStateConnecting to be 1, got %d", ConnectionStateConnecting)
	}
	if ConnectionStateConnected != 2 {
		t.Errorf("Expected ConnectionStateConnected to be 2, got %d", ConnectionStateConnected)
	}
	if ConnectionStateFailed != 3 {
		t.Errorf("Expected ConnectionStateFailed to be 3, got %d", ConnectionStateFailed)
	}
}
