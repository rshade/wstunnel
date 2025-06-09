package tunnel

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/inconshreveable/log15.v2"
)

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "root path",
			input:    "/",
			expected: "/",
		},
		{
			name:     "simple path",
			input:    "/wstunnel",
			expected: "/wstunnel",
		},
		{
			name:     "path without leading slash",
			input:    "wstunnel",
			expected: "/wstunnel",
		},
		{
			name:     "path with trailing slash",
			input:    "/wstunnel/",
			expected: "/wstunnel",
		},
		{
			name:     "nested path",
			input:    "/api/v1/tunnel",
			expected: "/api/v1/tunnel",
		},
		{
			name:     "nested path with trailing slash",
			input:    "/api/v1/tunnel/",
			expected: "/api/v1/tunnel",
		},
		{
			name:     "path with whitespace",
			input:    "  /wstunnel  ",
			expected: "/wstunnel",
		},
		{
			name:     "path without leading slash and with trailing slash",
			input:    "wstunnel/",
			expected: "/wstunnel",
		},
		{
			name:     "root path with extra slashes",
			input:    "///",
			expected: "/",
		},
		{
			name:     "path traversal attempt with ..",
			input:    "/wstunnel/../admin",
			expected: "",
		},
		{
			name:     "path traversal at start",
			input:    "../wstunnel",
			expected: "",
		},
		{
			name:     "path traversal in middle",
			input:    "/api/../tunnel",
			expected: "",
		},
		{
			name:     "encoded path traversal attempt",
			input:    "/wstunnel/%2e%2e/admin",
			expected: "/wstunnel/%2e%2e/admin", // Not decoded, so allowed
		},
		{
			name:     "path exceeding max length",
			input:    "/" + strings.Repeat("a", 300),
			expected: "",
		},
		{
			name:     "path with null byte",
			input:    "/wstunnel\x00/test",
			expected: "",
		},
		{
			name:     "path with control characters",
			input:    "/wstunnel\x01\x02",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeBasePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeBasePath(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestShouldStripBasePath(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		basePath    string
		expected    bool
	}{
		{
			name:        "exact match",
			requestPath: "/wstunnel",
			basePath:    "/wstunnel",
			expected:    true,
		},
		{
			name:        "base path with trailing slash in request",
			requestPath: "/wstunnel/",
			basePath:    "/wstunnel",
			expected:    true,
		},
		{
			name:        "base path with sub path",
			requestPath: "/wstunnel/_tunnel",
			basePath:    "/wstunnel",
			expected:    true,
		},
		{
			name:        "partial match (should not strip)",
			requestPath: "/wstunnel2/_tunnel",
			basePath:    "/wstunnel",
			expected:    false,
		},
		{
			name:        "no match",
			requestPath: "/other/_tunnel",
			basePath:    "/wstunnel",
			expected:    false,
		},
		{
			name:        "nested base path exact match",
			requestPath: "/api/v1",
			basePath:    "/api/v1",
			expected:    true,
		},
		{
			name:        "nested base path with sub path",
			requestPath: "/api/v1/_tunnel",
			basePath:    "/api/v1",
			expected:    true,
		},
		{
			name:        "nested base path partial match (should not strip)",
			requestPath: "/api/v1extra/_tunnel",
			basePath:    "/api/v1",
			expected:    false,
		},
		{
			name:        "root base path (should not strip)",
			requestPath: "/",
			basePath:    "/",
			expected:    false,
		},
		{
			name:        "root base path with sub path (should not strip)",
			requestPath: "/_tunnel",
			basePath:    "/",
			expected:    false,
		},
		{
			name:        "empty base path",
			requestPath: "/_tunnel",
			basePath:    "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldStripBasePath(tt.requestPath, tt.basePath)
			if result != tt.expected {
				t.Errorf("shouldStripBasePath(%q, %q) = %v, expected %v", tt.requestPath, tt.basePath, result, tt.expected)
			}
		})
	}
}

