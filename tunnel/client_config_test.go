// Copyright (c) 2024 RightScale, Inc. - see LICENSE

package tunnel

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseClientConfig", func() {
	Describe("Token parsing", func() {
		It("should parse token without password", func() {
			args := []string{"-token", "mytoken12345678901", "-tunnel", "ws://localhost:8080"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Token).To(Equal("mytoken12345678901"))
			Expect(config.Password).To(Equal(""))
		})

		It("should parse token with password", func() {
			args := []string{"-token", "mytoken12345678901:mypassword", "-tunnel", "ws://localhost:8080"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Token).To(Equal("mytoken12345678901:mypassword"))
			Expect(config.Password).To(Equal(""))
		})
	})

	Describe("URL flags validation", func() {
		It("should parse valid tunnel URL", func() {
			args := []string{"-tunnel", "ws://example.com:8080", "-token", "test"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Tunnel).To(Equal("ws://example.com:8080"))
		})

		It("should parse valid secure tunnel URL", func() {
			args := []string{"-tunnel", "wss://user:pass@example.com:8443/path", "-token", "test"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Tunnel).To(Equal("wss://user:pass@example.com:8443/path"))
		})

		It("should parse valid server URL", func() {
			args := []string{"-server", "http://localhost:3000", "-tunnel", "ws://example.com", "-token", "test"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Server).To(Equal("http://localhost:3000"))
		})

		It("should parse valid secure server URL", func() {
			args := []string{"-server", "https://localhost:3443", "-tunnel", "ws://example.com", "-token", "test"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Server).To(Equal("https://localhost:3443"))
		})
	})

	Describe("Boolean flags", func() {
		It("should default insecure to false", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Insecure).To(BeFalse())
		})

		It("should parse insecure flag", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-insecure"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Insecure).To(BeTrue())
		})
	})

	Describe("Numeric flags", func() {
		It("should use default timeout", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Timeout).To(Equal(30))
		})

		It("should parse custom timeout", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-timeout", "60"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Timeout).To(Equal(60))
		})

		It("should use default reconnect-delay", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ReconnectDelay).To(Equal(5))
		})

		It("should parse custom reconnect-delay", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-reconnect-delay", "10"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ReconnectDelay).To(Equal(10))
		})

		It("should use default max-retries", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.MaxRetries).To(Equal(0))
		})

		It("should parse custom max-retries", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-max-retries", "3"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.MaxRetries).To(Equal(3))
		})
	})

	Describe("String flags", func() {
		It("should parse regexp flag", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-regexp", "^https?://[a-z]+\\.example\\.com"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Regexp).To(Equal("^https?://[a-z]+\\.example\\.com"))
		})

		It("should parse pidfile flag", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-pidfile", "/var/run/wstunnel.pid"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.PidFile).To(Equal("/var/run/wstunnel.pid"))
		})

		It("should parse logfile flag", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-logfile", "/var/log/wstunnel.log"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.LogFile).To(Equal("/var/log/wstunnel.log"))
		})

		It("should parse statusfile flag", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-statusfile", "/var/run/wstunnel.status"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.StatusFile).To(Equal("/var/run/wstunnel.status"))
		})

		It("should parse certfile flag", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-certfile", "/etc/ssl/certs/ca.pem"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.CertFile).To(Equal("/etc/ssl/certs/ca.pem"))
		})

		It("should parse proxy flag", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-proxy", "http://user:pass@proxy.example.com:8080"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Proxy).To(Equal("http://user:pass@proxy.example.com:8080"))
		})
	})

	Describe("Client-ports parsing", func() {
		It("should parse single port", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-client-ports", "8080"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ClientPorts).To(Equal("8080"))
		})

		It("should parse multiple ports", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-client-ports", "8080,8081,8082"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ClientPorts).To(Equal("8080,8081,8082"))
		})

		It("should parse port ranges", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-client-ports", "8000..8100"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ClientPorts).To(Equal("8000..8100"))
		})

		It("should parse mixed ports and ranges", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-client-ports", "8000..8100,8300..8400,8500,8505"}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ClientPorts).To(Equal("8000..8100,8300..8400,8500,8505"))
		})
	})

	Describe("Error conditions", func() {
		It("should handle empty arguments", func() {
			args := []string{}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
		})

		It("should handle unknown flags", func() {
			args := []string{"-unknown-flag", "value", "-token", "test", "-tunnel", "ws://localhost"}
			_, err := ParseClientConfig(args)
			// With ContinueOnError, unknown flags cause an error
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown"))
		})

		It("should handle missing flag values", func() {
			args := []string{"-token"}
			// This will cause flag.Parse to return an error
			_, err := ParseClientConfig(args)
			Expect(err).To(HaveOccurred())
		})

		It("should handle invalid numeric values", func() {
			args := []string{"-token", "test", "-tunnel", "ws://localhost", "-timeout", "invalid"}
			_, err := ParseClientConfig(args)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("All flags combined", func() {
		It("should parse all flags correctly", func() {
			args := []string{
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
			}
			config, err := ParseClientConfig(args)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Token).To(Equal("mytoken12345678901:password123"))
			Expect(config.Tunnel).To(Equal("wss://user:pass@tunnel.example.com:8443/path"))
			Expect(config.Server).To(Equal("https://backend.local:3443"))
			Expect(config.Insecure).To(BeTrue())
			Expect(config.Regexp).To(Equal("^https?://.*\\.local"))
			Expect(config.Timeout).To(Equal(120))
			Expect(config.PidFile).To(Equal("/var/run/wstunnel.pid"))
			Expect(config.LogFile).To(Equal("/var/log/wstunnel.log"))
			Expect(config.StatusFile).To(Equal("/var/run/wstunnel.status"))
			Expect(config.Proxy).To(Equal("http://proxy.local:3128"))
			Expect(config.ClientPorts).To(Equal("8000..8100,9000,9005"))
			Expect(config.CertFile).To(Equal("/etc/ssl/ca.pem"))
			Expect(config.ReconnectDelay).To(Equal(15))
			Expect(config.MaxRetries).To(Equal(5))
		})
	})
})

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

var _ = Describe("Client Config Token:Password Parsing", func() {
	Describe("NewWSTunnelClientFromConfig token:password parsing", func() {
		It("should parse token without password", func() {
			config := &ClientConfig{
				Token:  "mytoken12345678901",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("mytoken12345678901"))
			Expect(client.Password).To(Equal(""))
		})

		It("should parse token with password", func() {
			config := &ClientConfig{
				Token:  "mytoken12345678901:mypassword",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("mytoken12345678901"))
			Expect(client.Password).To(Equal("mypassword"))
		})

		It("should handle token with multiple colons", func() {
			config := &ClientConfig{
				Token:  "mytoken12345678901:password:with:colons",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("mytoken12345678901"))
			Expect(client.Password).To(Equal("password:with:colons"))
		})

		It("should handle empty token before colon", func() {
			config := &ClientConfig{
				Token:  ":password",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal(""))
			Expect(client.Password).To(Equal("password"))
		})

		It("should handle empty password after colon", func() {
			config := &ClientConfig{
				Token:  "mytoken12345678901:",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("mytoken12345678901"))
			Expect(client.Password).To(Equal(""))
		})

		It("should handle completely empty token", func() {
			config := &ClientConfig{
				Token:  "",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal(""))
			Expect(client.Password).To(Equal(""))
		})

		It("should handle token with special characters", func() {
			config := &ClientConfig{
				Token:  "token!@#$%^&*()_+-=:pass!@#$%^&*()_+-=",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("token!@#$%^&*()_+-="))
			Expect(client.Password).To(Equal("pass!@#$%^&*()_+-="))
		})

		It("should handle token with URL-unsafe characters", func() {
			config := &ClientConfig{
				Token:  "token<>[]{}|\\:pass<>[]{}|\\",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("token<>[]{}|\\"))
			Expect(client.Password).To(Equal("pass<>[]{}|\\"))
		})

		It("should handle very long token", func() {
			longToken := strings.Repeat("a", 1000)
			config := &ClientConfig{
				Token:  longToken + ":password",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal(longToken))
			Expect(client.Password).To(Equal("password"))
		})

		It("should handle very long password", func() {
			longPassword := strings.Repeat("b", 1000)
			config := &ClientConfig{
				Token:  "mytoken12345678901:" + longPassword,
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("mytoken12345678901"))
			Expect(client.Password).To(Equal(longPassword))
		})

		It("should handle very long token and password", func() {
			longToken := strings.Repeat("a", 1000)
			longPassword := strings.Repeat("b", 1000)
			config := &ClientConfig{
				Token:  longToken + ":" + longPassword,
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal(longToken))
			Expect(client.Password).To(Equal(longPassword))
		})

		It("should handle only colon as token", func() {
			config := &ClientConfig{
				Token:  ":",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal(""))
			Expect(client.Password).To(Equal(""))
		})

		It("should handle multiple consecutive colons", func() {
			config := &ClientConfig{
				Token:  "token:::password",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("token"))
			Expect(client.Password).To(Equal("::password"))
		})

		It("should handle newlines in token and password", func() {
			config := &ClientConfig{
				Token:  "token\nwith\nnewline:pass\nwith\nnewline",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("token\nwith\nnewline"))
			Expect(client.Password).To(Equal("pass\nwith\nnewline"))
		})

		It("should handle tabs and spaces", func() {
			config := &ClientConfig{
				Token:  "token with spaces:pass\twith\ttabs",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("token with spaces"))
			Expect(client.Password).To(Equal("pass\twith\ttabs"))
		})

		It("should handle unicode characters", func() {
			config := &ClientConfig{
				Token:  "token🔒unicode:pass🔑word",
				Tunnel: "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("token🔒unicode"))
			Expect(client.Password).To(Equal("pass🔑word"))
		})

		It("should preserve explicit password parameter when no colon in token", func() {
			config := &ClientConfig{
				Token:    "mytoken12345678901",
				Password: "explicitpassword",
				Tunnel:   "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("mytoken12345678901"))
			Expect(client.Password).To(Equal("explicitpassword"))
		})

		It("should override explicit password parameter when colon in token", func() {
			config := &ClientConfig{
				Token:    "mytoken12345678901:colonpassword",
				Password: "explicitpassword",
				Tunnel:   "ws://localhost:8080",
			}
			client, err := NewWSTunnelClientFromConfig(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(client.Token).To(Equal("mytoken12345678901"))
			Expect(client.Password).To(Equal("colonpassword"))
		})
	})
})

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
