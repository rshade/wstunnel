package tunnel

import (
	"encoding/base64"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestConstantTimeEquals(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		expected bool
	}{
		{name: "equal strings", a: "hello", b: "hello", expected: true},
		{name: "different strings same length", a: "hello", b: "world", expected: false},
		{name: "different lengths", a: "short", b: "longer", expected: false},
		{name: "both empty", a: "", b: "", expected: true},
		{name: "one empty", a: "hello", b: "", expected: false},
		{name: "other empty", a: "", b: "hello", expected: false},
		{name: "case sensitive", a: "Hello", b: "hello", expected: false},
		{name: "special characters", a: "p@ss!w0rd", b: "p@ss!w0rd", expected: true},
		{name: "unicode", a: "héllo", b: "héllo", expected: true},
		{name: "unicode different", a: "héllo", b: "hëllo", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constantTimeEquals(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("constantTimeEquals(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestHttpError(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		errMsg     string
		code       int
	}{
		{name: "bad request", identifier: "192.168.1.1", errMsg: "missing header", code: 400},
		{name: "unauthorized", identifier: "10.0.0.1", errMsg: "auth required", code: 401},
		{name: "not found", identifier: "abcd1234...", errMsg: "tunnel not found", code: 404},
		{name: "too many requests", identifier: "abcd1234...", errMsg: "max clients reached", code: 429},
		{name: "server error", identifier: "10.0.0.1", errMsg: "internal error", code: 500},
		{name: "html escaping", identifier: "addr", errMsg: "<script>alert('xss')</script>", code: 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			log := zerolog.Nop()

			httpError(log, w, tt.identifier, tt.errMsg, tt.code)

			if w.Code != tt.code {
				t.Errorf("expected status %d, got %d", tt.code, w.Code)
			}

			body := w.Body.String()
			// For the XSS test, verify the body is escaped
			if tt.name == "html escaping" {
				if strings.Contains(body, "<script>") {
					t.Error("response body should HTML-escape the error message")
				}
			} else {
				if !strings.Contains(body, tt.errMsg) {
					t.Errorf("expected body to contain %q, got %q", tt.errMsg, body)
				}
			}
		})
	}
}

func TestSafeResponseWriter(t *testing.T) {
	t.Run("prevents duplicate WriteHeader", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw := &safeResponseWriter{ResponseWriter: w}

		sw.WriteHeader(201)
		sw.WriteHeader(500) // should be ignored

		if w.Code != 201 {
			t.Errorf("expected status 201, got %d", w.Code)
		}
	})

	t.Run("Write auto-calls WriteHeader 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw := &safeResponseWriter{ResponseWriter: w}

		_, err := sw.Write([]byte("hello"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		if w.Code != 200 {
			t.Errorf("expected status 200, got %d", w.Code)
		}
		if w.Body.String() != "hello" {
			t.Errorf("expected body 'hello', got %q", w.Body.String())
		}
	})

	t.Run("WriteHeader then Write preserves status", func(t *testing.T) {
		w := httptest.NewRecorder()
		sw := &safeResponseWriter{ResponseWriter: w}

		sw.WriteHeader(404)
		_, err := sw.Write([]byte("not found"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		if w.Code != 404 {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestSafeError(t *testing.T) {
	t.Run("sets content type and status", func(t *testing.T) {
		w := httptest.NewRecorder()
		safeError(w, "test error", 503)

		if w.Code != 503 {
			t.Errorf("expected status 503, got %d", w.Code)
		}
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/plain") {
			t.Errorf("expected text/plain content type, got %q", ct)
		}
		if w.Body.String() != "test error" {
			t.Errorf("expected body 'test error', got %q", w.Body.String())
		}
	})

	t.Run("wraps non-safe writer", func(t *testing.T) {
		w := httptest.NewRecorder()
		safeError(w, "first", 200)
		safeError(w, "second", 500) // second call on already-wrapped writer

		if w.Code != 200 {
			t.Errorf("expected first status 200 preserved, got %d", w.Code)
		}
	})
}

func TestNewResponseWriter(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	rw := newResponseWriter(req)

	if rw.resp.StatusCode != -1 {
		t.Errorf("expected StatusCode -1, got %d", rw.resp.StatusCode)
	}
	if rw.resp.ContentLength != -1 {
		t.Errorf("expected ContentLength -1, got %d", rw.resp.ContentLength)
	}
	if rw.resp.Header == nil {
		t.Error("expected Header to be initialized")
	}
	if rw.resp.Proto != "HTTP/1.1" {
		t.Errorf("expected Proto HTTP/1.1, got %q", rw.resp.Proto)
	}
	if rw.resp.ProtoMajor != 1 || rw.resp.ProtoMinor != 1 {
		t.Errorf("expected Proto 1.1, got %d.%d", rw.resp.ProtoMajor, rw.resp.ProtoMinor)
	}
	if rw.buf == nil {
		t.Error("expected buffer to be initialized")
	}
}

func TestResponseWriterWrite(t *testing.T) {
	t.Run("auto sets status 200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rw := newResponseWriter(req)

		n, err := rw.Write([]byte("hello"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != 5 {
			t.Errorf("expected 5 bytes written, got %d", n)
		}
		if rw.resp.StatusCode != 200 {
			t.Errorf("expected auto-set status 200, got %d", rw.resp.StatusCode)
		}
	})

	t.Run("does not override existing status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rw := newResponseWriter(req)

		rw.WriteHeader(404)
		_, _ = rw.Write([]byte("not found"))

		if rw.resp.StatusCode != 404 {
			t.Errorf("expected status 404 preserved, got %d", rw.resp.StatusCode)
		}
	})

	t.Run("multiple writes accumulate", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rw := newResponseWriter(req)

		_, _ = rw.Write([]byte("hello "))
		_, _ = rw.Write([]byte("world"))

		if rw.buf.String() != "hello world" {
			t.Errorf("expected 'hello world', got %q", rw.buf.String())
		}
	})
}

func TestResponseWriterWriteHeader(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		wantStatus string
	}{
		{name: "200 OK", code: 200, wantStatus: "OK"},
		{name: "201 Created", code: 201, wantStatus: "Created"},
		{name: "404 Not Found", code: 404, wantStatus: "Not Found"},
		{name: "500 Internal Server Error", code: 500, wantStatus: "Internal Server Error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			rw := newResponseWriter(req)

			rw.WriteHeader(tt.code)

			if rw.resp.StatusCode != tt.code {
				t.Errorf("expected StatusCode %d, got %d", tt.code, rw.resp.StatusCode)
			}
			if rw.resp.Status != tt.wantStatus {
				t.Errorf("expected Status %q, got %q", tt.wantStatus, rw.resp.Status)
			}
		})
	}
}

func TestResponseWriterHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rw := newResponseWriter(req)

	h := rw.Header()
	h.Set("X-Custom", "value")

	if rw.resp.Header.Get("X-Custom") != "value" {
		t.Error("Header() should return the response header map")
	}
}

func TestResponseWriterFinishResponse(t *testing.T) {
	t.Run("error when no write or writeheader", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rw := newResponseWriter(req)

		err := rw.finishResponse()
		if err == nil {
			t.Error("expected error when StatusCode is -1")
		}
	})

	t.Run("success after Write", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rw := newResponseWriter(req)

		_, _ = rw.Write([]byte("hello"))
		err := rw.finishResponse()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if rw.resp.ContentLength != 5 {
			t.Errorf("expected ContentLength 5, got %d", rw.resp.ContentLength)
		}
	})

	t.Run("success after WriteHeader only", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rw := newResponseWriter(req)

		rw.WriteHeader(204)
		err := rw.finishResponse()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if rw.resp.ContentLength != 0 {
			t.Errorf("expected ContentLength 0, got %d", rw.resp.ContentLength)
		}
	})

	t.Run("body readable after finish", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rw := newResponseWriter(req)

		_, _ = rw.Write([]byte("response body"))
		err := rw.finishResponse()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		body, _ := io.ReadAll(rw.resp.Body)
		if string(body) != "response body" {
			t.Errorf("expected body 'response body', got %q", string(body))
		}
	})
}

