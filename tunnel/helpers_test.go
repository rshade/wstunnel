package tunnel

import (
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestSetVV(t *testing.T) {
	originalVV := VV
	defer func() { VV = originalVV }() // Restore original value

	testVersion := "1.2.3-test"
	SetVV(testVersion)

	if VV != testVersion {
		t.Errorf("Expected VV to be %q, got %q", testVersion, VV)
	}
}

func TestWritePid(t *testing.T) {
	tests := []struct {
		name        string
		setupFile   func() (string, func())
		expectError bool
		errorText   string
	}{
		{
			name: "empty filename",
			setupFile: func() (string, func()) {
				return "", func() {}
			},
			expectError: false,
		},
		{
			name: "valid filename",
			setupFile: func() (string, func()) {
				tmpFile, err := os.CreateTemp("", "test_pid")
				if err != nil {
					panic(err)
				}
				_ = tmpFile.Close()
				return tmpFile.Name(), func() { _ = os.Remove(tmpFile.Name()) }
			},
			expectError: false,
		},
		{
			name: "invalid directory",
			setupFile: func() (string, func()) {
				return "/nonexistent/directory/pid.file", func() {}
			},
			expectError: true,
			errorText:   "can't create pidfile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename, cleanup := tt.setupFile()
			defer cleanup()

			err := writePid(filename)

			if tt.expectError && err == nil {
				t.Error("Expected writePid to return an error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected writePid to succeed, got error: %v", err)
			}
			if tt.expectError && err != nil && !strings.Contains(err.Error(), tt.errorText) {
				t.Errorf("Expected error to contain %q, got %q", tt.errorText, err.Error())
			}

			// If successful and filename is not empty, check file contents
			if !tt.expectError && filename != "" && err == nil {
				content, readErr := os.ReadFile(filename)
				if readErr != nil {
					t.Errorf("Failed to read pid file: %v", readErr)
				} else {
					pidStr := string(content)
					pid, parseErr := strconv.Atoi(strings.TrimSpace(pidStr))
					if parseErr != nil {
						t.Errorf("Failed to parse PID from file: %v", parseErr)
					}
					if pid != os.Getpid() {
						t.Errorf("Expected PID %d in file, got %d", os.Getpid(), pid)
					}
				}
			}
		})
	}
}

func TestCalcWsTimeout(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected time.Duration
	}{
		{
			name:     "below minimum",
			input:    1,
			expected: 3 * time.Second,
		},
		{
			name:     "at minimum",
			input:    3,
			expected: 3 * time.Second,
		},
		{
			name:     "normal value",
			input:    30,
			expected: 30 * time.Second,
		},
		{
			name:     "at maximum",
			input:    600,
			expected: 600 * time.Second,
		},
		{
			name:     "above maximum",
			input:    800,
			expected: 600 * time.Second,
		},
		{
			name:     "zero",
			input:    0,
			expected: 3 * time.Second,
		},
		{
			name:     "negative",
			input:    -10,
			expected: 3 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.Nop()
			result := calcWsTimeout(logger, tt.input)

			if result != tt.expected {
				t.Errorf("Expected calcWsTimeout(%d) to return %v, got %v", tt.input, tt.expected, result)
			}
		})
	}
}

func TestCopyHeader(t *testing.T) {
	tests := []struct {
		name     string
		src      http.Header
		dst      http.Header
		expected http.Header
	}{
		{
			name: "copy single header",
			src: http.Header{
				"Content-Type": []string{"application/json"},
			},
			dst: http.Header{},
			expected: http.Header{
				"Content-Type": []string{"application/json"},
			},
		},
		{
			name: "copy multiple headers",
			src: http.Header{
				"Content-Type":  []string{"application/json"},
				"Authorization": []string{"Bearer token"},
				"X-Custom":      []string{"value"},
			},
			dst: http.Header{},
			expected: http.Header{
				"Content-Type":  []string{"application/json"},
				"Authorization": []string{"Bearer token"},
				"X-Custom":      []string{"value"},
			},
		},
		{
			name: "copy header with multiple values",
			src: http.Header{
				"Accept": []string{"application/json", "text/html"},
			},
			dst: http.Header{},
			expected: http.Header{
				"Accept": []string{"application/json", "text/html"},
			},
		},
		{
			name: "merge with existing headers",
			src: http.Header{
				"X-New": []string{"new-value"},
			},
			dst: http.Header{
				"X-Existing": []string{"existing-value"},
			},
			expected: http.Header{
				"X-Existing": []string{"existing-value"},
				"X-New":      []string{"new-value"},
			},
		},
		{
			name: "append to existing header",
			src: http.Header{
				"Accept": []string{"application/xml"},
			},
			dst: http.Header{
				"Accept": []string{"application/json"},
			},
			expected: http.Header{
				"Accept": []string{"application/json", "application/xml"},
			},
		},
		{
			name:     "empty source",
			src:      http.Header{},
			dst:      http.Header{"X-Existing": []string{"value"}},
			expected: http.Header{"X-Existing": []string{"value"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of dst since copyHeader modifies it
			dst := make(http.Header)
			for k, v := range tt.dst {
				dst[k] = append([]string(nil), v...)
			}

			copyHeader(dst, tt.src)

			if !reflect.DeepEqual(dst, tt.expected) {
				t.Errorf("Expected headers %v, got %v", tt.expected, dst)
			}
		})
	}
}
