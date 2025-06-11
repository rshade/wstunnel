# WStunnel Admin API Operations Guide

This document provides operational guidance for using WStunnel's admin API endpoints for monitoring, auditing, and troubleshooting.

## Overview

WStunnel provides admin API endpoints that return JSON data:

- `/admin/monitoring` - High-level statistics for dashboards and alerting
- `/admin/auditing` - Detailed tunnel and connection information for security and debugging
- `/admin/api-docs` - JSON schema documentation for all admin endpoints
- `/admin/ui` - Web-based admin dashboard interface
- `/admin` - Redirects to `/admin/ui` for convenience

These endpoints use SQLite for persistent data storage, automatically cleaning up records older than 7 days.

## Web Admin Interface

WStunnel includes a built-in web interface for monitoring and managing tunnels:

### Accessing the Admin UI

- **URL**: `http://localhost:8080/admin/ui` (or your configured host/port)
- **With Base Path**: If using `-base-path /wstunnel`, access via `http://localhost:8080/wstunnel/admin/ui`
- **Shortcut**: Navigate to `/admin` and you'll be automatically redirected to the UI

### Admin UI Features

- **Real-time Dashboard**: Auto-refreshing metrics with configurable intervals (5s, 10s, 30s, 1m, or manual)
- **Tunnel Overview**: View all active tunnels with detailed connection information
- **Search & Filter**: Find specific tunnels by token, IP address, or hostname
- **Connection Status**: Monitor tunnel health with color-coded status indicators
- **Responsive Design**: Works on desktop and mobile devices
- **Dark Theme**: Terminal-inspired interface for reduced eye strain

### UI Status Indicators

- **🟢 Active**: Tunnel has activity within the last minute
- **🟠 Idle**: Tunnel active within last 5 minutes but not recently
- **⚪ Inactive**: Tunnel hasn't been used for over 5 minutes
- **🔴 Error**: Recent errors detected on the tunnel

### Configuration

The admin UI automatically detects your base path configuration and adapts its API calls accordingly. No additional configuration is required beyond starting the WStunnel server.

## API Documentation

### Self-Documenting API

WStunnel provides a self-documenting API endpoint that returns JSON schema information for all admin endpoints:

```bash
curl http://localhost:8080/admin/api-docs | jq
```

This endpoint returns:

- **Version**: API version information
- **Endpoints**: Complete list of available endpoints with descriptions
- **Response Schemas**: Detailed field descriptions and data types for each endpoint

### Example API Documentation Response

```json
{
  "version": "1.0",
  "endpoints": [
    {
      "path": "/admin/monitoring",
      "method": "GET",
      "description": "Get high-level monitoring statistics for dashboards and alerting",
      "response": {
        "timestamp": {
          "type": "string",
          "format": "datetime",
          "description": "Time when statistics were collected"
        },
        "unique_tunnels": {
          "type": "integer",
          "description": "Number of unique tunnel tokens currently registered"
        }
      }
    }
  ]
}
```

### Integration Benefits

- **Automated Client Generation**: Use the schema to generate API clients
- **Validation**: Validate responses against the documented schema
- **Documentation**: Always up-to-date API documentation
- **Tooling**: Build custom monitoring tools with complete API knowledge

## Operational Use Cases

### 1. Monitoring Dashboard Integration

#### Prometheus/Grafana Integration

Create a simple exporter script to poll the monitoring endpoint:

```bash
#!/bin/bash
# wstunnel-exporter.sh
curl -s http://localhost:8080/admin/monitoring | jq -r '
  "wstunnel_unique_tunnels " + (.unique_tunnels | tostring),
  "wstunnel_active_connections " + (.tunnel_connections | tostring),
  "wstunnel_pending_requests " + (.pending_requests | tostring),
  "wstunnel_completed_requests " + (.completed_requests | tostring),
  "wstunnel_errored_requests " + (.errored_requests | tostring)
'
```

#### Basic Health Check

```bash
#!/bin/bash
# health-check.sh
RESPONSE=$(curl -s -w "%{http_code}" http://localhost:8080/admin/monitoring)
HTTP_CODE=${RESPONSE: -3}
BODY=${RESPONSE%???}

if [ "$HTTP_CODE" -eq 200 ]; then
    TUNNELS=$(echo "$BODY" | jq -r '.unique_tunnels')
    if [ "$TUNNELS" -gt 0 ]; then
        echo "OK: $TUNNELS tunnels active"
        exit 0
    else
        echo "WARNING: No active tunnels"
        exit 1
    fi
else
    echo "CRITICAL: Admin API not responding (HTTP $HTTP_CODE)"
    exit 2
fi
```

### 2. Security Auditing

#### Daily Security Report

