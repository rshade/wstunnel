package tunnel

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"gopkg.in/inconshreveable/log15.v2"
)

// TestClientVersionTracking tests that client version is properly tracked and displayed
func TestClientVersionTracking(t *testing.T) {
	// Create a test server
	ts := NewTestServer()
	defer ts.Close()

	// Set VV to a test version
	oldVV := VV
	VV = "test-client-v1.2.3"
	defer func() { VV = oldVV }()

	// Set up a simple handler
	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hello" {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("WORLD"))
		}
	}))

	// Start the client
	ts.startClient()
	ts.waitConnected()

	// Make a request to ensure connection is established
	resp, err := http.Get(ts.wstunURL + "/_token/" + ts.wstunToken + "/hello")
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	// Now check the status endpoint to see if client version is reported
	statusResp, err := http.Get(ts.wstunURL + "/_stats")
	if err != nil {
		t.Fatalf("Error getting status: %v", err)
	}
	defer func() {
		if err := statusResp.Body.Close(); err != nil {
			t.Logf("Failed to close status response body: %v", err)
		}
	}()

	statusBody, err := io.ReadAll(statusResp.Body)
	if err != nil {
		t.Fatalf("Error reading status response: %v", err)
	}

	statusStr := string(statusBody)

	// Check that the client version is included in the status
	expectedVersion := fmt.Sprintf("tunnel00_client_version=%s", VV)
	if !strings.Contains(statusStr, expectedVersion) {
		t.Errorf("Expected status to contain %q, but got:\n%s", expectedVersion, statusStr)
	}
}

// TestClientVersionHeader tests that the X-Client-Version header is sent
func TestClientVersionHeader(t *testing.T) {
	// Set VV to a test version
	oldVV := VV
	VV = "test-header-v2.0.0"
	defer func() { VV = oldVV }()

	// Create a test server that captures headers
	var capturedVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the client version header regardless of the path
		capturedVersion = r.Header.Get("X-Client-Version")

		// The connection will fail, but we'll have captured the header
		w.WriteHeader(400)
	}))
	defer srv.Close()

	// Parse tunnel URL
	tunnelURL, _ := url.Parse(strings.Replace(srv.URL, "http://", "ws://", 1))

	// Create client with proper initialization
	client := &WSTunnelClient{
		Token:       "test-token",
		Tunnel:      tunnelURL,
		Log:         log15.Root(),
		exitChan:    make(chan struct{}),
		connManager: NewConnectionManager(5*time.Second, 0),
		Timeout:     30 * time.Second,
	}
	ch := NewConnectionHandler(client)

	// Attempt to connect (will fail but headers will be sent)
	_ = ch.Connect()

	// Verify the header was sent
	if capturedVersion != VV {
		t.Errorf("Expected X-Client-Version header to be %q, got %q", VV, capturedVersion)
	}
}

// TestStatusEndpointWithoutClientVersion tests status when no client version is set
func TestStatusEndpointWithoutClientVersion(t *testing.T) {
	// Create a test server
	ts := NewTestServer()
	defer ts.Close()

	// Clear VV to simulate no version
	oldVV := VV
	VV = ""
	defer func() { VV = oldVV }()

	// Set up a simple handler
	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hello" {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("WORLD"))
		}
	}))

	// Start the client
	ts.startClient()
	ts.waitConnected()

	// Make a request to ensure connection is established
	resp, err := http.Get(ts.wstunURL + "/_token/" + ts.wstunToken + "/hello")
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	// Now check the status endpoint
	statusResp, err := http.Get(ts.wstunURL + "/_stats")
	if err != nil {
		t.Fatalf("Error getting status: %v", err)
	}
	defer func() {
		if err := statusResp.Body.Close(); err != nil {
			t.Logf("Failed to close status response body: %v", err)
		}
	}()

	statusBody, err := io.ReadAll(statusResp.Body)
	if err != nil {
		t.Fatalf("Error reading status response: %v", err)
	}

	statusStr := string(statusBody)

	// When client version is empty, it should not be included in status
	if strings.Contains(statusStr, "tunnel00_client_version=") {
		t.Errorf("Status should not contain client_version when version is empty, but got:\n%s", statusStr)
	}
}
