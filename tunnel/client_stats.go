package tunnel

import (
	"sync"
	"time"
)

// ClientStats tracks connection statistics
type ClientStats struct {
	TotalConnections   int64
	FailedConnections  int64
	SuccessfulRequests int64
	FailedRequests     int64
	LastError          error
	LastErrorTime      time.Time
	LastSuccessTime    time.Time
	TotalBytesSent     int64
	TotalBytesReceived int64
	mu                 sync.RWMutex
}

// NewClientStats creates a new ClientStats instance
func NewClientStats() *ClientStats {
	return &ClientStats{}
}

// RecordConnection records a new connection attempt
func (s *ClientStats) RecordConnection(success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalConnections++
	if !success {
		s.FailedConnections++
	}
}

// RecordRequest records a new request attempt
func (s *ClientStats) RecordRequest(success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if success {
		s.SuccessfulRequests++
		s.LastSuccessTime = time.Now()
	} else {
		s.FailedRequests++
	}
}

// RecordError records a new error
func (s *ClientStats) RecordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = err
	s.LastErrorTime = time.Now()
}

// RecordBytes records bytes sent/received
func (s *ClientStats) RecordBytes(sent, received int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalBytesSent += sent
	s.TotalBytesReceived += received
}

// GetStats returns a copy of the current statistics
func (s *ClientStats) GetStats() ClientStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ClientStats{
		TotalConnections:   s.TotalConnections,
		FailedConnections:  s.FailedConnections,
		SuccessfulRequests: s.SuccessfulRequests,
		FailedRequests:     s.FailedRequests,
		LastError:          s.LastError,
		LastErrorTime:      s.LastErrorTime,
		LastSuccessTime:    s.LastSuccessTime,
		TotalBytesSent:     s.TotalBytesSent,
		TotalBytesReceived: s.TotalBytesReceived,
	}
}
