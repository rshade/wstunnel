package tunnel

import (
	"errors"
	"testing"
	"time"
)

func TestNewClientStats(t *testing.T) {
	stats := NewClientStats()
	if stats == nil {
		t.Fatal("NewClientStats returned nil")
	}

	// Check initial values
	if stats.TotalConnections != 0 {
		t.Errorf("Expected TotalConnections to be 0, got %d", stats.TotalConnections)
	}
	if stats.FailedConnections != 0 {
		t.Errorf("Expected FailedConnections to be 0, got %d", stats.FailedConnections)
	}
	if stats.SuccessfulRequests != 0 {
		t.Errorf("Expected SuccessfulRequests to be 0, got %d", stats.SuccessfulRequests)
	}
	if stats.FailedRequests != 0 {
		t.Errorf("Expected FailedRequests to be 0, got %d", stats.FailedRequests)
	}
	if stats.LastError != nil {
		t.Errorf("Expected LastError to be nil, got %v", stats.LastError)
	}
	if stats.TotalBytesSent != 0 {
		t.Errorf("Expected TotalBytesSent to be 0, got %d", stats.TotalBytesSent)
	}
	if stats.TotalBytesReceived != 0 {
		t.Errorf("Expected TotalBytesReceived to be 0, got %d", stats.TotalBytesReceived)
	}
}

func TestClientStats_RecordConnection(t *testing.T) {
	tests := []struct {
		name           string
		success        bool
		expectedTotal  int64
		expectedFailed int64
	}{
		{
			name:           "successful connection",
			success:        true,
			expectedTotal:  1,
			expectedFailed: 0,
		},
		{
			name:           "failed connection",
			success:        false,
			expectedTotal:  1,
			expectedFailed: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := NewClientStats()
			stats.RecordConnection(tt.success)

			if stats.TotalConnections != tt.expectedTotal {
				t.Errorf("Expected TotalConnections to be %d, got %d", tt.expectedTotal, stats.TotalConnections)
			}
			if stats.FailedConnections != tt.expectedFailed {
				t.Errorf("Expected FailedConnections to be %d, got %d", tt.expectedFailed, stats.FailedConnections)
			}
		})
	}
}

func TestClientStats_RecordConnection_Multiple(t *testing.T) {
	stats := NewClientStats()

	// Record multiple connections
	stats.RecordConnection(true)
	stats.RecordConnection(false)
	stats.RecordConnection(true)
	stats.RecordConnection(false)
	stats.RecordConnection(false)

	expectedTotal := int64(5)
	expectedFailed := int64(3)

	if stats.TotalConnections != expectedTotal {
		t.Errorf("Expected TotalConnections to be %d, got %d", expectedTotal, stats.TotalConnections)
	}
	if stats.FailedConnections != expectedFailed {
		t.Errorf("Expected FailedConnections to be %d, got %d", expectedFailed, stats.FailedConnections)
	}
}

func TestClientStats_RecordRequest(t *testing.T) {
	tests := []struct {
		name                 string
		success              bool
		expectedSuccessful   int64
		expectedFailed       int64
		checkLastSuccessTime bool
	}{
		{
			name:                 "successful request",
			success:              true,
			expectedSuccessful:   1,
			expectedFailed:       0,
			checkLastSuccessTime: true,
		},
		{
			name:                 "failed request",
			success:              false,
			expectedSuccessful:   0,
			expectedFailed:       1,
			checkLastSuccessTime: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := NewClientStats()
			beforeTime := time.Now()
			stats.RecordRequest(tt.success)
			afterTime := time.Now()

			if stats.SuccessfulRequests != tt.expectedSuccessful {
				t.Errorf("Expected SuccessfulRequests to be %d, got %d", tt.expectedSuccessful, stats.SuccessfulRequests)
			}
			if stats.FailedRequests != tt.expectedFailed {
				t.Errorf("Expected FailedRequests to be %d, got %d", tt.expectedFailed, stats.FailedRequests)
			}

			if tt.checkLastSuccessTime {
				if stats.LastSuccessTime.Before(beforeTime) || stats.LastSuccessTime.After(afterTime) {
					t.Errorf("LastSuccessTime not properly set. Expected between %v and %v, got %v", beforeTime, afterTime, stats.LastSuccessTime)
				}
			}
		})
	}
}

func TestClientStats_RecordRequest_Multiple(t *testing.T) {
	stats := NewClientStats()

	// Record multiple requests
	stats.RecordRequest(true)
	stats.RecordRequest(false)
	stats.RecordRequest(true)
	stats.RecordRequest(true)
	stats.RecordRequest(false)

	expectedSuccessful := int64(3)
	expectedFailed := int64(2)

	if stats.SuccessfulRequests != expectedSuccessful {
		t.Errorf("Expected SuccessfulRequests to be %d, got %d", expectedSuccessful, stats.SuccessfulRequests)
	}
	if stats.FailedRequests != expectedFailed {
		t.Errorf("Expected FailedRequests to be %d, got %d", expectedFailed, stats.FailedRequests)
	}
}

func TestClientStats_RecordError(t *testing.T) {
	stats := NewClientStats()
	testError := errors.New("test error")

	beforeTime := time.Now()
	stats.RecordError(testError)
	afterTime := time.Now()

	if stats.LastError != testError {
		t.Errorf("Expected LastError to be %v, got %v", testError, stats.LastError)
	}

	if stats.LastErrorTime.Before(beforeTime) || stats.LastErrorTime.After(afterTime) {
		t.Errorf("LastErrorTime not properly set. Expected between %v and %v, got %v", beforeTime, afterTime, stats.LastErrorTime)
	}
}

