// Copyright (c) 2014 RightScale, Inc. - see LICENSE

// Websockets tunnel client, which runs at the HTTP server end (yes, I know, it's confusing)
// This client connects to a websockets tunnel server and waits to receive HTTP requests
// tunneled through the websocket, then issues these HTTP requests locally to an HTTP server
// grabs the response and ships that back through the tunnel.
//
// This client is highly concurrent: it spawns a goroutine for each received request and issues
// that concurrently to the HTTP server and then sends the response back whenever the HTTP
// request returns. The response can thus go back out of order and multiple HTTP requests can
// be in flight at a time.
//
// This client also sends periodic ping messages through the websocket and expects prompt
// responses. If no response is received, it closes the websocket and opens a new one.
//
// The main limitation of this client is that responses have to go throught the same socket
// that the requests arrived on. Thus, if the websocket dies while an HTTP request is in progress
// it impossible for the response to travel on the next websocket, instead it will be dropped
// on the floor. This should not be difficult to fix, though.
//
// Another limitation is that it keeps a single websocket open and can thus get stuck for
// many seconds until the timeout on the websocket hits and a new one is opened.

package tunnel

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"os"
	"regexp"
	"runtime"

	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"

	// imported per documentation - https://golang.org/pkg/net/http/pprof/
	_ "net/http/pprof"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// WSTunnelClient represents a persistent tunnel that can cycle through many websockets. The
// fields in this struct are relatively static/constant. The conn field points to the latest
// websocket, but it's important to realize that there may be goroutines handling older
// websockets that are not fully closed yet running at any point in time
type WSTunnelClient struct {
	Token          string         // Rendez-vous token
	Password       string         // Optional password for token authentication
	Tunnel         *url.URL       // websocket server to connect to (ws[s]://hostname:port)
	Server         string         // local HTTP(S) server to send received requests to (default server)
	InternalServer http.Handler   // internal Server to dispatch HTTP requests to
	Regexp         *regexp.Regexp // regexp for allowed local HTTP(S) servers
	Insecure       bool           // accept self-signed SSL certs from local HTTPS servers
	Cert           string         // accept provided certificate from local HTTPS servers
	Timeout        time.Duration  // timeout on websocket
	Proxy          *url.URL       // if non-nil, external proxy to use
	Log            zerolog.Logger // logger with "pkg=WStuncli"
	StatusFd       *os.File       // output periodic tunnel status information
	Connected      bool           // true when we have an active connection to wstunsrv
	connMutex      sync.RWMutex   // protects Connected field
	exitChan       chan struct{}  // channel to tell the tunnel goroutines to end
	conn           *WSConnection
	ClientPorts    []int              // array of ports for client to listen on.
	connManager    *ConnectionManager // connection manager for retry logic
	//ws             *websocket.Conn // websocket connection
}

// WSConnection represents a single websocket connection
type WSConnection struct {
	Log zerolog.Logger  // logger with "ws=0x1234"
	ws  *websocket.Conn // websocket connection
	tun *WSTunnelClient // link back to tunnel
}

var httpClient http.Client // client used for all requests, gets special transport for -insecure

// IsConnected returns true if the client has an active connection to wstunsrv
func (t *WSTunnelClient) IsConnected() bool {
	t.connMutex.RLock()
	defer t.connMutex.RUnlock()
	return t.Connected
}

// setConnected sets the connection status
func (t *WSTunnelClient) setConnected(connected bool) {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()
	t.Connected = connected
}

//===== Main =====

