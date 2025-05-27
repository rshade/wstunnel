// Copyright (c) 2024 RightScale, Inc. - see LICENSE

package tunnel

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestTokenPasswordValidation tests server-side password validation
func TestTokenPasswordValidation(t *testing.T) {
	// Create a test HTTP server that will be proxied
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "Response from test server")
	}))
	defer server.Close()

	// Create tunnel server with token passwords
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	serverAddr := listener.Addr().String()

	wstunsrv := NewWSTunnelServer([]string{
		"-port", "0",
		"-passwords", "test-token-with-password:secret123,another-token-16chars:password456",
	})

	wstunsrv.Start(listener)
	defer wstunsrv.Stop()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	t.Run("AcceptConnectionWithCorrectPassword", func(t *testing.T) {
		// Create client with password
		wstuncli := NewWSTunnelClient([]string{
			"-token", "test-token-with-password:secret123",
			"-tunnel", "ws://" + serverAddr,
			"-server", server.URL,
			"-timeout", "30",
		})
		err := wstuncli.Start()
		if err != nil {
			t.Fatalf("Failed to start client: %v", err)
		}
		defer wstuncli.Stop()

		// Wait for connection
		time.Sleep(500 * time.Millisecond)
		if !wstuncli.IsConnected() {
			t.Errorf("Expected client to be connected. Token: %s, Password: %s", wstuncli.Token, wstuncli.Password)
		}
	})

	t.Run("RejectConnectionWithIncorrectPassword", func(t *testing.T) {
		// Create client with wrong password
		wstuncli := NewWSTunnelClient([]string{
			"-token", "test-token-with-password:wrongpassword",
			"-tunnel", "ws://" + serverAddr,
			"-server", server.URL,
			"-timeout", "30",
		})
		err := wstuncli.Start()
		if err != nil {
			t.Fatalf("Failed to start client: %v", err)
		}
		defer wstuncli.Stop()

		// Wait and check that connection is not established
		time.Sleep(300 * time.Millisecond)
		if wstuncli.IsConnected() {
			t.Error("Expected client to not be connected")
		}
	})

	t.Run("RejectConnectionWithoutPasswordWhenPasswordIsRequired", func(t *testing.T) {
		// Create client without password
		wstuncli := NewWSTunnelClient([]string{
			"-token", "test-token-with-password",
			"-tunnel", "ws://" + serverAddr,
			"-server", server.URL,
			"-timeout", "30",
		})
		err := wstuncli.Start()
		if err != nil {
			t.Fatalf("Failed to start client: %v", err)
		}
		defer wstuncli.Stop()

		// Wait and check that connection is not established
		time.Sleep(300 * time.Millisecond)
		if wstuncli.IsConnected() {
			t.Error("Expected client to not be connected")
		}
	})

	t.Run("AcceptConnectionWithoutPasswordWhenNoPasswordIsConfigured", func(t *testing.T) {
		// Create client for token without password requirement
		wstuncli := NewWSTunnelClient([]string{
			"-token", "test-token-no-password",
			"-tunnel", "ws://" + serverAddr,
			"-server", server.URL,
			"-timeout", "30",
		})
		err := wstuncli.Start()
		if err != nil {
			t.Fatalf("Failed to start client: %v", err)
		}
		defer wstuncli.Stop()

		// Wait for connection
		time.Sleep(500 * time.Millisecond)
		if !wstuncli.IsConnected() {
			t.Errorf("Expected client to be connected. Token: %s, Password: %s", wstuncli.Token, wstuncli.Password)
		}
	})
}

// TestEndToEndWithPassword tests end-to-end functionality with password authentication
func TestEndToEndWithPassword(t *testing.T) {
	// Create a test HTTP server that will be proxied
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "Response from test server")
	}))
	defer server.Close()

	// Create tunnel server with token passwords
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	serverAddr := listener.Addr().String()

	wstunsrv := NewWSTunnelServer([]string{
		"-port", "0",
		"-passwords", "test-token-with-password:secret123",
	})

	wstunsrv.Start(listener)
	defer wstunsrv.Stop()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Create client with correct password
	wstuncli := NewWSTunnelClient([]string{
		"-token", "test-token-with-password:secret123",
		"-tunnel", "ws://" + serverAddr,
		"-server", server.URL,
		"-timeout", "30",
	})
	err := wstuncli.Start()
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer wstuncli.Stop()

	// Wait for connection
	time.Sleep(200 * time.Millisecond)
	if !wstuncli.IsConnected() {
		t.Fatal("Expected client to be connected")
	}

	// Make a request through the tunnel
	resp, err := http.Get(fmt.Sprintf("http://%s/_token/test-token-with-password/test", serverAddr))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	expected := "Response from test server"
	if string(body) != expected {
		t.Errorf("Expected response body to be %q, got %q", expected, string(body))
	}
}

