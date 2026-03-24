package tunnel

import (
	"testing"

	"github.com/rs/zerolog"
)

// clientBaseArgs returns the minimum args for NewWSTunnelClient with optional extras appended.
func clientBaseArgs(extras ...string) []string {
	base := []string{
		"-token", "test567890123456",
		"-tunnel", "ws://localhost:8080",
		"-server", "http://localhost:9090",
	}
	return append(base, extras...)
}

func TestNewWSTunnelClientLogLevel(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedLevel zerolog.Level
	}{
		{
			name:          "default log level is info",
			args:          clientBaseArgs(),
			expectedLevel: zerolog.InfoLevel,
		},
		{
			name:          "debug log level",
			args:          clientBaseArgs("-log-level", "debug"),
			expectedLevel: zerolog.DebugLevel,
		},
		{
			name:          "warn log level",
			args:          clientBaseArgs("-log-level", "warn"),
			expectedLevel: zerolog.WarnLevel,
		},
		{
			name:          "error log level",
			args:          clientBaseArgs("-log-level", "error"),
			expectedLevel: zerolog.ErrorLevel,
		},
		{
			name:          "invalid log level falls back to info",
			args:          clientBaseArgs("-log-level", "invalid_level"),
			expectedLevel: zerolog.InfoLevel,
		},
		{
			name:          "empty log level parses as nolevel",
			args:          clientBaseArgs("-log-level", ""),
			expectedLevel: zerolog.NoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupLogCapture(t)

			client := NewWSTunnelClient(tt.args)
			if client == nil {
				t.Fatal("Expected non-nil client")
			}

			if zerolog.GlobalLevel() != tt.expectedLevel {
				t.Errorf("Expected global log level %v, got %v", tt.expectedLevel, zerolog.GlobalLevel())
			}
		})
	}
}

func TestNewWSTunnelClientLogPretty(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		expectedLogPretty bool
	}{
		{
			name:              "default log pretty is false",
			args:              clientBaseArgs(),
			expectedLogPretty: false,
		},
		{
			name:              "log pretty flag set to true",
			args:              clientBaseArgs("-log-pretty"),
			expectedLogPretty: true,
		},
		{
			name:              "log pretty flag set to false explicitly",
			args:              clientBaseArgs("-log-pretty=false"),
			expectedLogPretty: false,
		},
		{
			name:              "log pretty flag set to true explicitly",
			args:              clientBaseArgs("-log-pretty=true"),
			expectedLogPretty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupLogCapture(t)

			client := NewWSTunnelClient(tt.args)
			if client == nil {
				t.Fatal("Expected non-nil client")
			}

			if LogPretty != tt.expectedLogPretty {
				t.Errorf("Expected LogPretty=%v, got %v", tt.expectedLogPretty, LogPretty)
			}
		})
	}
}

func TestNewWSTunnelClientTokenPasswordParsing(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		expectedToken    string
		expectedPassword string
	}{
		{
			name:             "token without password",
			args:             clientBaseArgs(),
			expectedToken:    "test567890123456",
			expectedPassword: "",
		},
		{
			name: "token with password",
			args: []string{
				"-token", "mytoken12345678:mypassword",
				"-tunnel", "ws://localhost:8080",
				"-server", "http://localhost:9090",
			},
			expectedToken:    "mytoken12345678",
			expectedPassword: "mypassword",
		},
		{
			name: "token with password containing colon",
			args: []string{
				"-token", "mytoken12345678:pass:word:123",
				"-tunnel", "ws://localhost:8080",
				"-server", "http://localhost:9090",
			},
			expectedToken:    "mytoken12345678",
			expectedPassword: "pass:word:123",
		},
		{
			name: "token with empty password",
			args: []string{
				"-token", "mytoken12345678:",
				"-tunnel", "ws://localhost:8080",
				"-server", "http://localhost:9090",
			},
			expectedToken:    "mytoken12345678",
			expectedPassword: "",
		},
		{
			name: "token with special characters in password",
			args: []string{
				"-token", "test_token_123456:pass!@#$%^&*()",
				"-tunnel", "ws://localhost:8080",
				"-server", "http://localhost:9090",
			},
			expectedToken:    "test_token_123456",
			expectedPassword: "pass!@#$%^&*()",
		},
		{
			name: "token with spaces trimmed",
			args: []string{
				"-token", " token123456789 : password ",
				"-tunnel", "ws://localhost:8080",
				"-server", "http://localhost:9090",
			},
			expectedToken:    "token123456789",
			expectedPassword: "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupLogCapture(t)

			client := NewWSTunnelClient(tt.args)
			if client == nil {
				t.Fatal("Expected non-nil client")
			}

			if client.Token != tt.expectedToken {
				t.Errorf("Expected token %q, got %q", tt.expectedToken, client.Token)
			}
			if client.Password != tt.expectedPassword {
				t.Errorf("Expected password %q, got %q", tt.expectedPassword, client.Password)
			}
		})
	}
}

func TestNewWSTunnelClientMinimalArgs(t *testing.T) {
	setupLogCapture(t)

	client := NewWSTunnelClient(clientBaseArgs())
	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.Token != "test567890123456" {
		t.Errorf("Expected token 'test567890123456', got %q", client.Token)
	}
	if client.Server != "http://localhost:9090" {
		t.Errorf("Expected server 'http://localhost:9090', got %q", client.Server)
	}
	if client.Tunnel == nil {
		t.Error("Expected Tunnel URL to be parsed")
	} else if client.Tunnel.String() != "ws://localhost:8080" {
		t.Errorf("Expected tunnel URL 'ws://localhost:8080', got %q", client.Tunnel.String())
	}
}

func TestNewWSTunnelClientCombinedFlags(t *testing.T) {
	setupLogCapture(t)

	client := NewWSTunnelClient([]string{
		"-token", "mytoken123456789:mypassword",
		"-tunnel", "ws://localhost:8080",
		"-server", "http://localhost:9090",
		"-log-level", "debug",
		"-log-pretty",
	})
	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.Token != "mytoken123456789" {
		t.Errorf("Expected token 'mytoken123456789', got %q", client.Token)
	}
	if client.Password != "mypassword" {
		t.Errorf("Expected password 'mypassword', got %q", client.Password)
	}
	if !LogPretty {
		t.Error("Expected LogPretty to be true")
	}
}

func TestNewWSTunnelClientValidTunnelURLs(t *testing.T) {
	tests := []struct {
		name      string
		tunnelURL string
	}{
		{name: "ws URL", tunnelURL: "ws://localhost:8080"},
		{name: "wss URL", tunnelURL: "wss://localhost:8443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupLogCapture(t)

			client := NewWSTunnelClient([]string{
				"-token", "test567890123456",
				"-tunnel", tt.tunnelURL,
				"-server", "http://localhost:9090",
			})
			if client == nil {
				t.Error("Expected client to be created for valid URL")
			}
		})
	}
}
