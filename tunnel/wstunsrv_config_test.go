package tunnel

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"testing"

	"gopkg.in/inconshreveable/log15.v2"
)

func TestNewWSTunnelServer_MaxRequestsPerTunnel_Validation(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		expectLog string
		expectVal int
	}{
		{
			name:      "negative value",
			args:      []string{"-port", "0", "-max-requests-per-tunnel", "-5"},
			expectLog: "max-requests-per-tunnel cannot be negative, using default",
			expectVal: defaultMaxReq,
		},
		{
			name:      "zero value",
			args:      []string{"-port", "0", "-max-requests-per-tunnel", "0"},
			expectLog: "max-requests-per-tunnel set to 0 – interpreting as unlimited",
			expectVal: defaultMaxReq,
		},
		{
			name:      "very high value",
			args:      []string{"-port", "0", "-max-requests-per-tunnel", "1500"},
			expectLog: "max-requests-per-tunnel is very high, may cause resource issues",
			expectVal: 1500,
		},
		{
			name:      "normal value",
			args:      []string{"-port", "0", "-max-requests-per-tunnel", "50"},
			expectLog: "",
			expectVal: 50,
		},
		{
			name:      "boundary value 1000",
			args:      []string{"-port", "0", "-max-requests-per-tunnel", "1000"},
			expectLog: "",
			expectVal: 1000,
		},
		{
			name:      "boundary value 1001",
			args:      []string{"-port", "0", "-max-requests-per-tunnel", "1001"},
			expectLog: "max-requests-per-tunnel is very high",
			expectVal: 1001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output
			var logOutput bytes.Buffer
			log15.Root().SetHandler(log15.StreamHandler(&logOutput, log15.LogfmtFormat()))

			server := NewWSTunnelServer(tt.args)
			if server == nil {
				t.Fatal("Expected server to be created")
			}

			// Check the value was set correctly
			if server.MaxRequestsPerTunnel != tt.expectVal {
				t.Errorf("Expected MaxRequestsPerTunnel to be %d, got %d", tt.expectVal, server.MaxRequestsPerTunnel)
			}

			// Check log output
			logStr := logOutput.String()
			if tt.expectLog != "" {
				if !strings.Contains(logStr, tt.expectLog) {
					t.Errorf("Expected log to contain %q, but log was: %s", tt.expectLog, logStr)
				}
			}
		})
	}
}

func TestNewWSTunnelServer_MaxClientsPerToken_Validation(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		expectLog string
		expectVal int
	}{
		{
			name:      "negative value",
			args:      []string{"-port", "0", "-max-clients-per-token", "-5"},
			expectLog: "max-clients-per-token cannot be negative, disabling limit",
			expectVal: 0,
		},
		{
			name:      "zero value (unlimited)",
			args:      []string{"-port", "0", "-max-clients-per-token", "0"},
			expectLog: "",
			expectVal: 0,
		},
		{
			name:      "very high value",
			args:      []string{"-port", "0", "-max-clients-per-token", "1500"},
			expectLog: "max-clients-per-token is very high, may cause resource issues",
			expectVal: 1500,
		},
		{
			name:      "normal value",
			args:      []string{"-port", "0", "-max-clients-per-token", "50"},
			expectLog: "",
			expectVal: 50,
		},
		{
			name:      "boundary value 1000",
			args:      []string{"-port", "0", "-max-clients-per-token", "1000"},
			expectLog: "",
			expectVal: 1000,
		},
		{
			name:      "boundary value 1001",
			args:      []string{"-port", "0", "-max-clients-per-token", "1001"},
			expectLog: "max-clients-per-token is very high",
			expectVal: 1001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output
			var logOutput bytes.Buffer
			log15.Root().SetHandler(log15.StreamHandler(&logOutput, log15.LogfmtFormat()))

			server := NewWSTunnelServer(tt.args)
			if server == nil {
				t.Fatal("Expected server to be created")
			}

			// Check the value was set correctly
			if server.MaxClientsPerToken != tt.expectVal {
				t.Errorf("Expected MaxClientsPerToken to be %d, got %d", tt.expectVal, server.MaxClientsPerToken)
			}

			// Check log output
			logStr := logOutput.String()
			if tt.expectLog != "" {
				if !strings.Contains(logStr, tt.expectLog) {
					t.Errorf("Expected log to contain %q, but log was: %s", tt.expectLog, logStr)
				}
			}
		})
	}
}