func TestBuildPath(t *testing.T) {
	tests := []struct {
		name      string
		basePath  string
		routePath string
		expected  string
	}{
		{
			name:      "empty base path",
			basePath:  "",
			routePath: "/",
			expected:  "/",
		},
		{
			name:      "empty base path with route",
			basePath:  "",
			routePath: "/_tunnel",
			expected:  "/_tunnel",
		},
		{
			name:      "base path with root route",
			basePath:  "/wstunnel",
			routePath: "/",
			expected:  "/wstunnel/",
		},
		{
			name:      "base path with specific route",
			basePath:  "/wstunnel",
			routePath: "/_tunnel",
			expected:  "/wstunnel/_tunnel",
		},
		{
			name:      "base path with token route",
			basePath:  "/wstunnel",
			routePath: "/_token/",
			expected:  "/wstunnel/_token/",
		},
		{
			name:      "nested base path",
			basePath:  "/api/v1",
			routePath: "/_health_check",
			expected:  "/api/v1/_health_check",
		},
		{
			name:      "nested base path with stats",
			basePath:  "/api/v1",
			routePath: "/_stats",
			expected:  "/api/v1/_stats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPath(tt.basePath, tt.routePath)
			if result != tt.expected {
				t.Errorf("buildPath(%q, %q) = %q, expected %q", tt.basePath, tt.routePath, result, tt.expected)
			}
		})
	}
}

func TestNewWSTunnelServer_BasePath_Configuration(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		expectedPath string
		expectLog    string
	}{
		{
			name:         "default empty base path",
			args:         []string{"-port", "0"},
			expectedPath: "",
			expectLog:    "",
		},
		{
			name:         "simple base path",
			args:         []string{"-port", "0", "-base-path", "/wstunnel"},
			expectedPath: "/wstunnel",
			expectLog:    "Base path configured",
		},
		{
			name:         "base path without leading slash",
			args:         []string{"-port", "0", "-base-path", "wstunnel"},
			expectedPath: "/wstunnel",
			expectLog:    "Base path configured",
		},
		{
			name:         "base path with trailing slash",
			args:         []string{"-port", "0", "-base-path", "/wstunnel/"},
			expectedPath: "/wstunnel",
			expectLog:    "Base path configured",
		},
		{
			name:         "nested base path",
			args:         []string{"-port", "0", "-base-path", "/api/v1/tunnel"},
			expectedPath: "/api/v1/tunnel",
			expectLog:    "Base path configured",
		},
		{
			name:         "base path with whitespace",
			args:         []string{"-port", "0", "-base-path", "  /wstunnel  "},
			expectedPath: "/wstunnel",
			expectLog:    "Base path configured",
		},
		{
			name:         "empty base path string",
			args:         []string{"-port", "0", "-base-path", ""},
			expectedPath: "",
			expectLog:    "",
		},
		{
			name:         "whitespace only base path",
			args:         []string{"-port", "0", "-base-path", "   "},
			expectedPath: "",
			expectLog:    "",
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

			// Check the base path was set correctly
			if server.BasePath != tt.expectedPath {
				t.Errorf("Expected BasePath to be %q, got %q", tt.expectedPath, server.BasePath)
			}

			// Check log output
			logStr := logOutput.String()
			if tt.expectLog != "" {
				if !strings.Contains(logStr, tt.expectLog) {
					t.Errorf("Expected log to contain %q, but log was: %s", tt.expectLog, logStr)
				}
				if !strings.Contains(logStr, fmt.Sprintf("basePath=%s", tt.expectedPath)) {
					t.Errorf("Expected log to contain basePath=%s, but log was: %s", tt.expectedPath, logStr)
				}
			} else {
				if strings.Contains(logStr, "Base path configured") {
					t.Errorf("Expected no base path log, but found: %s", logStr)
				}
			}
		})
	}
}