func TestClientStats_RecordError_Multiple(t *testing.T) {
	stats := NewClientStats()

	error1 := errors.New("first error")
	error2 := errors.New("second error")

	stats.RecordError(error1)
	firstErrorTime := stats.LastErrorTime

	time.Sleep(time.Millisecond) // Ensure different timestamps

	stats.RecordError(error2)
	secondErrorTime := stats.LastErrorTime

	// Should have the latest error
	if stats.LastError != error2 {
		t.Errorf("Expected LastError to be %v, got %v", error2, stats.LastError)
	}

	// Should have the latest error time
	if !secondErrorTime.After(firstErrorTime) {
		t.Errorf("Expected second error time (%v) to be after first error time (%v)", secondErrorTime, firstErrorTime)
	}
}

func TestClientStats_RecordBytes(t *testing.T) {
	tests := []struct {
		name         string
		sent         int64
		received     int64
		expectedSent int64
		expectedRecv int64
	}{
		{
			name:         "record positive bytes",
			sent:         100,
			received:     200,
			expectedSent: 100,
			expectedRecv: 200,
		},
		{
			name:         "record zero bytes",
			sent:         0,
			received:     0,
			expectedSent: 0,
			expectedRecv: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := NewClientStats()
			stats.RecordBytes(tt.sent, tt.received)

			if stats.TotalBytesSent != tt.expectedSent {
				t.Errorf("Expected TotalBytesSent to be %d, got %d", tt.expectedSent, stats.TotalBytesSent)
			}
			if stats.TotalBytesReceived != tt.expectedRecv {
				t.Errorf("Expected TotalBytesReceived to be %d, got %d", tt.expectedRecv, stats.TotalBytesReceived)
			}
		})
	}
}

func TestClientStats_RecordBytes_Multiple(t *testing.T) {
	stats := NewClientStats()

	// Record multiple byte transfers
	stats.RecordBytes(100, 200)
	stats.RecordBytes(50, 75)
	stats.RecordBytes(25, 125)

	expectedSent := int64(175)
	expectedReceived := int64(400)

	if stats.TotalBytesSent != expectedSent {
		t.Errorf("Expected TotalBytesSent to be %d, got %d", expectedSent, stats.TotalBytesSent)
	}
	if stats.TotalBytesReceived != expectedReceived {
		t.Errorf("Expected TotalBytesReceived to be %d, got %d", expectedReceived, stats.TotalBytesReceived)
	}
}

func TestClientStats_GetStats(t *testing.T) {
	stats := NewClientStats()
	testError := errors.New("test error")

	// Populate stats with various data
	stats.RecordConnection(true)
	stats.RecordConnection(false)
	stats.RecordRequest(true)
	stats.RecordRequest(false)
	stats.RecordError(testError)
	stats.RecordBytes(100, 200)

	// Get stats copy
	statsCopy := stats.GetStats()

	// Verify all fields are copied correctly
	if statsCopy.TotalConnections != 2 {
		t.Errorf("Expected TotalConnections to be 2, got %d", statsCopy.TotalConnections)
	}
	if statsCopy.FailedConnections != 1 {
		t.Errorf("Expected FailedConnections to be 1, got %d", statsCopy.FailedConnections)
	}
	if statsCopy.SuccessfulRequests != 1 {
		t.Errorf("Expected SuccessfulRequests to be 1, got %d", statsCopy.SuccessfulRequests)
	}
	if statsCopy.FailedRequests != 1 {
		t.Errorf("Expected FailedRequests to be 1, got %d", statsCopy.FailedRequests)
	}
	if statsCopy.LastError != testError {
		t.Errorf("Expected LastError to be %v, got %v", testError, statsCopy.LastError)
	}
	if statsCopy.TotalBytesSent != 100 {
		t.Errorf("Expected TotalBytesSent to be 100, got %d", statsCopy.TotalBytesSent)
	}
	if statsCopy.TotalBytesReceived != 200 {
		t.Errorf("Expected TotalBytesReceived to be 200, got %d", statsCopy.TotalBytesReceived)
	}

	// Verify it's a copy by modifying original
	stats.RecordConnection(true)

	// Copy should not change
	if statsCopy.TotalConnections != 2 {
		t.Errorf("Expected copy TotalConnections to remain 2, got %d", statsCopy.TotalConnections)
	}
}

func TestClientStats_GetStats_TimestampsPreserved(t *testing.T) {
	stats := NewClientStats()
	testError := errors.New("test error")

	stats.RecordError(testError)
	stats.RecordRequest(true)

	statsCopy := stats.GetStats()

	// Verify timestamps are preserved
	if !statsCopy.LastErrorTime.Equal(stats.LastErrorTime) {
		t.Errorf("LastErrorTime not preserved in copy")
	}
	if !statsCopy.LastSuccessTime.Equal(stats.LastSuccessTime) {
		t.Errorf("LastSuccessTime not preserved in copy")
	}
}

func TestClientStats_ConcurrentAccess(t *testing.T) {
	stats := NewClientStats()
	done := make(chan struct{})

	// Start multiple goroutines that modify stats concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()

			for j := 0; j < 100; j++ {
				stats.RecordConnection(j%2 == 0)
				stats.RecordRequest(j%3 == 0)
				stats.RecordBytes(int64(j), int64(j*2))
				stats.RecordError(errors.New("error"))
				_ = stats.GetStats() // Read stats
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final stats make sense
	finalStats := stats.GetStats()
	if finalStats.TotalConnections != 1000 {
		t.Errorf("Expected TotalConnections to be 1000, got %d", finalStats.TotalConnections)
	}
	if finalStats.SuccessfulRequests+finalStats.FailedRequests != 1000 {
		t.Errorf("Expected total requests to be 1000, got %d", finalStats.SuccessfulRequests+finalStats.FailedRequests)
	}
}
