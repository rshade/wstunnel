// Copyright (c) 2014 RightScale, Inc. - see LICENSE

package tunnel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"gopkg.in/inconshreveable/log15.v2"
	_ "modernc.org/sqlite"
)

// AdminService provides administrative endpoints for monitoring and auditing
type AdminService struct {
	db     *sql.DB
	server *WSTunnelServer
	log    log15.Logger
	mu     sync.RWMutex
	done   chan struct{}
	wg     sync.WaitGroup
}

// TunnelDetail provides detailed tunnel information for auditing
type TunnelDetail struct {
	Token             string              `json:"token"`
	RemoteAddr        string              `json:"remote_addr"`
	RemoteName        string              `json:"remote_name"`
	RemoteWhois       string              `json:"remote_whois"`
	ClientVersion     string              `json:"client_version"`
	LastActivity      time.Time           `json:"last_activity"`
	ActiveConnections []*ConnectionDetail `json:"active_connections"`
	LastErrorTime     *time.Time          `json:"last_error_time,omitempty"`
	LastErrorAddr     string              `json:"last_error_addr,omitempty"`
	LastSuccessTime   *time.Time          `json:"last_success_time,omitempty"`
	LastSuccessAddr   string              `json:"last_success_addr,omitempty"`
	PendingRequests   int                 `json:"pending_requests"`
}

// ConnectionDetail provides information about active connections
type ConnectionDetail struct {
	RequestID  int16     `json:"request_id"`
	Method     string    `json:"method"`
	URI        string    `json:"uri"`
	RemoteAddr string    `json:"remote_addr"`
	StartTime  time.Time `json:"start_time"`
}

// AuditingResponse represents the JSON response for /admin/auditing
type AuditingResponse struct {
	Timestamp time.Time                `json:"timestamp"`
	Tunnels   map[string]*TunnelDetail `json:"tunnels"`
}

// MonitoringResponse represents the JSON response for /admin/monitoring
type MonitoringResponse struct {
	Timestamp         time.Time `json:"timestamp"`
	UniqueTunnels     int       `json:"unique_tunnels"`
	TunnelConnections int       `json:"tunnel_connections"`
	PendingRequests   int64     `json:"pending_requests"`
	CompletedRequests int64     `json:"completed_requests"`
	ErroredRequests   int64     `json:"errored_requests"`
}

