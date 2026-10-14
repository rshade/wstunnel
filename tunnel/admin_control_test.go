// Copyright (c) 2014 RightScale, Inc. - see LICENSE

package tunnel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// --- Unit tests for remoteServer connection tracking ---

func TestRegisterAndDeregisterConnection(t *testing.T) {
	rs := &remoteServer{
		connCloseChans: make([]chan struct{}, 0),
	}

	ch1 := make(chan struct{})
	ch2 := make(chan struct{})

	deregister1 := rs.RegisterConnection(ch1)
	deregister2 := rs.RegisterConnection(ch2)

	rs.connCloseMutex.Lock()
	if len(rs.connCloseChans) != 2 {
		t.Fatalf("expected 2 registered connections, got %d", len(rs.connCloseChans))
	}
	rs.connCloseMutex.Unlock()

	// Deregister first connection
	deregister1()

	rs.connCloseMutex.Lock()
	if len(rs.connCloseChans) != 1 {
		t.Fatalf("expected 1 registered connection after deregister, got %d", len(rs.connCloseChans))
	}
	if rs.connCloseChans[0] != ch2 {
		t.Fatal("expected remaining channel to be ch2")
	}
	rs.connCloseMutex.Unlock()

	// Deregister second connection
	deregister2()

	rs.connCloseMutex.Lock()
	if len(rs.connCloseChans) != 0 {
		t.Fatalf("expected 0 registered connections after both deregister, got %d", len(rs.connCloseChans))
	}
	rs.connCloseMutex.Unlock()
}

func TestCloseAllConnections(t *testing.T) {
	rs := &remoteServer{
		connCloseChans: make([]chan struct{}, 0),
	}

	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	ch3 := make(chan struct{})

	rs.RegisterConnection(ch1)
	rs.RegisterConnection(ch2)
	rs.RegisterConnection(ch3)

	count := rs.CloseAllConnections()

	if count != 3 {
		t.Fatalf("expected 3 disconnected, got %d", count)
	}

	// Verify all channels are closed
	for i, ch := range []chan struct{}{ch1, ch2, ch3} {
		select {
		case <-ch:
			// good, channel is closed
		default:
			t.Fatalf("channel %d was not closed", i)
		}
	}

	// Verify internal slice is cleared
	rs.connCloseMutex.Lock()
	if len(rs.connCloseChans) != 0 {
		t.Fatalf("expected connCloseChans to be empty, got %d", len(rs.connCloseChans))
	}
	rs.connCloseMutex.Unlock()
}

func TestCloseAllConnectionsEmpty(t *testing.T) {
	rs := &remoteServer{
		connCloseChans: make([]chan struct{}, 0),
	}

	count := rs.CloseAllConnections()
	if count != 0 {
		t.Fatalf("expected 0 disconnected from empty server, got %d", count)
	}
}

func TestCloseAllConnectionsIdempotent(t *testing.T) {
	rs := &remoteServer{
		connCloseChans: make([]chan struct{}, 0),
	}

	ch := make(chan struct{})
	rs.RegisterConnection(ch)

	// Close once
	count1 := rs.CloseAllConnections()
	if count1 != 1 {
		t.Fatalf("expected 1, got %d", count1)
	}

	// Close again should return 0
	count2 := rs.CloseAllConnections()
	if count2 != 0 {
		t.Fatalf("expected 0 on second call, got %d", count2)
	}
}

func TestCloseAllConnectionsConcurrent(t *testing.T) {
	rs := &remoteServer{
		connCloseChans: make([]chan struct{}, 0),
	}

	const n = 10
	channels := make([]chan struct{}, n)
	for i := range channels {
		channels[i] = make(chan struct{})
		rs.RegisterConnection(channels[i])
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		rs.CloseAllConnections()
	}()
	go func() {
		defer wg.Done()
		rs.CloseAllConnections()
	}()
	wg.Wait()

	// All channels should be closed regardless
	for i, ch := range channels {
		select {
		case <-ch:
		default:
			t.Fatalf("channel %d was not closed", i)
		}
	}
}

// --- Unit tests for WSTunnelServer token blocking ---

