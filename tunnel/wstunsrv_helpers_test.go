package tunnel

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func newTestRemoteServer(log zerolog.Logger, queueSize int) *remoteServer {
	rs := &remoteServer{
		token:        token("test-token-12345678"),
		log:          log,
		requestQueue: make(chan *remoteRequest, queueSize),
		requestSet:   make(map[int16]*remoteRequest),
		lastActivity: time.Now(),
	}
	rs.readCond = sync.NewCond(&rs.readMutex)
	return rs
}

func TestCutToken(t *testing.T) {
	tests := []struct {
		name     string
		input    token
		expected string
	}{
		{name: "normal token 16 chars", input: token("1234567890123456"), expected: "12345678..."},
		{name: "long token 20 chars", input: token("12345678901234567890"), expected: "12345678..."},
		{name: "exactly 8 chars", input: token("12345678"), expected: "12345678"},
		{name: "short token 4 chars", input: token("1234"), expected: "1234"},
		{name: "very short token 1 char", input: token("1"), expected: "1"},
		{name: "empty token", input: token(""), expected: ""},
		{name: "special characters", input: token("abc!@#$%^&*()1234567"), expected: "abc!@#$%..."},
		{name: "with spaces", input: token("hello world test 123"), expected: "hello wo..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cutToken(tt.input)
			if result != tt.expected {
				t.Errorf("cutToken(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRemoteServerAbortRequests(t *testing.T) {
	tests := []struct {
		name            string
		requestsToQueue int
	}{
		{name: "empty request queue", requestsToQueue: 0},
		{name: "single pending request", requestsToQueue: 1},
		{name: "multiple pending requests", requestsToQueue: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := newTestRemoteServer(zerolog.Nop(), 100)

			requests := make([]*remoteRequest, tt.requestsToQueue)
			for i := range tt.requestsToQueue {
				requests[i] = &remoteRequest{
					id:        int16(i),
					replyChan: make(chan responseBuffer, 1),
					log:       zerolog.Nop(),
				}
				rs.requestQueue <- requests[i]
			}

			rs.AbortRequests()

			if len(rs.requestQueue) != 0 {
				t.Errorf("Expected queue empty, got %d", len(rs.requestQueue))
			}

			// Verify aborted requests received error responses
			for i, req := range requests {
				select {
				case rb := <-req.replyChan:
					if rb.err == nil {
						t.Errorf("request %d: expected error in response, got nil", i)
					}
				default:
					t.Errorf("request %d: no response received after AbortRequests", i)
				}
			}
		})
	}
}

func TestRemoteServerAbortRequestsWithFullReplyChan(t *testing.T) {
	rs := newTestRemoteServer(zerolog.Nop(), 10)

	req := &remoteRequest{
		id:        int16(1),
		replyChan: make(chan responseBuffer, 1),
		log:       zerolog.Nop(),
	}
	// Fill the reply channel so AbortRequests uses the non-blocking default case
	req.replyChan <- responseBuffer{err: nil}
	rs.requestQueue <- req

	// Should not panic even though the reply channel is full
	rs.AbortRequests()

	if len(rs.requestQueue) != 0 {
		t.Errorf("Expected queue empty, got %d", len(rs.requestQueue))
	}
}

func TestRemoteServerAbortRequestsMultipleCalls(t *testing.T) {
	rs := newTestRemoteServer(zerolog.Nop(), 10)

	for i := range 3 {
		rs.requestQueue <- &remoteRequest{
			id:        int16(i),
			replyChan: make(chan responseBuffer, 1),
			log:       zerolog.Nop(),
		}
	}

	rs.AbortRequests()
	rs.AbortRequests() // second call on empty queue should not panic
	rs.AbortRequests()

	if len(rs.requestQueue) != 0 {
		t.Errorf("Expected queue empty, got %d", len(rs.requestQueue))
	}
}

func TestAbortRequestsLogging(t *testing.T) {
	logOutput := &bytes.Buffer{}
	logger := zerolog.New(logOutput).With().Timestamp().Logger()

	rs := newTestRemoteServer(logger, 10)
	rs.AbortRequests()

	if !bytes.Contains(logOutput.Bytes(), []byte("WS tunnel closed")) {
		t.Errorf("Expected log to contain 'WS tunnel closed', got: %s", logOutput.String())
	}
}

func BenchmarkCutToken(b *testing.B) {
	tok := token("this_is_a_long_token_that_needs_cutting_12345678")
	b.ResetTimer()
	for b.Loop() {
		_ = cutToken(tok)
	}
}
