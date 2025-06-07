# Proxy Configuration Examples for WStunnel Base Path

This document provides configuration examples for running WStunnel behind various reverse proxies using the `-base-path` option.

## Overview

When deploying WStunnel in containerized environments or behind reverse proxies, you often need to host it under a specific path prefix. The `-base-path` option allows WStunnel to work correctly in these scenarios by:

1. Configuring all endpoints under the specified base path
2. Automatically stripping the base path from incoming requests
3. Ensuring proper routing for WebSocket upgrades and HTTP requests

## Envoy Proxy

### Basic Configuration

```yaml
static_resources:
  listeners:
  - name: listener_0
    address:
      socket_address:
        protocol: TCP
        address: 0.0.0.0
        port_value: 8080
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains: ["*"]
              routes:
              - match:
                  prefix: "/wstunnel/"
                route:
                  cluster: wstunnel_service
                  prefix_rewrite: "/"
              - match:
                  prefix: "/wstunnel"
                route:
                  cluster: wstunnel_service
                  prefix_rewrite: "/"
          http_filters:
          - name: envoy.filters.http.router
          upgrade_configs:
          - upgrade_type: websocket

  clusters:
  - name: wstunnel_service
    connect_timeout: 30s
    type: STRICT_DNS
    dns_lookup_family: V4_ONLY
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: wstunnel_service
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: wstunnel-server
                port_value: 8080
```

### WStunnel Server Configuration

```bash
./wstunnel srv -port 8080 -base-path /wstunnel
```

### Usage Examples

```bash
# Health check
curl http://proxy.example.com:8080/wstunnel/_health_check

# Stats endpoint
curl http://proxy.example.com:8080/wstunnel/_stats

# WebSocket tunnel connection
./wstunnel cli -tunnel ws://proxy.example.com:8080/wstunnel -server http://localhost -token 'your-token'

# Token-based HTTP request
curl 'http://proxy.example.com:8080/wstunnel/_token/your-token/api/endpoint'
```

## Istio Ingress Gateway

### Gateway Configuration

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: wstunnel-gateway
  namespace: wstunnel
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 80
      name: http
      protocol: HTTP
    hosts:
    - tunnel.example.com
  - port:
      number: 443
      name: https
      protocol: HTTPS
    tls:
      mode: SIMPLE
      credentialName: tunnel-tls-secret
    hosts:
    - tunnel.example.com
```

### VirtualService Configuration

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: wstunnel-vs
  namespace: wstunnel
spec:
  hosts:
  - tunnel.example.com
  gateways:
  - wstunnel-gateway
  http:
  - match:
    - uri:
        prefix: /api/v1/wstunnel/
    rewrite:
      uri: /
    route:
    - destination:
        host: wstunnel-service
        port:
          number: 8080
  - match:
    - uri:
        exact: /api/v1/wstunnel
    rewrite:
      uri: /
    route:
    - destination:
        host: wstunnel-service
        port:
          number: 8080
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wstunnel-server
  namespace: wstunnel
spec:
  replicas: 2
  selector:
    matchLabels:
      app: wstunnel-server
  template:
    metadata:
      labels:
        app: wstunnel-server
    spec:
      containers:
      - name: wstunnel
        image: wstunnel:latest
        args:
        - "srv"
        - "-port"
        - "8080"
        - "-base-path"
        - "/api/v1/wstunnel"
        - "-max-requests-per-tunnel"
        - "50"
        - "-max-clients-per-token"
        - "10"
        ports:
        - containerPort: 8080
          name: http
        livenessProbe:
          httpGet:
            path: /api/v1/wstunnel/_health_check
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /api/v1/wstunnel/_health_check
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: wstunnel-service
  namespace: wstunnel
spec:
  selector:
    app: wstunnel-server
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  type: ClusterIP
```

### Usage Examples

```bash
# Health check
curl https://tunnel.example.com/api/v1/wstunnel/_health_check

# WebSocket tunnel connection
./wstunnel cli -tunnel wss://tunnel.example.com/api/v1/wstunnel -server http://localhost -token 'your-token'

# Token-based HTTPS request
curl 'https://tunnel.example.com/api/v1/wstunnel/_token/your-token/api/endpoint'
```

## Nginx

### Configuration

```nginx
upstream wstunnel_backend {
    server wstunnel-server:8080;
}

server {
    listen 80;
    listen 443 ssl;
    server_name tunnel.example.com;

    ssl_certificate /etc/ssl/certs/tunnel.crt;
    ssl_certificate_key /etc/ssl/private/tunnel.key;

    # WStunnel with base path
    location /tunnel/ {
        proxy_pass http://wstunnel_backend/;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # Standard proxy headers
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Disable buffering for real-time applications
        proxy_buffering off;
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }

    # Exact match for base path without trailing slash
    location = /tunnel {
        proxy_pass http://wstunnel_backend/;
        
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_buffering off;
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }
}
```

### WStunnel Server Configuration

