// Copyright (c) 2014 RightScale, Inc. - see LICENSE

package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"

	// imported per documentation - https://golang.org/pkg/net/http/pprof/
	_ "net/http/pprof"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rshade/wstunnel/whois"
)

// ErrRetry Error when sending request
var ErrRetry = errors.New("error sending request, please retry")

// pkgLog is used by functions that don't have access to an instance logger.
var pkgLog = zerolog.New(os.Stderr).With().Timestamp().Str("pkg", "tunnel").Logger()

const tunnelInactiveKillTimeout = 60 * time.Minute // close dead tunnels

//===== Data Structures =====

const (
	//defaultMaxReq default max queued requests per remote server
	defaultMaxReq = 20
	//minTokenLen min number of chars in a token
	minTokenLen = 16
	// httpStatusParseError is returned when the tunneled response cannot be parsed
	httpStatusParseError = 506
)

type token string

type responseBuffer struct {
	err      error
	response io.Reader
}

// A request for a remote server
type remoteRequest struct {
	id         int16         // unique (scope=server) request id
	info       string        // http method + uri for debug/logging
	remoteAddr string        // remote address for debug/logging
	buffer     *bytes.Buffer // request buffer to send
	replyChan  chan responseBuffer
	deadline   time.Time // timeout
	startTime  time.Time // when the request started
	log        zerolog.Logger
}

// A remote server
type remoteServer struct {
	token           token                    // rendez-vous token for debug/logging
	lastID          int16                    // id of last request
	lastActivity    time.Time                // last activity on tunnel
	remoteAddr      string                   // last remote addr of tunnel (debug)
	remoteName      string                   // reverse DNS resolution of remoteAddr
	remoteWhois     string                   // whois lookup of remoteAddr
	clientVersion   string                   // version of the connected client
	infoMutex       sync.RWMutex             // mutex to protect remoteName, remoteWhois, clientVersion
	requestQueue    chan *remoteRequest      // queue of requests to be sent
	requestSet      map[int16]*remoteRequest // all requests in queue/flight indexed by ID
	requestSetMutex sync.Mutex
	log             zerolog.Logger
	readMutex       sync.Mutex      // ensure that no more than one goroutine calls the websocket read methods concurrently
	readCond        *sync.Cond      // (NextReader, SetReadDeadline, SetPingHandler, ...)
	connCloseChans  []chan struct{} // close channels for each active WebSocket connection
	connCloseMutex  sync.Mutex      // protects connCloseChans
}

// setClientVersion safely sets the client version
func (rs *remoteServer) setClientVersion(version string) {
	rs.infoMutex.Lock()
	defer rs.infoMutex.Unlock()
	rs.clientVersion = version
}

// getClientVersion safely gets the client version
func (rs *remoteServer) getClientVersion() string {
	rs.infoMutex.RLock()
	defer rs.infoMutex.RUnlock()
	return rs.clientVersion
}

// setRemoteInfo safely sets the remote name and whois information
func (rs *remoteServer) setRemoteInfo(name, whois string) {
	rs.infoMutex.Lock()
	defer rs.infoMutex.Unlock()
	rs.remoteName = name
	rs.remoteWhois = whois
}

// getRemoteInfo safely gets the remote name and whois information
func (rs *remoteServer) getRemoteInfo() (name, whois string) {
	rs.infoMutex.RLock()
	defer rs.infoMutex.RUnlock()
	return rs.remoteName, rs.remoteWhois
}

// RegisterConnection adds a close channel for a new WebSocket connection.
// Returns a deregister function that must be called on disconnect.
func (rs *remoteServer) RegisterConnection(closeCh chan struct{}) func() {
	rs.connCloseMutex.Lock()
	rs.connCloseChans = append(rs.connCloseChans, closeCh)
	rs.connCloseMutex.Unlock()

	return func() {
		rs.connCloseMutex.Lock()
		defer rs.connCloseMutex.Unlock()
		for i, ch := range rs.connCloseChans {
			if ch == closeCh {
				rs.connCloseChans = append(rs.connCloseChans[:i], rs.connCloseChans[i+1:]...)
				break
			}
		}
	}
}

// CloseAllConnections signals all active WebSocket connections to close gracefully.
// Returns the number of connections signaled.
func (rs *remoteServer) CloseAllConnections() int {
	rs.connCloseMutex.Lock()
	defer rs.connCloseMutex.Unlock()
	count := len(rs.connCloseChans)
	for _, ch := range rs.connCloseChans {
		select {
		case <-ch:
			// already closed
		default:
			close(ch)
		}
	}
	rs.connCloseChans = nil
	return count
}

// WSTunnelServer a wstunnel server construct
type WSTunnelServer struct {
	Port                 int                     // port to listen on
	Host                 string                  // host to listen on
	BasePath             string                  // base path for routing (e.g., "/wstunnel")
	WSTimeout            time.Duration           // timeout on websockets
	HTTPTimeout          time.Duration           // timeout for HTTP requests
	MaxRequestsPerTunnel int                     // max queued requests per tunnel
	MaxClientsPerToken   int                     // max clients allowed per token
	Log                  zerolog.Logger          // logger with "pkg=WStunsrv"
	exitChan             chan struct{}           // channel to tell the tunnel goroutines to end
	serverRegistry       map[token]*remoteServer // active remote servers indexed by token
	serverRegistryMutex  sync.Mutex              // mutex to protect map
	tokenPasswords       map[token]string        // optional passwords for tokens
	tokenPasswordsMutex  sync.RWMutex            // mutex to protect password map
	tokenClients         map[token]int           // track number of clients per token
	tokenClientsMutex    sync.RWMutex            // mutex to protect client count map
	adminService         *AdminService           // admin service for monitoring and auditing
	adminServiceMutex    sync.RWMutex            // mutex to protect admin service access
	blockedTokens        map[token]time.Time     // in-memory blocklist of tokens with block timestamp
	blockedTokensMutex   sync.RWMutex            // mutex to protect blockedTokens
}

