// Copyright (c) 2014 RightScale, Inc. - see LICENSE

package tunnel

import (
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"

	// imported per documentation - https://golang.org/pkg/net/http/pprof/
	_ "net/http/pprof"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/inconshreveable/log15.v2"
)

func httpError(log log15.Logger, w http.ResponseWriter, token, err string, code int) {
	log.Info("HTTP Error", "token", token, "error", err, "code", code)
	
	// Use safeError to avoid superfluous WriteHeader warnings
	safeError(w, html.EscapeString(err), code)
}

// safeResponseWriter is a custom ResponseWriter that prevents multiple WriteHeader calls
type safeResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *safeResponseWriter) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.ResponseWriter.WriteHeader(statusCode)
		w.wroteHeader = true
	}
}

func (w *safeResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// safeError is a safer replacement for http.Error that sets headers before WriteHeader
func safeError(w http.ResponseWriter, error string, code int) {
	// Wrap the response writer if it's not already wrapped
	safeW, ok := w.(*safeResponseWriter)
	if !ok {
		safeW = &safeResponseWriter{ResponseWriter: w}
	}
	
	safeW.Header().Set("Content-Type", "text/plain; charset=utf-8")
	safeW.Header().Set("X-Content-Type-Options", "nosniff")
	safeW.WriteHeader(code)
	_, _ = safeW.Write([]byte(error))
}

// websocket error constants
// const (
// 	wsReadClose  = iota
// 	wsReadError  = iota
// 	wsWriteError = iota
// )

func wsp(ws *websocket.Conn) string { return fmt.Sprintf("%p", ws) }

// Handler for websockets tunnel establishment requests
func wsHandler(t *WSTunnelServer, w http.ResponseWriter, r *http.Request) {
	addr := r.Header.Get("X-Forwarded-For")
	if addr == "" {
		addr = r.RemoteAddr
	}
	// Verify that an origin header with a token is provided
	tok := r.Header.Get("Origin")
	if tok == "" {
		httpError(t.Log, w, addr, "Origin header with rendez-vous token required", 400)
		return
	}
	if len(tok) < minTokenLen {
		httpError(t.Log, w, addr,
			fmt.Sprintf("Rendez-vous token (%s) is too short (must be %d chars)",
				tok, minTokenLen), 400)
		return
	}
	logTok := cutToken(token(tok))
	// Upgrade to web sockets
	upgrader := websocket.Upgrader{
		ReadBufferSize:  100 * 1024,
		WriteBufferSize: 100 * 1024,
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); ok {
			t.Log.Info("WS new tunnel connection rejected", "token", logTok, "addr", addr,
				"err", "Not a websocket handshake")
			httpError(t.Log, w, logTok, "Not a websocket handshake", 400)
			return
		}
		t.Log.Info("WS new tunnel connection rejected", "token", logTok, "addr", addr,
			"err", err.Error())
		httpError(t.Log, w, logTok, err.Error(), 400)
		return
	}
	// Get/Create RemoteServer
	rs := t.getRemoteServer(token(tok), true)
	rs.remoteAddr = addr
	rs.lastActivity = time.Now()
	t.Log.Info("WS new tunnel connection", "token", logTok, "addr", addr, "ws", wsp(ws),
		"rs", rs)
	// do reverse DNS lookup asynchronously
	go func() {
		rs.remoteName, rs.remoteWhois = ipAddrLookup(t.Log, rs.remoteAddr)
	}()
	// Start timeout handling
	wsSetPingHandler(t, ws, rs)
	// Create synchronization channel
	ch := make(chan int, 2)
	// Spawn goroutine to read responses
	go wsReader(rs, ws, ch)
	// Send requests
	wsWriter(rs, ws, ch)
}

func wsSetPingHandler(t *WSTunnelServer, ws *websocket.Conn, rs *remoteServer) {
	// timeout handler sends a close message, waits a few seconds, then kills the socket
	timeout := func() {
		if err := ws.WriteControl(websocket.CloseMessage, nil, time.Now().Add(1*time.Second)); err != nil {
			rs.log.Error("Error writing control message", "err", err)
		}
		time.Sleep(5 * time.Second)
		rs.log.Info("WS closing due to ping timeout", "ws", wsp(ws))
		if err := ws.Close(); err != nil {
			rs.log.Error("Failed to close websocket", "err", err)
		}
	}
	// timeout timer
	timer := time.AfterFunc(t.WSTimeout, timeout)
	// ping handler resets last ping time
	ph := func(message string) error {
		timer.Reset(t.WSTimeout)
		if err := ws.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(t.WSTimeout/3)); err != nil {
			rs.log.Error("Error writing pong message", "err", err)
			return err
		}
		// update lastActivity
		rs.lastActivity = time.Now()
		return nil
	}
	ws.SetPingHandler(ph)
}