// TestServerPasswordParsing tests server password configuration parsing
func TestServerPasswordParsing(t *testing.T) {
	t.Run("ParseValidPasswordConfiguration", func(t *testing.T) {
		args := []string{
			"-port", "8080",
			"-passwords", "token12345678901:pass1,token22345678901:pass2,token32345678901:pass3",
		}
		srv := NewWSTunnelServer(args)
		if srv == nil {
			t.Fatal("Expected server to be created")
		}
		if len(srv.tokenPasswords) != 3 {
			t.Errorf("Expected 3 token passwords, got %d", len(srv.tokenPasswords))
		}
		if srv.tokenPasswords[token("token12345678901")] != "pass1" {
			t.Errorf("Expected token12345678901 password to be 'pass1', got %q", srv.tokenPasswords[token("token12345678901")])
		}
		if srv.tokenPasswords[token("token22345678901")] != "pass2" {
			t.Errorf("Expected token22345678901 password to be 'pass2', got %q", srv.tokenPasswords[token("token22345678901")])
		}
		if srv.tokenPasswords[token("token32345678901")] != "pass3" {
			t.Errorf("Expected token32345678901 password to be 'pass3', got %q", srv.tokenPasswords[token("token32345678901")])
		}
	})

	t.Run("HandleMalformedPasswordPairs", func(t *testing.T) {
		args := []string{
			"-port", "8080",
			"-passwords", "token12345678901:pass1,invalid_no_colon,token22345678901:pass2",
		}
		srv := NewWSTunnelServer(args)
		if srv == nil {
			t.Fatal("Expected server to be created")
		}
		if len(srv.tokenPasswords) != 2 {
			t.Errorf("Expected 2 token passwords (skipping invalid), got %d", len(srv.tokenPasswords))
		}
		if srv.tokenPasswords[token("token12345678901")] != "pass1" {
			t.Errorf("Expected token12345678901 password to be 'pass1', got %q", srv.tokenPasswords[token("token12345678901")])
		}
		if srv.tokenPasswords[token("token22345678901")] != "pass2" {
			t.Errorf("Expected token22345678901 password to be 'pass2', got %q", srv.tokenPasswords[token("token22345678901")])
		}
	})
}

// TestClientPasswordParsing tests client password configuration parsing
func TestClientPasswordParsing(t *testing.T) {
	t.Run("TokenWithoutPassword", func(t *testing.T) {
		args := []string{
			"-token", "mytoken12345678901",
			"-tunnel", "ws://localhost:8080",
		}
		cli := NewWSTunnelClient(args)
		if cli == nil {
			t.Fatal("Expected client to be created")
		}
		if cli.Token != "mytoken12345678901" {
			t.Errorf("Expected token to be 'mytoken12345678901', got %q", cli.Token)
		}
		if cli.Password != "" {
			t.Errorf("Expected password to be empty, got %q", cli.Password)
		}
	})

	t.Run("TokenWithPassword", func(t *testing.T) {
		args := []string{
			"-token", "mytoken12345678901:mypassword",
			"-tunnel", "ws://localhost:8080",
		}
		cli := NewWSTunnelClient(args)
		if cli == nil {
			t.Fatal("Expected client to be created")
		}
		if cli.Token != "mytoken12345678901" {
			t.Errorf("Expected token to be 'mytoken12345678901', got %q", cli.Token)
		}
		if cli.Password != "mypassword" {
			t.Errorf("Expected password to be 'mypassword', got %q", cli.Password)
		}
	})
}

// TestServerPasswordValidation tests the new validation logic for server password configuration
func TestServerPasswordValidation(t *testing.T) {
	t.Run("RejectEmptyTokens", func(t *testing.T) {
		args := []string{
			"-port", "8080",
			"-passwords", ":pass1,token12345678901:pass2",
		}
		srv := NewWSTunnelServer(args)
		if srv == nil {
			t.Fatal("Expected server to be created")
		}
		if len(srv.tokenPasswords) != 1 {
			t.Errorf("Expected 1 token password (skipping empty token), got %d", len(srv.tokenPasswords))
		}
		if srv.tokenPasswords[token("token12345678901")] != "pass2" {
			t.Errorf("Expected token12345678901 password to be 'pass2', got %q", srv.tokenPasswords[token("token12345678901")])
		}
	})

	t.Run("RejectEmptyPasswords", func(t *testing.T) {
		args := []string{
			"-port", "8080",
			"-passwords", "token12345678901:,token22345678901:pass2",
		}
		srv := NewWSTunnelServer(args)
		if srv == nil {
			t.Fatal("Expected server to be created")
		}
		if len(srv.tokenPasswords) != 1 {
			t.Errorf("Expected 1 token password (skipping empty password), got %d", len(srv.tokenPasswords))
		}
		if srv.tokenPasswords[token("token22345678901")] != "pass2" {
			t.Errorf("Expected token22345678901 password to be 'pass2', got %q", srv.tokenPasswords[token("token22345678901")])
		}
	})

	t.Run("RejectShortTokens", func(t *testing.T) {
		args := []string{
			"-port", "8080",
			"-passwords", "shorttoken:pass1,token12345678901:pass2",
		}
		srv := NewWSTunnelServer(args)
		if srv == nil {
			t.Fatal("Expected server to be created")
		}
		if len(srv.tokenPasswords) != 1 {
			t.Errorf("Expected 1 token password (skipping short token), got %d", len(srv.tokenPasswords))
		}
		if srv.tokenPasswords[token("token12345678901")] != "pass2" {
			t.Errorf("Expected token12345678901 password to be 'pass2', got %q", srv.tokenPasswords[token("token12345678901")])
		}
	})

	t.Run("WarnOnDuplicateTokens", func(t *testing.T) {
		args := []string{
			"-port", "8080",
			"-passwords", "token12345678901:pass1,token12345678901:pass2",
		}
		srv := NewWSTunnelServer(args)
		if srv == nil {
			t.Fatal("Expected server to be created")
		}
		if len(srv.tokenPasswords) != 1 {
			t.Errorf("Expected 1 token password (duplicate overwrites), got %d", len(srv.tokenPasswords))
		}
		// The second password should overwrite the first
		if srv.tokenPasswords[token("token12345678901")] != "pass2" {
			t.Errorf("Expected token12345678901 password to be 'pass2' (overwritten), got %q", srv.tokenPasswords[token("token12345678901")])
		}
	})
}