func (t *WSTunnelServer) getAdminService() *AdminService {
	t.adminServiceMutex.RLock()
	defer t.adminServiceMutex.RUnlock()
	return t.adminService
}

// IsTokenBlocked checks if a token is in the blocklist
func (t *WSTunnelServer) IsTokenBlocked(tok token) bool {
	t.blockedTokensMutex.RLock()
	defer t.blockedTokensMutex.RUnlock()
	_, blocked := t.blockedTokens[tok]
	return blocked
}

// BlockToken adds a token to the in-memory blocklist
func (t *WSTunnelServer) BlockToken(tok token) {
	t.blockedTokensMutex.Lock()
	defer t.blockedTokensMutex.Unlock()
	t.blockedTokens[tok] = time.Now()
}

// UnblockToken removes a token from the blocklist. Returns true if the token was blocked.
func (t *WSTunnelServer) UnblockToken(tok token) bool {
	t.blockedTokensMutex.Lock()
	defer t.blockedTokensMutex.Unlock()
	_, existed := t.blockedTokens[tok]
	delete(t.blockedTokens, tok)
	return existed
}

// GetBlockedTokens returns a copy of the blocked tokens map
func (t *WSTunnelServer) GetBlockedTokens() map[token]time.Time {
	t.blockedTokensMutex.RLock()
	defer t.blockedTokensMutex.RUnlock()
	result := make(map[token]time.Time, len(t.blockedTokens))
	for k, v := range t.blockedTokens {
		result[k] = v
	}
	return result
}

// DisconnectToken force-disconnects all WebSocket connections for a token.
// Returns the number of connections disconnected.
func (t *WSTunnelServer) DisconnectToken(tok token) int {
	rs := t.getRemoteServer(tok, false)
	if rs == nil {
		return 0
	}
	return rs.CloseAllConnections()
}

// name Lookups
var whoToken string                      // token for the whois service
var dnsCache = make(map[string]string)   // ip_address -> reverse DNS lookup
var whoisCache = make(map[string]string) // ip_address -> whois lookup
var cacheMutex sync.Mutex

func ipAddrLookup(log zerolog.Logger, ipAddr string) (dns, who string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	dns, ok := dnsCache[ipAddr]
	if !ok {
		names, _ := net.LookupAddr(ipAddr)
		dns = strings.Join(names, ",")
		dnsCache[ipAddr] = dns
		log.Info().Str("addr", ipAddr).Str("dns", dns).Msg("DNS lookup")
	}
	// whois lookup
	who, ok = whoisCache[ipAddr]
	if !ok && whoToken != "" {
		who = whois.Whois(ipAddr, whoToken)
		whoisCache[ipAddr] = who
	}
	return
}

// validateLimit validates and adjusts configuration limits
func validateLimit(logger zerolog.Logger, name string, value, min, max, defaultValue int) int {
	if value < min {
		logger.Info().Str("param", name).Int("value", value).Int("min", min).Int("default", defaultValue).Msg("Configuration limit below minimum, using default")
		return defaultValue
	}
	if value > max {
		logger.Info().Str("param", name).Int("value", value).Int("max", max).Msg("Configuration limit above maximum, using maximum")
		return max
	}
	if value > 100 {
		logger.Info().Str("param", name).Int("value", value).Str("recommended", "10-100").Msg("Configuration limit is high")
	}
	return value
}

//===== Main =====