// NewWSTunnelClient Creates a new WSTunnelClient from command line
func NewWSTunnelClient(args []string) *WSTunnelClient {
	wstunCli := WSTunnelClient{}

	var cliFlag = flag.NewFlagSet("client", flag.ExitOnError)
	var tokenArg = cliFlag.String("token", "",
		"rendez-vous token identifying this server (format: token or token:password)")
	var tunnel = cliFlag.String("tunnel", "",
		"websocket server ws[s]://user:pass@hostname:port to connect to")
	cliFlag.StringVar(&wstunCli.Server, "server", "",
		"http server http[s]://hostname:port to send received requests to")
	cliFlag.BoolVar(&wstunCli.Insecure, "insecure", false,
		"accept self-signed SSL certs from local HTTPS servers")
	var sre = cliFlag.String("regexp", "",
		"regexp for local HTTP(S) server to allow sending received requests to")
	var tout = cliFlag.Int("timeout", 30, "timeout on websocket in seconds")
	var pidf = cliFlag.String("pidfile", "", "path for pidfile")
	var logf = cliFlag.String("logfile", "", "path for log file")
	var statf = cliFlag.String("statusfile", "", "path for status file")
	var proxy = cliFlag.String("proxy", "",
		"use HTTPS proxy http://user:pass@hostname:port")
	var cliports = cliFlag.String("client-ports", "",
		"comma separated list of client listening ports ex: -client-ports 8000..8100,8300..8400,8500,8505")
	cliFlag.StringVar(&wstunCli.Cert, "certfile", "", "path for trusted certificate in PEM-encoded format")
	var logLevel = cliFlag.String("log-level", "info", "log level (debug, info, warn, error)")
	var logPretty = cliFlag.Bool("log-pretty", false, "use human-readable console log output")

	// Bootstrap logger for pre-flag-parse errors. Uses stderr directly since
	// LogPretty is not yet available (flags haven't been parsed).
	bootstrap := zerolog.New(DefaultLogWriter).With().Timestamp().Str("pkg", "WStuncli").Logger()

	// Fix flag parsing
	if err := cliFlag.Parse(args); err != nil {
		bootstrap.Fatal().Err(err).Msg("Failed to parse client flags")
		return nil
	}

	LogPretty = *logPretty
	wstunCli.Log = makeLogger("WStuncli", *logf, "")
	pkgLog = newPkgLogger()
	if err := writePid(*pidf); err != nil {
		wstunCli.Log.Error().Err(err).Msg("Failed to write pidfile")
	}

	// Set log level
	level, err := zerolog.ParseLevel(*logLevel)
	if err != nil {
		wstunCli.Log.Warn().Str("level", *logLevel).Msg("Invalid log level, using info")
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	wstunCli.Timeout = calcWsTimeout(wstunCli.Log, *tout)

	// Parse token:password format
	if *tokenArg != "" {
		parts := strings.SplitN(*tokenArg, ":", 2)
		wstunCli.Token = strings.TrimSpace(parts[0])
		if len(parts) == 2 {
			wstunCli.Password = strings.TrimSpace(parts[1])
		}
	}

	// process -statusfile
	if *statf != "" {
		fd, err := os.Create(*statf)
		if err != nil {
			bootstrap.Fatal().Err(err).Msg("Can't create statusfile")
		}
		wstunCli.StatusFd = fd
	}

	// process -regexp
	if *sre != "" {
		var err error
		wstunCli.Regexp, err = regexp.Compile(*sre)
		if err != nil {
			bootstrap.Fatal().Err(err).Msg("Can't parse -regexp")
		}
	}

	if *tunnel != "" {
		tunnelUrl, err := url.Parse(*tunnel)
		if err != nil {
			bootstrap.Fatal().Err(err).Str("tunnel", *tunnel).Msg("Invalid tunnel address")
		}

		if tunnelUrl.Scheme != "ws" && tunnelUrl.Scheme != "wss" {
			bootstrap.Fatal().Msg("Remote tunnel (-tunnel option) must begin with ws:// or wss://")
		}

		wstunCli.Tunnel = tunnelUrl
	} else {
		bootstrap.Fatal().Msg("Must specify tunnel server ws://hostname:port using -tunnel option")
	}

	// process -proxy or look for standard unix env variables
	if *proxy == "" {
		envNames := []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"}
		for _, n := range envNames {
			if p := os.Getenv(n); p != "" {
				*proxy = p
				break
			}
		}
	}
	if *proxy != "" {
		proxyURL, err := url.Parse(*proxy)
		if err != nil || !strings.HasPrefix(proxyURL.Scheme, "http") {
			// proxy was bogus. Try prepending "http://" to it and
			// see if that parses correctly. If not, we fall
			// through and complain about the original one.
			if proxyURL, err = url.Parse("http://" + *proxy); err != nil {
				bootstrap.Fatal().Err(err).Str("proxy", *proxy).Msg("Invalid proxy address")
			}
		}

		wstunCli.Proxy = proxyURL
	}

	if *cliports != "" {
		portList := strings.Split(*cliports, ",")
		clientPorts := []int{}
		bootstrap.Info().Str("ports", *cliports).Msg("Attempting to start client with ports")

		for _, v := range portList {
			if strings.Contains(v, "..") {
				k := strings.Split(v, "..")
				bInt, err := strconv.Atoi(k[0])
				if err != nil {
					bootstrap.Fatal().Str("port", k[0]).Str("range", v).Msg("Invalid Port Assignment")
				}

				eInt, err := strconv.Atoi(k[1])
				if err != nil {
					bootstrap.Fatal().Str("port", k[1]).Str("range", v).Msg("Invalid Port Assignment")
				}

				if eInt < bInt {
					bootstrap.Fatal().Int("start", bInt).Int("end", eInt).Msg("End port can not be less than beginning port")
				}

				for n := bInt; n <= eInt; n++ {
					intPort := n
					clientPorts = append(clientPorts, intPort)
				}
			} else {
				intPort, err := strconv.Atoi(v)
				if err != nil {
					bootstrap.Fatal().Str("value", v).Msg("Can not convert to integer")
				}
				clientPorts = append(clientPorts, intPort)
			}
		}
		wstunCli.ClientPorts = clientPorts
	}

	// Initialize connection manager with default values
	wstunCli.connManager = NewConnectionManager(5*time.Second, 0)

	return &wstunCli
}

// Start creates the wstunnel connection.
func (t *WSTunnelClient) Start() error {
	t.Log.Info().Msg(VV)

	// validate -server
	if t.InternalServer != nil {
		t.Server = ""
	} else if t.Server != "" {
		if !strings.HasPrefix(t.Server, "http://") && !strings.HasPrefix(t.Server, "https://") {
			return fmt.Errorf("local server (-server option) must begin with http:// or https://")
		}
		t.Server = strings.TrimSuffix(t.Server, "/")
	}

	// validate token and timeout
	if t.Token == "" {
		return fmt.Errorf("must specify rendez-vous token using -token option")
	}

	tlsClientConfig := tls.Config{}
	if t.Insecure {
		t.Log.Info().Msg("Accepting unverified SSL certs from local HTTPS servers")
		tlsClientConfig.InsecureSkipVerify = t.Insecure
	}
	if t.Cert != "" {
		// Get the SystemCertPool, continue with an empty pool on error
		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		// Read in the cert file
		certs, err := os.ReadFile(t.Cert)
		if err != nil {
			return fmt.Errorf("failed to read certificate file %q: %v", t.Cert, err)
		}
		// Append our cert to the system pool
		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			return fmt.Errorf("failed to appended certificate file %q to pool: %v", t.Cert, err)
		}
		t.Log.Info().Msg("Explicitly accepting provided SSL certificate")
		tlsClientConfig.RootCAs = rootCAs
	}
	httpClient = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tlsClientConfig,
		},
	}

	if t.InternalServer != nil {
		t.Log.Info().Msg("Dispatching to internal server")
	} else if t.Server != "" || t.Regexp != nil {
		t.Log.Info().Str("server", t.Server).Interface("regexp", t.Regexp).Msg("Dispatching to external server(s)")
	} else {
		return fmt.Errorf("must specify internal server or server or regexp")
	}

	if t.Proxy != nil {
		username := "(none)"
		if u := t.Proxy.User; u != nil {
			username = u.Username()
		}
		t.Log.Info().Str("url", t.Proxy.Host).Str("user", username).Msg("Using HTTPS proxy")
	}

	// for test purposes we have a signal that tells wstuncli to exit instead of reopening
	// a fresh connection.
	t.exitChan = make(chan struct{}, 1)

	//===== Goroutine =====

	// Keep opening websocket connections to tunnel requests
	go func() {
		for {
			d := &websocket.Dialer{
				NetDial:         t.wsProxyDialer,
				ReadBufferSize:  wsBufferSize,
				WriteBufferSize: wsBufferSize,
				TLSClientConfig: &tlsClientConfig,
			}
			h := make(http.Header)
			h.Add("Origin", t.Token)
			// Add client version header
			h.Add("X-Client-Version", VV)
			// Add Authorization header for token password if provided
			if t.Password != "" {
				credentials := t.Token + ":" + t.Password
				encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
				h.Add("Authorization", "Basic "+encoded)
			}

			// Also add tunnel URL-based auth if present (supports dual authentication)
			if auth := proxyAuth(t.Tunnel); auth != "" {
				// If we already have token auth, this becomes secondary auth
				// Some servers may use both for different purposes
				if t.Password == "" {
					// No token password, so this is primary auth
					h.Add("Authorization", auth)
				} else {
					// We already have Authorization header for token auth
					// Add URL auth as a custom header that server can check if needed
					h.Add("X-Tunnel-Authorization", auth)
				}
			}
			url := fmt.Sprintf("%s://%s/_tunnel", t.Tunnel.Scheme, t.Tunnel.Host)
			timer := time.NewTimer(10 * time.Second)
			t.Log.Info().Str("url", url).Msg("WS   Opening")
			ws, resp, err := d.Dial(url, h)
			if err != nil {
				extra := ""
				if resp != nil {
					extra = resp.Status
					buf := make([]byte, 80)
					n, err := resp.Body.Read(buf)
					if err != nil && err != io.EOF {
						t.Log.Error().Err(err).Msg("Failed to read response body")
						return
					}
					if len(buf) > 0 {
						extra = extra + " -- " + string(buf[:n])
					}
					if err := resp.Body.Close(); err != nil {
						t.Log.Error().Err(err).Msg("Failed to close response body")
					}
				}
				t.Log.Error().Err(err).Str("info", extra).Msg("Error opening connection")
			} else {
				t.conn = &WSConnection{ws: ws, tun: t,
					Log: t.Log.With().Str("ws", fmt.Sprintf("%p", ws)).Logger()}
				// Safety setting
				ws.SetReadLimit(100 * 1024 * 1024)
				// Request Loop
				srv := t.Server
				if t.InternalServer != nil {
					srv = "<internal>"
				}
				t.conn.Log.Info().Str("server", srv).Msg("WS   ready")
				t.setConnected(true)
				t.conn.handleRequests()
				t.setConnected(false)
			}
			// check whether we need to exit
			exitLoop := false
			select {
			case <-t.exitChan:
				exitLoop = true
			default: // non-blocking receive
			}
			if exitLoop {
				break
			}

			<-timer.C // ensure we don't open connections too rapidly
		}
	}()

	return nil
}

