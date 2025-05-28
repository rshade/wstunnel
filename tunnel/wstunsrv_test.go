package tunnel

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/inconshreveable/log15.v2"
)

// TestStatusHandlerWithClientVersion tests the status handler with client version
func TestStatusHandlerWithClientVersion(t *testing.T) {
	// Create a test server
	ts := &WSTunnelServer{
		Log:                 log15.Root(),
		Host:                "0.0.0.0",
		Port:                0,
		exitChan:            make(chan struct{}),
		serverRegistry:      make(map[token]*remoteServer),
		serverRegistryMutex: sync.Mutex{},
		tokenPasswords:      make(map[token]string),
		tokenPasswordsMutex: sync.RWMutex{},
	}

	// Create a remote server with client version
	testToken := token("test-token")
	rs := &remoteServer{
		token:         testToken,
		log:           ts.Log.New("token", "test-token"),
		requestQueue:  make(chan *remoteRequest, 100),
		requestSet:    make(map[int16]*remoteRequest),
		lastActivity:  time.Now(),
		clientVersion: "test-client-v1.2.3",
	}

	// Add to registry
	ts.serverRegistryMutex.Lock()
	ts.serverRegistry[testToken] = rs
	ts.serverRegistryMutex.Unlock()

	// Create request and recorder
	req := httptest.NewRequest("GET", "/_status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	// Call the handler
	statsHandler(ts, w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response: %v", err)
	}

	bodyStr := string(body)

	// Check that client version is included
	expectedVersion := "tunnel00_client_version=test-client-v1.2.3"
	if !strings.Contains(bodyStr, expectedVersion) {
		t.Errorf("Expected status to contain %q, but got:\n%s", expectedVersion, bodyStr)
	}
}

// TestStatusHandlerWithoutClientVersion tests the status handler without client version
func TestStatusHandlerWithoutClientVersion(t *testing.T) {
	// Create a test server
	ts := &WSTunnelServer{
		Log:                 log15.Root(),
		Host:                "0.0.0.0",
		Port:                0,
		exitChan:            make(chan struct{}),
		serverRegistry:      make(map[token]*remoteServer),
		serverRegistryMutex: sync.Mutex{},
		tokenPasswords:      make(map[token]string),
		tokenPasswordsMutex: sync.RWMutex{},
	}

	// Create a remote server without client version
	testToken := token("test-token")
	rs := &remoteServer{
		token:         testToken,
		log:           ts.Log.New("token", "test-token"),
		requestQueue:  make(chan *remoteRequest, 100),
		requestSet:    make(map[int16]*remoteRequest),
		lastActivity:  time.Now(),
		clientVersion: "", // No client version
	}

	// Add to registry
	ts.serverRegistryMutex.Lock()
	ts.serverRegistry[testToken] = rs
	ts.serverRegistryMutex.Unlock()

	// Create request and recorder
	req := httptest.NewRequest("GET", "/_status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	// Call the handler
	statsHandler(ts, w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response: %v", err)
	}

	bodyStr := string(body)

	// When client version is empty, it should not be included
	if strings.Contains(bodyStr, "tunnel00_client_version=") {
		t.Errorf("Status should not contain client_version when version is empty, but got:\n%s", bodyStr)
	}
}

// TestWsHandlerClientVersion tests that client version is extracted from headers
func TestWsHandlerClientVersion(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This simulates the websocket upgrade check
		if r.Header.Get("X-Token") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, "Missing X-Token header")
			return
		}

		// Check if client version header is present
		clientVersion := r.Header.Get("X-Client-Version")
		if clientVersion == "" {
			t.Error("Expected X-Client-Version header to be present")
		}

		// For testing, just return OK
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "Client version: %s", clientVersion)
	}))
	defer server.Close()

	// Make a request with client version header
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Token", "test-token")
	req.Header.Set("X-Client-Version", "test-v1.0.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(body), "Client version: test-v1.0.0") {
		t.Errorf("Expected response to contain client version, got: %s", string(body))
	}
}
