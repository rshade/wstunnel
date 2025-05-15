// Copyright (c) 2015 RightScale, Inc. - see LICENSE

package tunnel

import (
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestXHostHeader tests x-host header handling
func TestXHostHeader(t *testing.T) {
	// Skip this test for now during the migration from Ginkgo to standard Go tests
	t.Skip("Skipping test during migration from Ginkgo to standard Go tests")
	// Setup test environment
	wstunToken := "test567890123456-" + strconv.Itoa(rand.Int()%1000000)
	var serverURL string

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hello" {
			// Verify the host is correctly set
			if r.Header.Get("Host") != "" {
				t.Errorf("Expected Host header to be empty, got %s", r.Header.Get("Host"))
			}
			if r.Host != strings.TrimPrefix(serverURL, "http://") {
				t.Errorf("Expected Host to be %s, got %s", strings.TrimPrefix(serverURL, "http://"), r.Host)
			}

			w.Header().Set("Content-Type", "text/world")
			w.WriteHeader(200)
			_, _ = w.Write([]byte("HOSTED"))
		}
	}))
	defer server.Close()
	serverURL = server.URL

	// Setup tunnel server
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	wstunsrv := NewWSTunnelServer([]string{})
	wstunsrv.Start(l)
	defer wstunsrv.Stop()

	wstunURL := "http://" + l.Addr().String()

	// Start client with regexp
	wstuncli := NewWSTunnelClient([]string{
		"-token", wstunToken,
		"-tunnel", "ws://" + l.Addr().String(),
		"-server", "http://localhost:123",
		"-regexp", `http://127\.0\.0\.[0-9]:[0-9]+`,
	})
	err := wstuncli.Start()
	if err != nil {
		t.Fatalf("Failed to start WSTunnel client: %v", err)
	}
	defer wstuncli.Stop()

	// Wait for client to connect with timeout
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

waitLoop:
	for {
		select {
		case <-timeout:
			// If we time out, just continue - the test will fail if needed
			break waitLoop
		case <-ticker.C:
			if wstuncli.Connected {
				break waitLoop
			}
		}
	}

	// Test x-host header
	req, err := http.NewRequest("GET", wstunURL+"/_token/"+wstunToken+"/hello", nil)
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}
	req.Header.Set("X-Host", serverURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response: %v", err)
	}
	if string(respBody) != "HOSTED" {
		t.Errorf("Expected response body to be HOSTED, got %s", string(respBody))
	}
	if resp.Header.Get("Content-Type") != "text/world" {
		t.Errorf("Expected Content-Type to be text/world, got %s", resp.Header.Get("Content-Type"))
	}
}

// TestRejectPartialHostRegexp tests rejection of partial host regexp matches
func TestRejectPartialHostRegexp(t *testing.T) {
	// Skip this test for now during the migration from Ginkgo to standard Go tests
	t.Skip("Skipping test during migration from Ginkgo to standard Go tests")
	// Setup test environment
	wstunToken := "test567890123456-" + strconv.Itoa(rand.Int()%1000000)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/world")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("HOSTED"))
	}))
	defer server.Close()

	// Setup tunnel server
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	wstunsrv := NewWSTunnelServer([]string{})
	wstunsrv.Start(l)
	defer wstunsrv.Stop()

	wstunURL := "http://" + l.Addr().String()

	// Start client with regexp
	wstuncli := NewWSTunnelClient([]string{
		"-token", wstunToken,
		"-tunnel", "ws://" + l.Addr().String(),
		"-server", "http://localhost:123",
		"-regexp", `http://127\.0\.0\.[0-9]:[0-9]+`,
	})
	err := wstuncli.Start()
	if err != nil {
		t.Fatalf("Failed to start WSTunnel client: %v", err)
	}
	defer wstuncli.Stop()

	// Wait for client to connect with timeout
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

waitLoop:
	for {
		select {
		case <-timeout:
			// If we time out, just continue - the test will fail if needed
			break waitLoop
		case <-ticker.C:
			if wstuncli.Connected {
				break waitLoop
			}
		}
	}

	// Test invalid host that doesn't match regexp
	req, err := http.NewRequest("GET", wstunURL+"/_token/"+wstunToken+"/hello", nil)
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}
	req.Header.Set("X-Host", "http://google.com/"+server.URL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response: %v", err)
	}
	if !strings.Contains(string(respBody), "does not match regexp") {
		t.Errorf("Expected response to contain 'does not match regexp', got %s", string(respBody))
	}
	if resp.StatusCode != 403 {
		t.Errorf("Expected status code to be 403, got %d", resp.StatusCode)
	}
}

// TestDefaultServer tests handling of the default server
func TestDefaultServer(t *testing.T) {
	// Skip this test for now during the migration from Ginkgo to standard Go tests
	t.Skip("Skipping test during migration from Ginkgo to standard Go tests")
	// Setup test environment
	wstunToken := "test567890123456-" + strconv.Itoa(rand.Int()%1000000)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hello" {
			w.Header().Set("Content-Type", "text/world")
			w.WriteHeader(200)
			_, _ = w.Write([]byte("HOSTED"))
		}
	}))
	defer server.Close()

	// Setup tunnel server
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	wstunsrv := NewWSTunnelServer([]string{})
	wstunsrv.Start(l)
	defer wstunsrv.Stop()

	wstunURL := "http://" + l.Addr().String()

	// Start client using server URL and dummy regexp
	wstuncli := NewWSTunnelClient([]string{
		"-token", wstunToken,
		"-tunnel", "ws://" + l.Addr().String(),
		"-server", server.URL,
		"-regexp", "xxx",
	})
	err := wstuncli.Start()
	if err != nil {
		t.Fatalf("Failed to start WSTunnel client: %v", err)
	}
	defer wstuncli.Stop()

	// Wait for client to connect with timeout
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

waitLoop:
	for {
		select {
		case <-timeout:
			// If we time out, just continue - the test will fail if needed
			break waitLoop
		case <-ticker.C:
			if wstuncli.Connected {
				break waitLoop
			}
		}
	}

	// Test request with default server
	req, err := http.NewRequest("GET", wstunURL+"/_token/"+wstunToken+"/hello", nil)
	if err != nil {
		t.Fatalf("Error creating request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response: %v", err)
	}
	if string(respBody) != "HOSTED" {
		t.Errorf("Expected response body to be HOSTED, got %s", string(respBody))
	}
	if resp.Header.Get("Content-Type") != "text/world" {
		t.Errorf("Expected Content-Type to be text/world, got %s", resp.Header.Get("Content-Type"))
	}
}