// NewWSTunnelServer function to create wstunnel from cli
func NewWSTunnelServer(args []string) *WSTunnelServer {
	wstunSrv := WSTunnelServer{}

	var srvFlag = flag.NewFlagSet("server", flag.ExitOnError)
	srvFlag.IntVar(&wstunSrv.Port, "port", 80, "port for http/ws server to listen on")
	srvFlag.StringVar(&wstunSrv.Host, "host", "0.0.0.0", "host for http/ws server to listen on")
	srvFlag.StringVar(&wstunSrv.BasePath, "base-path", "", "base path for routing when behind proxy (e.g., '/wstunnel')")
	var pidf = srvFlag.String("pidfile", "", "path for pidfile")
	var logf = srvFlag.String("logfile", "", "path for log file")
	var tout = srvFlag.Int("wstimeout", 30, "timeout on websocket in seconds")
	var httpTout = srvFlag.Int("httptimeout", 20*60, "timeout for http requests in seconds")
	var slog = srvFlag.String("syslog", "", "syslog facility to log to")
	var whoTok = srvFlag.String("robowhois", "", "robowhois.com API token")
	var tokenPass = srvFlag.String("passwords", "", "comma-separated list of token:password pairs")
	srvFlag.IntVar(&wstunSrv.MaxRequestsPerTunnel, "max-requests-per-tunnel", defaultMaxReq, "maximum number of queued requests per tunnel (recommended: 10-100, max: 10000)")
	srvFlag.IntVar(&wstunSrv.MaxClientsPerToken, "max-clients-per-token", 0, "maximum number of clients per token (0 for unlimited, recommended: 10-100, max: 10000)")
	var logLevel = srvFlag.String("log-level", "info", "log level (debug, info, warn, error)")
	var logPrettyFlag = srvFlag.Bool("log-pretty", false, "use human-readable console log output")

	if err := srvFlag.Parse(args); err != nil {
		bootstrapLog := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
		bootstrapLog.Fatal().Err(err).Msg("Failed to parse server flags")
	}

	LogPretty = *logPrettyFlag
	wstunSrv.Log = makeLogger("WStunsrv", *logf, *slog)
	pkgLog = newPkgLogger()
	if err := writePid(*pidf); err != nil {
		wstunSrv.Log.Error().Err(err).Msg("Failed to write pidfile")
	}

	level, err := zerolog.ParseLevel(*logLevel)
	if err != nil {
		wstunSrv.Log.Warn().Str("level", *logLevel).Msg("Invalid log level, using info")
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Normalize base path
	wstunSrv.BasePath = normalizeBasePath(wstunSrv.BasePath)
	if wstunSrv.BasePath != "" {
		wstunSrv.Log.Info().Str("basePath", wstunSrv.BasePath).Msg("Base path configured")
	}

	// Validate MaxRequestsPerTunnel
	if wstunSrv.MaxRequestsPerTunnel < 0 {
		wstunSrv.Log.Error().Int("value", wstunSrv.MaxRequestsPerTunnel).Int("default", defaultMaxReq).Msg("max-requests-per-tunnel cannot be negative, using default")
		wstunSrv.MaxRequestsPerTunnel = defaultMaxReq
	} else if wstunSrv.MaxRequestsPerTunnel == 0 {
		// Treat 0 as unlimited by falling back to the default buffer size
		wstunSrv.Log.Warn().Int("default", defaultMaxReq).Msg("max-requests-per-tunnel set to 0 – interpreting as unlimited (using default buffer size)")
		wstunSrv.MaxRequestsPerTunnel = defaultMaxReq
	} else if wstunSrv.MaxRequestsPerTunnel > 1000 {
		wstunSrv.Log.Warn().Int("value", wstunSrv.MaxRequestsPerTunnel).Str("recommended", "10-100 for typical use cases").Msg("max-requests-per-tunnel is very high, may cause resource issues")
	}

	// Validate MaxClientsPerToken
	if wstunSrv.MaxClientsPerToken < 0 {
		wstunSrv.Log.Error().Int("value", wstunSrv.MaxClientsPerToken).Msg("max-clients-per-token cannot be negative, disabling limit")
		wstunSrv.MaxClientsPerToken = 0
	} else if wstunSrv.MaxClientsPerToken > 1000 {
		wstunSrv.Log.Warn().Int("value", wstunSrv.MaxClientsPerToken).Msg("max-clients-per-token is very high, may cause resource issues")
	}
	wstunSrv.WSTimeout = calcWsTimeout(wstunSrv.Log, *tout)
	cacheMutex.Lock()
	whoToken = *whoTok
	cacheMutex.Unlock()

	wstunSrv.HTTPTimeout = time.Duration(*httpTout) * time.Second
	wstunSrv.Log.Info().Dur("timeout", wstunSrv.HTTPTimeout).Msg("Setting remote request timeout")

	wstunSrv.exitChan = make(chan struct{}, 1)

	// Initialize token client count map
	wstunSrv.tokenClients = make(map[token]int)

	// Initialize token blocklist
	wstunSrv.blockedTokens = make(map[token]time.Time)

	// Parse token passwords
	wstunSrv.tokenPasswords = make(map[token]string)
	if *tokenPass != "" {
		pairs := strings.Split(*tokenPass, ",")
		for i, pair := range pairs {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) == 2 {
				tok := token(strings.TrimSpace(parts[0]))
				pass := strings.TrimSpace(parts[1])

				// Validate empty token
				if tok == "" {
					wstunSrv.Log.Error().Int("index", i).Msg("Empty token in token:password pair")
					continue
				}

				// Validate empty password
				if pass == "" {
					wstunSrv.Log.Error().Str("token", cutToken(tok)).Msg("Empty password for token")
					continue
				}

				// Enforce minimum token length
				if len(tok) < minTokenLen {
					wstunSrv.Log.Error().Str("token", cutToken(tok)).Int("minLength", minTokenLen).Msg("Token too short")
					continue
				}

				// Check for duplicate tokens
				if _, exists := wstunSrv.tokenPasswords[tok]; exists {
					wstunSrv.Log.Warn().Str("token", cutToken(tok)).Msg("Duplicate token found, overwriting previous entry")
				}

				wstunSrv.tokenPasswords[tok] = pass
				wstunSrv.Log.Info().Str("token", cutToken(tok)).Msg("Token password configured")
			} else {
				wstunSrv.Log.Warn().Int("index", i).Msg("Invalid token:password pair (missing ':')")
			}
		}
	}

	return &wstunSrv
}