```bash
./wstunnel srv -port 8080 -base-path /tunnel
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/conf.d/default.conf
      - ./ssl:/etc/ssl
    depends_on:
      - wstunnel

  wstunnel:
    image: wstunnel:latest
    command:
      - "srv"
      - "-port"
      - "8080"
      - "-base-path"
      - "/tunnel"
      - "-max-requests-per-tunnel"
      - "30"
    ports:
      - "8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/tunnel/_health_check"]
      interval: 30s
      timeout: 10s
      retries: 3
```

## HAProxy

### Configuration

```haproxy
global
    daemon
    maxconn 4096

defaults
    mode http
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms

frontend wstunnel_frontend
    bind *:80
    bind *:443 ssl crt /etc/ssl/certs/tunnel.pem

    # Route requests with /ws-tunnel prefix to WStunnel
    acl is_wstunnel path_beg /ws-tunnel
    use_backend wstunnel_backend if is_wstunnel

    # Default backend for other requests
    default_backend default_backend

backend wstunnel_backend
    # Strip the /ws-tunnel prefix before forwarding
    http-request replace-path /ws-tunnel(/.*) \1
    http-request replace-path /ws-tunnel$ /
    
    # WebSocket support
    option http-server-close
    option forwardfor
    
    server wstunnel1 wstunnel-server:8080 check
    server wstunnel2 wstunnel-server2:8080 check backup

backend default_backend
    server web1 web-server:80 check
```

### WStunnel Server Configuration

```bash
./wstunnel srv -port 8080 -base-path /ws-tunnel
```

## Apache HTTP Server

### Configuration

```apache
<VirtualHost *:80>
    ServerName tunnel.example.com
    
    # Enable modules
    LoadModule proxy_module modules/mod_proxy.so
    LoadModule proxy_http_module modules/mod_proxy_http.so
    LoadModule proxy_wstunnel_module modules/mod_proxy_wstunnel.so
    LoadModule rewrite_module modules/mod_rewrite.so

    # Proxy WebSocket requests
    ProxyPreserveHost On
    ProxyRequests Off

    # Handle WebSocket upgrade
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} websocket [NC]
    RewriteCond %{HTTP:Connection} upgrade [NC]
    RewriteRule ^/api/tunnel/(.*)$ ws://wstunnel-server:8080/$1 [P,L]

    # Handle regular HTTP requests
    ProxyPass /api/tunnel/ http://wstunnel-server:8080/
    ProxyPassReverse /api/tunnel/ http://wstunnel-server:8080/
    
    # Handle exact match without trailing slash
    ProxyPass /api/tunnel http://wstunnel-server:8080/
    ProxyPassReverse /api/tunnel http://wstunnel-server:8080/
</VirtualHost>
```

### WStunnel Server Configuration

```bash
./wstunnel srv -port 8080 -base-path /api/tunnel
```

## Kubernetes Ingress (nginx-ingress)

### Ingress Configuration

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: wstunnel-ingress
  namespace: wstunnel
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /$2
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    nginx.ingress.kubernetes.io/websocket-services: "wstunnel-service"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - tunnel.example.com
    secretName: tunnel-tls
  rules:
  - host: tunnel.example.com
    http:
      paths:
      - path: /v1/wstunnel(/|$)(.*)
        pathType: ImplementationSpecific
        backend:
          service:
            name: wstunnel-service
            port:
              number: 8080
```

### WStunnel Server Configuration

```bash
./wstunnel srv -port 8080 -base-path /v1/wstunnel
```

## Troubleshooting

### Common Issues

1. **WebSocket connections fail**: Ensure your proxy supports WebSocket upgrades and has appropriate timeout settings.

2. **Base path not stripped**: Verify that your proxy is correctly rewriting paths before forwarding to WStunnel.

3. **Health checks fail**: Make sure health check paths include the base path (e.g., `/wstunnel/_health_check`).

4. **SSL/TLS issues**: When using HTTPS, ensure proper certificate configuration and that the proxy forwards the correct protocol headers.

### Debugging

1. **Check WStunnel logs** for base path configuration:
   ```
   INFO Base path configured basePath=/your-path
   ```

2. **Test endpoints individually**:
   ```bash
   # Health check
   curl -v http://proxy.example.com/your-path/_health_check
   
   # Stats
   curl -v http://proxy.example.com/your-path/_stats
   ```

3. **Verify WebSocket upgrade headers**:
   ```bash
   curl -H "Connection: Upgrade" -H "Upgrade: websocket" \
        -v ws://proxy.example.com/your-path/_tunnel
   ```

4. **Check proxy access logs** for correct path rewriting.

### Performance Considerations

- Set appropriate timeout values for long-lived WebSocket connections
- Use connection pooling when available
- Configure proper buffering settings for real-time applications
- Monitor resource usage and scale accordingly
- Consider using multiple WStunnel instances behind a load balancer for high availability

## Security Considerations

- Always use HTTPS/WSS in production environments
- Implement proper authentication and authorization at the proxy level
- Use secure headers (HSTS, CSP, etc.) when terminating SSL at the proxy
- Regularly update proxy software and security configurations
- Monitor and log access for security analysis
- Consider rate limiting and DDoS protection at the proxy level