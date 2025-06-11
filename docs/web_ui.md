# WStunnel Admin Web UI Implementation Plan

## Overview
This document outlines the implementation plan for a lightweight web frontend for WStunnel's admin API endpoints. The goal is to create a simple, maintainable SPA that stays in sync with the backend API automatically.

## Architecture Decisions

### Core Principles
1. **Zero Build Process**: No Node.js, npm, webpack, or build tools required
2. **Embedded in Binary**: HTML/CSS/JS served directly from the Go application
3. **Auto-Sync**: UI updates automatically when API changes through code generation
4. **Lightweight**: Total size < 50KB (excluding optional CDN libraries)
5. **Single File**: Everything in one HTML file for simplicity

### Technology Stack
- **Backend**: Go with embedded assets
- **Frontend**: Vanilla JavaScript (ES6+)
- **Styling**: Inline CSS or minimal external CSS
- **Charts**: Chart.js (optional, via CDN)
- **Icons**: Inline SVGs

## Implementation Steps

### Phase 1: API Documentation Endpoint
1. Add `/admin/api-docs` endpoint that returns JSON schema
2. Schema includes:
   - Available endpoints
   - Request/response formats
   - Field descriptions
   - Data types
3. Generate schema from Go structs using reflection

### Phase 2: Backend Changes
1. Add static file serving handler at `/admin/ui`
2. Embed HTML file using `embed` package
3. Add CORS headers if needed
4. Ensure proper Content-Type headers

### Phase 3: Frontend Development
1. Create single HTML file with:
   - Inline CSS for styling
   - Inline JavaScript for functionality
   - No external dependencies (except optional CDN)
2. Features:
   - Real-time monitoring dashboard
   - Tunnel details view
   - Search/filter capabilities
   - Auto-refresh with configurable interval
   - Responsive design

### Phase 4: Auto-Generation Tool
1. Create `cmd/generate-admin-ui/main.go`
2. Tool reads admin API structs
3. Generates:
   - API documentation JSON
   - TypeScript-like interfaces for frontend
   - Update HTML file with latest API structure
4. Hook into `go generate`

### Phase 5: Testing & Documentation
1. Add tests for new endpoints
2. Update CLAUDE.md with UI information
3. Update docs/ADMIN_API.md with UI access instructions
4. Add examples to README.md

## File Structure
```
wstunnel/
├── tunnel/
│   ├── admin_service.go          # Existing admin service
│   ├── admin_ui.go              # New: UI serving handler
│   └── admin_api_docs.go        # New: API documentation endpoint
├── cmd/
│   └── generate-admin-ui/       # New: Generation tool
│       └── main.go
├── web/
│   └── admin.html               # New: Single-file SPA
└── docs/
    └── web_ui.md                # This file
```

## API Documentation Format
```json
{
  "version": "1.0",
  "endpoints": [
    {
      "path": "/admin/monitoring",
      "method": "GET",
      "description": "Get high-level monitoring statistics",
      "response": {
        "timestamp": {"type": "string", "format": "datetime"},
        "unique_tunnels": {"type": "integer", "description": "Number of unique tunnel tokens"},
        "tunnel_connections": {"type": "integer", "description": "Active tunnel connections"},
        "pending_requests": {"type": "integer", "description": "Requests waiting for response"},
        "completed_requests": {"type": "integer", "description": "Total completed requests"},
        "errored_requests": {"type": "integer", "description": "Total errored requests"}
      }
    },
    {
      "path": "/admin/auditing",
      "method": "GET",
      "description": "Get detailed tunnel and connection information",
      "response": {
        "timestamp": {"type": "string", "format": "datetime"},
        "tunnels": {
          "type": "object",
          "additionalProperties": {
            "type": "object",
            "properties": {
              "token": {"type": "string"},
              "remote_addr": {"type": "string"},
              "remote_name": {"type": "string"},
              "remote_whois": {"type": "string"},
              "client_version": {"type": "string"},
              "last_activity": {"type": "string", "format": "datetime"},
              "active_connections": {"type": "array"},
              "pending_requests": {"type": "integer"}
            }
          }
        }
      }
    }
  ]
}
```