// Start wstunnel server start
func (t *WSTunnelServer) Start(listener net.Listener) {
	t.Log.Info().Str("version", VV).Msg("WStunnel server starting")
	if t.serverRegistry != nil {
		return // already started...
	}
	t.serverRegistry = make(map[token]*remoteServer)

	// Initialize admin service if not already set
	t.adminServiceMutex.Lock()
	if t.adminService == nil {
		adminService, err := NewAdminService(t, ":memory:")
		if err != nil {
			t.Log.Error().Err(err).Msg("Failed to create admin service")
		} else {
			t.adminService = adminService
		}
	}
	t.adminServiceMutex.Unlock()

	go t.idleTunnelReaper()

	//===== HTTP Server =====

	var httpServer http.Server

	// Convert a handler that takes a tunnel as first arg to a std http handler
	wrap := func(h func(t *WSTunnelServer, w http.ResponseWriter, r *http.Request)) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			// Recover from panics to prevent server crashes
			defer func() {
				if err := recover(); err != nil {
					t.Log.Error().Interface("error", err).Str("path", r.URL.Path).Msg("Panic in HTTP handler")
					// Try to send error response if possible
					safeError(w, "Internal server error", http.StatusInternalServerError)
				}
			}()

			// Strip base path from request URL if configured
			if t.BasePath != "" && shouldStripBasePath(r.URL.Path, t.BasePath) {
				// Create a new URL with the base path stripped
				newPath := strings.TrimPrefix(r.URL.Path, t.BasePath)
				if newPath == "" {
					newPath = "/"
				}
				r.URL.Path = newPath
			}
			h(t, w, r)
		}
	}

	// Register handlers with default mux
	httpMux := http.NewServeMux()
	httpMux.HandleFunc(buildPath(t.BasePath, "/"), wrap(payloadHeaderHandler))
	httpMux.HandleFunc(buildPath(t.BasePath, "/_token/"), wrap(payloadPrefixHandler))
	httpMux.HandleFunc(buildPath(t.BasePath, "/_tunnel"), wrap(tunnelHandler))
	httpMux.HandleFunc(buildPath(t.BasePath, "/_health_check"), wrap(checkHandler))
	httpMux.HandleFunc(buildPath(t.BasePath, "/_stats"), wrap(statsHandler))
	// Register admin endpoints
	t.adminServiceMutex.RLock()
	if t.adminService != nil {
		httpMux.HandleFunc(buildPath(t.BasePath, "/admin/auditing"), t.adminService.HandleAuditing)
		httpMux.HandleFunc(buildPath(t.BasePath, "/admin/monitoring"), t.adminService.HandleMonitoring)
		httpMux.HandleFunc(buildPath(t.BasePath, "/admin/api-docs"), t.adminService.HandleAPIDocs)
		httpMux.HandleFunc(buildPath(t.BasePath, "/admin/ui"), t.adminService.HandleAdminUI)
		// Tunnel control endpoints: ServeMux matches the most specific (longest)
		// pattern, so /admin/tokens/blocked is preferred over /admin/tokens/
		// regardless of registration order.
		httpMux.HandleFunc(buildPath(t.BasePath, "/admin/tunnels/"), t.adminService.HandleTunnelDisconnect)
		httpMux.HandleFunc(buildPath(t.BasePath, "/admin/tokens/blocked"), t.adminService.HandleBlockedTokens)
		httpMux.HandleFunc(buildPath(t.BasePath, "/admin/tokens/"), t.adminService.HandleTokenBlock)
		httpMux.HandleFunc(buildPath(t.BasePath, "/admin"), t.adminService.HandleAdminUIRedirect)
	}
	t.adminServiceMutex.RUnlock()
	httpServer.Handler = httpMux

	// Read/Write timeouts disabled for now due to bug:
	// https://code.google.com/p/go/issues/detail?id=6410
	// https://groups.google.com/forum/#!topic/golang-nuts/oBIh_R7-pJQ
	//ReadTimeout: time.Duration(cliTout) * time.Second, // read and idle timeout
	//WriteTimeout: time.Duration(cliTout) * time.Second, // timeout while writing response

	// Now create the listener and hook it all up
	if listener == nil {
		t.Log.Info().Str("host", t.Host).Int("port", t.Port).Msg("Listening")
		laddr := fmt.Sprintf("%s:%d", t.Host, t.Port)
		var err error
		listener, err = net.Listen("tcp", laddr)
		if err != nil {
			t.Log.Fatal().Str("addr", laddr).Msg("Cannot listen")
		}
	} else {
		t.Log.Info().Str("addr", listener.Addr().String()).Msg("Listener")
	}
	go func() {
		t.Log.Debug().Msg("Server started")
		if err := httpServer.Serve(listener); err != nil {
			t.Log.Error().Err(err).Msg("HTTP server error")
		}
		t.Log.Debug().Msg("Server ended")
	}()

	go func() {
		<-t.exitChan
		if err := listener.Close(); err != nil {
			t.Log.Error().Err(err).Msg("Failed to close listener")
		}
	}()
}

// Stop wstunnelserver stop
func (t *WSTunnelServer) Stop() {
	t.adminServiceMutex.RLock()
	if t.adminService != nil {
		if err := t.adminService.Close(); err != nil {
			t.Log.Error().Err(err).Msg("failed to close admin service")
		}
	}
	t.adminServiceMutex.RUnlock()
	t.exitChan <- struct{}{}
}

//===== Handlers =====

// Handler for health check
func checkHandler(t *WSTunnelServer, w http.ResponseWriter, r *http.Request) {
	// Wrap the response writer with our safe wrapper
	safeW := &safeResponseWriter{ResponseWriter: w}

	if _, err := fmt.Fprintln(safeW, "WSTUNSRV RUNNING"); err != nil {
		t.Log.Error().Err(err).Msg("Failed to write response")
	}
}

