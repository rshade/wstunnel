// Copyright (c) 2024 RightScale, Inc. - see LICENSE

package tunnel

import (
	"strings"
	"testing"
)

// TestParseClientConfigFlags tests the ParseClientConfig function
func TestParseClientConfigFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		errContains string
		checkConfig func(*testing.T, *ClientConfig)
	}{
		// Token parsing tests
		{
			name: "parse token without password",
			args: []string{"-token", "mytoken12345678901", "-tunnel", "ws://localhost:8080"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Token != "mytoken12345678901" {
					t.Errorf("Expected token 'mytoken12345678901', got %q", cfg.Token)
				}
				if cfg.Password != "" {
					t.Errorf("Expected empty password, got %q", cfg.Password)
				}
			},
		},
		{
			name: "parse token with password",
			args: []string{"-token", "mytoken12345678901:mypassword", "-tunnel", "ws://localhost:8080"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Token != "mytoken12345678901:mypassword" {
					t.Errorf("Expected token 'mytoken12345678901:mypassword', got %q", cfg.Token)
				}
				if cfg.Password != "" {
					t.Errorf("Expected empty password, got %q", cfg.Password)
				}
			},
		},
		// URL flags validation tests
		{
			name: "parse valid tunnel URL",
			args: []string{"-tunnel", "ws://example.com:8080", "-token", "test"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Tunnel != "ws://example.com:8080" {
					t.Errorf("Expected tunnel 'ws://example.com:8080', got %q", cfg.Tunnel)
				}
			},
		},
		{
			name: "parse valid secure tunnel URL",
			args: []string{"-tunnel", "wss://user:pass@example.com:8443/path", "-token", "test"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Tunnel != "wss://user:pass@example.com:8443/path" {
					t.Errorf("Expected tunnel 'wss://user:pass@example.com:8443/path', got %q", cfg.Tunnel)
				}
			},
		},
		{
			name: "parse valid server URL",
			args: []string{"-server", "http://localhost:3000", "-tunnel", "ws://example.com", "-token", "test"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Server != "http://localhost:3000" {
					t.Errorf("Expected server 'http://localhost:3000', got %q", cfg.Server)
				}
			},
		},
		{
			name: "parse valid secure server URL",
			args: []string{"-server", "https://localhost:3443", "-tunnel", "ws://example.com", "-token", "test"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Server != "https://localhost:3443" {
					t.Errorf("Expected server 'https://localhost:3443', got %q", cfg.Server)
				}
			},
		},
		// Boolean flags tests
		{
			name: "default insecure to false",
			args: []string{"-token", "test", "-tunnel", "ws://localhost"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Insecure {
					t.Error("Expected insecure to be false")
				}
			},
		},
		{
			name: "parse insecure flag",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-insecure"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if !cfg.Insecure {
					t.Error("Expected insecure to be true")
				}
			},
		},
		// Numeric flags tests
		{
			name: "use default timeout",
			args: []string{"-token", "test", "-tunnel", "ws://localhost"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Timeout != 30 {
					t.Errorf("Expected timeout 30, got %d", cfg.Timeout)
				}
			},
		},
		{
			name: "parse custom timeout",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-timeout", "60"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Timeout != 60 {
					t.Errorf("Expected timeout 60, got %d", cfg.Timeout)
				}
			},
		},
		{
			name: "use default reconnect-delay",
			args: []string{"-token", "test", "-tunnel", "ws://localhost"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.ReconnectDelay != 5 {
					t.Errorf("Expected reconnect-delay 5, got %d", cfg.ReconnectDelay)
				}
			},
		},
		{
			name: "parse custom reconnect-delay",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-reconnect-delay", "10"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.ReconnectDelay != 10 {
					t.Errorf("Expected reconnect-delay 10, got %d", cfg.ReconnectDelay)
				}
			},
		},
		{
			name: "use default max-retries",
			args: []string{"-token", "test", "-tunnel", "ws://localhost"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.MaxRetries != 0 {
					t.Errorf("Expected max-retries 0, got %d", cfg.MaxRetries)
				}
			},
		},
		{
			name: "parse custom max-retries",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-max-retries", "3"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.MaxRetries != 3 {
					t.Errorf("Expected max-retries 3, got %d", cfg.MaxRetries)
				}
			},
		},
		// String flags tests
		{
			name: "parse regexp flag",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-regexp", "^https?://[a-z]+\\.example\\.com"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Regexp != "^https?://[a-z]+\\.example\\.com" {
					t.Errorf("Expected regexp '^https?://[a-z]+\\.example\\.com', got %q", cfg.Regexp)
				}
			},
		},
		{
			name: "parse pidfile flag",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-pidfile", "/var/run/wstunnel.pid"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.PidFile != "/var/run/wstunnel.pid" {
					t.Errorf("Expected pidfile '/var/run/wstunnel.pid', got %q", cfg.PidFile)
				}
			},
		},
		{
			name: "parse logfile flag",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-logfile", "/var/log/wstunnel.log"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.LogFile != "/var/log/wstunnel.log" {
					t.Errorf("Expected logfile '/var/log/wstunnel.log', got %q", cfg.LogFile)
				}
			},
		},
		{
			name: "parse statusfile flag",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-statusfile", "/var/run/wstunnel.status"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.StatusFile != "/var/run/wstunnel.status" {
					t.Errorf("Expected statusfile '/var/run/wstunnel.status', got %q", cfg.StatusFile)
				}
			},
		},
		{
			name: "parse certfile flag",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-certfile", "/etc/ssl/certs/ca.pem"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.CertFile != "/etc/ssl/certs/ca.pem" {
					t.Errorf("Expected certfile '/etc/ssl/certs/ca.pem', got %q", cfg.CertFile)
				}
			},
		},
		{
			name: "parse proxy flag",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-proxy", "http://user:pass@proxy.example.com:8080"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Proxy != "http://user:pass@proxy.example.com:8080" {
					t.Errorf("Expected proxy 'http://user:pass@proxy.example.com:8080', got %q", cfg.Proxy)
				}
			},
		},
		// Client-ports parsing tests
		{
			name: "parse single port",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-client-ports", "8080"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.ClientPorts != "8080" {
					t.Errorf("Expected client-ports '8080', got %q", cfg.ClientPorts)
				}
			},
		},
		{
			name: "parse multiple ports",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-client-ports", "8080,8081,8082"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.ClientPorts != "8080,8081,8082" {
					t.Errorf("Expected client-ports '8080,8081,8082', got %q", cfg.ClientPorts)
				}
			},
		},
		{
			name: "parse port ranges",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-client-ports", "8000..8100"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.ClientPorts != "8000..8100" {
					t.Errorf("Expected client-ports '8000..8100', got %q", cfg.ClientPorts)
				}
			},
		},
		{
			name: "parse mixed ports and ranges",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-client-ports", "8000..8100,8300..8400,8500,8505"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.ClientPorts != "8000..8100,8300..8400,8500,8505" {
					t.Errorf("Expected client-ports '8000..8100,8300..8400,8500,8505', got %q", cfg.ClientPorts)
				}
			},
		},
		// Error conditions tests
		{
			name: "handle empty arguments",
			args: []string{},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg == nil {
					t.Error("Expected non-nil config")
				}
			},
		},
		{
			name:        "handle unknown flags",
			args:        []string{"-unknown-flag", "value", "-token", "test", "-tunnel", "ws://localhost"},
			wantErr:     true,
			errContains: "unknown",
		},
		{
			name:    "handle missing flag values",
			args:    []string{"-token"},
			wantErr: true,
		},
		{
			name:    "handle invalid numeric values",
			args:    []string{"-token", "test", "-tunnel", "ws://localhost", "-timeout", "invalid"},
			wantErr: true,
		},
		// All flags combined test
		{
			name: "parse all flags correctly",
			args: []string{
				"-token", "mytoken12345678901:password123",
				"-tunnel", "wss://user:pass@tunnel.example.com:8443/path",
				"-server", "https://backend.local:3443",
				"-insecure",
				"-regexp", "^https?://.*\\.local",
				"-timeout", "120",
				"-pidfile", "/var/run/wstunnel.pid",
				"-logfile", "/var/log/wstunnel.log",
				"-statusfile", "/var/run/wstunnel.status",
				"-proxy", "http://proxy.local:3128",
				"-client-ports", "8000..8100,9000,9005",
				"-certfile", "/etc/ssl/ca.pem",
				"-reconnect-delay", "15",
				"-max-retries", "5",
			},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Token != "mytoken12345678901:password123" {
					t.Errorf("Expected token 'mytoken12345678901:password123', got %q", cfg.Token)
				}
				if cfg.Tunnel != "wss://user:pass@tunnel.example.com:8443/path" {
					t.Errorf("Expected tunnel 'wss://user:pass@tunnel.example.com:8443/path', got %q", cfg.Tunnel)
				}
				if cfg.Server != "https://backend.local:3443" {
					t.Errorf("Expected server 'https://backend.local:3443', got %q", cfg.Server)
				}
				if !cfg.Insecure {
					t.Error("Expected insecure to be true")
				}
				if cfg.Regexp != "^https?://.*\\.local" {
					t.Errorf("Expected regexp '^https?://.*\\.local', got %q", cfg.Regexp)
				}
				if cfg.Timeout != 120 {
					t.Errorf("Expected timeout 120, got %d", cfg.Timeout)
				}
				if cfg.PidFile != "/var/run/wstunnel.pid" {
					t.Errorf("Expected pidfile '/var/run/wstunnel.pid', got %q", cfg.PidFile)
				}
				if cfg.LogFile != "/var/log/wstunnel.log" {
					t.Errorf("Expected logfile '/var/log/wstunnel.log', got %q", cfg.LogFile)
				}
				if cfg.StatusFile != "/var/run/wstunnel.status" {
					t.Errorf("Expected statusfile '/var/run/wstunnel.status', got %q", cfg.StatusFile)
				}
				if cfg.Proxy != "http://proxy.local:3128" {
					t.Errorf("Expected proxy 'http://proxy.local:3128', got %q", cfg.Proxy)
				}
				if cfg.ClientPorts != "8000..8100,9000,9005" {
					t.Errorf("Expected client-ports '8000..8100,9000,9005', got %q", cfg.ClientPorts)
				}
				if cfg.CertFile != "/etc/ssl/ca.pem" {
					t.Errorf("Expected certfile '/etc/ssl/ca.pem', got %q", cfg.CertFile)
				}
				if cfg.ReconnectDelay != 15 {
					t.Errorf("Expected reconnect-delay 15, got %d", cfg.ReconnectDelay)
				}
				if cfg.MaxRetries != 5 {
					t.Errorf("Expected max-retries 5, got %d", cfg.MaxRetries)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseClientConfig(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseClientConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ParseClientConfig() error = %v, want error containing %q", err, tt.errContains)
				}
			}
			if !tt.wantErr && tt.checkConfig != nil {
				tt.checkConfig(t, config)
			}
		})
	}
}