// RequestEvent represents a request event for tracking
type RequestEvent struct {
	ID         int64      `json:"id"`
	Token      string     `json:"token"`
	Method     string     `json:"method"`
	URI        string     `json:"uri"`
	RemoteAddr string     `json:"remote_addr"`
	Status     string     `json:"status"` // pending, completed, errored
	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// TunnelEvent represents a tunnel lifecycle event
type TunnelEvent struct {
	ID            int64     `json:"id"`
	Token         string    `json:"token"`
	Event         string    `json:"event"` // connected, disconnected, error
	RemoteAddr    string    `json:"remote_addr"`
	RemoteName    string    `json:"remote_name"`
	RemoteWhois   string    `json:"remote_whois"`
	ClientVersion string    `json:"client_version"`
	Timestamp     time.Time `json:"timestamp"`
	Details       string    `json:"details,omitempty"`
}

// APIDocsResponse represents the JSON response for /admin/api-docs
type APIDocsResponse struct {
	Version   string        `json:"version"`
	Endpoints []APIEndpoint `json:"endpoints"`
}

// APIEndpoint represents a single API endpoint documentation
type APIEndpoint struct {
	Path        string                 `json:"path"`
	Method      string                 `json:"method"`
	Description string                 `json:"description"`
	Response    map[string]interface{} `json:"response"`
}

// NewAdminService creates a new admin service with SQLite tracking
func NewAdminService(server *WSTunnelServer, dbPath string) (*AdminService, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	service := &AdminService{
		db:     db,
		server: server,
		log:    server.Log.New("component", "admin"),
		done:   make(chan struct{}),
	}

	if err := service.initDB(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			service.log.Error("failed to close database after init error", "error", closeErr)
		}
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Start cleanup goroutine
	service.wg.Add(1)
	go service.cleanupOldRecords()

	return service, nil
}

// Close closes the admin service and database connection
func (as *AdminService) Close() error {
	// Signal cleanup goroutine to stop
	close(as.done)
	// Wait for cleanup goroutine to finish
	as.wg.Wait()
	return as.db.Close()
}

// initDB initializes the database schema
func (as *AdminService) initDB() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS request_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT NOT NULL,
			method TEXT NOT NULL,
			uri TEXT NOT NULL,
			remote_addr TEXT NOT NULL,
			status TEXT NOT NULL,
			start_time DATETIME NOT NULL,
			end_time DATETIME,
			error TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS tunnel_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT NOT NULL,
			event TEXT NOT NULL,
			remote_addr TEXT NOT NULL,
			remote_name TEXT,
			remote_whois TEXT,
			client_version TEXT,
			timestamp DATETIME NOT NULL,
			details TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_request_events_token ON request_events(token)`,
		`CREATE INDEX IF NOT EXISTS idx_request_events_status ON request_events(status)`,
		`CREATE INDEX IF NOT EXISTS idx_request_events_start_time ON request_events(start_time)`,
		`CREATE INDEX IF NOT EXISTS idx_tunnel_events_token ON tunnel_events(token)`,
		`CREATE INDEX IF NOT EXISTS idx_tunnel_events_timestamp ON tunnel_events(timestamp)`,
	}

	for _, query := range queries {
		if _, err := as.db.ExecContext(context.Background(), query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	return nil
}

// RecordRequestStart records the start of a request
func (as *AdminService) RecordRequestStart(ctx context.Context, token, method, uri, remoteAddr string) (int64, error) {
	// Validate inputs
	if token == "" || method == "" || uri == "" {
		return 0, fmt.Errorf("token, method, and uri are required")
	}
	if len(token) > 255 || len(method) > 10 || len(uri) > 2048 {
		return 0, fmt.Errorf("input exceeds maximum length")
	}

	as.mu.Lock()
	defer as.mu.Unlock()

	result, err := as.db.ExecContext(ctx, `
		INSERT INTO request_events (token, method, uri, remote_addr, status, start_time)
		VALUES (?, ?, ?, ?, ?, ?)
	`, token, method, uri, remoteAddr, "pending", time.Now())

	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// RecordRequestComplete records the completion of a request
func (as *AdminService) RecordRequestComplete(ctx context.Context, requestID int64, success bool, errorMsg string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	status := "completed"
	if !success {
		status = "errored"
	}

	result, err := as.db.ExecContext(ctx, `
		UPDATE request_events 
		SET status = ?, end_time = ?, error = ?
		WHERE id = ?
	`, status, time.Now(), errorMsg, requestID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("request ID %d not found", requestID)
	}

	return nil
}

// RecordTunnelEvent records a tunnel lifecycle event
func (as *AdminService) RecordTunnelEvent(ctx context.Context, token, event, remoteAddr, remoteName, remoteWhois, clientVersion, details string) error {
	// Validate inputs
	if token == "" || event == "" {
		return fmt.Errorf("token and event are required")
	}
	if len(token) > 255 || len(event) > 50 || len(details) > 1000 {
		return fmt.Errorf("input exceeds maximum length")
	}

	as.mu.Lock()
	defer as.mu.Unlock()

	_, err := as.db.ExecContext(ctx, `
		INSERT INTO tunnel_events (token, event, remote_addr, remote_name, remote_whois, client_version, timestamp, details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, token, event, remoteAddr, remoteName, remoteWhois, clientVersion, time.Now(), details)

	return err
}