// Handler for stats
func statsHandler(t *WSTunnelServer, w http.ResponseWriter, r *http.Request) {
	// Wrap the response writer with our safe wrapper
	safeW := &safeResponseWriter{ResponseWriter: w}

	// let's start by doing a GC to ensure we reclaim file descriptors (?)
	runtime.GC()

	// make a copy of the set of remoteServers
	t.serverRegistryMutex.Lock()
	rss := make([]*remoteServer, 0, len(t.serverRegistry))
	for _, rs := range t.serverRegistry {
		rss = append(rss, rs)
	}
	// print out the number of tunnels
	if _, err := fmt.Fprintf(safeW, "tunnels=%d\n", len(t.serverRegistry)); err != nil {
		t.Log.Error().Err(err).Msg("Failed to write response")
	}
	t.serverRegistryMutex.Unlock()

	// print configuration limits
	if _, err := fmt.Fprintf(safeW, "max_requests_per_tunnel=%d\n", t.MaxRequestsPerTunnel); err != nil {
		t.Log.Error().Err(err).Msg("Failed to write response")
	}
	if _, err := fmt.Fprintf(safeW, "max_clients_per_token=%d\n", t.MaxClientsPerToken); err != nil {
		t.Log.Error().Err(err).Msg("Failed to write response")
	}

	// print current token client counts
	if t.MaxClientsPerToken > 0 {
		t.tokenClientsMutex.RLock()
		totalClients := 0
		for tok, count := range t.tokenClients {
			if _, err := fmt.Fprintf(safeW, "token_clients_%s=%d\n", cutToken(tok), count); err != nil {
				t.Log.Error().Err(err).Msg("Failed to write response")
			}
			totalClients += count
		}
		t.tokenClientsMutex.RUnlock()
		if _, err := fmt.Fprintf(safeW, "total_clients=%d\n", totalClients); err != nil {
			t.Log.Error().Err(err).Msg("Failed to write response")
		}
	}

	// cut off here if not called from localhost
	addr := r.Header.Get("X-Forwarded-For")
	if addr == "" {
		addr = r.RemoteAddr
	}
	if !strings.HasPrefix(addr, "127.0.0.1") {
		if _, err := fmt.Fprintln(safeW, "More stats available when called from localhost..."); err != nil {
			t.Log.Error().Err(err).Msg("Failed to write response")
		}
		return
	}

	reqPending := 0
	badTunnels := 0
	for i, rs := range rss {
		if _, err := fmt.Fprintf(safeW, "\ntunnel%02d_token=%s\n", i, cutToken(rs.token)); err != nil {
			rs.log.Error().Err(err).Msg("Failed to write response")
		}
		if _, err := fmt.Fprintf(safeW, "tunnel%02d_req_pending=%d\n", i, len(rs.requestSet)); err != nil {
			rs.log.Error().Err(err).Msg("Failed to write response")
		}
		reqPending += len(rs.requestSet)
		if _, err := fmt.Fprintf(safeW, "tunnel%02d_tun_addr=%s\n", i, rs.remoteAddr); err != nil {
			rs.log.Error().Err(err).Msg("Failed to write response")
		}
		remoteName, remoteWhois := rs.getRemoteInfo()
		if remoteName != "" {
			if _, err := fmt.Fprintf(safeW, "tunnel%02d_tun_dns=%s\n", i, remoteName); err != nil {
				rs.log.Error().Err(err).Msg("Failed to write response")
			}
		}
		if remoteWhois != "" {
			if _, err := fmt.Fprintf(safeW, "tunnel%02d_tun_whois=%s\n", i, remoteWhois); err != nil {
				rs.log.Error().Err(err).Msg("Failed to write response")
			}
		}
		clientVersion := rs.getClientVersion()
		if clientVersion != "" {
			if _, err := fmt.Fprintf(safeW, "tunnel%02d_client_version=%s\n", i, clientVersion); err != nil {
				rs.log.Error().Err(err).Msg("Failed to write response")
			}
		}
		if rs.lastActivity.IsZero() {
			if _, err := fmt.Fprintf(safeW, "tunnel%02d_idle_secs=NaN\n", i); err != nil {
				rs.log.Error().Err(err).Msg("Failed to write response")
			}
			badTunnels++
		} else {
			if _, err := fmt.Fprintf(safeW, "tunnel%02d_idle_secs=%.1f\n", i, time.Since(rs.lastActivity).Seconds()); err != nil {
				rs.log.Error().Err(err).Msg("Failed to write response")
			}
			if time.Since(rs.lastActivity).Seconds() > 60 {
				badTunnels++
			}
		}
		if len(rs.requestSet) > 0 {
			rs.requestSetMutex.Lock()
			if r, ok := rs.requestSet[rs.lastID]; ok {
				if _, err := fmt.Fprintf(safeW, "tunnel%02d_cli_addr=%s\n", i, r.remoteAddr); err != nil {
					rs.log.Error().Err(err).Msg("Failed to write response")
				}
			}
			rs.requestSetMutex.Unlock()
		}
	}
	if _, err := fmt.Fprintln(safeW, ""); err != nil {
		t.Log.Error().Err(err).Msg("Failed to write response")
	}
	if _, err := fmt.Fprintf(safeW, "req_pending=%d\n", reqPending); err != nil {
		t.Log.Error().Err(err).Msg("Failed to write response")
	}
	if _, err := fmt.Fprintf(safeW, "dead_tunnels=%d\n", badTunnels); err != nil {
		t.Log.Error().Err(err).Msg("Failed to write response")
	}
}