func TestWSTunnelServer_URLRewriting(t *testing.T) {
	tests := []struct {
		name          string
		basePath      string
		requestPath   string
		expectedPath  string
		shouldRewrite bool
	}{
		{
			name:          "no base path configured",
			basePath:      "",
			requestPath:   "/_tunnel",
			expectedPath:  "/_tunnel",
			shouldRewrite: false,
		},
		{
			name:          "base path matches - tunnel endpoint",
			basePath:      "/wstunnel",
			requestPath:   "/wstunnel/_tunnel",
			expectedPath:  "/_tunnel",
			shouldRewrite: true,
		},
		{
			name:          "base path matches - health check",
			basePath:      "/wstunnel",
			requestPath:   "/wstunnel/_health_check",
			expectedPath:  "/_health_check",
			shouldRewrite: true,
		},
		{
			name:          "base path matches - stats",
			basePath:      "/wstunnel",
			requestPath:   "/wstunnel/_stats",
			expectedPath:  "/_stats",
			shouldRewrite: true,
		},
		{
			name:          "base path matches - token prefix",
			basePath:      "/wstunnel",
			requestPath:   "/wstunnel/_token/test123/some/path",
			expectedPath:  "/_token/test123/some/path",
			shouldRewrite: true,
		},
		{
			name:          "base path matches - root becomes root",
			basePath:      "/wstunnel",
			requestPath:   "/wstunnel",
			expectedPath:  "/",
			shouldRewrite: true,
		},
		{
			name:          "base path matches - root with slash",
			basePath:      "/wstunnel",
			requestPath:   "/wstunnel/",
			expectedPath:  "/",
			shouldRewrite: true,
		},
		{
			name:          "base path doesn't match",
			basePath:      "/wstunnel",
			requestPath:   "/other/_tunnel",
			expectedPath:  "/other/_tunnel",
			shouldRewrite: false,
		},
		{
			name:          "nested base path",
			basePath:      "/api/v1",
			requestPath:   "/api/v1/_tunnel",
			expectedPath:  "/_tunnel",
			shouldRewrite: true,
		},
		{
			name:          "partial base path match (should not rewrite)",
			basePath:      "/wstunnel",
			requestPath:   "/wstunnel2/_tunnel",
			expectedPath:  "/wstunnel2/_tunnel",
			shouldRewrite: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := &WSTunnelServer{
				BasePath: tt.basePath,
				Log:      log15.New(),
			}
			server.Log.SetHandler(log15.DiscardHandler())

			// Create a test handler that records the rewritten URL
			var rewrittenPath string
			testHandler := func(t *WSTunnelServer, w http.ResponseWriter, r *http.Request) {
				rewrittenPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
			}

			// Create the wrapper function (simulating the wrap function from Start)
			wrap := func(h func(t *WSTunnelServer, w http.ResponseWriter, r *http.Request)) func(http.ResponseWriter, *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					// Strip base path from request URL if configured
					if server.BasePath != "" && shouldStripBasePath(r.URL.Path, server.BasePath) {
						// Create a new URL with the base path stripped
						newPath := strings.TrimPrefix(r.URL.Path, server.BasePath)
						if newPath == "" {
							newPath = "/"
						}
						r.URL.Path = newPath
					}
					h(server, w, r)
				}
			}

			// Create a test request
			req := httptest.NewRequest("GET", tt.requestPath, nil)
			w := httptest.NewRecorder()

			// Call the wrapped handler
			wrappedHandler := wrap(testHandler)
			wrappedHandler(w, req)

			// Check that the path was rewritten correctly
			if rewrittenPath != tt.expectedPath {
				t.Errorf("Expected rewritten path to be %q, got %q", tt.expectedPath, rewrittenPath)
			}

			// Verify the original request URL is modified as expected
			if tt.shouldRewrite {
				if req.URL.Path != tt.expectedPath {
					t.Errorf("Expected request URL path to be modified to %q, got %q", tt.expectedPath, req.URL.Path)
				}
			}
		})
	}
}

