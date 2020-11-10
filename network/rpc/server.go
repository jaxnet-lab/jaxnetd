// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpc

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/corelog"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"go.uber.org/zap"
)

func init() {
	// rpcHandlers = rpcHandlersBeforeInit
	rand.Seed(time.Now().UnixNano())
}

type ServerCore struct {
	started  int32
	shutdown int32
	cfg      *Config

	authSHA                [sha256.Size]byte
	limitAuthSHA           [sha256.Size]byte
	//wsManager              *wsManager
	numClients             int32
	statusLines            map[int]string
	statusLock             sync.RWMutex
	wg                     sync.WaitGroup
	requestProcessShutdown chan struct{}
	quit                   chan int
	logger                 corelog.ILogger
}

func NewRPCCore(config *Config, logger *zap.Logger) *ServerCore {
	rpc := &ServerCore{
		cfg:                    config,
		statusLines:            make(map[int]string),
		requestProcessShutdown: make(chan struct{}),
		quit:                   make(chan int),
		logger:                 corelog.Adapter(logger),

		started:      0,
		shutdown:     0,
		numClients:   0,
		authSHA:      [32]byte{},
		limitAuthSHA: [32]byte{},
		statusLock:   sync.RWMutex{},
		wg:           sync.WaitGroup{},
	}

	if rpc.cfg.User != "" && rpc.cfg.Password != "" {
		login := rpc.cfg.User + ":" + rpc.cfg.Password
		auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(login))
		rpc.authSHA = sha256.Sum256([]byte(auth))
	}
	if rpc.cfg.LimitUser != "" && rpc.cfg.LimitPass != "" {
		login := rpc.cfg.LimitUser + ":" + rpc.cfg.LimitPass
		auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(login))
		rpc.limitAuthSHA = sha256.Sum256([]byte(auth))
	}

	//rpc.wsManager = newWsNotificationManager(rpc)
	return rpc
}

func (server *ServerCore) StartRPC(ctx context.Context, rpcServeMux *http.ServeMux) {
	if atomic.AddInt32(&server.started, 1) != 1 {
		return
	}

	server.logger.Debug("Starting RPC Server")
	httpServer := &http.Server{
		Handler: rpcServeMux,
		// Timeout connections which don't complete the initial
		// handshake within the allowed timeframe.
		ReadTimeout: time.Second * rpcAuthTimeoutSeconds,
	}

	for _, listener := range server.cfg.Listeners {
		server.wg.Add(1)
		go func(listener net.Listener) {
			server.logger.Infof("RPC Server listening on %s", listener.Addr())
			httpServer.Serve(listener)
			server.logger.Tracef("RPC listener done for %s", listener.Addr())
			server.wg.Done()
		}(listener)
	}

	<-ctx.Done()

	server.logger.Info("Shutting down the API Server...")
	if err := server.Stop(); err != nil {
		server.logger.Error("Can not stop RPC Core gracefully: " + err.Error())
		return

	}
	server.logger.Info("Api Server gracefully stopped")
}

// Stop is used by rpc.go to stop the rpc listener.
func (server *ServerCore) Stop() error {
	if atomic.AddInt32(&server.shutdown, 1) != 1 {
		server.logger.Infof("RPC Server is already in the process of shutting down")
		return nil
	}
	server.logger.Warnf("RPC Server shutting down")
	for _, listener := range server.cfg.Listeners {
		err := listener.Close()
		if err != nil {
			server.logger.Errorf("Problem shutting down rpc: %v", err)
			return err
		}
	}
	//server.wsManager.Shutdown()
	//server.wsManager.WaitForShutdown()
	close(server.quit)
	server.wg.Wait()
	server.logger.Infof("RPC Server shutdown complete")
	return nil
}

// RequestedProcessShutdown returns a channel that is sent to when an authorized
// RPC client requests the process to shutdown.  If the request can not be read
// immediately, it is dropped.
func (server *ServerCore) RequestedProcessShutdown() <-chan struct{} {
	return server.requestProcessShutdown
}

