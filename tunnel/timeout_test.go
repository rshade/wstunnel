// Copyright (c) 2023 RightScale, Inc. - see LICENSE

package tunnel

import (
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
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
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
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
		log15.Error("Error starting client", "error", err)
		os.Exit(1)
	}
	defer wstuncli.Stop()

	wstunURL := "http://" + listener.Addr().String()
	for !wstuncli.IsConnected() {
		time.Sleep(10 * time.Millisecond)
	}

	// Make a request that should time out
	client := &http.Client{
		Timeout: 5 * time.Second, // Client timeout to prevent hanging
	}

	start := time.Now()
	_, err := client.Get(wstunURL + "/_token/" + wstunToken + "/delayed")
	elapsed := time.Since(start)

	// Since the server timeout only applies to tunnel communication,
	// and the backend takes 3 seconds to respond, we expect a client timeout
	if err == nil {
		t.Fatal("Expected timeout error, but got none")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("Expected timeout or deadline exceeded error, got: %v", err)
	}

	// The timeout should occur around the configured client timeout (5 seconds)
	// Allow some margin for processing time
	if elapsed < 4500*time.Millisecond {
		t.Fatalf("Timeout occurred too early: %v", elapsed)
	}
	if elapsed > 5500*time.Millisecond {
		t.Fatalf("Timeout occurred too late: %v", elapsed)
	}
}