// GetMonitoringStats returns monitoring statistics
func (as *AdminService) GetMonitoringStats(ctx context.Context) (*MonitoringResponse, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	// Get unique tunnels from server registry
	as.server.serverRegistryMutex.Lock()
	uniqueTunnels := len(as.server.serverRegistry)

	// Count active tunnel connections
	tunnelConnections := 0
	for _, rs := range as.server.serverRegistry {
		if time.Since(rs.lastActivity) < tunnelInactiveKillTimeout {
			tunnelConnections++
		}
	}
	as.server.serverRegistryMutex.Unlock()

	// Get request statistics from database
	var pendingRequests, completedRequests, erroredRequests int64

	if err := as.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM request_events WHERE status = 'pending'").Scan(&pendingRequests); err != nil {
		as.log.Error("Failed to get pending requests count", "err", err)
		return nil, fmt.Errorf("failed to get pending requests count: %w", err)
	}

	if err := as.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM request_events WHERE status = 'completed'").Scan(&completedRequests); err != nil {
		as.log.Error("Failed to get completed requests count", "err", err)
		return nil, fmt.Errorf("failed to get completed requests count: %w", err)
	}

	if err := as.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM request_events WHERE status = 'errored'").Scan(&erroredRequests); err != nil {
		as.log.Error("Failed to get errored requests count", "err", err)
		return nil, fmt.Errorf("failed to get errored requests count: %w", err)
	}

	return &MonitoringResponse{
		Timestamp:         time.Now(),
		UniqueTunnels:     uniqueTunnels,
		TunnelConnections: tunnelConnections,
		PendingRequests:   pendingRequests,
		CompletedRequests: completedRequests,
		ErroredRequests:   erroredRequests,
	}, nil
}

// GetAuditingData returns detailed auditing information
func (as *AdminService) GetAuditingData(ctx context.Context) (*AuditingResponse, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	as.server.serverRegistryMutex.Lock()
	tunnels := make(map[string]*TunnelDetail)

	for tokenStr, rs := range as.server.serverRegistry {
		remoteName, remoteWhois := rs.getRemoteInfo()
		clientVersion := rs.getClientVersion()

		// Get active connections
		rs.requestSetMutex.Lock()
		activeConnections := make([]*ConnectionDetail, 0, len(rs.requestSet))
		for _, req := range rs.requestSet {
			// Parse method and URI from info string (format: "METHOD URI")
			parts := strings.SplitN(req.info, " ", 2)
			method := "UNKNOWN"
			uri := req.info
			if len(parts) == 2 {
				method = parts[0]
				uri = parts[1]
			}

			activeConnections = append(activeConnections, &ConnectionDetail{
				RequestID:  req.id,
				Method:     method,
				URI:        uri,
				RemoteAddr: req.remoteAddr,
				StartTime:  req.startTime,
			})
		}
		rs.requestSetMutex.Unlock()

		// Get last error and success times from database
		var lastErrorTime, lastSuccessTime *time.Time
		var lastErrorAddr, lastSuccessAddr string

		// Get last error
		err := as.db.QueryRowContext(ctx, `
			SELECT start_time, remote_addr 
			FROM request_events 
			WHERE token = ? AND status = 'errored' 
			ORDER BY start_time DESC LIMIT 1
		`, string(tokenStr)).Scan(&lastErrorTime, &lastErrorAddr)
		if err != nil && err != sql.ErrNoRows {
			as.log.Error("Failed to query last error time", "token", cutToken(tokenStr), "err", err)
		}

		// Get last success
		err = as.db.QueryRowContext(ctx, `
			SELECT end_time, remote_addr 
			FROM request_events 
			WHERE token = ? AND status = 'completed' 
			ORDER BY end_time DESC LIMIT 1
		`, string(tokenStr)).Scan(&lastSuccessTime, &lastSuccessAddr)
		if err != nil && err != sql.ErrNoRows {
			as.log.Error("Failed to query last success time", "token", cutToken(tokenStr), "err", err)
		}

		tunnels[string(tokenStr)] = &TunnelDetail{
			Token:             string(tokenStr),
			RemoteAddr:        rs.remoteAddr,
			RemoteName:        remoteName,
			RemoteWhois:       remoteWhois,
			ClientVersion:     clientVersion,
			LastActivity:      rs.lastActivity,
			ActiveConnections: activeConnections,
			LastErrorTime:     lastErrorTime,
			LastErrorAddr:     lastErrorAddr,
			LastSuccessTime:   lastSuccessTime,
			LastSuccessAddr:   lastSuccessAddr,
			PendingRequests:   len(rs.requestSet),
		}
	}
	as.server.serverRegistryMutex.Unlock()

	return &AuditingResponse{
		Timestamp: time.Now(),
		Tunnels:   tunnels,
	}, nil
}

