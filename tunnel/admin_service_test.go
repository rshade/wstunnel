// Copyright (c) 2014 RightScale, Inc. - see LICENSE

package tunnel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/inconshreveable/log15.v2"
)

func TestNewAdminService(t *testing.T) {
	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "admin_test*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create mock server
	server := &WSTunnelServer{
		Log:            log15.New("pkg", "test"),
		serverRegistry: make(map[token]*remoteServer),
		tokenClients:   make(map[token]int),
	}

	// Create admin service
	adminService, err := NewAdminService(server, tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create admin service: %v", err)
	}
	defer func() {
		if err := adminService.Close(); err != nil {
			t.Logf("Failed to close admin service: %v", err)
		}
	}()

	// Test database initialization
	tables := []string{"request_events", "tunnel_events"}
	for _, table := range tables {
		var name string
		err = adminService.db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("Expected table %s to exist: %v", table, err)
		}
	}
}

func TestRecordRequestLifecycle(t *testing.T) {
	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "admin_test*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create mock server
	server := &WSTunnelServer{
		Log:            log15.New("pkg", "test"),
		serverRegistry: make(map[token]*remoteServer),
		tokenClients:   make(map[token]int),
	}

	// Create admin service
	adminService, err := NewAdminService(server, tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create admin service: %v", err)
	}
	defer func() {
		if err := adminService.Close(); err != nil {
			t.Logf("Failed to close admin service: %v", err)
		}
	}()

	// Test recording request start
	requestID, err := adminService.RecordRequestStart("test-token", "GET", "/test", "192.168.1.1")
	if err != nil {
		t.Fatalf("Failed to record request start: %v", err)
	}
	if requestID <= 0 {
		t.Errorf("Expected positive request ID, got %d", requestID)
	}

	// Test recording successful completion
	err = adminService.RecordRequestComplete(requestID, true, "")
	if err != nil {
		t.Fatalf("Failed to record request completion: %v", err)
	}

	// Test recording failed request
	requestID2, err := adminService.RecordRequestStart("test-token", "POST", "/fail", "192.168.1.2")
	if err != nil {
		t.Fatalf("Failed to record second request start: %v", err)
	}

	err = adminService.RecordRequestComplete(requestID2, false, "Connection timeout")
	if err != nil {
		t.Fatalf("Failed to record request failure: %v", err)
	}

	// Verify database contents
	var completedCount, erroredCount int
	err = adminService.db.QueryRow("SELECT COUNT(*) FROM request_events WHERE status = 'completed'").Scan(&completedCount)
	if err != nil {
		t.Fatalf("Failed to query completed requests: %v", err)
	}
	if completedCount != 1 {
		t.Errorf("Expected 1 completed request, got %d", completedCount)
	}

	err = adminService.db.QueryRow("SELECT COUNT(*) FROM request_events WHERE status = 'errored'").Scan(&erroredCount)
	if err != nil {
		t.Fatalf("Failed to query errored requests: %v", err)
	}
	if erroredCount != 1 {
		t.Errorf("Expected 1 errored request, got %d", erroredCount)
	}
}

func TestRecordTunnelEvent(t *testing.T) {
	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "admin_test*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create mock server
	server := &WSTunnelServer{
		Log:            log15.New("pkg", "test"),
		serverRegistry: make(map[token]*remoteServer),
		tokenClients:   make(map[token]int),
	}

	// Create admin service
	adminService, err := NewAdminService(server, tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create admin service: %v", err)
	}
	defer func() {
		if err := adminService.Close(); err != nil {
			t.Logf("Failed to close admin service: %v", err)
		}
	}()

	// Test recording tunnel event
	err = adminService.RecordTunnelEvent(
		"test-token",
		"connected",
		"192.168.1.100",
		"client.example.com",
		"Example Corp",
		"wstunnel-1.0",
		"Client connected successfully",
	)
	if err != nil {
		t.Fatalf("Failed to record tunnel event: %v", err)
	}

	// Verify database contents
	var count int
	err = adminService.db.QueryRow("SELECT COUNT(*) FROM tunnel_events WHERE event = 'connected'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tunnel events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 tunnel event, got %d", count)
	}
}