// Pick requests off the RemoteServer queue and send them into the tunnel
func wsWriter(rs *remoteServer, ws *websocket.Conn, ch chan int) {
	var req *remoteRequest
	var err error
	for {
		// fetch a request
		select {
		case req = <-rs.requestQueue:
			// awesome...
		case <-ch:
			// time to close shop
			rs.log.Info("WS closing on signal", "ws", wsp(ws))
			if err := ws.Close(); err != nil {
				rs.log.Error("Failed to close websocket", "err", err)
			}
			return
		}
		//log.Printf("WS->%s#%d start %s", req.token, req.id, req.info)
		// See whether the request has already expired
		if req.deadline.Before(time.Now()) {
			req.replyChan <- responseBuffer{
				err: errors.New("timeout before forwarding the request"),
			}
			req.log.Info("WS   SND timeout before sending", "ago",
				time.Since(req.deadline).Seconds())
			continue
		}
		// write the request into the tunnel
		if err = ws.SetWriteDeadline(time.Time{}); err != nil {
			rs.log.Error("Failed to set write deadline", "err", err)
			break // Break out of write loop on error
		}
		var w io.WriteCloser
		w, err = ws.NextWriter(websocket.BinaryMessage)
		// got an error, reply with a "hey, retry" to the request handler
		if err != nil {
			break
		}
		// write the request Id
		_, err = fmt.Fprintf(w, "%04x", req.id)
		if err != nil {
			break
		}
		// write the request itself
		_, err = req.buffer.WriteTo(w)
		if err != nil {
			break
		}
		// done
		err = w.Close()
		if err != nil {
			break
		}
		req.log.Info("WS   SND", "info", req.info)
	}
	// tell the sender to retry the request
	req.replyChan <- responseBuffer{err: ErrRetry}
	req.log.Info("WS error causes retry")
	// close up shop
	if err := ws.WriteControl(websocket.CloseMessage, nil, time.Now().Add(5*time.Second)); err != nil {
		rs.log.Error("Error writing control message", "err", err)
	}
	time.Sleep(2 * time.Second)
	if err := ws.Close(); err != nil {
		rs.log.Error("Failed to close websocket", "err", err)
	}
}

// Read responses from the tunnel and fulfill pending requests
func wsReader(rs *remoteServer, ws *websocket.Conn, ch chan int) {
	var err error
	logToken := cutToken(rs.token)

	// the mutex remains locked unless we are within Cond.Wait()
	rs.readCond.L.Lock()
	defer func() {
		rs.readCond.L.Unlock()
		rs.readCond.Signal()
	}()

	// continue reading until we get an error
	for {
		if err = ws.SetReadDeadline(time.Time{}); err != nil {
			rs.log.Error("Failed to set read deadline", "err", err)
			break // Break out of read loop on error
		}
		// read a message from the tunnel
		var t int
		var r io.Reader
		t, r, err = ws.NextReader()
		if err != nil {
			break
		}
		if t != websocket.BinaryMessage {
			err = fmt.Errorf("non-binary message received, type=%d", t)
			break
		}
		// get request id
		var id int16
		_, err = fmt.Fscanf(io.LimitReader(r, 4), "%04x", &id)
		if err != nil {
			break
		}
		// try to match request
		rs.requestSetMutex.Lock()
		req := rs.requestSet[id]
		rs.lastActivity = time.Now()
		rs.requestSetMutex.Unlock()
		// let's see...
		if req != nil {
			rb := responseBuffer{response: r}
			// try to enqueue response
			select {
			case req.replyChan <- rb:
				rs.log.Info("WS   RCV enqueued response", "id", id, "ws", wsp(ws))
				rs.readCond.Wait() // wait for response to be sent
			default:
				rs.log.Info("WS   RCV can't enqueue response", "id", id, "ws", wsp(ws))
			}
		} else {
			rs.log.Info("%s #%d: WS   RCV orphan response", "id", id, "ws", wsp(ws))
		}
	}
	// print error message
	if err != nil {
		rs.log.Info("WS   closing", "token", logToken, "err", err.Error(), "ws", wsp(ws))
	}
	// close up shop
	ch <- 0 // notify sender
	time.Sleep(2 * time.Second)
	if err := ws.Close(); err != nil {
		rs.log.Error("Failed to close websocket", "err", err)
	}
}