// cleanupOldRecords periodically cleans up old database records
func (as *AdminService) cleanupOldRecords() {
	defer as.wg.Done()
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-as.done:
			as.log.Debug("Stopping cleanup goroutine")
			return
		case <-ticker.C:
			// Clean up records older than 7 days
			cutoff := time.Now().AddDate(0, 0, -7)

			// Create a context with timeout for cleanup operations
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			as.mu.Lock()
			if _, err := as.db.ExecContext(ctx, "DELETE FROM request_events WHERE created_at < ?", cutoff); err != nil {
				as.log.Error("Failed to cleanup old request events", "err", err)
			}

			if _, err := as.db.ExecContext(ctx, "DELETE FROM tunnel_events WHERE created_at < ?", cutoff); err != nil {
				as.log.Error("Failed to cleanup old tunnel events", "err", err)
			}
			as.mu.Unlock()

			cancel() // Clean up the context
			as.log.Debug("Cleaned up old admin records", "cutoff", cutoff)
		}
	}
}

// HandleAuditing handles /admin/auditing requests
func (as *AdminService) HandleAuditing(w http.ResponseWriter, r *http.Request) {
	safeW := &safeResponseWriter{ResponseWriter: w}

	if r.Method != "GET" {
		safeError(safeW, "Only GET requests are supported", http.StatusMethodNotAllowed)
		return
	}

	response, err := as.GetAuditingData(r.Context())
	if err != nil {
		as.log.Error("Failed to get auditing data", "err", err)
		safeError(safeW, "Internal server error", http.StatusInternalServerError)
		return
	}

	safeW.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(safeW).Encode(response); err != nil {
		as.log.Error("Failed to encode auditing response", "err", err)
		safeError(safeW, "Internal server error", http.StatusInternalServerError)
	}
}

// HandleMonitoring handles /admin/monitoring requests
func (as *AdminService) HandleMonitoring(w http.ResponseWriter, r *http.Request) {
	safeW := &safeResponseWriter{ResponseWriter: w}

	if r.Method != "GET" {
		safeError(safeW, "Only GET requests are supported", http.StatusMethodNotAllowed)
		return
	}

	response, err := as.GetMonitoringStats(r.Context())
	if err != nil {
		as.log.Error("Failed to get monitoring stats", "err", err)
		safeError(safeW, "Internal server error", http.StatusInternalServerError)
		return
	}

	safeW.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(safeW).Encode(response); err != nil {
		as.log.Error("Failed to encode monitoring response", "err", err)
		safeError(safeW, "Internal server error", http.StatusInternalServerError)
	}
}

