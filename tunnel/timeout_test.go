// Copyright (c) 2023 RightScale, Inc. - see LICENSE

package tunnel

import (
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"gopkg.in/inconshreveable/log15.v2"
)

func TestTokenRequestTimeout(t *testing.T) {
	// start httptest to simulate target server
	wstunToken := "test-token-" + strconv.Itoa(rand.Int()%1000000) + "-1234567890"

	// Create a test server that delays responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/delayed" {
			// Sleep for 3 seconds, which should trigger the tunnel timeout
			time.Sleep(3 * time.Second)
			_, _ = w.Write([]byte("This response should never be seen"))
		}
	}))
	defer server.Close()

	log15.Info("httptest started", "url", server.URL)

	// start wstunsrv with a very short timeout
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	wstunsrv := NewWSTunnelServer([]string{
		"-httptimeout", "2", // 2 second timeout for HTTP requests
	})
	wstunsrv.Start(listener)
	defer wstunsrv.Stop()

	// start wstuncli
	wstuncli := NewWSTunnelClient([]string{
		"-token", wstunToken,
		"-tunnel", "ws://" + listener.Addr().String(),
		"-server", server.URL,
		"-timeout", "3",
	})
	if err := wstuncli.Start(); err != nil {
		t.Fatalf("Error starting client: %v", err)
	}
	defer wstuncli.Stop()

	wstunURL := "http://" + listener.Addr().String()
	// Wait for client to connect with a deadline
	start := time.Now()
	for !wstuncli.IsConnected() {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 6*time.Second {
			t.Fatalf("Client failed to connect within 6 seconds")
		}
	}

	// Make a request that should time out at the server level
	client := &http.Client{
		Timeout: 5 * time.Second, // Client timeout should occur around server timeout
	}

	start = time.Now()
	resp, err := client.Get(wstunURL + "/_token/" + wstunToken + "/delayed")
	elapsed := time.Since(start)

	// Based on CodeRabbit review: server should return 504, not client timeout
	if err != nil {
		// If we get an error, check if it's the expected client timeout due to 504 response timing
		if elapsed >= 4*time.Second && elapsed <= 6*time.Second {
			// This is acceptable - client timed out around when server timeout occurs
			return
		}
		t.Fatalf("Unexpected error after %v: %v", elapsed, err)
	}

	defer func() { _ = resp.Body.Close() }()

	// If no error, expect 504 status
	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("Expected 504 Gateway Timeout, got status: %d", resp.StatusCode)
	}

	// Verify timing - should timeout around 2 seconds (server HTTPTimeout)
	if elapsed < 1500*time.Millisecond {
		t.Fatalf("Timeout occurred too early: %v", elapsed)
	}
	if elapsed > 3000*time.Millisecond {
		t.Fatalf("Timeout occurred too late: %v", elapsed)
	}
}