// TestParseClientConfig provides additional unit tests using Go's standard testing
func TestParseClientConfig(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		checkConfig func(*testing.T, *ClientConfig)
	}{
		{
			name: "minimal valid config",
			args: []string{"-token", "test123456789012", "-tunnel", "ws://localhost:8080"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Token != "test123456789012" {
					t.Errorf("Expected token 'test123456789012', got %q", cfg.Token)
				}
				if cfg.Tunnel != "ws://localhost:8080" {
					t.Errorf("Expected tunnel 'ws://localhost:8080', got %q", cfg.Tunnel)
				}
			},
		},
		{
			name: "token with special characters",
			args: []string{"-token", "token!@#$%^&*()_+-=:pass!@#$", "-tunnel", "ws://localhost"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Token != "token!@#$%^&*()_+-=:pass!@#$" {
					t.Errorf("Expected token with special chars, got %q", cfg.Token)
				}
			},
		},
		{
			name: "empty token allowed",
			args: []string{"-token", "", "-tunnel", "ws://localhost"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Token != "" {
					t.Errorf("Expected empty token, got %q", cfg.Token)
				}
			},
		},
		{
			name:    "invalid timeout value",
			args:    []string{"-token", "test", "-tunnel", "ws://localhost", "-timeout", "not-a-number"},
			wantErr: true,
		},
		{
			name:    "negative timeout value",
			args:    []string{"-token", "test", "-tunnel", "ws://localhost", "-timeout", "-5"},
			wantErr: false, // Flag package allows negative values
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Timeout != -5 {
					t.Errorf("Expected timeout -5, got %d", cfg.Timeout)
				}
			},
		},
		{
			name: "boolean flag variations",
			args: []string{"-token", "test", "-tunnel", "ws://localhost", "-insecure=true"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if !cfg.Insecure {
					t.Error("Expected insecure to be true")
				}
			},
		},
		{
			name: "flag with equals sign",
			args: []string{"-token=test123456789012", "-tunnel=ws://localhost:8080"},
			checkConfig: func(t *testing.T, cfg *ClientConfig) {
				if cfg.Token != "test123456789012" {
					t.Errorf("Expected token 'test123456789012', got %q", cfg.Token)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseClientConfig(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseClientConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkConfig != nil {
				tt.checkConfig(t, config)
			}
		})
	}
}