// GetAPIDocumentation returns API documentation for all admin endpoints
func (as *AdminService) GetAPIDocumentation() *APIDocsResponse {
	return &APIDocsResponse{
		Version: "1.0",
		Endpoints: []APIEndpoint{
			{
				Path:        "/admin/monitoring",
				Method:      "GET",
				Description: "Get high-level monitoring statistics for dashboards and alerting",
				Response: map[string]interface{}{
					"timestamp": map[string]string{
						"type":        "string",
						"format":      "datetime",
						"description": "Time when statistics were collected",
					},
					"unique_tunnels": map[string]string{
						"type":        "integer",
						"description": "Number of unique tunnel tokens currently registered",
					},
					"tunnel_connections": map[string]string{
						"type":        "integer",
						"description": "Number of active tunnel WebSocket connections",
					},
					"pending_requests": map[string]string{
						"type":        "integer",
						"description": "Number of requests currently waiting for response",
					},
					"completed_requests": map[string]string{
						"type":        "integer",
						"description": "Total number of successfully completed requests",
					},
					"errored_requests": map[string]string{
						"type":        "integer",
						"description": "Total number of requests that ended with errors",
					},
				},
			},
			{
				Path:        "/admin/auditing",
				Method:      "GET",
				Description: "Get detailed tunnel and connection information for security auditing and debugging",
				Response: map[string]interface{}{
					"timestamp": map[string]string{
						"type":        "string",
						"format":      "datetime",
						"description": "Time when audit data was collected",
					},
					"tunnels": map[string]interface{}{
						"type":        "object",
						"description": "Map of tunnel tokens to detailed tunnel information",
						"additionalProperties": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"token": map[string]string{
									"type":        "string",
									"description": "Tunnel authentication token",
								},
								"remote_addr": map[string]string{
									"type":        "string",
									"description": "IP address of the tunnel client",
								},
								"remote_name": map[string]string{
									"type":        "string",
									"description": "Hostname of the tunnel client (if available)",
								},
								"remote_whois": map[string]string{
									"type":        "string",
									"description": "WHOIS information for client IP (if available)",
								},
								"client_version": map[string]string{
									"type":        "string",
									"description": "Version of the tunnel client software",
								},
								"last_activity": map[string]string{
									"type":        "string",
									"format":      "datetime",
									"description": "Timestamp of last activity on this tunnel",
								},
								"active_connections": map[string]interface{}{
									"type":        "array",
									"description": "List of currently active HTTP requests through this tunnel",
									"items": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"request_id": map[string]string{
												"type":        "integer",
												"description": "Unique identifier for this request",
											},
											"method": map[string]string{
												"type":        "string",
												"description": "HTTP method (GET, POST, etc.)",
											},
											"uri": map[string]string{
												"type":        "string",
												"description": "Request URI path and query string",
											},
											"remote_addr": map[string]string{
												"type":        "string",
												"description": "IP address of the HTTP client",
											},
											"start_time": map[string]string{
												"type":        "string",
												"format":      "datetime",
												"description": "When this request started",
											},
										},
									},
								},
								"last_error_time": map[string]string{
									"type":        "string",
									"format":      "datetime",
									"description": "Timestamp of most recent error (if any)",
								},
								"last_error_addr": map[string]string{
									"type":        "string",
									"description": "IP address associated with most recent error",
								},
								"last_success_time": map[string]string{
									"type":        "string",
									"format":      "datetime",
									"description": "Timestamp of most recent successful request",
								},
								"last_success_addr": map[string]string{
									"type":        "string",
									"description": "IP address associated with most recent success",
								},
								"pending_requests": map[string]string{
									"type":        "integer",
									"description": "Number of requests currently pending for this tunnel",
								},
							},
						},
					},
				},
			},
			{
				Path:        "/admin/api-docs",
				Method:      "GET",
				Description: "Get API documentation for all admin endpoints in JSON format",
				Response: map[string]interface{}{
					"version": map[string]string{
						"type":        "string",
						"description": "API version number",
					},
					"endpoints": map[string]interface{}{
						"type":        "array",
						"description": "List of available API endpoints with their documentation",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"path": map[string]string{
									"type":        "string",
									"description": "URL path for this endpoint",
								},
								"method": map[string]string{
									"type":        "string",
									"description": "HTTP method (GET, POST, etc.)",
								},
								"description": map[string]string{
									"type":        "string",
									"description": "Human-readable description of endpoint purpose",
								},
								"response": map[string]string{
									"type":        "object",
									"description": "JSON schema describing the response format",
								},
							},
						},
					},
				},
			},
		},
	}
}

// HandleAPIDocs handles /admin/api-docs requests
func (as *AdminService) HandleAPIDocs(w http.ResponseWriter, r *http.Request) {
	safeW := &safeResponseWriter{ResponseWriter: w}

	if r.Method != "GET" {
		safeError(safeW, "Only GET requests are supported", http.StatusMethodNotAllowed)
		return
	}

	response := as.GetAPIDocumentation()

	safeW.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(safeW).Encode(response); err != nil {
		as.log.Error("Failed to encode API docs response", "err", err)
		safeError(safeW, "Internal server error", http.StatusInternalServerError)
	}
}