// Stop closes the wstunnel channel
func (t *WSTunnelClient) Stop() {
	t.exitChan <- struct{}{}
	if t.conn != nil && t.conn.ws != nil {
		if err := t.conn.ws.Close(); err != nil {
			t.Log.Error().Err(err).Msg("Failed to close websocket")
		}
	}
}

// Main function to handle WS requests: it reads a request from the socket, then forks
// a goroutine to perform the actual http request and return the result
func (wsc *WSConnection) handleRequests() {
	go wsc.pinger()
	for {
		if err := wsc.ws.SetReadDeadline(time.Time{}); err != nil {
			wsc.Log.Error().Err(err).Msg("Failed to set read deadline")
			return
		}
		typ, r, err := wsc.ws.NextReader()
		if err != nil {
			wsc.Log.Info().Err(err).Msg("WS   ReadMessage")
			break
		}
		if typ != websocket.BinaryMessage {
			wsc.Log.Warn().Int("type", int(typ)).Msg("WS   invalid message type")
			break
		}
		// read request id
		var id int16
		_, err = fmt.Fscanf(io.LimitReader(r, 4), "%04x", &id)
		if err != nil {
			wsc.Log.Warn().Err(err).Msg("WS   cannot read request ID")
			break
		}
		// read the whole message, this is bounded (to something large) by the
		// SetReadLimit on the websocket. We have to do this because we want to handle
		// the request in a goroutine (see "go finish..Request" calls below) and the
		// websocket doesn't allow us to have multiple goroutines reading...
		buf, err := io.ReadAll(r)
		if err != nil {
			wsc.Log.Warn().Int16("id", id).Err(err).Msg("WS   cannot read request message")
			break
		}
		if len(buf) > 1024*1024 {
			wsc.Log.Info().Int("len", len(buf)).Msg("WS   long message")
		}
		wsc.Log.Debug().Int("len", len(buf)).Msg("WS   message")
		r = bytes.NewReader(buf)
		// read request itself
		req, err := http.ReadRequest(bufio.NewReader(r))
		if err != nil {
			wsc.Log.Warn().Int16("id", id).Err(err).Msg("WS   cannot read request body")
			break
		}
		// Hand off to goroutine to finish off while we read the next request
		if wsc.tun.InternalServer != nil {
			go wsc.finishInternalRequest(id, req)
		} else {
			go wsc.finishRequest(id, req)
		}
	}
	// delay a few seconds to allow for writes to drain and then force-close the socket
	go func() {
		time.Sleep(5 * time.Second)
		if err := wsc.ws.Close(); err != nil {
			wsc.Log.Error().Err(err).Msg("Failed to close websocket")
		}
	}()
}