// payloadHeaderHandler handles payload requests with the tunnel token in the Host header.
// Payload requests are requests that are to be forwarded through the tunnel.
func payloadHeaderHandler(t *WSTunnelServer, w http.ResponseWriter, r *http.Request) {
	// Wrap the response writer with our safe wrapper
	safeW := &safeResponseWriter{ResponseWriter: w}

	tok := r.Header.Get("X-Token")
	if tok == "" {
		t.Log.Info().Str("path", r.URL.Path).Msg("HTTP Missing X-Token header")
		safeError(safeW, "Missing X-Token header", 400)
		return
	}
	payloadHandler(t, safeW, r, token(tok))
}

// Regexp for extracting the tunnel token from the URI
var matchToken = regexp.MustCompile("^/_token/([^/]+)(/.*)")

// payloadPrefixHandler handles payload requests with the tunnel token in a URI prefix.
// Payload requests are requests that are to be forwarded through the tunnel.
func payloadPrefixHandler(t *WSTunnelServer, w http.ResponseWriter, r *http.Request) {
	// Wrap the response writer with our safe wrapper
	safeW := &safeResponseWriter{ResponseWriter: w}

	reqURL := r.URL.String()
	m := matchToken.FindStringSubmatch(reqURL)
	if len(m) != 3 {
		t.Log.Info().Str("url", reqURL).Msg("HTTP Missing token or URI")
		safeError(safeW, "Missing token in URI", 400)
		return
	}

	// Parse the extracted URL and handle errors
	parsedURL, err := url.Parse(m[2])
	if err != nil {
		t.Log.Info().Str("url", m[2]).Err(err).Msg("HTTP Invalid URI format")
		safeError(safeW, "Invalid URI format", 400)
		return
	}
	r.URL = parsedURL
	payloadHandler(t, safeW, r, token(m[1]))
}

// payloadHandler is called by payloadHeaderHandler and payloadPrefixHandler to do the real work.
func payloadHandler(t *WSTunnelServer, w http.ResponseWriter, r *http.Request, tok token) {
	safeW, ok := w.(*safeResponseWriter)
	if !ok {
		safeW = &safeResponseWriter{ResponseWriter: w}
	}

	// create the request object
	req := makeRequest(r, t.HTTPTimeout)
	req.log = t.Log.With().Str("token", cutToken(tok)).Logger()

	req.remoteAddr = r.Header.Get("X-Forwarded-For")
	if req.remoteAddr == "" {
		req.remoteAddr = r.RemoteAddr
	}

	as := t.getAdminService()

	var requestID int64
	if as != nil {
		id, err := as.RecordRequestStart(r.Context(), string(tok), r.Method, r.URL.String(), req.remoteAddr)
		if err != nil {
			t.Log.Warn().Err(err).Msg("Failed to record request start")
		} else {
			requestID = id
		}
	}

	// repeatedly try to get a response
	for tries := 1; tries <= 3; tries++ {
		retry := getResponse(t, req, safeW, r, tok, tries)
		if !retry {
			break
		}
		if tries == 3 {
			safeError(safeW, "Tunnel retry exhausted", http.StatusGatewayTimeout)
		}
	}

	if requestID > 0 && as != nil {
		var success bool
		var errMsg string
		switch {
		case safeW.statusCode >= 200 && safeW.statusCode < 400:
			success = true
		case safeW.statusCode > 0:
			errMsg = fmt.Sprintf("HTTP %d", safeW.statusCode)
		default:
			errMsg = "no response written"
		}
		if err := as.RecordRequestComplete(context.Background(), requestID, success, errMsg); err != nil {
			t.Log.Warn().Err(err).Msg("Failed to record request complete")
		}
	}
}

