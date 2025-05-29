//go:build !race

package tunnel

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/inconshreveable/log15.v2"
)

// TestMaxClientsPerTokenNoRace tests the max clients per token configuration through actual WebSocket connections
// This test is excluded when running with race detector due to known race conditions in the logging code
func TestMaxClientsPerTokenNoRace(t *testing.T) {

	// Create a backend HTTP server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "OK")
	}))
	defer backend.Close()

	// Create tunnel server with max clients limit
	srv := NewWSTunnelServer([]string{
		"-wstimeout", "5",
		"-max-clients-per-token", "2", // Allow only 2 clients per token
	})
	srv.Log.SetHandler(log15.DiscardHandler())
	// Start tunnel server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()

	go srv.Start(listener)
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	srvAddr := listener.Addr().String()
	testToken := "test-token-12345678"

	// Log max clients setting for debugging
	t.Logf("Server started with MaxClientsPerToken=%d", srv.MaxClientsPerToken)

	// Helper function to create a WebSocket client connection that stays alive
	createClient := func() (*websocket.Conn, *http.Response, func(), error) {
		h := http.Header{}
		h.Set("Origin", testToken)
		url := fmt.Sprintf("ws://%s/_tunnel", srvAddr)
		conn, resp, err := websocket.DefaultDialer.Dial(url, h)
		stopChan := make(chan struct{})
		stopped := false
		var stopOnce sync.Once
		if err == nil && conn != nil {
			// Start a goroutine to send periodic pings to keep connection alive
			go func() {
				ticker := time.NewTicker(1 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
							return
						}
					case <-stopChan:
						return
					}
				}
			}()
		}
		stopFunc := func() {
			stopOnce.Do(func() {
				if !stopped {
					close(stopChan)
					stopped = true
				}
			})
		}
		return conn, resp, stopFunc, err
	}

	// Create first client - should succeed
	client1, _, stop1, err := createClient()
	if err != nil {
		t.Fatalf("First client connection failed: %v", err)
	}
	defer func() {
		stop1()
		_ = client1.Close()
	}()

	// Wait a bit to ensure connection is fully established
	time.Sleep(100 * time.Millisecond)

	// Create second client - should succeed
	client2, _, stop2, err := createClient()
	if err != nil {
		t.Fatalf("Second client connection failed: %v", err)
	}
	defer func() {
		stop2()
		_ = client2.Close()
	}()

	// Wait a bit to ensure connection is fully established
	time.Sleep(100 * time.Millisecond)

	// Create third client - should fail with 429
	h := http.Header{}
	h.Set("Origin", testToken)
	url := fmt.Sprintf("ws://%s/_tunnel", srvAddr)

	conn3, resp, err := websocket.DefaultDialer.Dial(url, h)
	if err == nil {
		if conn3 != nil {
			_ = conn3.Close()
		}
		t.Fatal("Third client connection should have failed")
	}
	if resp == nil {
		t.Fatalf("Response is nil, error: %v", err)
	}
	if resp.StatusCode != 429 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status code 429, got %d, body: %s", resp.StatusCode, string(body))
	}

	// Test cleanup: close first client
	stop1()
	_ = client1.Close()

	// Wait for cleanup to process
	time.Sleep(3 * time.Second) // wsReader sleeps 2 seconds before cleanup

	// Now third client should succeed
	client3, _, stop3, err := createClient()
	if err != nil {
		t.Fatalf("Third client connection should succeed after first client disconnected: %v", err)
	}
	defer func() {
		stop3()
		_ = client3.Close()
	}()

	// Check final client count via status endpoint with authentication
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/_stats", srvAddr), nil)
	if err != nil {
		t.Fatalf("Failed to create status request: %v", err)
	}
	req.Header.Set("X-Token", testToken)

	statusResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	defer func() { _ = statusResp.Body.Close() }()

	body, err := io.ReadAll(statusResp.Body)
	if err != nil {
		t.Fatalf("Failed to read status response: %v", err)
	}

	// Parse response and check token_clients_* count
	statusStr := string(body)
	// Check that the status contains a token_clients entry with value 2
	// The exact format may vary based on token truncation implementation
	lines := strings.Split(statusStr, "\n")
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "token_clients_") && strings.HasSuffix(line, "=2") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected status to contain a token_clients_* entry with value 2, got:\n%s", statusStr)
	}
}

// TestMaxClientsPerTokenConcurrentNoRace tests concurrent client connection attempts
// This test is excluded when running with race detector due to known race conditions in the logging code
func TestMaxClientsPerTokenConcurrentNoRace(t *testing.T) {

	// Create tunnel server with max clients limit
	srv := NewWSTunnelServer([]string{
		"-wstimeout", "5",
		"-max-clients-per-token", "3", // Allow only 3 clients per token
	})
	srv.Log.SetHandler(log15.DiscardHandler())
	// Start tunnel server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()

	go srv.Start(listener)
	defer srv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	srvAddr := listener.Addr().String()
	testToken := "test-token-concurrent"

	// Track successful and failed connections
	var successCount, failCount int
	var mu sync.Mutex

	// Create multiple concurrent connection attempts
	numAttempts := 10
	var wg sync.WaitGroup
	wg.Add(numAttempts)

	connections := make([]*websocket.Conn, 0, 3)
	var connMu sync.Mutex

	for i := 0; i < numAttempts; i++ {
		go func(idx int) {
			defer wg.Done()

			h := http.Header{}
			h.Set("Origin", testToken)
			url := fmt.Sprintf("ws://%s/_tunnel", srvAddr)

			conn, resp, err := websocket.DefaultDialer.Dial(url, h)

			mu.Lock()
			if err == nil {
				successCount++
				connMu.Lock()
				connections = append(connections, conn)
				connMu.Unlock()
			} else {
				failCount++
				// Verify it's a 429 error
				if resp == nil || resp.StatusCode != 429 {
					t.Errorf("Failed connection %d: expected 429, got %v", idx, resp)
				}
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Clean up connections
	connMu.Lock()
	for _, conn := range connections {
		_ = conn.Close()
	}
	connMu.Unlock()

	// Verify results
	if successCount != 3 {
		t.Errorf("Expected exactly 3 successful connections, got %d", successCount)
	}
	if failCount != 7 {
		t.Errorf("Expected exactly 7 failed connections, got %d", failCount)
	}

	// Verify server state after all connections closed
	// Wait longer to ensure all cleanup is complete
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		srv.tokenClientsMutex.RLock()
		count := srv.tokenClients[token(testToken)]
		srv.tokenClientsMutex.RUnlock()
		if count == 0 {
			break
		}
	}

	srv.tokenClientsMutex.RLock()
	finalCount := srv.tokenClients[token(testToken)]
	srv.tokenClientsMutex.RUnlock()

	if finalCount != 0 {
		t.Errorf("Expected client count to be 0 after all connections closed, got %d", finalCount)
	}
}