func (server *ServerCore) HandleFunc(handler commandMux) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "close")
		w.Header().Set("Content-Type", "application/json")
		r.Close = true

		// Limit the number of connections to max allowed.
		if server.limitConnections(w, r.RemoteAddr) {
			return
		}

		// Keep track of the number of connected clientserver.
		server.incrementClients()
		defer server.decrementClients()
		_, isAdmin, err := server.checkAuth(r, true)
		if err != nil {
			jsonAuthFail(w)
			return
		}

		// Read and respond to the request.
		server.ReadJsonRPC(w, r, isAdmin, handler)
	}
}

// ReadJsonRPC handles reading and responding to RPC messages.
func (server *ServerCore) ReadJsonRPC(w http.ResponseWriter, r *http.Request, isAdmin bool, handler commandMux) {
	if atomic.LoadInt32(&server.shutdown) != 0 {
		return
	}

	// Read and close the JSON-RPC request body from the caller.
	body, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		errCode := http.StatusBadRequest
		http.Error(w, fmt.Sprintf("%d error reading JSON message: %v",
			errCode, err), errCode)
		return
	}

	// Unfortunately, the http Server doesn't provide the ability to
	// change the read deadline for the new connection and having one breaks
	// long polling.  However, not having a read deadline on the initial
	// connection would mean clients can connect and idle forever.  Thus,
	// hijack the connecton from the HTTP Server, clear the read deadline,
	// and handle writing the response manually.
	hj, ok := w.(http.Hijacker)
	if !ok {
		errMsg := "webserver doesn't support hijacking"
		server.logger.Warnf(errMsg)
		errCode := http.StatusInternalServerError
		http.Error(w, strconv.Itoa(errCode)+" "+errMsg, errCode)
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		server.logger.Warnf("Failed to hijack HTTP connection: %v", err)
		errCode := http.StatusInternalServerError
		http.Error(w, strconv.Itoa(errCode)+" "+err.Error(), errCode)
		return
	}

	defer conn.Close()
	defer buf.Flush()
	conn.SetReadDeadline(timeZeroVal)

	// Attempt to parse the raw body into a JSON-RPC request.
	var responseID interface{}
	var jsonErr error
	var result interface{}
	var request btcjson.Request
	// data := strings.Replace(string(body), "-0\",\"method\"", "\",\"method\"", 1)
	data := string(body)
	if strings.HasPrefix(data, "[") {
		data = data[1 : len(data)-1]
	}
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		jsonErr = &btcjson.RPCError{
			Code:    btcjson.ErrRPCParse.Code,
			Message: "Failed to parse request: " + err.Error(),
		}
	}

	_ = request.ShardID
	_ = request.Scope
	_ = request.Method

	if jsonErr == nil {
		// The JSON-RPC 1.0 spec defines that notifications must have their "id"
		// set to null and states that notifications do not have a response.
		//
		// A JSON-RPC 2.0 notification is a request with "json-rpc":"2.0", and
		// without an "id" member. The specification states that notifications
		// must not be responded to. JSON-RPC 2.0 permits the null value as a
		// valid request id, therefore such requests are not notifications.
		//
		// Bitcoin Core serves requests with "id":null or even an absent "id",
		// and responds to such requests with "id":null in the response.
		//
		// Btcd does not respond to any request without and "id" or "id":null,
		// regardless the indicated JSON-RPC protocol version unless RPC quirks
		// are enabled. With RPC quirks enabled, such requests will be responded
		// to if the reqeust does not indicate JSON-RPC version.
		//
		// RPC quirks can be enabled by the user to avoid compatibility issues
		// with software relying on Core'server behavior.
		if request.ID == nil {
			return
		}

		// The parse was at least successful enough to have an ID so
		// set it for the response.
		responseID = request.ID

		// Setup a close notifier.  Since the connection is hijacked,
		// the CloseNotifer on the ResponseWriter is not available.
		closeChan := make(chan struct{}, 1)
		go func() {
			_, err := conn.Read(make([]byte, 1))
			if err != nil {
				close(closeChan)
			}
		}()

		// Check if the user is limited and set error if method unauthorized
		if !isAdmin {
			if _, ok := rpcLimited[request.Method]; !ok {
				jsonErr = &btcjson.RPCError{
					Code:    btcjson.ErrRPCInvalidParams.Code,
					Message: "limited user not authorized for this method",
				}
			}
		}

		if jsonErr == nil {
			// Attempt to parse the JSON-RPC request into a known concrete
			// command.
			parsedCmd := parseCmd(&request)
			if parsedCmd.err != nil {
				jsonErr = parsedCmd.err
			} else {
				result, jsonErr = handler(parsedCmd, closeChan)
			}
		}
	}

	// Marshal the response.
	msg, err := server.createMarshalledReply(responseID, result, jsonErr)
	if err != nil {
		server.logger.Errorf("Failed to marshal reply: %v", err)
		return
	}

	// Write the response.
	err = server.writeHTTPResponseHeaders(r, w.Header(), http.StatusOK, buf)
	if err != nil {
		server.logger.Error(err.Error())
		return
	}
	if _, err := buf.Write(msg); err != nil {
		server.logger.Errorf("Failed to write marshalled reply: %v", err)
	}

	// Terminate with newline to maintain compatibility with Bitcoin Core.
	if err := buf.WriteByte('\n'); err != nil {
		server.logger.Errorf("Failed to append terminating newline to reply: %v", err)
	}
}

