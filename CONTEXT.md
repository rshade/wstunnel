# WStunnel Context & Boundaries

## Core Architectural Identity

WStunnel is a **stateless, HTTP-only reverse tunneling relay** built on
WebSockets. It enables access to HTTP services behind firewalls by
establishing persistent outbound WebSocket connections from a client
(wstuncli) behind the firewall to a publicly accessible server (wstunsrv).
The server then routes incoming HTTP requests through the appropriate
WebSocket tunnel using token-based addressing.

Unlike multi-protocol tools (frp, chisel, ngrok), WStunnel intentionally
constrains itself to HTTP traffic over WebSocket tunnels. This makes it
lightweight, auditable, and compatible with restrictive corporate network
environments where only HTTP/WebSocket traffic is permitted.

## Technical Boundaries ("Hard No's")

WStunnel does NOT and SHOULD NOT:

- **Tunnel raw TCP or UDP traffic** — HTTP protocol only; generic TCP/UDP
  is out of scope and would fundamentally change the architecture
- **Persist state to disk** — All tunnel state is ephemeral; the admin
  SQLite database runs in-memory (`:memory:`) with no durability guarantees
- **Manage TLS certificates** — TLS termination is delegated to a reverse
  proxy (nginx, envoy); wstunnel handles `ws://` or connects to `wss://`
  endpoints managed externally
- **Perform service discovery** — Clients must explicitly provide tokens
  and server addresses; there is no registry, DNS, or mDNS integration
- **Implement load balancing** — Routing is token-based first-match; no
  round-robin, weighted, or health-aware distribution
- **Provide user management or OAuth/OIDC** — Authentication is limited
  to token + optional password with constant-time comparison
- **Act as a general-purpose proxy** — It is a tunnel relay, not an HTTP
  proxy; it does not cache, transform, or inspect request content
- **Export metrics in Prometheus/OpenTelemetry format** — Admin endpoints
  return JSON; integration with monitoring stacks is left to the operator.
  *Under review: #273 proposes a Prometheus endpoint since the data already
  exists in JSON form*

## Data Source of Truth

- **Tunnel state**: In-memory `serverRegistry` map keyed by token
- **Admin metrics**: In-memory SQLite with 7-day retention auto-cleanup
- **Configuration**: Command-line flags only; no config files or
  environment variable parsing (except standard proxy env vars on client)
- **Client identity**: Token string (16+ characters) with optional
  password; no persistent identity beyond the current connection

## Interaction Model

### Inbound (Server)

| Interface           | Purpose                                  |
| ------------------- | ---------------------------------------- |
| `/_token/{token}/*` | HTTP requests to forward through tunnel  |
| `X-Token` header    | Alternative token routing via header     |
| `POST /_tunnel`     | WebSocket upgrade for client connections |
| `/_health_check`    | Liveness probe                           |
| `/_stats`           | Text-format operational statistics       |
| `/admin/*`          | Web dashboard and JSON API               |

### Outbound (Client)

| Interface                | Purpose                                           |
| ------------------------ | ------------------------------------------------- |
| `ws[s]://server/_tunnel` | Persistent WebSocket to server                    |
| `http[s]://target`       | Local HTTP requests dispatched per tunnel request |

### Protocol

- WebSocket binary messages: 4-hex-char request ID + HTTP payload
- 16-bit request ID space (max 65,536 concurrent requests per tunnel)
- Ping/pong keepalive with configurable timeout

## Verification

To check if a proposed feature violates these boundaries, ask:

1. **Does it require persisting state across restarts?** → Violates
   stateless design
2. **Does it handle non-HTTP traffic?** → Out of scope
3. **Does it require managing TLS certificates?** → Delegate to reverse
   proxy
4. **Does it add user management beyond token/password?** → Delegate to
   an auth proxy
5. **Does it require the server to initiate outbound connections?** →
   Violates reverse-tunnel model (clients always initiate)