func TestBlockUnblockToken(t *testing.T) {
	server := &WSTunnelServer{
		blockedTokens: make(map[token]time.Time),
	}

	tok := token("test-token-1234567890")

	if server.IsTokenBlocked(tok) {
		t.Fatal("token should not be blocked initially")
	}

	server.BlockToken(tok)

	if !server.IsTokenBlocked(tok) {
		t.Fatal("token should be blocked after BlockToken")
	}

	existed := server.UnblockToken(tok)
	if !existed {
		t.Fatal("UnblockToken should return true for previously blocked token")
	}

	if server.IsTokenBlocked(tok) {
		t.Fatal("token should not be blocked after UnblockToken")
	}
}

func TestUnblockNonexistentToken(t *testing.T) {
	server := &WSTunnelServer{
		blockedTokens: make(map[token]time.Time),
	}

	existed := server.UnblockToken(token("never-blocked-token"))
	if existed {
		t.Fatal("UnblockToken should return false for never-blocked token")
	}
}

func TestGetBlockedTokens(t *testing.T) {
	server := &WSTunnelServer{
		blockedTokens: make(map[token]time.Time),
	}

	tok1 := token("test-token-aaaa1234")
	tok2 := token("test-token-bbbb5678")

	server.BlockToken(tok1)
	server.BlockToken(tok2)

	blocked := server.GetBlockedTokens()

	if len(blocked) != 2 {
		t.Fatalf("expected 2 blocked tokens, got %d", len(blocked))
	}

	if _, ok := blocked[tok1]; !ok {
		t.Fatal("tok1 should be in blocked list")
	}
	if _, ok := blocked[tok2]; !ok {
		t.Fatal("tok2 should be in blocked list")
	}

	// Verify it's a copy by modifying it
	delete(blocked, tok1)
	if !server.IsTokenBlocked(tok1) {
		t.Fatal("modifying returned map should not affect server state")
	}
}

func TestIsTokenBlocked(t *testing.T) {
	server := &WSTunnelServer{
		blockedTokens: make(map[token]time.Time),
	}

	if server.IsTokenBlocked(token("unknown-token-12345")) {
		t.Fatal("unknown token should not be blocked")
	}

	tok := token("blocked-token-123456")
	server.BlockToken(tok)

	if !server.IsTokenBlocked(tok) {
		t.Fatal("blocked token should report as blocked")
	}
}

func TestDisconnectToken(t *testing.T) {
	server := &WSTunnelServer{
		serverRegistry:      make(map[token]*remoteServer),
		serverRegistryMutex: sync.Mutex{},
		blockedTokens:       make(map[token]time.Time),
	}

	tok := token("disconnect-test-token")

	// Disconnect non-existent token should return 0
	count := server.DisconnectToken(tok)
	if count != 0 {
		t.Fatalf("expected 0 for non-existent token, got %d", count)
	}

	// Create a remote server with connections
	rs := &remoteServer{
		token:          tok,
		connCloseChans: make([]chan struct{}, 0),
	}
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	rs.RegisterConnection(ch1)
	rs.RegisterConnection(ch2)

	server.serverRegistryMutex.Lock()
	server.serverRegistry[tok] = rs
	server.serverRegistryMutex.Unlock()

	count = server.DisconnectToken(tok)
	if count != 2 {
		t.Fatalf("expected 2 disconnected, got %d", count)
	}
}

// --- Handler tests ---

