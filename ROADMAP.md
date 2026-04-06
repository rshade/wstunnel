# WStunnel Strategic Roadmap

## Vision

WStunnel is a lightweight, stateless HTTP reverse tunneling relay built on
WebSockets. It enables secure access to HTTP services behind firewalls with
minimal configuration and zero persistent state. See [CONTEXT.md](CONTEXT.md)
for architectural boundaries.

**LOE Key**: `[S]` = Small (1-2h), `[M]` = Medium (half day-1d),
`[L]` = Large (multi-day)

## Immediate Focus

Active development — observability foundation and documentation.

- [ ] #140 Disambiguate "tunnel server" vs "tunnel" parameter in docs [S]
- [ ] #272 Wire up admin request recording in request flow [S]
- [ ] #275 Add admin tunnel control API (force disconnect, token blocking) [M]

## Near-Term Vision

Feature enhancements that extend wstunnel's capabilities within its
HTTP-tunneling scope.

### Observability & Operations

- [ ] #274 Add Server-Sent Events endpoint for real-time tunnel events [M]
- [ ] #273 Add Prometheus metrics endpoint [M] — *Boundary discussion:
  relaxes "no metrics export" rule*

### Feature Enhancements

- [ ] #145 Support multiple tunnels in wstuncli [L]
- [ ] #138 Sending client certificates for authentication [M]
- [ ] #135 Make token optional for simpler deployments [M]
- [ ] #130 Support audio streams (chunked/streaming responses) [M]

## Future Vision (Long-Term)

Research items and feature requests that need design spikes or may push
architectural boundaries.

- [ ] #276 Webhook notifications for tunnel lifecycle events [M]
- [ ] #139 Redirect UDP as well [L] — *Boundary: violates HTTP-only scope;
  would require new protocol handler or delegation to external tool*
- [ ] #133 Tunnel WebSocket over WebSocket tunnel [L] — *Requires protocol
  changes to support upgrading tunneled connections*
- [ ] #127 Reverse connection documentation/clarification [S]

## Completed Milestones

### 2026-Q1

- [x] #277 Migrate logging from log15 to zerolog [L]
- [x] #147 Timeout of requests to \_token doesn't work [M]

### 2025-Q2

- [x] #146 Allow configuration of max number of requests/tunnel [M]
- [x] #144 HTTP requests for monitoring [M]
- [x] #143 HTTP request for auditing [M]
- [x] #142 Add wstuncli version to status on wstunsrv side [S]
- [x] #141 Release binary for 32-bit Linux [S]
- [x] #137 Add support for limiting clients per token [M]
- [x] #136 WStunnel on Android — answered [S]
- [x] #134 ARM binary builds [S]
- [x] #132 Add support for token authentication [M]
- [x] #131 Support TCP and UDP traffic — closed (HTTP-only scope) [S]
- [x] #129 Base path support for reverse proxy deployments [M]
- [x] #128 Dynamic IPv6 network compatibility — answered [S]
- [x] #126 Reverse connections clarification — duplicate of #127 [S]

### 2025-Q1

- [x] #67 Fix Renovate configuration [S]

## Boundary Safeguards

The following boundaries from [CONTEXT.md](CONTEXT.md) constrain this
roadmap:

- **No raw TCP/UDP tunneling** — #139 (UDP support) would violate this;
  consider recommending frp or chisel for those use cases
- **No persistent state** — Features must not require data to survive
  server restarts
- **No TLS certificate management** — Delegate to reverse proxy (nginx,
  envoy)
- **No service discovery** — Tokens are the addressing mechanism
- **No user management beyond token/password** — Delegate to auth proxy