## UI Mockup Structure
```html
<!DOCTYPE html>
<html>
<head>
    <title>WStunnel Admin</title>
    <style>
        /* Minimal CSS for dark theme terminal look */
        body { 
            background: #1a1a1a; 
            color: #e0e0e0; 
            font-family: monospace; 
            margin: 0;
            padding: 20px;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        .metric { 
            background: #2a2a2a; 
            padding: 15px; 
            margin: 10px; 
            border-radius: 5px; 
            display: inline-block;
        }
        .metric-value { font-size: 2em; color: #4fc3f7; }
        .metric-label { color: #888; }
        table { width: 100%; border-collapse: collapse; margin-top: 20px; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #333; }
        th { background: #2a2a2a; }
        tr:hover { background: #2a2a2a; }
        .status-active { color: #4caf50; }
        .status-error { color: #f44336; }
        .search-box { 
            width: 100%; 
            padding: 10px; 
            background: #2a2a2a; 
            border: 1px solid #444; 
            color: #e0e0e0;
            margin-bottom: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>WStunnel Admin Dashboard</h1>
        
        <!-- Monitoring Metrics -->
        <div id="metrics">
            <div class="metric">
                <div class="metric-value" id="unique_tunnels">-</div>
                <div class="metric-label">Active Tunnels</div>
            </div>
            <div class="metric">
                <div class="metric-value" id="tunnel_connections">-</div>
                <div class="metric-label">Connections</div>
            </div>
            <div class="metric">
                <div class="metric-value" id="pending_requests">-</div>
                <div class="metric-label">Pending</div>
            </div>
            <div class="metric">
                <div class="metric-value" id="completed_requests">-</div>
                <div class="metric-label">Completed</div>
            </div>
            <div class="metric">
                <div class="metric-value" id="errored_requests">-</div>
                <div class="metric-label">Errors</div>
            </div>
        </div>
        
        <!-- Search -->
        <input type="text" class="search-box" id="search" placeholder="Search by token, IP, or hostname...">
        
        <!-- Tunnels Table -->
        <table id="tunnels">
            <thead>
                <tr>
                    <th>Token</th>
                    <th>Remote Address</th>
                    <th>Hostname</th>
                    <th>Version</th>
                    <th>Connections</th>
                    <th>Pending</th>
                    <th>Last Activity</th>
                    <th>Status</th>
                </tr>
            </thead>
            <tbody id="tunnels-body">
                <!-- Populated by JavaScript -->
            </tbody>
        </table>
    </div>
    
    <script>
        // Auto-refresh every 5 seconds
        const REFRESH_INTERVAL = 5000;
        
        async function fetchData() {
            try {
                // Fetch monitoring data
                const monitoringResponse = await fetch('/admin/monitoring');
                const monitoring = await monitoringResponse.json();
                
                // Update metrics
                Object.keys(monitoring).forEach(key => {
                    const element = document.getElementById(key);
                    if (element) {
                        element.textContent = monitoring[key].toLocaleString();
                    }
                });
                
                // Fetch auditing data
                const auditingResponse = await fetch('/admin/auditing');
                const auditing = await auditingResponse.json();
                
                // Update tunnels table
                updateTunnelsTable(auditing.tunnels);
            } catch (error) {
                console.error('Failed to fetch data:', error);
            }
        }
        
        function updateTunnelsTable(tunnels) {
            const tbody = document.getElementById('tunnels-body');
            const searchTerm = document.getElementById('search').value.toLowerCase();
            
            tbody.innerHTML = '';
            
            Object.entries(tunnels).forEach(([token, tunnel]) => {
                // Filter based on search
                if (searchTerm && !matchesSearch(tunnel, searchTerm)) {
                    return;
                }
                
                const row = tbody.insertRow();
                row.innerHTML = `
                    <td>${tunnel.token.substring(0, 8)}...</td>
                    <td>${tunnel.remote_addr}</td>
                    <td>${tunnel.remote_name || '-'}</td>
                    <td>${tunnel.client_version || '-'}</td>
                    <td>${tunnel.active_connections.length}</td>
                    <td>${tunnel.pending_requests}</td>
                    <td>${formatTime(tunnel.last_activity)}</td>
                    <td class="${getStatusClass(tunnel)}">${getStatus(tunnel)}</td>
                `;
            });
        }
        
        function matchesSearch(tunnel, searchTerm) {
            return tunnel.token.toLowerCase().includes(searchTerm) ||
                   tunnel.remote_addr.toLowerCase().includes(searchTerm) ||
                   (tunnel.remote_name && tunnel.remote_name.toLowerCase().includes(searchTerm));
        }
        
        function formatTime(timestamp) {
            const date = new Date(timestamp);
            const now = new Date();
            const diffSeconds = Math.floor((now - date) / 1000);
            
            if (diffSeconds < 60) return `${diffSeconds}s ago`;
            if (diffSeconds < 3600) return `${Math.floor(diffSeconds / 60)}m ago`;
            if (diffSeconds < 86400) return `${Math.floor(diffSeconds / 3600)}h ago`;
            return `${Math.floor(diffSeconds / 86400)}d ago`;
        }
        
        function getStatus(tunnel) {
            const lastActivity = new Date(tunnel.last_activity);
            const now = new Date();
            const diffMinutes = (now - lastActivity) / 60000;
            
            if (tunnel.last_error_time) {
                const lastError = new Date(tunnel.last_error_time);
                if (lastError > lastActivity) return 'Error';
            }
            
            if (diffMinutes < 1) return 'Active';
            if (diffMinutes < 5) return 'Idle';
            return 'Inactive';
        }
        
        function getStatusClass(tunnel) {
            const status = getStatus(tunnel);
            if (status === 'Active') return 'status-active';
            if (status === 'Error') return 'status-error';
            return '';
        }
        
        // Initial load
        fetchData();
        
        // Auto-refresh
        setInterval(fetchData, REFRESH_INTERVAL);
        
        // Search functionality
        document.getElementById('search').addEventListener('input', fetchData);
    </script>
</body>
</html>
```

## Security Considerations
1. **Access Control**: Admin UI should respect same access controls as API endpoints
2. **XSS Prevention**: All dynamic content must be properly escaped
3. **CORS**: Only allow same-origin requests
4. **Authentication**: Integrate with existing auth mechanisms
5. **Read-Only**: UI provides no mutation capabilities

## Future Enhancements
1. **WebSocket Support**: Real-time updates without polling
2. **Export Options**: CSV/JSON export of current data
3. **Historical Views**: Show trends over time
4. **Alert Configuration**: Set thresholds for notifications
5. **Mobile App**: Progressive Web App capabilities

## Success Criteria
1. UI loads in < 100ms
2. Updates reflect API changes without manual intervention
3. Works on all modern browsers (Chrome, Firefox, Safari, Edge)
4. Accessible without external network (no required CDN)
5. Total bundle size < 50KB

## Maintenance
1. Run `go generate` after API changes
2. Test UI with `make test`
3. Update documentation when adding features
4. Keep JavaScript simple and readable