//===== Keep-alive ping-pong =====

// Pinger that keeps connections alive and terminates them if they seem stuck
func (wsc *WSConnection) pinger() {
	defer func() {
		// panics may occur in WriteControl (in unit tests at least) for closed
		// websocket connections
		if x := recover(); x != nil {
			wsc.Log.Error().Interface("err", x).Msg("Panic in pinger")
		}
	}()
	wsc.Log.Info().Msg("pinger starting")
	tunTimeout := wsc.tun.Timeout

	// timeout handler sends a close message, waits a few seconds, then kills the socket
	timeout := func() {
		if wsc.ws == nil {
			return
		}
		if err := wsc.ws.WriteControl(websocket.CloseMessage, nil, time.Now().Add(1*time.Second)); err != nil {
			wsc.Log.Error().Err(err).Msg("Failed to send close message")
			// Don't return error on close
		}
		wsc.Log.Info().Msg("ping timeout, closing WS")
		time.Sleep(5 * time.Second)
		if wsc.ws != nil {
			if err := wsc.ws.Close(); err != nil {
				wsc.Log.Error().Err(err).Msg("Failed to close websocket")
			}
		}
	}
	// timeout timer
	timer := time.AfterFunc(tunTimeout, timeout)
	// pong handler resets last pong time
	ph := func(message string) error {
		timer.Reset(tunTimeout)
		if sf := wsc.tun.StatusFd; sf != nil {
			if _, err := sf.Seek(0, 0); err != nil {
				wsc.Log.Error().Err(err).Msg("Failed to seek file")
				return err
			}
			wsc.writeStatus()
			pos, _ := sf.Seek(0, 1)
			if err := sf.Truncate(pos); err != nil {
				wsc.Log.Error().Err(err).Msg("Failed to truncate file")
				return err
			}
		}
		return nil
	}
	wsc.ws.SetPongHandler(ph)
	// ping loop, ends when socket is closed...
	for wsc.ws != nil {
		if err := wsc.ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(tunTimeout/3)); err != nil {
			break
		}
		time.Sleep(tunTimeout / 3)
	}
	wsc.Log.Info().Msg("pinger ending (WS errored or closed)")
	if err := wsc.ws.Close(); err != nil {
		wsc.Log.Error().Err(err).Msg("Failed to close websocket")
	}
}