// TestClientConfigURLPathHandling tests URL path handling in NewWSTunnelClientFromConfig
func TestClientConfigURLPathHandling(t *testing.T) {
	tests := []struct {
		name        string
		config      *ClientConfig
		checkClient func(*testing.T, *WSTunnelClient)
	}{
		{
			name: "strip custom path from tunnel URL",
			config: &ClientConfig{
				Token:  "mytoken12345678901",
				Tunnel: "ws://localhost:8080/custom/path",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Tunnel.Path != "" {
					t.Errorf("Expected empty path, got %q", client.Tunnel.Path)
				}
			},
		},
		{
			name: "strip custom path from secure tunnel URL",
			config: &ClientConfig{
				Token:  "mytoken12345678901",
				Tunnel: "wss://user:pass@example.com:8443/path/to/tunnel",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Tunnel.Path != "" {
					t.Errorf("Expected empty path, got %q", client.Tunnel.Path)
				}
			},
		},
		{
			name: "handle tunnel URL without path",
			config: &ClientConfig{
				Token:  "mytoken12345678901",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Tunnel.Path != "" {
					t.Errorf("Expected empty path, got %q", client.Tunnel.Path)
				}
			},
		},
		{
			name: "preserve other URL components when stripping path",
			config: &ClientConfig{
				Token:  "mytoken12345678901",
				Tunnel: "wss://user:pass@example.com:8443/path?query=value#fragment",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Tunnel.Scheme != "wss" {
					t.Errorf("Expected scheme 'wss', got %q", client.Tunnel.Scheme)
				}
				if client.Tunnel.Host != "example.com:8443" {
					t.Errorf("Expected host 'example.com:8443', got %q", client.Tunnel.Host)
				}
				if client.Tunnel.User == nil {
					t.Error("Expected non-nil user info")
				} else {
					if client.Tunnel.User.Username() != "user" {
						t.Errorf("Expected username 'user', got %q", client.Tunnel.User.Username())
					}
					pass, _ := client.Tunnel.User.Password()
					if pass != "pass" {
						t.Errorf("Expected password 'pass', got %q", pass)
					}
				}
				if client.Tunnel.Path != "" {
					t.Errorf("Expected empty path, got %q", client.Tunnel.Path)
				}
				// Query and fragment are also stripped
				if client.Tunnel.RawQuery != "" {
					t.Errorf("Expected empty RawQuery, got %q", client.Tunnel.RawQuery)
				}
				if client.Tunnel.Fragment != "" {
					t.Errorf("Expected empty Fragment, got %q", client.Tunnel.Fragment)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewWSTunnelClientFromConfig(tt.config)
			if err != nil {
				t.Errorf("NewWSTunnelClientFromConfig() error = %v", err)
				return
			}
			if tt.checkClient != nil {
				tt.checkClient(t, client)
			}
		})
	}
}

// TestClientConfigTokenPasswordParsing tests token:password parsing in NewWSTunnelClientFromConfig
func TestClientConfigTokenPasswordParsing(t *testing.T) {
	tests := []struct {
		name        string
		config      *ClientConfig
		checkClient func(*testing.T, *WSTunnelClient)
	}{
		{
			name: "parse token without password",
			config: &ClientConfig{
				Token:  "mytoken12345678901",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "mytoken12345678901" {
					t.Errorf("Expected token 'mytoken12345678901', got %q", client.Token)
				}
				if client.Password != "" {
					t.Errorf("Expected empty password, got %q", client.Password)
				}
			},
		},
		{
			name: "parse token with password",
			config: &ClientConfig{
				Token:  "mytoken12345678901:mypassword",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "mytoken12345678901" {
					t.Errorf("Expected token 'mytoken12345678901', got %q", client.Token)
				}
				if client.Password != "mypassword" {
					t.Errorf("Expected password 'mypassword', got %q", client.Password)
				}
			},
		},
		{
			name: "handle token with multiple colons",
			config: &ClientConfig{
				Token:  "mytoken12345678901:password:with:colons",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "mytoken12345678901" {
					t.Errorf("Expected token 'mytoken12345678901', got %q", client.Token)
				}
				if client.Password != "password:with:colons" {
					t.Errorf("Expected password 'password:with:colons', got %q", client.Password)
				}
			},
		},
		{
			name: "handle empty token before colon",
			config: &ClientConfig{
				Token:  ":password",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "" {
					t.Errorf("Expected empty token, got %q", client.Token)
				}
				if client.Password != "password" {
					t.Errorf("Expected password 'password', got %q", client.Password)
				}
			},
		},
		{
			name: "handle empty password after colon",
			config: &ClientConfig{
				Token:  "mytoken12345678901:",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "mytoken12345678901" {
					t.Errorf("Expected token 'mytoken12345678901', got %q", client.Token)
				}
				if client.Password != "" {
					t.Errorf("Expected empty password, got %q", client.Password)
				}
			},
		},
		{
			name: "handle completely empty token",
			config: &ClientConfig{
				Token:  "",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "" {
					t.Errorf("Expected empty token, got %q", client.Token)
				}
				if client.Password != "" {
					t.Errorf("Expected empty password, got %q", client.Password)
				}
			},
		},
		{
			name: "handle token with special characters",
			config: &ClientConfig{
				Token:  "token!@#$%^&*()_+-=:pass!@#$%^&*()_+-=",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "token!@#$%^&*()_+-=" {
					t.Errorf("Expected token 'token!@#$%%^&*()_+-=', got %q", client.Token)
				}
				if client.Password != "pass!@#$%^&*()_+-=" {
					t.Errorf("Expected password 'pass!@#$%%^&*()_+-=', got %q", client.Password)
				}
			},
		},
		{
			name: "handle token with URL-unsafe characters",
			config: &ClientConfig{
				Token:  "token<>[]{}|\\:pass<>[]{}|\\",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "token<>[]{}|\\" {
					t.Errorf("Expected token 'token<>[]{}|\\', got %q", client.Token)
				}
				if client.Password != "pass<>[]{}|\\" {
					t.Errorf("Expected password 'pass<>[]{}|\\', got %q", client.Password)
				}
			},
		},
		{
			name: "handle very long token",
			config: &ClientConfig{
				Token:  strings.Repeat("a", 1000) + ":password",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				longToken := strings.Repeat("a", 1000)
				if client.Token != longToken {
					t.Errorf("Expected long token, got %q", client.Token)
				}
				if client.Password != "password" {
					t.Errorf("Expected password 'password', got %q", client.Password)
				}
			},
		},
		{
			name: "handle very long password",
			config: &ClientConfig{
				Token:  "mytoken12345678901:" + strings.Repeat("b", 1000),
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "mytoken12345678901" {
					t.Errorf("Expected token 'mytoken12345678901', got %q", client.Token)
				}
				longPassword := strings.Repeat("b", 1000)
				if client.Password != longPassword {
					t.Errorf("Expected long password, got %q", client.Password)
				}
			},
		},
		{
			name: "handle very long token and password",
			config: &ClientConfig{
				Token:  strings.Repeat("a", 1000) + ":" + strings.Repeat("b", 1000),
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				longToken := strings.Repeat("a", 1000)
				longPassword := strings.Repeat("b", 1000)
				if client.Token != longToken {
					t.Errorf("Expected long token, got %q", client.Token)
				}
				if client.Password != longPassword {
					t.Errorf("Expected long password, got %q", client.Password)
				}
			},
		},
		{
			name: "handle only colon as token",
			config: &ClientConfig{
				Token:  ":",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "" {
					t.Errorf("Expected empty token, got %q", client.Token)
				}
				if client.Password != "" {
					t.Errorf("Expected empty password, got %q", client.Password)
				}
			},
		},
		{
			name: "handle multiple consecutive colons",
			config: &ClientConfig{
				Token:  "token:::password",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "token" {
					t.Errorf("Expected token 'token', got %q", client.Token)
				}
				if client.Password != "::password" {
					t.Errorf("Expected password '::password', got %q", client.Password)
				}
			},
		},
		{
			name: "handle newlines in token and password",
			config: &ClientConfig{
				Token:  "token\nwith\nnewline:pass\nwith\nnewline",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "token\nwith\nnewline" {
					t.Errorf("Expected token 'token\\nwith\\nnewline', got %q", client.Token)
				}
				if client.Password != "pass\nwith\nnewline" {
					t.Errorf("Expected password 'pass\\nwith\\nnewline', got %q", client.Password)
				}
			},
		},
		{
			name: "handle tabs and spaces",
			config: &ClientConfig{
				Token:  "token with spaces:pass\twith\ttabs",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "token with spaces" {
					t.Errorf("Expected token 'token with spaces', got %q", client.Token)
				}
				if client.Password != "pass\twith\ttabs" {
					t.Errorf("Expected password 'pass\\twith\\ttabs', got %q", client.Password)
				}
			},
		},
		{
			name: "handle unicode characters",
			config: &ClientConfig{
				Token:  "token🔒unicode:pass🔑word",
				Tunnel: "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "token🔒unicode" {
					t.Errorf("Expected token 'token🔒unicode', got %q", client.Token)
				}
				if client.Password != "pass🔑word" {
					t.Errorf("Expected password 'pass🔑word', got %q", client.Password)
				}
			},
		},
		{
			name: "preserve explicit password parameter when no colon in token",
			config: &ClientConfig{
				Token:    "mytoken12345678901",
				Password: "explicitpassword",
				Tunnel:   "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "mytoken12345678901" {
					t.Errorf("Expected token 'mytoken12345678901', got %q", client.Token)
				}
				if client.Password != "explicitpassword" {
					t.Errorf("Expected password 'explicitpassword', got %q", client.Password)
				}
			},
		},
		{
			name: "override explicit password parameter when colon in token",
			config: &ClientConfig{
				Token:    "mytoken12345678901:colonpassword",
				Password: "explicitpassword",
				Tunnel:   "ws://localhost:8080",
			},
			checkClient: func(t *testing.T, client *WSTunnelClient) {
				if client.Token != "mytoken12345678901" {
					t.Errorf("Expected token 'mytoken12345678901', got %q", client.Token)
				}
				if client.Password != "colonpassword" {
					t.Errorf("Expected password 'colonpassword', got %q", client.Password)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewWSTunnelClientFromConfig(tt.config)
			if err != nil {
				t.Errorf("NewWSTunnelClientFromConfig() error = %v", err)
				return
			}
			if tt.checkClient != nil {
				tt.checkClient(t, client)
			}
		})
	}
}