func TestBasicAuth(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		expected string
	}{
		{
			name:     "standard credentials",
			username: "user",
			password: "pass",
			expected: base64.StdEncoding.EncodeToString([]byte("user:pass")),
		},
		{
			name:     "empty username",
			username: "",
			password: "pass",
			expected: base64.StdEncoding.EncodeToString([]byte(":pass")),
		},
		{
			name:     "empty password",
			username: "user",
			password: "",
			expected: base64.StdEncoding.EncodeToString([]byte("user:")),
		},
		{
			name:     "both empty",
			username: "",
			password: "",
			expected: base64.StdEncoding.EncodeToString([]byte(":")),
		},
		{
			name:     "special characters",
			username: "user@example.com",
			password: "p@ss!w0rd#123",
			expected: base64.StdEncoding.EncodeToString([]byte("user@example.com:p@ss!w0rd#123")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := basicAuth(tt.username, tt.password)
			if result != tt.expected {
				t.Errorf("basicAuth(%q, %q) = %q, want %q", tt.username, tt.password, result, tt.expected)
			}

			// Verify roundtrip
			decoded, err := base64.StdEncoding.DecodeString(result)
			if err != nil {
				t.Fatalf("failed to decode result: %v", err)
			}
			expected := tt.username + ":" + tt.password
			if string(decoded) != expected {
				t.Errorf("decoded %q, want %q", string(decoded), expected)
			}
		})
	}
}

func TestProxyAuth(t *testing.T) {
	tests := []struct {
		name     string
		proxyURL string
		expected string
	}{
		{
			name:     "with user and password",
			proxyURL: "http://user:pass@proxy.example.com:8080",
			expected: "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
		},
		{
			name:     "with user only",
			proxyURL: "http://user@proxy.example.com:8080",
			expected: "Basic " + base64.StdEncoding.EncodeToString([]byte("user:")),
		},
		{
			name:     "without credentials",
			proxyURL: "http://proxy.example.com:8080",
			expected: "",
		},
		{
			name:     "with empty password",
			proxyURL: "http://user:@proxy.example.com:8080",
			expected: "Basic " + base64.StdEncoding.EncodeToString([]byte("user:")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.proxyURL)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}
			result := proxyAuth(u)
			if result != tt.expected {
				t.Errorf("proxyAuth(%q) = %q, want %q", tt.proxyURL, result, tt.expected)
			}
		})
	}
}

func TestWsp(t *testing.T) {
	result := wsp(nil)
	if result == "" {
		t.Error("wsp(nil) should return a non-empty string")
	}
}

// TestConcoctResponse tests the error response constructor
func TestConcoctResponse(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	resp := concoctResponse(req, "test error", 502)

	if resp.StatusCode != 502 {
		t.Errorf("expected status 502, got %d", resp.StatusCode)
	}
	if resp.Proto != "HTTP/1.1" {
		t.Errorf("expected proto HTTP/1.1, got %q", resp.Proto)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "test error") {
		t.Errorf("expected body to contain 'test error', got %q", string(body))
	}
	if resp.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %q", resp.Header.Get("Content-Type"))
	}
}