```bash
#!/bin/bash
# daily-security-audit.sh
DATE=$(date +%Y-%m-%d)
curl -s http://localhost:8080/admin/auditing | jq -r "
.tunnels | to_entries[] | 
\"$DATE,\" + .key + \",\" + .value.remote_addr + \",\" + .value.remote_name + \",\" + (.value.active_connections | length | tostring)
" > "/var/log/wstunnel/audit-$DATE.csv"

echo "Date,Token,RemoteAddr,RemoteName,ActiveConnections" > "/var/log/wstunnel/audit-$DATE.csv"
curl -s http://localhost:8080/admin/auditing | jq -r '
.tunnels | to_entries[] | 
[
  (.value.last_activity | split("T")[0]),
  .key,
  .value.remote_addr,
  .value.remote_name,
  (.value.active_connections | length)
] | @csv
' >> "/var/log/wstunnel/audit-$DATE.csv"
```

#### Detect Suspicious Activity

```bash
#!/bin/bash
# detect-suspicious.sh
curl -s http://localhost:8080/admin/auditing | jq -r '
.tunnels | to_entries[] | select(
  (.value.active_connections | length) > 10 or
  (.value.pending_requests > 5) or
  (.value.remote_name == "") or
  (.value.last_error_time != null and (.value.last_error_time | fromdateiso8601) > (now - 300))
) | 
"ALERT: Suspicious activity on token " + .key + " from " + .value.remote_addr
'
```

### 3. Performance Monitoring

#### Track Request Performance

```python
#!/usr/bin/env python3
# performance-monitor.py
import requests
import json
import time
from datetime import datetime

def collect_metrics():
    try:
        response = requests.get('http://localhost:8080/admin/monitoring', timeout=5)
        data = response.json()
        
        timestamp = datetime.now().isoformat()
        metrics = {
            'timestamp': timestamp,
            'unique_tunnels': data['unique_tunnels'],
            'tunnel_connections': data['tunnel_connections'],
            'pending_requests': data['pending_requests'],
            'completed_requests': data['completed_requests'],
            'errored_requests': data['errored_requests'],
            'error_rate': data['errored_requests'] / max(data['completed_requests'], 1)
        }
        
        # Log to file or send to monitoring system
        with open('/var/log/wstunnel/metrics.jsonl', 'a') as f:
            f.write(json.dumps(metrics) + '\n')
            
        # Alert on high error rate
        if metrics['error_rate'] > 0.05:  # 5% error rate
            print(f"HIGH ERROR RATE: {metrics['error_rate']:.2%}")
            
    except Exception as e:
        print(f"Failed to collect metrics: {e}")

if __name__ == '__main__':
    collect_metrics()
```

### 4. Troubleshooting

#### Debug Connection Issues

```bash
#!/bin/bash
# debug-connections.sh
TOKEN="$1"
if [ -z "$TOKEN" ]; then
    echo "Usage: $0 <token>"
    exit 1
fi

curl -s http://localhost:8080/admin/auditing | jq -r --arg token "$TOKEN" '
.tunnels[$token] | 
if . == null then
  "Token not found"
else
  "Token: " + .token,
  "Remote: " + .remote_addr + " (" + .remote_name + ")",
  "Client Version: " + .client_version,
  "Last Activity: " + .last_activity,
  "Pending Requests: " + (.pending_requests | tostring),
  "Active Connections: " + (.active_connections | length | tostring),
  "",
  "Recent Errors:",
  (if .last_error_time then "  " + .last_error_time + " from " + .last_error_addr else "  None" end),
  "",
  "Recent Success:",
  (if .last_success_time then "  " + .last_success_time + " from " + .last_success_addr else "  None" end),
  "",
  "Active Connections:",
  (.active_connections[] | "  " + (.request_id | tostring) + ": " + .method + " " + .uri + " from " + .remote_addr + " (started: " + .start_time + ")")
end
'
```

#### Find Long-Running Requests

```bash
#!/bin/bash
# find-long-requests.sh
curl -s http://localhost:8080/admin/auditing | jq -r '
.tunnels | to_entries[] |
.value.active_connections[] |
select((now - (.start_time | fromdateiso8601)) > 300) |
"Long-running request: " + (.request_id | tostring) + " " + .method + " " + .uri + 
" from " + .remote_addr + " (running for " + ((now - (.start_time | fromdateiso8601)) | tostring) + "s)"
'
```

### 5. Capacity Planning

#### Generate Usage Report