// httpStatusLine returns a response Status-Line (RFC 2616 Section 6.1)
// for the given request and response status code.  This function was lifted and
// adapted from the standard library HTTP server code since it's not exported.
func (server *ServerCore) httpStatusLine(req *http.Request, code int) string {
	// Fast path:
	key := code
	proto11 := req.ProtoAtLeast(1, 1)
	if !proto11 {
		key = -key
	}
	server.statusLock.RLock()
	line, ok := server.statusLines[key]
	server.statusLock.RUnlock()
	if ok {
		return line
	}

	// Slow path:
	proto := "HTTP/1.0"
	if proto11 {
		proto = "HTTP/1.1"
	}
	codeStr := strconv.Itoa(code)
	text := http.StatusText(code)
	if text != "" {
		line = proto + " " + codeStr + " " + text + "\r\n"
		server.statusLock.Lock()
		server.statusLines[key] = line
		server.statusLock.Unlock()
	} else {
		text = "status code " + codeStr
		line = proto + " " + codeStr + " " + text + "\r\n"
	}

	return line
}

// writeHTTPResponseHeaders writes the necessary response headers prior to
// writing an HTTP body given a request to use for protocol negotiation, headers
// to write, a status code, and a writer.
func (server *ServerCore) writeHTTPResponseHeaders(req *http.Request, headers http.Header, code int, w io.Writer) error {
	_, err := io.WriteString(w, server.httpStatusLine(req, code))
	if err != nil {
		return err
	}

	err = headers.Write(w)
	if err != nil {
		return err
	}

	_, err = io.WriteString(w, "\r\n")
	return err
}

// limitConnections responds with a 503 service unavailable and returns true if
// adding another client would exceed the maximum allow RPC clients.
//
// This function is safe for concurrent access.
func (server *ServerCore) limitConnections(w http.ResponseWriter, remoteAddr string) bool {
	if int(atomic.LoadInt32(&server.numClients)+1) > server.cfg.MaxClients {
		server.logger.Infof("Max RPC clients exceeded [%d] - "+
			"disconnecting client %s", server.cfg.MaxClients,
			remoteAddr)
		http.Error(w, "503 Too busy.  Try again later.",
			http.StatusServiceUnavailable)
		return true
	}
	return false
}

