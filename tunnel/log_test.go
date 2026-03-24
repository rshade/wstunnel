//go:build !windows

package tunnel

import (
	"bytes"
	"io"
	"log/syslog"
	"os"
	"testing"

	"github.com/rs/zerolog"
)

// setupLogCapture saves and restores global logging state, redirects log output
// to a buffer, and returns that buffer. Cleanup is automatic via t.Cleanup.
func setupLogCapture(t *testing.T) *bytes.Buffer {
	t.Helper()
	origWriter := DefaultLogWriter
	origPretty := LogPretty
	origLevel := zerolog.GlobalLevel()
	t.Cleanup(func() {
		DefaultLogWriter = origWriter
		LogPretty = origPretty
		zerolog.SetGlobalLevel(origLevel)
	})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	buf := &bytes.Buffer{}
	DefaultLogWriter = buf
	LogPretty = false
	return buf
}

func TestParseSyslogFacility(t *testing.T) {
	tests := []struct {
		name             string
		facility         string
		expectedPriority syslog.Priority
		expectedOk       bool
	}{
		{name: "kern", facility: "kern", expectedPriority: syslog.LOG_KERN, expectedOk: true},
		{name: "user", facility: "user", expectedPriority: syslog.LOG_USER, expectedOk: true},
		{name: "mail", facility: "mail", expectedPriority: syslog.LOG_MAIL, expectedOk: true},
		{name: "daemon", facility: "daemon", expectedPriority: syslog.LOG_DAEMON, expectedOk: true},
		{name: "auth", facility: "auth", expectedPriority: syslog.LOG_AUTH, expectedOk: true},
		{name: "syslog", facility: "syslog", expectedPriority: syslog.LOG_SYSLOG, expectedOk: true},
		{name: "lpr", facility: "lpr", expectedPriority: syslog.LOG_LPR, expectedOk: true},
		{name: "news", facility: "news", expectedPriority: syslog.LOG_NEWS, expectedOk: true},
		{name: "uucp", facility: "uucp", expectedPriority: syslog.LOG_UUCP, expectedOk: true},
		{name: "cron", facility: "cron", expectedPriority: syslog.LOG_CRON, expectedOk: true},
		{name: "authpriv", facility: "authpriv", expectedPriority: syslog.LOG_AUTHPRIV, expectedOk: true},
		{name: "ftp", facility: "ftp", expectedPriority: syslog.LOG_FTP, expectedOk: true},
		{name: "local0", facility: "local0", expectedPriority: syslog.LOG_LOCAL0, expectedOk: true},
		{name: "local1", facility: "local1", expectedPriority: syslog.LOG_LOCAL1, expectedOk: true},
		{name: "local2", facility: "local2", expectedPriority: syslog.LOG_LOCAL2, expectedOk: true},
		{name: "local3", facility: "local3", expectedPriority: syslog.LOG_LOCAL3, expectedOk: true},
		{name: "local4", facility: "local4", expectedPriority: syslog.LOG_LOCAL4, expectedOk: true},
		{name: "local5", facility: "local5", expectedPriority: syslog.LOG_LOCAL5, expectedOk: true},
		{name: "local6", facility: "local6", expectedPriority: syslog.LOG_LOCAL6, expectedOk: true},
		{name: "local7", facility: "local7", expectedPriority: syslog.LOG_LOCAL7, expectedOk: true},
		{name: "uppercase KERN", facility: "KERN", expectedPriority: syslog.LOG_KERN, expectedOk: true},
		{name: "mixed case Daemon", facility: "Daemon", expectedPriority: syslog.LOG_DAEMON, expectedOk: true},
		{name: "mixed case Local0", facility: "Local0", expectedPriority: syslog.LOG_LOCAL0, expectedOk: true},
		{name: "whitespace leading", facility: " kern", expectedPriority: syslog.LOG_KERN, expectedOk: true},
		{name: "whitespace trailing", facility: "daemon ", expectedPriority: syslog.LOG_DAEMON, expectedOk: true},
		{name: "whitespace both", facility: "  user  ", expectedPriority: syslog.LOG_USER, expectedOk: true},
		{name: "whitespace tabs", facility: "\tsyslog\t", expectedPriority: syslog.LOG_SYSLOG, expectedOk: true},
		{name: "invalid", facility: "invalid", expectedPriority: 0, expectedOk: false},
		{name: "unknown", facility: "unknown", expectedPriority: 0, expectedOk: false},
		{name: "empty", facility: "", expectedPriority: 0, expectedOk: false},
		{name: "random", facility: "notafacility", expectedPriority: 0, expectedOk: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority, ok := parseSyslogFacility(tt.facility)
			if ok != tt.expectedOk {
				t.Errorf("Expected ok=%v, got ok=%v", tt.expectedOk, ok)
			}
			if ok && priority != tt.expectedPriority {
				t.Errorf("Expected priority=%v, got priority=%v", tt.expectedPriority, priority)
			}
		})
	}
}

func TestGetWriter(t *testing.T) {
	originalLogPretty := LogPretty
	defer func() { LogPretty = originalLogPretty }()

	tests := []struct {
		name      string
		logPretty bool
		checkType func(io.Writer) bool
	}{
		{
			name:      "LogPretty false returns same writer",
			logPretty: false,
			checkType: func(w io.Writer) bool {
				_, ok := w.(*bytes.Buffer)
				return ok
			},
		},
		{
			name:      "LogPretty true returns ConsoleWriter",
			logPretty: true,
			checkType: func(w io.Writer) bool {
				_, ok := w.(zerolog.ConsoleWriter)
				return ok
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			LogPretty = tt.logPretty
			buffer := &bytes.Buffer{}
			result := getWriter(buffer)

			if !tt.checkType(result) {
				t.Errorf("getWriter() returned unexpected type: %T", result)
			}

			if !tt.logPretty && result != buffer {
				t.Error("When LogPretty is false, should return the same writer instance")
			}
		})
	}
}

func TestNewPkgLogger(t *testing.T) {
	logBuffer := setupLogCapture(t)

	logger := newPkgLogger()
	logger.Info().Msg("test message")

	if logBuffer.Len() == 0 {
		t.Fatal("Expected logger to write output")
	}
}

func TestMakeLoggerDefaultPath(t *testing.T) {
	logBuffer := setupLogCapture(t)

	logger := makeLogger("testpkg", "", "")
	logger.Info().Msg("test message")

	if logBuffer.Len() == 0 {
		t.Fatal("Expected bootstrap logger to write output")
	}
}

func TestMakeLoggerWithFilePath(t *testing.T) {
	setupLogCapture(t)

	tmpFile, err := os.CreateTemp("", "wstunnel_test_log_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_ = tmpFile.Close()

	logger := makeLogger("testpkg", tmpFile.Name(), "")
	logger.Info().Msg("test message")

	content, readErr := os.ReadFile(tmpFile.Name())
	if readErr != nil {
		t.Errorf("Failed to read log file: %v", readErr)
		return
	}
	if len(content) == 0 {
		t.Error("Expected log file to contain data")
	}
}

func TestMakeLoggerWithSyslogFacility(t *testing.T) {
	setupLogCapture(t)

	tests := []struct {
		name     string
		facility string
	}{
		{name: "local0", facility: "local0"},
		{name: "daemon", facility: "daemon"},
		{name: "invalid", facility: "invalid_facility"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Recovered from panic (expected if syslog unavailable): %v", r)
				}
			}()

			logger := makeLogger("testpkg", "", tt.facility)
			logger.Info().Msg("test message")
		})
	}
}