func TestGetMonitoringStats(t *testing.T) {
	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "admin_test*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create mock server with some tunnels
	server := &WSTunnelServer{
		Log:            log15.New("pkg", "test"),
		serverRegistry: make(map[token]*remoteServer),
		tokenClients:   make(map[token]int),
	}

	// Add some mock tunnels
	server.serverRegistry[token("token1")] = &remoteServer{
		token:        token("token1"),
		lastActivity: time.Now(),
	}
	server.serverRegistry[token("token2")] = &remoteServer{
		token:        token("token2"),
		lastActivity: time.Now().Add(-30 * time.Minute), // active
	}

	// Create admin service
	adminService, err := NewAdminService(server, tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create admin service: %v", err)
	}
	defer func() {
		if err := adminService.Close(); err != nil {
			t.Logf("Failed to close admin service: %v", err)
		}
	}()

	// Add some test data
	_, err = adminService.RecordRequestStart("token1", "GET", "/test1", "1.1.1.1")
	if err != nil {
		t.Fatalf("Failed to record request: %v", err)
	}

	requestID, err := adminService.RecordRequestStart("token1", "POST", "/test2", "2.2.2.2")
	if err != nil {
		t.Fatalf("Failed to record request: %v", err)
	}
	err = adminService.RecordRequestComplete(requestID, true, "")
	if err != nil {
		t.Fatalf("Failed to complete request: %v", err)
	}

	// Get monitoring stats
	stats, err := adminService.GetMonitoringStats()
	if err != nil {
		t.Fatalf("Failed to get monitoring stats: %v", err)
	}

	// Verify results
	if stats.UniqueTunnels != 2 {
		t.Errorf("Expected 2 unique tunnels, got %d", stats.UniqueTunnels)
	}
	if stats.TunnelConnections != 2 {
		t.Errorf("Expected 2 tunnel connections, got %d", stats.TunnelConnections)
	}
	if stats.PendingRequests != 1 {
		t.Errorf("Expected 1 pending request, got %d", stats.PendingRequests)
	}
	if stats.CompletedRequests != 1 {
		t.Errorf("Expected 1 completed request, got %d", stats.CompletedRequests)
	}
	if stats.ErroredRequests != 0 {
		t.Errorf("Expected 0 errored requests, got %d", stats.ErroredRequests)
	}
}

func TestGetAuditingData(t *testing.T) {
	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "admin_test*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create mock server with some tunnels
	server := &WSTunnelServer{
		Log:            log15.New("pkg", "test"),
		serverRegistry: make(map[token]*remoteServer),
		tokenClients:   make(map[token]int),
	}

	// Add mock tunnel with active requests
	rs := &remoteServer{
		token:         token("test-token"),
		lastActivity:  time.Now(),
		remoteAddr:    "192.168.1.100",
		remoteName:    "client.example.com",
		remoteWhois:   "Example Corp",
		clientVersion: "wstunnel-1.0",
		requestSet:    make(map[int16]*remoteRequest),
	}

	// Add mock active request
	req := &remoteRequest{
		id:         1,
		info:       "GET /api/test",
		remoteAddr: "10.0.0.1",
		startTime:  time.Now().Add(-5 * time.Minute),
	}
	rs.requestSet[1] = req

	server.serverRegistry[token("test-token")] = rs

	// Create admin service
	adminService, err := NewAdminService(server, tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create admin service: %v", err)
	}
	defer func() {
		if err := adminService.Close(); err != nil {
			t.Logf("Failed to close admin service: %v", err)
		}
	}()

	// Add some request history
	requestID, err := adminService.RecordRequestStart("test-token", "POST", "/api/success", "10.0.0.2")
	if err != nil {
		t.Fatalf("Failed to record successful request: %v", err)
	}
	err = adminService.RecordRequestComplete(requestID, true, "")
	if err != nil {
		t.Fatalf("Failed to complete successful request: %v", err)
	}

	requestID2, err := adminService.RecordRequestStart("test-token", "PUT", "/api/error", "10.0.0.3")
	if err != nil {
		t.Fatalf("Failed to record error request: %v", err)
	}
	err = adminService.RecordRequestComplete(requestID2, false, "Server error")
	if err != nil {
		t.Fatalf("Failed to complete error request: %v", err)
	}

	// Get auditing data
	data, err := adminService.GetAuditingData()
	if err != nil {
		t.Fatalf("Failed to get auditing data: %v", err)
	}

	// Verify results
	if len(data.Tunnels) != 1 {
		t.Fatalf("Expected 1 tunnel, got %d", len(data.Tunnels))
	}

	tunnel := data.Tunnels["test-token"]
	if tunnel == nil {
		t.Fatal("Expected tunnel data for test-token")
	}

	if tunnel.Token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", tunnel.Token)
	}
	if tunnel.RemoteAddr != "192.168.1.100" {
		t.Errorf("Expected remote addr '192.168.1.100', got '%s'", tunnel.RemoteAddr)
	}
	if tunnel.ClientVersion != "wstunnel-1.0" {
		t.Errorf("Expected client version 'wstunnel-1.0', got '%s'", tunnel.ClientVersion)
	}
	if len(tunnel.ActiveConnections) != 1 {
		t.Errorf("Expected 1 active connection, got %d", len(tunnel.ActiveConnections))
	}
	if tunnel.PendingRequests != 1 {
		t.Errorf("Expected 1 pending request, got %d", tunnel.PendingRequests)
	}

	// Verify active connection details
	conn := tunnel.ActiveConnections[0]
	if conn.RequestID != 1 {
		t.Errorf("Expected request ID 1, got %d", conn.RequestID)
	}
	if conn.Method != "GET" {
		t.Errorf("Expected method 'GET', got '%s'", conn.Method)
	}
	if conn.URI != "/api/test" {
		t.Errorf("Expected URI '/api/test', got '%s'", conn.URI)
	}
	if conn.RemoteAddr != "10.0.0.1" {
		t.Errorf("Expected remote addr '10.0.0.1', got '%s'", conn.RemoteAddr)
	}

	// Verify last success and error times are set
	if tunnel.LastSuccessTime == nil {
		t.Error("Expected last success time to be set")
	}
	if tunnel.LastErrorTime == nil {
		t.Error("Expected last error time to be set")
	}
}

