// Copyright (c) 2014 RightScale, Inc. - see LICENSE

package tunnel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/inconshreveable/log15.v2"
)

// setupTestAdminService creates a temporary database and admin service for testing
func setupTestAdminService(t *testing.T) (*AdminService, func()) {
	t.Helper()

	// Create temporary database file
	tmpfile, err := os.CreateTemp("", "admin_test*.db")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		if err := os.Remove(tmpfile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}

	if err := tmpfile.Close(); err != nil {
		cleanup()
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
		cleanup()
		t.Fatalf("Failed to create admin service: %v", err)
	}

	finalCleanup := func() {
		if err := adminService.Close(); err != nil {
			t.Logf("Failed to close admin service: %v", err)
		}
		cleanup()
	}

	return adminService, finalCleanup
}

func TestNewAdminService(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	// Test database initialization
	tables := []string{"request_events", "tunnel_events"}
	for _, table := range tables {
		var name string
		err := adminService.db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("Expected table %s to exist: %v", table, err)
		}
	}
}

func TestRecordRequestLifecycle(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	// Test recording request start
	requestID, err := adminService.RecordRequestStart(context.Background(), "test-token", "GET", "/test", "192.168.1.1")
	if err != nil {
		t.Fatalf("Failed to record request start: %v", err)
	}
	if requestID <= 0 {
		t.Errorf("Expected positive request ID, got %d", requestID)
	}

	// Test recording successful completion
	err = adminService.RecordRequestComplete(context.Background(), requestID, true, "")
	if err != nil {
		t.Fatalf("Failed to record request completion: %v", err)
	}

	// Test recording failed request
	requestID2, err := adminService.RecordRequestStart(context.Background(), "test-token", "POST", "/fail", "192.168.1.2")
	if err != nil {
		t.Fatalf("Failed to record second request start: %v", err)
	}

	err = adminService.RecordRequestComplete(context.Background(), requestID2, false, "Connection timeout")
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
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	// Test recording tunnel event
	err := adminService.RecordTunnelEvent(
		context.Background(),
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
	_, err = adminService.RecordRequestStart(context.Background(), "token1", "GET", "/test1", "1.1.1.1")
	if err != nil {
		t.Fatalf("Failed to record request: %v", err)
	}

	requestID, err := adminService.RecordRequestStart(context.Background(), "token1", "POST", "/test2", "2.2.2.2")
	if err != nil {
		t.Fatalf("Failed to record request: %v", err)
	}
	err = adminService.RecordRequestComplete(context.Background(), requestID, true, "")
	if err != nil {
		t.Fatalf("Failed to complete request: %v", err)
	}

	// Get monitoring stats
	stats, err := adminService.GetMonitoringStats(context.Background())
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
	requestID, err := adminService.RecordRequestStart(context.Background(), "test-token", "POST", "/api/success", "10.0.0.2")
	if err != nil {
		t.Fatalf("Failed to record successful request: %v", err)
	}
	err = adminService.RecordRequestComplete(context.Background(), requestID, true, "")
	if err != nil {
		t.Fatalf("Failed to complete successful request: %v", err)
	}

	requestID2, err := adminService.RecordRequestStart(context.Background(), "test-token", "PUT", "/api/error", "10.0.0.3")
	if err != nil {
		t.Fatalf("Failed to record error request: %v", err)
	}
	err = adminService.RecordRequestComplete(context.Background(), requestID2, false, "Server error")
	if err != nil {
		t.Fatalf("Failed to complete error request: %v", err)
	}

	// Get auditing data
	data, err := adminService.GetAuditingData(context.Background())
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
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

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
	err := json.NewDecoder(resp.Body).Decode(&auditingResp)
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

	// Note: Error scenario testing for database errors is complex to simulate
	// without interfering with other operations. The error handling code path
	// exists in HandleAuditing and GetAuditingData methods.
}

func TestHandleMonitoring(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

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
	err := json.NewDecoder(resp.Body).Decode(&monitoringResp)
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

func TestHandleAPIDocs(t *testing.T) {
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
	req := httptest.NewRequest("GET", "/admin/api-docs", nil)
	w := httptest.NewRecorder()

	adminService.HandleAPIDocs(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	// Verify JSON response
	var apiDocsResp APIDocsResponse
	err = json.NewDecoder(resp.Body).Decode(&apiDocsResp)
	if err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if apiDocsResp.Version == "" {
		t.Error("Expected version to be set")
	}

	if len(apiDocsResp.Endpoints) == 0 {
		t.Error("Expected at least one endpoint to be documented")
	}

	// Verify essential endpoints are documented
	foundEndpoints := make(map[string]bool)
	for _, endpoint := range apiDocsResp.Endpoints {
		foundEndpoints[endpoint.Path] = true

		// Verify each endpoint has required fields
		if endpoint.Path == "" {
			t.Error("Endpoint path should not be empty")
		}
		if endpoint.Method == "" {
			t.Error("Endpoint method should not be empty")
		}
		if endpoint.Description == "" {
			t.Error("Endpoint description should not be empty")
		}
		if endpoint.Response == nil {
			t.Error("Endpoint response schema should not be nil")
		}
	}

	expectedEndpoints := []string{"/admin/monitoring", "/admin/auditing", "/admin/api-docs"}
	for _, expected := range expectedEndpoints {
		if !foundEndpoints[expected] {
			t.Errorf("Expected endpoint %s to be documented", expected)
		}
	}

	// Test non-GET request
	req = httptest.NewRequest("PUT", "/admin/api-docs", nil)
	w = httptest.NewRecorder()

	adminService.HandleAPIDocs(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestHandleAdminUI(t *testing.T) {
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
	req := httptest.NewRequest("GET", "/admin/ui", nil)
	w := httptest.NewRecorder()

	adminService.HandleAdminUI(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}

	// Verify cache control headers
	cacheControl := resp.Header.Get("Cache-Control")
	if !strings.Contains(cacheControl, "no-cache") {
		t.Errorf("Expected no-cache in Cache-Control header, got %s", cacheControl)
	}

	// Verify HTML content contains essential elements
	body := w.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("Expected HTML DOCTYPE")
	}
	if !strings.Contains(body, "WStunnel Admin Dashboard") {
		t.Error("Expected page title in HTML")
	}
	if !strings.Contains(body, "/admin/monitoring") {
		t.Error("Expected admin API endpoints referenced in HTML")
	}
	if !strings.Contains(body, "fetchData") {
		t.Error("Expected JavaScript functionality in HTML")
	}

	// Test non-GET request
	req = httptest.NewRequest("POST", "/admin/ui", nil)
	w = httptest.NewRecorder()

	adminService.HandleAdminUI(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestHandleAdminUIRedirect(t *testing.T) {
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

	// Test redirect from /admin
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()

	adminService.HandleAdminUIRedirect(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("Expected status 307, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/admin/ui" {
		t.Errorf("Expected redirect to /admin/ui, got %s", location)
	}

	// Test redirect with base path
	req = httptest.NewRequest("GET", "/wstunnel/admin", nil)
	w = httptest.NewRecorder()

	adminService.HandleAdminUIRedirect(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("Expected status 307, got %d", resp.StatusCode)
	}

	location = resp.Header.Get("Location")
	if location != "/wstunnel/admin/ui" {
		t.Errorf("Expected redirect to /wstunnel/admin/ui, got %s", location)
	}
}

func TestGetAPIDocumentation(t *testing.T) {
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

	// Get API documentation
	docs := adminService.GetAPIDocumentation()

	// Verify structure
	if docs.Version == "" {
		t.Error("Expected version to be set")
	}

	if len(docs.Endpoints) == 0 {
		t.Error("Expected at least one endpoint")
	}

	// Verify each endpoint has proper structure
	for _, endpoint := range docs.Endpoints {
		if endpoint.Path == "" {
			t.Error("Endpoint path should not be empty")
		}
		if endpoint.Method != "GET" {
			t.Errorf("Expected all endpoints to be GET, got %s", endpoint.Method)
		}
		if endpoint.Description == "" {
			t.Error("Endpoint description should not be empty")
		}
		if endpoint.Response == nil {
			t.Error("Endpoint response should not be nil")
		}

		// Verify response schema has timestamp field for data endpoints
		if endpoint.Path == "/admin/monitoring" || endpoint.Path == "/admin/auditing" {
			timestampField, exists := endpoint.Response["timestamp"]
			if !exists {
				t.Errorf("Expected timestamp field in %s response schema", endpoint.Path)
			}
			if timestampField == nil {
				t.Errorf("Expected timestamp field to be defined in %s", endpoint.Path)
			}
		}
	}
}

func TestRecordRequestStartValidation(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	// Test empty token
	_, err := adminService.RecordRequestStart(context.Background(), "", "GET", "/test", "192.168.1.1")
	if err == nil {
		t.Error("Expected error for empty token")
	}

	// Test empty method
	_, err = adminService.RecordRequestStart(context.Background(), "test-token", "", "/test", "192.168.1.1")
	if err == nil {
		t.Error("Expected error for empty method")
	}

	// Test empty URI
	_, err = adminService.RecordRequestStart(context.Background(), "test-token", "GET", "", "192.168.1.1")
	if err == nil {
		t.Error("Expected error for empty URI")
	}

	// Test token too long
	longToken := make([]byte, 256)
	for i := range longToken {
		longToken[i] = 'a'
	}
	_, err = adminService.RecordRequestStart(context.Background(), string(longToken), "GET", "/test", "192.168.1.1")
	if err == nil {
		t.Error("Expected error for token too long")
	}
}

func TestRecordTunnelEventValidation(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	// Test empty token
	err := adminService.RecordTunnelEvent(context.Background(), "", "connected", "192.168.1.1", "client.example.com", "Example Corp", "wstunnel-1.0", "Connected successfully")
	if err == nil {
		t.Error("Expected error for empty token")
	}

	// Test empty event
	err = adminService.RecordTunnelEvent(context.Background(), "test-token", "", "192.168.1.1", "client.example.com", "Example Corp", "wstunnel-1.0", "Connected successfully")
	if err == nil {
		t.Error("Expected error for empty event")
	}

	// Test details too long
	longDetails := make([]byte, 1001)
	for i := range longDetails {
		longDetails[i] = 'a'
	}
	err = adminService.RecordTunnelEvent(context.Background(), "test-token", "connected", "192.168.1.1", "client.example.com", "Example Corp", "wstunnel-1.0", string(longDetails))
	if err == nil {
		t.Error("Expected error for details too long")
	}
}

func TestRecordRequestCompleteInvalidID(t *testing.T) {
	adminService, cleanup := setupTestAdminService(t)
	defer cleanup()

	// Test with non-existent request ID
	err := adminService.RecordRequestComplete(context.Background(), 99999, true, "")
	if err == nil {
		t.Error("Expected error for non-existent request ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}