// incrementClients adds one to the number of connected RPC clients.  Note
// this only applies to standard clients.  Websocket clients have their own
// limits and are tracked separately.
//
// This function is safe for concurrent access.
func (server *ServerCore) incrementClients() {
	atomic.AddInt32(&server.numClients, 1)
}

// decrementClients subtracts one from the number of connected RPC clients.
// Note this only applies to standard clients.  Websocket clients have their own
// limits and are tracked separately.
//
// This function is safe for concurrent access.
func (server *ServerCore) decrementClients() {
	atomic.AddInt32(&server.numClients, -1)
}

// checkAuth checks the HTTP Basic authentication supplied by a wallet
// or RPC client in the HTTP request r.  If the supplied authentication
// does not match the username and password expected, a non-nil error is
// returned.
//
// This check is time-constant.
//
// The first bool return value signifies auth success (true if successful) and
// the second bool return value specifies whether the user can change the state
// of the server (true) or whether the user is limited (false). The second is
// always false if the first is.
func (server *ServerCore) checkAuth(r *http.Request, require bool) (bool, bool, error) {
	authhdr := r.Header["Authorization"]
	if len(authhdr) <= 0 {
		if require {
			server.logger.Warnf("RPC authentication failure from %s",
				r.RemoteAddr)
			return false, false, errors.New("auth failure")
		}

		return false, false, nil
	}

	authsha := sha256.Sum256([]byte(authhdr[0]))

	// Check for limited auth first as in environments with limited users, those
	// are probably expected to have a higher volume of calls
	limitcmp := subtle.ConstantTimeCompare(authsha[:], server.limitAuthSHA[:])
	if limitcmp == 1 {
		return true, false, nil
	}

	// Check for admin-level auth
	cmp := subtle.ConstantTimeCompare(authsha[:], server.authSHA[:])
	if cmp == 1 {
		return true, true, nil
	}

	// Request'server auth doesn't match either user
	server.logger.Warnf("RPC authentication failure from %s", r.RemoteAddr)
	return false, false, errors.New("auth failure")
}

// genCertPair generates a key/cert pair to the paths provided.
func (server *ServerCore) genCertPair(certFile, keyFile string) error {
	server.logger.Infof("Generating TLS certificates...")

	org := "btcd autogenerated cert"
	validUntil := time.Now().Add(10 * 365 * 24 * time.Hour)
	cert, key, err := btcutil.NewTLSCertPair(org, validUntil, nil)
	if err != nil {
		return err
	}

	// Write cert and key files.
	if err = ioutil.WriteFile(certFile, cert, 0666); err != nil {
		return err
	}
	if err = ioutil.WriteFile(keyFile, key, 0600); err != nil {
		os.Remove(certFile)
		return err
	}

	server.logger.Infof("Done generating TLS certificates")
	return nil
}

// createMarshalledReply returns a new marshalled JSON-RPC response given the
// passed parameters.  It will automatically convert errors that are not of
// the type *btcjson.RPCError to the appropriate type as needed.
func (server *ServerCore) createMarshalledReply(id, result interface{}, replyErr error) ([]byte, error) {
	var jsonErr *btcjson.RPCError
	if replyErr != nil {
		if jErr, ok := replyErr.(*btcjson.RPCError); ok {
			jsonErr = jErr
		} else {
			jsonErr = server.internalRPCError(replyErr.Error(), "")
		}
	}

	return btcjson.MarshalResponse(id, result, jsonErr)
}

// internalRPCError is a convenience function to convert an internal error to
// an RPC error with the appropriate code set.  It also logs the error to the
// RPC server subsystem since internal errors really should not occur.  The
// context parameter is only used in the log message and may be empty if it's
// not needed.
func (server *ServerCore) internalRPCError(errStr, context string) *btcjson.RPCError {
	logStr := errStr
	if context != "" {
		logStr = context + ": " + errStr
	}
	server.logger.Error(logStr)
	return btcjson.NewRPCError(btcjson.ErrRPCInternal.Code, errStr)
}