// getResponse adds the request to a remote server and then waits to get a response back, and it
// writes it. It returns true if the whole thing needs to be retried and false if we're done
// sucessfully or not)
func getResponse(t *WSTunnelServer, req *remoteRequest, w http.ResponseWriter, r *http.Request,
	tok token, tries int) (retry bool) {
	retry = false

	// get a hold of the remote server
	rs := t.getRemoteServer(token(tok), false)
	if rs == nil {
		req.log.Info().Str("addr", req.remoteAddr).Str("status", "404").Str("err", "Tunnel not found").Msg("HTTP RCV")
		// Set headers before calling WriteHeader to avoid superfluous warning
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(404)
		_, _ = w.Write([]byte("Tunnel not found (or not seen in a long time)"))
		return
	}

	// Ensure we retire the request when we pop out of this function
	// and signal the tunnel reader to continue.
	// The signal is sent in a goroutine because wsReader holds readCond.L
	// while reading from the WebSocket. Blocking here would prevent the
	// HTTP response from being flushed to the client.
	defer func() {
		rs.RetireRequest(req)
		if !retry {
			go func() {
				rs.readCond.L.Lock()
				rs.readCond.Signal()
				rs.readCond.L.Unlock()
			}()
		}
	}()

	// enqueue request
	err := rs.AddRequest(req)
	if err != nil {
		req.log.Info().Str("addr", req.remoteAddr).Str("status", "504").Str("err", err.Error()).Msg("HTTP RCV")
		safeError(w, err.Error(), http.StatusGatewayTimeout)
		return
	}
	try := ""
	if tries > 1 {
		try = fmt.Sprintf("(attempt #%d)", tries)
	}
	req.log.Info().Str("verb", r.Method).Str("url", r.URL.String()).Str("addr", req.remoteAddr).Str("x-host", r.Header.Get("X-Host")).Str("try", try).Msg("HTTP RCV")

	// Calculate timeout based on request deadline
	timeoutRemaining := time.Until(req.deadline)
	if timeoutRemaining <= 0 {
		// Already past deadline
		req.log.Info().Str("status", "504").Str("err", "Request deadline already expired").Msg("HTTP RET")
		safeError(w, "Request deadline already expired", http.StatusGatewayTimeout)
		return
	}

	// wait for response
	timer := time.NewTimer(timeoutRemaining)
	defer timer.Stop()

	select {
	case resp := <-req.replyChan:
		timer.Stop()
		// if there's no error just respond
		if resp.err == nil {
			code := writeResponse(w, resp.response)
			req.log.Info().Int("status", code).Msg("HTTP RET")
			return
		}
		// if it's a non-retryable error then write the error
		if resp.err != ErrRetry {
			req.log.Info().Str("status", "504").Str("err", resp.err.Error()).Msg("HTTP RET")
			safeError(w, resp.err.Error(), http.StatusGatewayTimeout)
		} else {
			// else we're gonna retry
			req.log.Info().Str("verb", r.Method).Str("url", r.URL.String()).Msg("WS   retrying")
			retry = true
		}
	case <-timer.C:
		// it timed out...
		req.log.Info().Str("status", "504").Str("err", "Tunnel timeout").Msg("HTTP RET")
		safeError(w, "Tunnel timeout", http.StatusGatewayTimeout)
	case <-r.Context().Done():
		// client disconnected before we got a response
		req.log.Info().Str("status", "499").Str("err", "Client disconnected").Msg("HTTP RET")
	}
	return
}

// tunnelHandler handles tunnel establishment requests
func tunnelHandler(t *WSTunnelServer, w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Pass the original response writer for websocket upgrade
		wsHandler(t, w, r)
	} else {
		// Wrap the response writer with our safe wrapper for error responses
		safeW := &safeResponseWriter{ResponseWriter: w}
		safeError(safeW, "Only GET requests are supported", 400)
	}
}

//===== Helpers =====

// normalizeBasePath normalizes a base path for HTTP routing
// Ensures proper leading slash and removes trailing slash (except for root)
// Also validates against path traversal attempts
func normalizeBasePath(basePath string) string {
	// Handle empty string
	if basePath == "" {
		return ""
	}

	// Trim whitespace
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return ""
	}

	// Validate maximum length (prevent excessive memory usage)
	const maxBasePathLength = 256
	if len(basePath) > maxBasePathLength {
		pkgLog.Warn().Str("basePath", basePath).Int("maxLength", maxBasePathLength).Msg("Base path exceeds maximum length, ignoring")
		return ""
	}

	// Check for path traversal attempts
	if strings.Contains(basePath, "..") {
		// Log warning and return empty to disable base path
		pkgLog.Warn().Str("basePath", basePath).Msg("Base path contains path traversal sequence '..', ignoring")
		return ""
	}

	// Check for invalid characters (null bytes, control characters)
	for i, r := range basePath {
		if r == 0 || (r > 0 && r < 32) {
			pkgLog.Warn().Str("basePath", basePath).Int("position", i).Int("char", int(r)).Msg("Base path contains invalid control character, ignoring")
			return ""
		}
	}

	// Ensure it starts with "/"
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	// Special case: if basePath consists only of slashes, return "/"
	if strings.Trim(basePath, "/") == "" {
		return "/"
	}

	// Remove trailing slashes
	basePath = strings.TrimRight(basePath, "/")

	return basePath
}

// buildPath builds a full path by joining the base path with a route path
func buildPath(basePath, routePath string) string {
	if basePath == "" {
		return routePath
	}
	if routePath == "/" {
		return basePath + "/"
	}
	return basePath + routePath
}

// shouldStripBasePath determines if a request path should have the base path stripped
// It ensures the base path is followed by "/" or is at the end of the path
func shouldStripBasePath(requestPath, basePath string) bool {
	// Empty base path or root base path should never be stripped
	if basePath == "" || basePath == "/" {
		return false
	}

	if !strings.HasPrefix(requestPath, basePath) {
		return false
	}

	// If exact match, should strip
	if requestPath == basePath {
		return true
	}

	// If base path is followed by "/", should strip
	if len(requestPath) > len(basePath) && requestPath[len(basePath)] == '/' {
		return true
	}

	return false
}

// Sanitize the token for logging
func cutToken(tok token) string {
	s := string(tok)
	if len(s) <= 8 {
		return s
	}
	return s[:8] + "..."
}

func (t *WSTunnelServer) getRemoteServer(tok token, create bool) *remoteServer {
	t.serverRegistryMutex.Lock()
	defer t.serverRegistryMutex.Unlock()

	// lookup and return existing remote server
	rs, ok := t.serverRegistry[tok]
	if ok {
		t.Log.Debug().Str("token", cutToken(tok)).Msg("WS tunnel exists")
		return rs
	}

	if !create { // return null if create flag is not set
		t.Log.Info().Str("token", cutToken(tok)).Msg("WS tunnel not found")
		return nil
	}

	// construct new remote server
	// Clamp MaxRequestsPerTunnel to prevent excessive memory allocation
	maxRequests := t.MaxRequestsPerTunnel
	if maxRequests > 1000 {
		maxRequests = 1000
	}
	rs = &remoteServer{
		token:          tok,
		lastActivity:   time.Now(),
		requestQueue:   make(chan *remoteRequest, maxRequests),
		requestSet:     make(map[int16]*remoteRequest),
		connCloseChans: make([]chan struct{}, 0),
		log:            t.Log.With().Str("token", cutToken(tok)).Logger(),
	}
	rs.readCond = sync.NewCond(&rs.readMutex)
	t.serverRegistry[tok] = rs
	t.Log.Info().Str("token", cutToken(tok)).Msg("WS new tunnel created")
	return rs
}

