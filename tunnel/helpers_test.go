package tunnel

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/inconshreveable/log15.v2"
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

func TestSimpleFormat(t *testing.T) {
	tests := []struct {
		name       string
		timestamps bool
		level      log15.Lvl
		msg        string
		ctx        []interface{}
		checkFunc  func(string) bool
	}{
		{
			name:       "with timestamps",
			timestamps: true,
			level:      log15.LvlInfo,
			msg:        "test message",
			ctx:        []interface{}{"key", "value"},
			checkFunc: func(output string) bool {
				return strings.Contains(output, "INFO test message") &&
					strings.Contains(output, "key=value") &&
					strings.Contains(output, time.Now().Format("2006-01-02"))
			},
		},
		{
			name:       "without timestamps",
			timestamps: false,
			level:      log15.LvlError,
			msg:        "error message",
			ctx:        []interface{}{"error", "something bad"},
			checkFunc: func(output string) bool {
				return strings.Contains(output, "EROR error message") &&
					strings.Contains(output, "error=\"something bad\"") &&
					!strings.Contains(output, time.Now().Format("2006-01-02"))
			},
		},
		{
			name:       "empty context",
			timestamps: false,
			level:      log15.LvlWarn,
			msg:        "warning",
			ctx:        []interface{}{},
			checkFunc: func(output string) bool {
				return strings.Contains(output, "WARN warning") &&
					strings.HasSuffix(strings.TrimSpace(output), "warning")
			},
		},
		{
			name:       "short message with justification",
			timestamps: false,
			level:      log15.LvlDebug,
			msg:        "short",
			ctx:        []interface{}{"key", "value"},
			checkFunc: func(output string) bool {
				// Short messages should be justified with spaces
				return strings.Contains(output, "DBUG short") &&
					strings.Contains(output, "key=value")
			},
		},
		{
			name:       "multiple context pairs",
			timestamps: false,
			level:      log15.LvlInfo,
			msg:        "multi context",
			ctx:        []interface{}{"key1", "value1", "key2", "value2", "key3", "value3"},
			checkFunc: func(output string) bool {
				return strings.Contains(output, "key1=value1") &&
					strings.Contains(output, "key2=value2") &&
					strings.Contains(output, "key3=value3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := SimpleFormat(tt.timestamps)
			record := &log15.Record{
				Time: time.Now(),
				Lvl:  tt.level,
				Msg:  tt.msg,
				Ctx:  tt.ctx,
			}

			output := formatter.Format(record)
			outputStr := string(output)

			if !tt.checkFunc(outputStr) {
				t.Errorf("Format output did not match expected pattern: %q", outputStr)
			}
		})
	}
}

func TestFormatLogfmtValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "nil value",
			value:    nil,
			expected: "nil",
		},
		{
			name:     "boolean true",
			value:    true,
			expected: "true",
		},
		{
			name:     "boolean false",
			value:    false,
			expected: "false",
		},
		{
			name:     "integer",
			value:    42,
			expected: "42",
		},
		{
			name:     "float32",
			value:    float32(3.14),
			expected: "3.140",
		},
		{
			name:     "float64",
			value:    3.14159,
			expected: "3.142",
		},
		{
			name:     "string simple",
			value:    "hello",
			expected: "hello",
		},
		{
			name:     "string with spaces",
			value:    "hello world",
			expected: "\"hello world\"",
		},
		{
			name:     "string with equals",
			value:    "key=value",
			expected: "\"key=value\"",
		},
		{
			name:     "string with quotes",
			value:    "say \"hello\"",
			expected: "\"say \\\"hello\\\"\"",
		},
		{
			name:     "time value",
			value:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: "\"2023-01-01 12:00:00\"",
		},
		{
			name:     "error value",
			value:    errors.New("test error"),
			expected: "\"test error\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLogfmtValue(tt.value)
			if result != tt.expected {
				t.Errorf("Expected formatLogfmtValue(%v) to return %q, got %q", tt.value, tt.expected, result)
			}
		})
	}
}