func TestWSTunnelServer_RouteRegistration_BasePath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		routes   map[string]string // route -> expected registered path
	}{
		{
			name:     "no base path",
			basePath: "",
			routes: map[string]string{
				"/":              "/",
				"/_token/":       "/_token/",
				"/_tunnel":       "/_tunnel",
				"/_health_check": "/_health_check",
				"/_stats":        "/_stats",
			},
		},
		{
			name:     "simple base path",
			basePath: "/wstunnel",
			routes: map[string]string{
				"/":              "/wstunnel/",
				"/_token/":       "/wstunnel/_token/",
				"/_tunnel":       "/wstunnel/_tunnel",
				"/_health_check": "/wstunnel/_health_check",
				"/_stats":        "/wstunnel/_stats",
			},
		},
		{
			name:     "nested base path",
			basePath: "/api/v1",
			routes: map[string]string{
				"/":              "/api/v1/",
				"/_token/":       "/api/v1/_token/",
				"/_tunnel":       "/api/v1/_tunnel",
				"/_health_check": "/api/v1/_health_check",
				"/_stats":        "/api/v1/_stats",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test buildPath function for each route
			for routePath, expectedPath := range tt.routes {
				result := buildPath(tt.basePath, routePath)
				if result != expectedPath {
					t.Errorf("buildPath(%q, %q) = %q, expected %q", tt.basePath, routePath, result, expectedPath)
				}
			}
		})
	}
}

func TestWSTunnelServer_Integration_BasePath(t *testing.T) {
	tests := []struct {
		name        string
		basePath    string
		requestPath string
		expectCode  int
	}{
		{
			name:        "health check with base path",
			basePath:    "/wstunnel",
			requestPath: "/wstunnel/_health_check",
			expectCode:  200,
		},
		{
			name:        "request without base path prefix should 404",
			basePath:    "/wstunnel",
			requestPath: "/_health_check",
			expectCode:  404,
		},
		{
			name:        "incorrect base path prefix should 404",
			basePath:    "/wstunnel",
			requestPath: "/wrong/_health_check",
			expectCode:  404,
		},
		{
			name:        "health check without base path",
			basePath:    "",
			requestPath: "/_health_check",
			expectCode:  200,
		},
		{
			name:        "stats with base path",
			basePath:    "/wstunnel",
			requestPath: "/wstunnel/_stats",
			expectCode:  200,
		},
		{
			name:        "stats without base path",
			basePath:    "",
			requestPath: "/_stats",
			expectCode:  200,
		},
		{
			name:        "tunnel endpoint with base path",
			basePath:    "/wstunnel",
			requestPath: "/wstunnel/_tunnel",
			expectCode:  400, // GET to tunnel without proper WebSocket upgrade
		},
		{
			name:        "nested base path health check",
			basePath:    "/api/v1",
			requestPath: "/api/v1/_health_check",
			expectCode:  200,
		},
		{
			name:        "base path with special valid characters",
			basePath:    "/api-v1.2_test",
			requestPath: "/api-v1.2_test/_health_check",
			expectCode:  200,
		},
		{
			name:        "base path with URL encoded characters",
			basePath:    "/api%20v1",
			requestPath: "/api%20v1/_health_check",
			expectCode:  200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create server with specific base path
			args := []string{"-port", "0"}
			if tt.basePath != "" {
				args = append(args, "-base-path", tt.basePath)
			}

			server := NewWSTunnelServer(args)
			if server == nil {
				t.Fatal("Expected server to be created")
			}
			server.Log.SetHandler(log15.DiscardHandler())

			// Start the server
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = listener.Close() }()

			go server.Start(listener)
			defer server.Stop()

			// Create HTTP client and make request
			client := &http.Client{}
			url := fmt.Sprintf("http://%s%s", listener.Addr().String(), tt.requestPath)

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != tt.expectCode {
				t.Errorf("Expected status code %d, got %d", tt.expectCode, resp.StatusCode)
			}
		})
	}
}