func (wsc *WSConnection) writeStatus() {
	if _, err := fmt.Fprintf(wsc.tun.StatusFd, "Unix: %d\n", time.Now().Unix()); err != nil {
		wsc.Log.Error().Err(err).Msg("Failed to write to status file")
	}
	if _, err := fmt.Fprintf(wsc.tun.StatusFd, "Time: %s\n", time.Now().UTC().Format(time.RFC3339)); err != nil {
		wsc.Log.Error().Err(err).Msg("Failed to write to status file")
	}
}

func (t *WSTunnelClient) wsDialerLocalPort(network string, addr string, ports []int) (conn net.Conn, err error) {
	for _, port := range ports {
		client, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return nil, err
		}

		server, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, err
		}

		conn, err = net.DialTCP(network, client, server)
		if err == nil {
			return conn, nil
		}
		t.Log.Info().Int("port", port).Err(err).Msg("WS: error connecting with local port")
		continue
	}

	err = errors.New("WS: Could not connect using any of the ports in range: " + fmt.Sprint(ports))
	return nil, err
}

// ===== Proxy support =====
// Bits of this taken from golangs net/http/transport.go. Gorilla websocket lib
// allows you to pass in a custom net.Dial function, which it will call instead
// of net.Dial. net.Dial normally just opens up a tcp socket for you. We go one
// extra step and issue an HTTP CONNECT command after the socket is open. After
// HTTP CONNECT is issued and successful, we hand the reins back to gorilla,
// which will then set up SSL and handle the websocket UPGRADE request.
// Note this only handles HTTPS connections through the proxy. HTTP requires
// header rewriting.
func (t *WSTunnelClient) wsProxyDialer(network string, addr string) (conn net.Conn, err error) {
	if t.Proxy == nil {
		if len(t.ClientPorts) != 0 {
			conn, err = t.wsDialerLocalPort(network, addr, t.ClientPorts)
			return conn, err
		}
		return net.Dial(network, addr)
	}

	conn, err = net.Dial("tcp", t.Proxy.Host)
	if err != nil {
		err = fmt.Errorf("WS: error connecting to proxy %s: %s", t.Proxy.Host, err.Error())
		return nil, err
	}

	pa := proxyAuth(t.Proxy)

	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}

	if pa != "" {
		connectReq.Header.Set("Proxy-Authorization", pa)
	}
	if err := connectReq.Write(conn); err != nil {
		t.Log.Error().Err(err).Msg("Failed to write connect request")
		return nil, err
	}

	// Read and parse CONNECT response.
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, connectReq)
	if err != nil {
		if err := conn.Close(); err != nil {
			t.Log.Error().Err(err).Msg("Failed to close connection")
		}
		return nil, err
	}
	if resp.StatusCode != 200 {
		//body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 500))
		//resp.Body.Close()
		//return nil, errors.New("proxy refused connection" + string(body))
		f := strings.SplitN(resp.Status, " ", 2)
		if err := conn.Close(); err != nil {
			t.Log.Error().Err(err).Msg("Failed to close connection")
		}
		return nil, fmt.Errorf("%s", f[1])
	}
	return conn, nil
}