// TestTokenPasswordParsingUnit provides additional unit tests using Go's standard testing
func TestTokenPasswordParsingUnit(t *testing.T) {
	tests := []struct {
		name             string
		token            string
		expectedToken    string
		expectedPassword string
	}{
		{
			name:             "token without password",
			token:            "simpletoken12345678",
			expectedToken:    "simpletoken12345678",
			expectedPassword: "",
		},
		{
			name:             "token with password",
			token:            "token12345678901:password123",
			expectedToken:    "token12345678901",
			expectedPassword: "password123",
		},
		{
			name:             "token with password containing colons",
			token:            "token12345678901:pass:word:123",
			expectedToken:    "token12345678901",
			expectedPassword: "pass:word:123",
		},
		{
			name:             "empty token with password",
			token:            ":justpassword",
			expectedToken:    "",
			expectedPassword: "justpassword",
		},
		{
			name:             "token with empty password",
			token:            "token12345678901:",
			expectedToken:    "token12345678901",
			expectedPassword: "",
		},
		{
			name:             "just a colon",
			token:            ":",
			expectedToken:    "",
			expectedPassword: "",
		},
		{
			name:             "empty string",
			token:            "",
			expectedToken:    "",
			expectedPassword: "",
		},
		{
			name:             "multiple consecutive colons at start",
			token:            ":::password",
			expectedToken:    "",
			expectedPassword: "::password",
		},
		{
			name:             "multiple consecutive colons at end",
			token:            "token:::",
			expectedToken:    "token",
			expectedPassword: "::",
		},
		{
			name:             "special characters in token",
			token:            "tok!@#$%^&*()_+-=en:pass!@#$%^&*()_+-=word",
			expectedToken:    "tok!@#$%^&*()_+-=en",
			expectedPassword: "pass!@#$%^&*()_+-=word",
		},
		{
			name:             "URL-like password",
			token:            "mytoken:https://user:pass@example.com",
			expectedToken:    "mytoken",
			expectedPassword: "https://user:pass@example.com",
		},
		{
			name:             "base64 encoded values",
			token:            "dG9rZW4=:cGFzc3dvcmQ=",
			expectedToken:    "dG9rZW4=",
			expectedPassword: "cGFzc3dvcmQ=",
		},
		{
			name:             "very long token (1000 chars)",
			token:            strings.Repeat("a", 1000) + ":password",
			expectedToken:    strings.Repeat("a", 1000),
			expectedPassword: "password",
		},
		{
			name:             "very long password (1000 chars)",
			token:            "token:" + strings.Repeat("b", 1000),
			expectedToken:    "token",
			expectedPassword: strings.Repeat("b", 1000),
		},
		{
			name:             "both very long (500 chars each)",
			token:            strings.Repeat("x", 500) + ":" + strings.Repeat("y", 500),
			expectedToken:    strings.Repeat("x", 500),
			expectedPassword: strings.Repeat("y", 500),
		},
		{
			name:             "whitespace in token and password",
			token:            "token with spaces:password with spaces",
			expectedToken:    "token with spaces",
			expectedPassword: "password with spaces",
		},
		{
			name:             "tabs and newlines",
			token:            "token\twith\ttabs:pass\nwith\nnewlines",
			expectedToken:    "token\twith\ttabs",
			expectedPassword: "pass\nwith\nnewlines",
		},
		{
			name:             "unicode characters",
			token:            "token🔐:password🔑",
			expectedToken:    "token🔐",
			expectedPassword: "password🔑",
		},
		{
			name:             "percent encoded characters",
			token:            "token%20with%20encoded:pass%3Aword",
			expectedToken:    "token%20with%20encoded",
			expectedPassword: "pass%3Aword",
		},
		{
			name:             "JSON-like password",
			token:            `token:{"key":"value","nested":{"a":1}}`,
			expectedToken:    "token",
			expectedPassword: `{"key":"value","nested":{"a":1}}`,
		},
		{
			name:             "SQL injection attempt in password",
			token:            "token:'; DROP TABLE users; --",
			expectedToken:    "token",
			expectedPassword: "'; DROP TABLE users; --",
		},
		{
			name:             "path traversal in password",
			token:            "token:../../../etc/passwd",
			expectedToken:    "token",
			expectedPassword: "../../../etc/passwd",
		},
		{
			name:             "null bytes",
			token:            "token\x00with\x00null:pass\x00word",
			expectedToken:    "token\x00with\x00null",
			expectedPassword: "pass\x00word",
		},
		{
			name:             "binary data simulation",
			token:            "token:\x01\x02\x03\x04\x05",
			expectedToken:    "token",
			expectedPassword: "\x01\x02\x03\x04\x05",
		},
	}

	for _, tt := range tests {
		tt := tt // capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the parsing logic from client_config.go
			var token, password string
			if tt.token != "" {
				parts := strings.SplitN(tt.token, ":", 2)
				token = parts[0]
				if len(parts) == 2 {
					password = parts[1]
				}
			}

			if token != tt.expectedToken {
				t.Errorf("Expected token %q, got %q", tt.expectedToken, token)
			}
			if password != tt.expectedPassword {
				t.Errorf("Expected password %q, got %q", tt.expectedPassword, password)
			}
		})
	}
}