func TestHandleTunnelDisconnect_Success(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	tok := token("test-disconnect-token")
	rs := &remoteServer{
		token:          tok,
		connCloseChans: make([]chan struct{}, 0),
	}
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	rs.RegisterConnection(ch1)
	rs.RegisterConnection(ch2)

	adminService.server.serverRegistry[tok] = rs

	req := httptest.NewRequest("POST", "/admin/tunnels/test-disconnect-token/disconnect", nil)
	w := httptest.NewRecorder()

	adminService.HandleTunnelDisconnect(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", result.StatusCode)
	}

	var resp DisconnectResponse
	if err := json.NewDecoder(result.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Disconnected != 2 {
		t.Fatalf("expected 2 disconnected, got %d", resp.Disconnected)
	}

	if resp.Message != "disconnected 2 connection(s)" {
		t.Fatalf("unexpected message: %s", resp.Message)
	}
}

func TestHandleTunnelDisconnect_NotFound(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/admin/tunnels/nonexistent-token-x/disconnect", nil)
	w := httptest.NewRecorder()

	adminService.HandleTunnelDisconnect(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", result.StatusCode)
	}

	var resp DisconnectResponse
	if err := json.NewDecoder(result.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Disconnected != 0 {
		t.Fatalf("expected 0 disconnected, got %d", resp.Disconnected)
	}
}

func TestHandleTunnelDisconnect_MethodNotAllowed(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/admin/tunnels/some-token-here123/disconnect", nil)
	w := httptest.NewRecorder()

	adminService.HandleTunnelDisconnect(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", result.StatusCode)
	}
}

func TestHandleTunnelDisconnect_InvalidPath(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/admin/tunnels/", nil)
	w := httptest.NewRecorder()

	adminService.HandleTunnelDisconnect(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", result.StatusCode)
	}
}

func TestHandleTokenBlock_Block(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	tok := token("block-test-token-1234")

	req := httptest.NewRequest("POST", "/admin/tokens/block-test-token-1234/block", nil)
	w := httptest.NewRecorder()

	adminService.HandleTokenBlock(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", result.StatusCode)
	}

	var resp BlockResponse
	if err := json.NewDecoder(result.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if !resp.Blocked {
		t.Fatal("expected blocked to be true")
	}

	// Verify token is actually blocked
	if !adminService.server.IsTokenBlocked(tok) {
		t.Fatal("token should be blocked on server")
	}
}

func TestHandleTokenBlock_BlockWithDisconnect(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	tok := token("block-disconnect-token")
	rs := &remoteServer{
		token:          tok,
		connCloseChans: make([]chan struct{}, 0),
	}
	ch := make(chan struct{})
	rs.RegisterConnection(ch)
	adminService.server.serverRegistry[tok] = rs

	req := httptest.NewRequest("POST", "/admin/tokens/block-disconnect-token/block?disconnect=true", nil)
	w := httptest.NewRecorder()

	adminService.HandleTokenBlock(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", result.StatusCode)
	}

	var resp BlockResponse
	if err := json.NewDecoder(result.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if !resp.Blocked {
		t.Fatal("expected blocked to be true")
	}

	if resp.Disconnected != 1 {
		t.Fatalf("expected 1 disconnected, got %d", resp.Disconnected)
	}

	// Verify channel was closed
	select {
	case <-ch:
		// good
	default:
		t.Fatal("connection channel should be closed")
	}
}

func TestHandleTokenBlock_Unblock(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	tok := token("unblock-test-token-12")

	// Block first
	adminService.server.BlockToken(tok)

	req := httptest.NewRequest("DELETE", "/admin/tokens/unblock-test-token-12/block", nil)
	w := httptest.NewRecorder()

	adminService.HandleTokenBlock(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", result.StatusCode)
	}

	var resp BlockResponse
	if err := json.NewDecoder(result.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Blocked {
		t.Fatal("expected blocked to be false after unblock")
	}

	// Verify token is actually unblocked
	if adminService.server.IsTokenBlocked(tok) {
		t.Fatal("token should be unblocked on server")
	}
}

func TestHandleTokenBlock_MethodNotAllowed(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	req := httptest.NewRequest("PUT", "/admin/tokens/some-test-token-xyz/block", nil)
	w := httptest.NewRecorder()

	adminService.HandleTokenBlock(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", result.StatusCode)
	}
}

func TestHandleBlockedTokens_Empty(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/admin/tokens/blocked", nil)
	w := httptest.NewRecorder()

	adminService.HandleBlockedTokens(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", result.StatusCode)
	}

	var resp BlockedTokensResponse
	if err := json.NewDecoder(result.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Count != 0 {
		t.Fatalf("expected 0 blocked tokens, got %d", resp.Count)
	}

	if len(resp.BlockedTokens) != 0 {
		t.Fatalf("expected empty blocked_tokens list, got %d", len(resp.BlockedTokens))
	}
}

func TestHandleBlockedTokens_WithTokens(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	adminService.server.BlockToken(token("blocked-token-aaaaaa"))
	adminService.server.BlockToken(token("blocked-token-bbbbbb"))

	req := httptest.NewRequest("GET", "/admin/tokens/blocked", nil)
	w := httptest.NewRecorder()

	adminService.HandleBlockedTokens(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", result.StatusCode)
	}

	var resp BlockedTokensResponse
	if err := json.NewDecoder(result.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Count != 2 {
		t.Fatalf("expected 2 blocked tokens, got %d", resp.Count)
	}

	if len(resp.BlockedTokens) != 2 {
		t.Fatalf("expected 2 entries in blocked_tokens, got %d", len(resp.BlockedTokens))
	}

	// Verify tokens are truncated (cutToken format: first 8 chars + "...")
	for _, bt := range resp.BlockedTokens {
		if len(bt.Token) > 11 {
			t.Fatalf("expected truncated token, got %q", bt.Token)
		}
	}
}

func TestHandleBlockedTokens_MethodNotAllowed(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/admin/tokens/blocked", nil)
	w := httptest.NewRecorder()

	adminService.HandleBlockedTokens(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", result.StatusCode)
	}
}

// --- Lifecycle test ---

func TestBlockDisconnectLifecycle(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	tok := token("lifecycle-test-token1")

	// 1. Token should not be blocked initially
	if adminService.server.IsTokenBlocked(tok) {
		t.Fatal("token should not be blocked initially")
	}

	// 2. Block the token
	blockReq := httptest.NewRequest("POST", "/admin/tokens/lifecycle-test-token1/block", nil)
	blockW := httptest.NewRecorder()
	adminService.HandleTokenBlock(blockW, blockReq)

	blockResult := blockW.Result()
	defer func() { _ = blockResult.Body.Close() }()

	if blockResult.StatusCode != http.StatusOK {
		t.Fatalf("block: expected 200, got %d", blockResult.StatusCode)
	}

	// 3. Verify token appears in blocked list
	listReq := httptest.NewRequest("GET", "/admin/tokens/blocked", nil)
	listW := httptest.NewRecorder()
	adminService.HandleBlockedTokens(listW, listReq)

	listResult := listW.Result()
	defer func() { _ = listResult.Body.Close() }()

	var listResp BlockedTokensResponse
	if err := json.NewDecoder(listResult.Body).Decode(&listResp); err != nil {
		t.Fatal(err)
	}

	if listResp.Count != 1 {
		t.Fatalf("expected 1 blocked token, got %d", listResp.Count)
	}

	// 4. Unblock the token
	unblockReq := httptest.NewRequest("DELETE", "/admin/tokens/lifecycle-test-token1/block", nil)
	unblockW := httptest.NewRecorder()
	adminService.HandleTokenBlock(unblockW, unblockReq)

	unblockResult := unblockW.Result()
	defer func() { _ = unblockResult.Body.Close() }()

	if unblockResult.StatusCode != http.StatusOK {
		t.Fatalf("unblock: expected 200, got %d", unblockResult.StatusCode)
	}

	// 5. Verify token no longer in blocked list
	listReq2 := httptest.NewRequest("GET", "/admin/tokens/blocked", nil)
	listW2 := httptest.NewRecorder()
	adminService.HandleBlockedTokens(listW2, listReq2)

	listResult2 := listW2.Result()
	defer func() { _ = listResult2.Body.Close() }()

	var listResp2 BlockedTokensResponse
	if err := json.NewDecoder(listResult2.Body).Decode(&listResp2); err != nil {
		t.Fatal(err)
	}

	if listResp2.Count != 0 {
		t.Fatalf("expected 0 blocked tokens after unblock, got %d", listResp2.Count)
	}

	// 6. Verify token is no longer blocked
	if adminService.server.IsTokenBlocked(tok) {
		t.Fatal("token should not be blocked after unblock")
	}
}

func TestHandleTunnelDisconnect_WithBasePath(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	adminService.server.BasePath = "/wstunnel"

	tok := token("basepath-test-token12")
	rs := &remoteServer{
		token:          tok,
		connCloseChans: make([]chan struct{}, 0),
	}
	ch := make(chan struct{})
	rs.RegisterConnection(ch)
	adminService.server.serverRegistry[tok] = rs

	req := httptest.NewRequest("POST", "/wstunnel/admin/tunnels/basepath-test-token12/disconnect", nil)
	w := httptest.NewRecorder()

	adminService.HandleTunnelDisconnect(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", result.StatusCode)
	}

	var resp DisconnectResponse
	if err := json.NewDecoder(result.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Disconnected != 1 {
		t.Fatalf("expected 1 disconnected with base path, got %d", resp.Disconnected)
	}
}

func TestHandleTokenBlock_WithBasePath(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	adminService.server.BasePath = "/wstunnel"

	req := httptest.NewRequest("POST", "/wstunnel/admin/tokens/basepath-block-token1/block", nil)
	w := httptest.NewRecorder()

	adminService.HandleTokenBlock(w, req)

	result := w.Result()
	defer func() { _ = result.Body.Close() }()

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", result.StatusCode)
	}

	if !adminService.server.IsTokenBlocked(token("basepath-block-token1")) {
		t.Fatal("token should be blocked even with base path")
	}
}