// proxyAuth returns the Proxy-Authorization header to set
// on requests, if applicable.
func proxyAuth(proxy *url.URL) string {
	if u := proxy.User; u != nil {
		username := u.Username()
		password, _ := u.Password()
		return "Basic " + basicAuth(username, password)
	}
	return ""
}

// See 2 (end of page 4) http://www.ietf.org/rfc/rfc2617.txt
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

//===== HTTP Header Stuff =====

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
	"Host",
}

//===== HTTP response writer, used for internal request handlers

type responseWriter struct {
	resp *http.Response
	buf  *bytes.Buffer
}

func newResponseWriter(req *http.Request) *responseWriter {
	buf := bytes.Buffer{}
	resp := http.Response{
		Header:        make(http.Header),
		Body:          io.NopCloser(&buf),
		StatusCode:    -1,
		ContentLength: -1,
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
	}
	return &responseWriter{
		resp: &resp,
		buf:  &buf,
	}

}

func (rw *responseWriter) Write(buf []byte) (int, error) {
	if rw.resp.StatusCode == -1 {
		rw.WriteHeader(200)
	}
	return rw.buf.Write(buf)
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.resp.StatusCode = code
	rw.resp.Status = http.StatusText(code)
}

func (rw *responseWriter) Header() http.Header { return rw.resp.Header }

