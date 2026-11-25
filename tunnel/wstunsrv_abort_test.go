// Copyright (c) 2023 RightScale, Inc. - see LICENSE

package tunnel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

type tunnelTestEnv struct {
	server   *httptest.Server
	wstunsrv *WSTunnelServer
	wstuncli *WSTunnelClient
	wstunURL string
	token    string
}

// setupTunnelTest starts a backend serving handler, a tunnel server started with
// srvArgs and a tunnel client connecting both. Everything is stopped on test cleanup.
func setupTunnelTest(t *testing.T, handler http.Handler, srvArgs ...string) *tunnelTestEnv {
	t.Helper()

	token := "test-token-" + strconv.Itoa(rand.Int()%1000000) + "-1234567890"

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	wstunsrv := NewWSTunnelServer(srvArgs)
	wstunsrv.Start(listener)
	t.Cleanup(wstunsrv.Stop)

	wstuncli := NewWSTunnelClient([]string{
		"-token", token,
		"-tunnel", "ws://" + listener.Addr().String(),
		"-server", server.URL,
		"-timeout", "10",
	})
	if err := wstuncli.Start(); err != nil {
		t.Fatalf("Error starting client: %v", err)
	}
	t.Cleanup(wstuncli.Stop)

	start := time.Now()
	for !wstuncli.IsConnected() {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 6*time.Second {
			t.Fatalf("Client failed to connect within 6 seconds")
		}
	}

	return &tunnelTestEnv{
		server:   server,
		wstunsrv: wstunsrv,
		wstuncli: wstuncli,
		wstunURL: "http://" + listener.Addr().String(),
		token:    token,
	}
}

// waitRequestsRetired waits until the tunnel for tok has no requests in flight.
func waitRequestsRetired(t *testing.T, srv *WSTunnelServer, tok string) {
	t.Helper()
	rs := srv.getRemoteServer(token(tok), false)
	if rs == nil {
		t.Fatalf("No remote server for token")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		rs.requestSetMutex.Lock()
		n := len(rs.requestSet)
		rs.requestSetMutex.Unlock()
		if n == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d request(s) still in flight after 5s", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestAbandonedRequestDoesNotBreakTunnel is a regression test: when the server
// stopped waiting for a response (browser aborted the request, or the tunnel
// timed out), it still signalled wsReader as if it had consumed a response.
// The stray signal made wsReader advance to the next websocket message while
// a later request was still reading its response, so that and every subsequent
// request failed with "unexpected EOF" / HTTP 506.
func TestAbandonedRequestDoesNotBreakTunnel(t *testing.T) {
	bigBody := bytes.Repeat([]byte("0123456789abcdef"), 64*1024) // 1 MiB

	tests := []struct {
		name    string
		srvArgs []string
		// abandon issues a request to /slow and returns once the server gave up on it
		abandon func(t *testing.T, env *tunnelTestEnv, entered <-chan struct{})
	}{
		{
			name:    "client aborts pending request",
			srvArgs: nil,
			abandon: func(t *testing.T, env *tunnelTestEnv, entered <-chan struct{}) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				req, err := http.NewRequestWithContext(ctx, http.MethodGet,
					env.wstunURL+"/_token/"+env.token+"/slow", nil)
				if err != nil {
					t.Fatalf("NewRequest: %v", err)
				}
				errCh := make(chan error, 1)
				go func() {
					resp, err := http.DefaultClient.Do(req)
					if err == nil {
						_ = resp.Body.Close()
					}
					errCh <- err
				}()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatalf("Slow request never reached the backend")
				}
				cancel()
				if err := <-errCh; !errors.Is(err, context.Canceled) {
					t.Fatalf("Expected aborted request to fail with context.Canceled, got %v", err)
				}
			},
		},
		{
			name:    "server times out pending request",
			srvArgs: []string{"-httptimeout", "1"},
			abandon: func(t *testing.T, env *tunnelTestEnv, _ <-chan struct{}) {
				resp, err := http.Get(env.wstunURL + "/_token/" + env.token + "/slow")
				if err != nil {
					t.Fatalf("Expected 504 response, got error: %v", err)
				}
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusGatewayTimeout {
					t.Fatalf("Expected 504 for slow request, got %d", resp.StatusCode)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entered := make(chan struct{}, 1)
			release := make(chan struct{})
			var releaseOnce sync.Once
			releaseSlow := func() { releaseOnce.Do(func() { close(release) }) }

			env := setupTunnelTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/slow":
					select {
					case entered <- struct{}{}:
					default:
					}
					<-release
					_, _ = w.Write([]byte("late response"))
				case "/big":
					_, _ = w.Write(bigBody)
				}
			}), tt.srvArgs...)
			// registered after setupTunnelTest so it runs before the backend is closed
			t.Cleanup(releaseSlow)

			tt.abandon(t, env, entered)
			waitRequestsRetired(t, env.wstunsrv, env.token)

			// let the abandoned response arrive at the server as an orphan
			releaseSlow()

			for i := range 5 {
				resp, err := http.Get(env.wstunURL + "/_token/" + env.token + "/big")
				if err != nil {
					t.Fatalf("Request %d after abandoned request: %v", i, err)
				}
				body, err := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if err != nil {
					t.Fatalf("Request %d: reading body: %v", i, err)
				}
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("Request %d after abandoned request: expected 200, got %d (%s)",
						i, resp.StatusCode, bytes.TrimSpace(body[:min(len(body), 200)]))
				}
				if !bytes.Equal(body, bigBody) {
					t.Fatalf("Request %d: body mismatch: got %d bytes, want %d", i, len(body), len(bigBody))
				}
			}
		})
	}
}