func TestHandleAuditing(t *testing.T) {
	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "admin_test*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create mock server
	server := &WSTunnelServer{
		Log:            log15.New("pkg", "test"),
		serverRegistry: make(map[token]*remoteServer),
		tokenClients:   make(map[token]int),
	}

	// Create admin service
	adminService, err := NewAdminService(server, tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create admin service: %v", err)
	}
	defer func() {
		if err := adminService.Close(); err != nil {
			t.Logf("Failed to close admin service: %v", err)
		}
	}()

	// Test GET request
	req := httptest.NewRequest("GET", "/admin/auditing", nil)
	w := httptest.NewRecorder()

	adminService.HandleAuditing(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	// Verify JSON response
	var auditingResp AuditingResponse
	err = json.NewDecoder(resp.Body).Decode(&auditingResp)
	if err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if auditingResp.Tunnels == nil {
		t.Error("Expected tunnels map to be initialized")
	}

	// Test non-GET request
	req = httptest.NewRequest("POST", "/admin/auditing", nil)
	w = httptest.NewRecorder()

	adminService.HandleAuditing(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestHandleMonitoring(t *testing.T) {
	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "admin_test*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create mock server
	server := &WSTunnelServer{
		Log:            log15.New("pkg", "test"),
		serverRegistry: make(map[token]*remoteServer),
		tokenClients:   make(map[token]int),
	}

	// Create admin service
	adminService, err := NewAdminService(server, tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create admin service: %v", err)
	}
	defer func() {
		if err := adminService.Close(); err != nil {
			t.Logf("Failed to close admin service: %v", err)
		}
	}()

	// Test GET request
	req := httptest.NewRequest("GET", "/admin/monitoring", nil)
	w := httptest.NewRecorder()

	adminService.HandleMonitoring(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	// Verify JSON response
	var monitoringResp MonitoringResponse
	err = json.NewDecoder(resp.Body).Decode(&monitoringResp)
	if err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if monitoringResp.UniqueTunnels < 0 {
		t.Error("Expected non-negative unique tunnels count")
	}

	// Test non-GET request
	req = httptest.NewRequest("DELETE", "/admin/monitoring", nil)
	w = httptest.NewRecorder()

	adminService.HandleMonitoring(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestAdminServiceBasePathIntegration(t *testing.T) {
	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "admin_test*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create mock server with base path
	server := &WSTunnelServer{
		Log:            log15.New("pkg", "test"),
		BasePath:       "/wstunnel",
		serverRegistry: make(map[token]*remoteServer),
		tokenClients:   make(map[token]int),
	}

	// Create admin service
	adminService, err := NewAdminService(server, tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create admin service: %v", err)
	}
	defer func() {
		if err := adminService.Close(); err != nil {
			t.Logf("Failed to close admin service: %v", err)
		}
	}()

	// Test that the service works regardless of base path
	// (base path handling is done at the HTTP mux level)
	req := httptest.NewRequest("GET", "/admin/monitoring", nil)
	w := httptest.NewRecorder()

	adminService.HandleMonitoring(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