func (rw *responseWriter) finishResponse() error {
	if rw.resp.StatusCode == -1 {
		return fmt.Errorf("HTTP internal handler did not call Write or WriteHeader")
	}
	rw.resp.ContentLength = int64(rw.buf.Len())

	return nil
}

//===== HTTP driver and response sender =====

var wsWriterMutex sync.Mutex // mutex to allow a single goroutine to send a response at a time

// Issue a request to an internal handler. This duplicates some logic found in
// net.http.serve http://golang.org/src/net/http/server.go?#L1124 and
// net.http.readRequest http://golang.org/src/net/http/server.go?#L
func (wsc *WSConnection) finishInternalRequest(id int16, req *http.Request) {
	log := wsc.Log.With().Int16("id", id).Str("verb", req.Method).Str("uri", req.RequestURI).Logger()
	log.Info().Msg("HTTP issuing internal request")

	// Remove hop-by-hop headers
	for _, h := range hopHeaders {
		req.Header.Del(h)
	}

	// Add fake protocol version
	req.Proto = "HTTP/1.0"
	req.ProtoMajor = 1
	req.ProtoMinor = 0

	// Dump the request into a buffer in case we want to log it
	dump, _ := httputil.DumpRequest(req, false)
	log.Debug().Str("req", strings.ReplaceAll(string(dump), "\r\n", " || ")).Msg("dump")

	// Make sure we don't die if a panic occurs in the handler
	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Error().Interface("err", err).Str("stack", string(buf)).Msg("HTTP panic in handler")
		}
	}()

	// Concoct Response
	rw := newResponseWriter(req)

	// Issue the request to the HTTP server
	wsc.tun.InternalServer.ServeHTTP(rw, req)

	err := rw.finishResponse()
	if err != nil {
		log.Info().Err(err).Msg("HTTP request error")
		wsc.writeResponseMessage(id, concoctResponse(req, err.Error(), 502))
		return
	}

	log.Info().Int("status", rw.resp.StatusCode).Msg("HTTP responded")
	wsc.writeResponseMessage(id, rw.resp)
}