func TestFormatShared(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected interface{}
	}{
		{
			name:     "time value",
			value:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: "2023-01-01 12:00:00",
		},
		{
			name:     "error value",
			value:    errors.New("test error"),
			expected: "test error",
		},
		{
			name:     "stringer value",
			value:    &testStringer{"test string"},
			expected: "test string",
		},
		{
			name:     "regular value",
			value:    42,
			expected: 42,
		},
		{
			name:     "nil pointer recovery",
			value:    (*testStringer)(nil),
			expected: "nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatShared(tt.value)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected formatShared(%v) to return %v, got %v", tt.value, tt.expected, result)
			}
		})
	}
}

// testStringer implements fmt.Stringer for testing
type testStringer struct {
	value string
}

func (ts *testStringer) String() string {
	return ts.value
}

func TestEscapeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "string with spaces",
			input:    "hello world",
			expected: "\"hello world\"",
		},
		{
			name:     "string with equals",
			input:    "key=value",
			expected: "\"key=value\"",
		},
		{
			name:     "string with quotes",
			input:    "say \"hello\"",
			expected: "\"say \\\"hello\\\"\"",
		},
		{
			name:     "string with newline",
			input:    "line1\nline2",
			expected: "\"line1\\nline2\"",
		},
		{
			name:     "string with carriage return",
			input:    "line1\rline2",
			expected: "\"line1\\rline2\"",
		},
		{
			name:     "string with tab",
			input:    "col1\tcol2",
			expected: "\"col1\\tcol2\"",
		},
		{
			name:     "string with backslash",
			input:    "path\\to\\file",
			expected: "path\\\\to\\\\file",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "string with control characters",
			input:    "test\x01\x02",
			expected: "\"test\x01\x02\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected escapeString(%q) to return %q, got %q", tt.input, tt.expected, result)
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
			// Capture log output to verify the function logs
			var logOutput bytes.Buffer
			logger := log15.New()
			logger.SetHandler(log15.StreamHandler(&logOutput, log15.LogfmtFormat()))

			// Temporarily replace the global logger
			originalLogger := log15.Root()
			log15.Root().SetHandler(logger.GetHandler())
			defer log15.Root().SetHandler(originalLogger.GetHandler())

			result := calcWsTimeout(tt.input)

			if result != tt.expected {
				t.Errorf("Expected calcWsTimeout(%d) to return %v, got %v", tt.input, tt.expected, result)
			}

			// Verify that the function logs the timeout value
			logStr := logOutput.String()
			if !strings.Contains(logStr, "Setting WS keep-alive") {
				t.Error("Expected log message about setting WS keep-alive")
			}
			if !strings.Contains(logStr, fmt.Sprintf("timeout=%v", tt.expected)) {
				t.Errorf("Expected log to contain timeout=%v", tt.expected)
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

func TestSimpleFormat_ErrorInContext(t *testing.T) {
	// Test case where context has invalid key (not a string)
	formatter := SimpleFormat(false)
	record := &log15.Record{
		Time: time.Now(),
		Lvl:  log15.LvlInfo,
		Msg:  "test",
		Ctx:  []interface{}{123, "value"}, // First element is not a string
	}

	output := formatter.Format(record)
	outputStr := string(output)

	// Should handle invalid key by using LOG_ERR
	if !strings.Contains(outputStr, "LOG_ERR=") {
		t.Errorf("Expected LOG_ERR for invalid key, got: %q", outputStr)
	}
}

func TestSimpleFormat_LongMessage(t *testing.T) {
	// Test that long messages don't get extra justification spaces
	longMsg := strings.Repeat("a", 50) // Longer than simpleMsgJust (40)

	formatter := SimpleFormat(false)
	record := &log15.Record{
		Time: time.Now(),
		Lvl:  log15.LvlInfo,
		Msg:  longMsg,
		Ctx:  []interface{}{"key", "value"},
	}

	output := formatter.Format(record)
	outputStr := string(output)

	// Should not have excessive spaces between message and context
	expectedPattern := longMsg + " key=value"
	if !strings.Contains(outputStr, expectedPattern) {
		t.Errorf("Expected pattern %q in output, got: %q", expectedPattern, outputStr)
	}
}