func TestNewWSTunnelServer_BothLimits(t *testing.T) {
	// Test setting both limits together
	args := []string{
		"-port", "0",
		"-max-requests-per-tunnel", "-10",
		"-max-clients-per-token", "-20",
	}

	var logOutput bytes.Buffer
	log15.Root().SetHandler(log15.StreamHandler(&logOutput, log15.LogfmtFormat()))

	server := NewWSTunnelServer(args)
	if server == nil {
		t.Fatal("Expected server to be created")
	}

	// Check both values were corrected
	if server.MaxRequestsPerTunnel != defaultMaxReq {
		t.Errorf("Expected MaxRequestsPerTunnel to be %d (default), got %d", defaultMaxReq, server.MaxRequestsPerTunnel)
	}
	if server.MaxClientsPerToken != 0 {
		t.Errorf("Expected MaxClientsPerToken to be 0 (disabled), got %d", server.MaxClientsPerToken)
	}

	// Check both error logs appeared
	logStr := logOutput.String()
	if !strings.Contains(logStr, "max-requests-per-tunnel cannot be negative") {
		t.Errorf("Expected log to contain max-requests-per-tunnel error")
	}
	if !strings.Contains(logStr, "max-clients-per-token cannot be negative") {
		t.Errorf("Expected log to contain max-clients-per-token error")
	}
}

func TestNewWSTunnelServer_DefaultValues(t *testing.T) {
	// Test that default values are applied when flags are not specified
	args := []string{"-port", "0"}

	server := NewWSTunnelServer(args)
	if server == nil {
		t.Fatal("Expected server to be created")
	}

	if server.MaxRequestsPerTunnel != defaultMaxReq {
		t.Errorf("Expected default MaxRequestsPerTunnel to be %d, got %d", defaultMaxReq, server.MaxRequestsPerTunnel)
	}
	if server.MaxClientsPerToken != 0 {
		t.Errorf("Expected default MaxClientsPerToken to be 0, got %d", server.MaxClientsPerToken)
	}
}

func TestNewWSTunnelServer_TokenClientsMapInitialization(t *testing.T) {
	// Test that tokenClients map is properly initialized
	args := []string{"-port", "0"}

	server := NewWSTunnelServer(args)
	if server == nil {
		t.Fatal("Expected server to be created")
	}

	if server.tokenClients == nil {
		t.Error("Expected tokenClients map to be initialized")
	}

	// Map should be empty initially
	if len(server.tokenClients) != 0 {
		t.Errorf("Expected tokenClients map to be empty, got %d entries", len(server.tokenClients))
	}
}

func TestRemoteServerRequestQueueClamping(t *testing.T) {
	// Test that request queue size is clamped to 1000 for very large values
	tests := []struct {
		name             string
		maxRequestsValue int
		expectedQueueCap int
	}{
		{
			name:             "normal value",
			maxRequestsValue: 50,
			expectedQueueCap: 50,
		},
		{
			name:             "boundary value 1000",
			maxRequestsValue: 1000,
			expectedQueueCap: 1000,
		},
		{
			name:             "value above 1000 gets clamped",
			maxRequestsValue: 5000,
			expectedQueueCap: 1000,
		},
		{
			name:             "very high value gets clamped",
			maxRequestsValue: 10000,
			expectedQueueCap: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create server with specific max requests value
			args := []string{"-port", "0", "-max-requests-per-tunnel", fmt.Sprintf("%d", tt.maxRequestsValue)}
			server := NewWSTunnelServer(args)
			if server == nil {
				t.Fatal("Expected server to be created")
			}
			server.Log.SetHandler(log15.DiscardHandler())

			// Initialize server registry before starting
			if server.serverRegistry == nil {
				server.serverRegistry = make(map[token]*remoteServer)
			}

			// Start the server
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = listener.Close() }()

			go server.Start(listener)
			defer server.Stop()

			// Create a remote server to test queue clamping
			testToken := token("test-token-12345678")
			rs := server.getRemoteServer(testToken, true)
			if rs == nil {
				t.Fatal("Expected remote server to be created")
			}

			// Check the actual capacity of the request queue
			actualCap := cap(rs.requestQueue)
			if actualCap != tt.expectedQueueCap {
				t.Errorf("Expected request queue capacity to be %d, got %d", tt.expectedQueueCap, actualCap)
			}

			// Clean up
			server.serverRegistryMutex.Lock()
			delete(server.serverRegistry, testToken)
			server.serverRegistryMutex.Unlock()
		})
	}
}