func (wsc *WSConnection) finishRequest(id int16, req *http.Request) {

	log := wsc.Log.With().Int16("id", id).Str("verb", req.Method).Str("uri", req.RequestURI).Logger()

	// Honor X-Host header
	host := wsc.tun.Server
	xHost := req.Header.Get("X-Host")
	if xHost != "" {
		re := wsc.tun.Regexp
		if re == nil {
			log.Info().Msg("WS   got x-host header but no regexp provided")
			wsc.writeResponseMessage(id, concoctResponse(req,
				"X-Host header disallowed by wstunnel cli (no -regexp option)", 403))
			return
		} else if re.FindString(xHost) == xHost {
			host = xHost
		} else {
			log.Info().Str("x-host", xHost).Str("regexp", re.String()).Str("match", re.FindString(xHost)).Msg("WS   x-host disallowed by regexp")
			wsc.writeResponseMessage(id, concoctResponse(req,
				"X-Host header '"+xHost+"' does not match regexp in wstunnel cli",
				403))
			return
		}
	} else if host == "" {
		log.Info().Msg("WS   no x-host header and -server not specified")
		wsc.writeResponseMessage(id, concoctResponse(req,
			"X-Host header required by wstunnel cli (no -server option)", 403))
		return
	}
	req.Header.Del("X-Host")

	// Construct the URL for the outgoing request
	var err error
	req.URL, err = url.Parse(fmt.Sprintf("%s%s", host, req.RequestURI))
	if err != nil {
		log.Warn().Err(err).Msg("WS   cannot parse requestURI")
		wsc.writeResponseMessage(id, concoctResponse(req, "Cannot parse request URI", 400))
		return
	}
	req.Host = req.URL.Host // we delete req.Header["Host"] further down
	req.RequestURI = ""
	log.Info().Str("url", req.URL.String()).Msg("HTTP issuing request")

	// Remove hop-by-hop headers
	for _, h := range hopHeaders {
		req.Header.Del(h)
	}
	// Issue the request to the HTTP server
	dump, err := httputil.DumpRequest(req, false)
	log.Debug().Str("req", strings.ReplaceAll(string(dump), "\r\n", " || ")).Msg("dump")
	if err != nil {
		log.Warn().Err(err).Msg("error dumping request")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Info().Err(err).Msg("HTTP request error")
		wsc.writeResponseMessage(id, concoctResponse(req, err.Error(), 502))
		return
	}
	log.Info().Str("status", resp.Status).Msg("HTTP responded")
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close response body")
		}
	}()

	wsc.writeResponseMessage(id, resp)
}

// Write the response message to the websocket
func (wsc *WSConnection) writeResponseMessage(id int16, resp *http.Response) {
	// Get writer's lock
	wsWriterMutex.Lock()
	defer wsWriterMutex.Unlock()
	// Write response into the tunnel
	if err := wsc.ws.SetWriteDeadline(time.Time{}); err != nil {
		wsc.Log.Error().Err(err).Msg("Failed to set write deadline")
		return
	}
	w, err := wsc.ws.NextWriter(websocket.BinaryMessage)
	// got an error, reply with a "hey, retry" to the request handler
	if err != nil {
		wsc.Log.Warn().Err(err).Msg("WS   NextWriter")
		if err := wsc.ws.Close(); err != nil {
			wsc.Log.Error().Err(err).Msg("Failed to close websocket")
		}
		return
	}

	// write the request Id
	_, err = fmt.Fprintf(w, "%04x", id)
	if err != nil {
		wsc.Log.Warn().Err(err).Msg("WS   cannot write request Id")
		if err := wsc.ws.Close(); err != nil {
			wsc.Log.Error().Err(err).Msg("Failed to close websocket")
		}
		return
	}

	// write the response itself
	err = resp.Write(w)
	if err != nil {
		wsc.Log.Warn().Err(err).Msg("WS   cannot write response")
		if err := wsc.ws.Close(); err != nil {
			wsc.Log.Error().Err(err).Msg("Failed to close websocket")
		}
		return
	}

	// done
	err = w.Close()
	if err != nil {
		wsc.Log.Warn().Err(err).Msg("WS   write-close failed")
		if err := wsc.ws.Close(); err != nil {
			wsc.Log.Error().Err(err).Msg("Failed to close websocket")
		}
		return
	}
}

// Create an http Response from scratch, there must be a better way that this but I
// don't know what it is
func concoctResponse(req *http.Request, message string, code int) *http.Response {
	r := http.Response{
		Status:     "Bad Gateway", //strconv.Itoa(code),
		StatusCode: code,
		Proto:      req.Proto,
		ProtoMajor: req.ProtoMajor,
		ProtoMinor: req.ProtoMinor,
		Header:     make(map[string][]string),
		Request:    req,
	}
	body := bytes.NewReader([]byte(message))
	r.Body = io.NopCloser(body)
	r.ContentLength = int64(body.Len())
	r.Header.Add("content-type", "text/plain")
	r.Header.Add("date", time.Now().Format(time.RFC1123))
	r.Header.Add("server", "wstunnel")
	r.Header.Set("WWW-Authenticate", "Basic realm=\"wstunnel\"")
	return &r
}