func (rs *remoteServer) AbortRequests() {
	// end any requests that are queued
l:
	for {
		select {
		case req := <-rs.requestQueue:
			err := fmt.Errorf("tunnel deleted due to inactivity, request cancelled")
			select {
			case req.replyChan <- responseBuffer{err: err}: // non-blocking send
			default:
			}
		default:
			break l
		}
	}
	idle := time.Since(rs.lastActivity).Minutes()
	rs.log.Info().Float64("inactive[min]", idle).Msg("WS tunnel closed")
}

func (rs *remoteServer) AddRequest(req *remoteRequest) error {
	rs.requestSetMutex.Lock()
	defer rs.requestSetMutex.Unlock()
	if req.id < 0 {
		rs.lastID = (rs.lastID + 1) % 32000
		req.id = rs.lastID
		req.log = req.log.With().Int16("id", req.id).Logger()
	}
	rs.requestSet[req.id] = req
	select {
	case rs.requestQueue <- req:
		// enqueued!
		return nil
	default:
		return errors.New("too many requests in-flight, tunnel broken?")
	}
}

func (rs *remoteServer) RetireRequest(req *remoteRequest) {
	rs.requestSetMutex.Lock()
	defer rs.requestSetMutex.Unlock()
	delete(rs.requestSet, req.id)
	// TODO: should we close the channel? problem is that a concurrent send on it causes a panic
}

func makeRequest(r *http.Request, httpTimeout time.Duration) *remoteRequest {
	buf := &bytes.Buffer{}
	_ = r.Write(buf)
	now := time.Now()
	return &remoteRequest{
		id:        -1,
		info:      r.Method + " " + r.URL.String(),
		buffer:    buf,
		replyChan: make(chan responseBuffer, 10),
		deadline:  now.Add(httpTimeout),
		startTime: now,
	}

}

// censoredHeaders, these are removed from the response before forwarding
var censoredHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
}

// Write an HTTP response from a byte buffer into a ResponseWriter
func writeResponse(w http.ResponseWriter, r io.Reader) int {
	// Ensure we're using our safe response writer
	safeW, ok := w.(*safeResponseWriter)
	if !ok {
		safeW = &safeResponseWriter{ResponseWriter: w}
	}

	resp, err := http.ReadResponse(bufio.NewReader(r), nil)
	if err != nil {
		pkgLog.Info().Err(err).Msg("WriteResponse: can't parse incoming response")
		// Set headers before calling WriteHeader to avoid superfluous warning
		safeW.Header().Set("Content-Type", "text/plain; charset=utf-8")
		safeW.Header().Set("X-Content-Type-Options", "nosniff")
		safeW.WriteHeader(httpStatusParseError)
		return httpStatusParseError
	}
	for _, h := range censoredHeaders {
		resp.Header.Del(h)
	}
	// write the response
	copyHeader(safeW.Header(), resp.Header)
	safeW.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(safeW, resp.Body); err != nil {
		pkgLog.Error().Err(err).Msg("Error copying response body")
	}
	if err := resp.Body.Close(); err != nil {
		pkgLog.Error().Err(err).Msg("Failed to close response body")
	}
	return resp.StatusCode
}

// idleTunnelReaper should be run in a goroutine to kill tunnels that are idle for a long time
func (t *WSTunnelServer) idleTunnelReaper() {
	type reapedTunnel struct {
		token      string
		remoteAddr string
		inactiveBy time.Duration
	}

	t.Log.Debug().Msg("idleTunnelReaper started")
	for {
		var reaped []reapedTunnel

		t.serverRegistryMutex.Lock()
		for _, rs := range t.serverRegistry {
			if time.Since(rs.lastActivity) > tunnelInactiveKillTimeout {
				rs.log.Warn().Dur("ago", time.Since(rs.lastActivity)).Msg("Tunnel not seen for a long time, deleting")
				reaped = append(reaped, reapedTunnel{
					token:      string(rs.token),
					remoteAddr: rs.remoteAddr,
					inactiveBy: time.Since(rs.lastActivity),
				})
				// unlink so new tunnels/tokens use a new RemoteServer object
				delete(t.serverRegistry, rs.token)
				go rs.AbortRequests()
			}
		}
		t.serverRegistryMutex.Unlock()

		// Record reap events outside the registry lock to avoid deadlock
		// with GetMonitoringStats/GetAuditingData which acquire locks in reverse order
		if as := t.getAdminService(); as != nil {
			for _, r := range reaped {
				if err := as.RecordTunnelEvent(context.Background(), r.token, TunnelEventReaped, r.remoteAddr, "", "", "", fmt.Sprintf("inactive for %s", r.inactiveBy)); err != nil {
					t.Log.Warn().Err(err).Msg("Failed to record tunnel reap event")
				}
			}
		}

		time.Sleep(time.Minute)
	}
}