```bash
#!/bin/bash
# usage-report.sh
WEEK_START=$(date -d 'last Monday' +%Y-%m-%d)
echo "WStunnel Usage Report - Week of $WEEK_START"
echo "================================================"

DATA=$(curl -s http://localhost:8080/admin/monitoring)
echo "Current Status:"
echo "$DATA" | jq -r '
"  Active Tunnels: " + (.unique_tunnels | tostring),
"  Active Connections: " + (.tunnel_connections | tostring),
"  Pending Requests: " + (.pending_requests | tostring),
"  Total Completed: " + (.completed_requests | tostring),
"  Total Errors: " + (.errored_requests | tostring),
"  Success Rate: " + ((.completed_requests / (.completed_requests + .errored_requests) * 100) | floor | tostring) + "%"
'

echo ""
echo "Top Active Tunnels:"
curl -s http://localhost:8080/admin/auditing | jq -r '
.tunnels | to_entries | sort_by(.value.active_connections | length) | reverse | .[0:5][] |
"  " + .key + ": " + (.value.active_connections | length | tostring) + " connections from " + .value.remote_addr
'
```

## Database Management

### Manual Database Operations

The admin service uses SQLite with automatic cleanup, but you can manually query the database:

```bash
# Find the database file (usually in memory or temp directory)
sqlite3 /path/to/admin.db

# View request statistics
.mode column
.headers on
SELECT 
  token,
  COUNT(*) as total_requests,
  SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed,
  SUM(CASE WHEN status = 'errored' THEN 1 ELSE 0 END) as errored,
  AVG(julianday(end_time) - julianday(start_time)) * 86400 as avg_duration_seconds
FROM request_events 
WHERE start_time > datetime('now', '-24 hours')
GROUP BY token;

# View tunnel events
SELECT 
  token,
  event,
  timestamp,
  remote_addr,
  details
FROM tunnel_events 
WHERE timestamp > datetime('now', '-24 hours')
ORDER BY timestamp DESC;
```

### Backup and Archival

```bash
#!/bin/bash
# backup-admin-data.sh
DATE=$(date +%Y%m%d)
BACKUP_DIR="/var/backups/wstunnel"
mkdir -p "$BACKUP_DIR"

# If using persistent database
if [ -f "/var/lib/wstunnel/admin.db" ]; then
    cp "/var/lib/wstunnel/admin.db" "$BACKUP_DIR/admin-$DATE.db"
    gzip "$BACKUP_DIR/admin-$DATE.db"
fi

# Keep backups for 30 days
find "$BACKUP_DIR" -name "admin-*.db.gz" -mtime +30 -delete
```

## Alerting Rules

### Recommended Alert Thresholds

1. **No Active Tunnels**: `unique_tunnels == 0` (Critical)
2. **High Error Rate**: `errored_requests / completed_requests > 0.05` (Warning)
3. **High Pending Requests**: `pending_requests > 100` (Warning)
4. **Tunnel Connection Loss**: `tunnel_connections < unique_tunnels` (Warning)
5. **Long-Running Requests**: Active connection `start_time > 5 minutes ago` (Info)

### Sample Alertmanager Rules

```yaml
groups:
- name: wstunnel
  rules:
  - alert: WSTunnelNoActiveTunnels
    expr: wstunnel_unique_tunnels == 0
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "No active WStunnel tunnels"
      description: "WStunnel server has no active tunnels for {{ $labels.instance }}"

  - alert: WSTunnelHighErrorRate
    expr: rate(wstunnel_errored_requests[5m]) / rate(wstunnel_completed_requests[5m]) > 0.05
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High WStunnel error rate"
      description: "WStunnel error rate is {{ $value | humanizePercentage }} for {{ $labels.instance }}"
```

## Security Considerations

1. **Network Access**: Restrict admin endpoints to localhost or trusted networks
2. **Reverse Proxy**: Place admin endpoints behind authentication when exposing externally
3. **Log Monitoring**: Monitor admin endpoint access logs for unauthorized usage
4. **Data Retention**: Ensure sensitive data in SQLite database is properly secured
5. **Rate Limiting**: Consider rate limiting admin endpoint access

## Troubleshooting Common Issues

### Admin Endpoints Not Responding

1. Check if WStunnel server is running: `ps aux | grep wstunnel`
2. Verify port binding: `netstat -tlnp | grep :8080`
3. Check server logs for admin service initialization errors
4. Test basic connectivity: `curl -v http://localhost:8080/admin/monitoring`

### SQLite Database Issues

1. Check disk space: `df -h`
2. Verify file permissions on database directory
3. Check for database locks: `lsof | grep admin.db`
4. Review server logs for SQLite error messages

### Performance Issues

1. Monitor response times: `time curl http://localhost:8080/admin/monitoring`
2. Check database size and cleanup frequency
3. Monitor system resources (CPU, memory, I/O)
4. Consider implementing caching for frequently accessed data
