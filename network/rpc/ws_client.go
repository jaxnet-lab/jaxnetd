package rpc

import (
	"container/list"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/websocket"
	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/node/chain"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/node/encoder"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
	"io"
	"net"
	"sync"
)

type semaphore chan struct{}

func makeSemaphore(n int) semaphore {
	return make(chan struct{}, n)
}

func (s semaphore) acquire() { s <- struct{}{} }
func (s semaphore) release() { <-s }

// wsClient provides an abstraction for handling a websocket client.  The
// overall data flow is split into 3 main goroutines, a possible 4th goroutine
// for long-running operations (only started if request is made), and a
// websocket manager which is used to allow things such as broadcasting
// requested notifications to all connected websocket clients.   Inbound
// messages are read via the inHandler goroutine and generally dispatched to
// their own handler.  However, certain potentially long-running operations such
// as rescans, are sent to the asyncHander goroutine and are limited to one at a
// time.  There are two outbound message types - one for responding to client
// requests and another for async notifications.  Responses to client requests
// use SendMessage which employs a buffered channel thereby limiting the number
// of outstanding requests that can be made.  Notifications are sent via
// QueueNotification which implements a queue via notificationQueueHandler to
// ensure sending notifications from other subsystems can't block.  Ultimately,
// all messages are sent via the outHandler.
type wsClient struct {
	sync.Mutex

	manager *WsManager
	chain   chain.IChainCtx
	// conn is the underlying websocket connection.
	conn *websocket.Conn

	// disconnected indicated whether or not the websocket client is
	// disconnected.
	disconnected bool

	// addr is the remote address of the client.
	addr string

	// authenticated specifies whether a client has been authenticated
	// and therefore is allowed to communicated over the websocket.
	authenticated bool

	// isAdmin specifies whether a client may change the state of the server;
	// false means its access is only to the limited set of RPC calls.
	isAdmin bool

	// sessionID is a random ID generated for each client when connected.
	// These IDs may be queried by a client using the session RPC.  A change
	// to the session ID indicates that the client reconnected.
	sessionID uint64

	// verboseTxUpdates specifies whether a client has requested verbose
	// information about all new transactions.
	verboseTxUpdates bool

	// addrRequests is a set of addresses the caller has requested to be
	// notified about.  It is maintained here so all requests can be removed
	// when a wallet disconnects.  Owned by the notification manager.
	addrRequests map[string]struct{}

	// spentRequests is a set of unspent Outpoints a wallet has requested
	// notifications for when they are spent by a processed transaction.
	// Owned by the notification manager.
	spentRequests map[wire.OutPoint]struct{}

	// filterData is the new generation transaction filter backported from
	// github.com/decred/dcrd for the new backported `loadtxfilter` and
	// `rescanblocks` methods.
	filterData *wsClientFilter

	logger zerolog.Logger

	// Networking infrastructure.
	serviceRequestSem semaphore
	ntfnChan          chan []byte
	sendChan          chan wsResponse
	quit              chan struct{}
	wg                sync.WaitGroup
}

// newWebsocketClient returns a new websocket client given the notification
// manager, websocket connection, remote address, and whether or not the client
// has already been authenticated (via HTTP Basic access authentication).  The
// returned client is ready to start.  Once started, the client will process
// incoming and outgoing messages in separate goroutines complete with queuing
// and asynchrous handling for long-running operations.
func newWebsocketClient(manager *WsManager, conn *websocket.Conn,
	remoteAddr string, authenticated bool, isAdmin bool) (*wsClient, error) {

	sessionID, err := encoder.RandomUint64()
	if err != nil {
		return nil, err
	}

	client := &wsClient{
		conn:              conn,
		addr:              remoteAddr,
		authenticated:     authenticated,
		isAdmin:           isAdmin,
		sessionID:         sessionID,
		manager:           manager,
		addrRequests:      make(map[string]struct{}),
		spentRequests:     make(map[wire.OutPoint]struct{}),
		logger:            log,
		serviceRequestSem: makeSemaphore(manager.server.cfg.MaxConcurrentReqs),
		ntfnChan:          make(chan []byte, 1), // nonblocking sync
		sendChan:          make(chan wsResponse, websocketSendBufferSize),
		quit:              make(chan struct{}),
	}
	return client, nil
}

func (c *wsClient) shouldLogReadError(err error) bool {
	// No logging when the connetion is being forcibly disconnected.
	select {
	case <-c.quit:
		return false
	default:
	}

	// No logging when the connection has been disconnected.
	if err == io.EOF {
		return false
	}
	if opErr, ok := err.(*net.OpError); ok && !opErr.Temporary() {
		return false
	}

	return true
}

// inHandler handles all incoming messages for the websocket connection.  It
// must be run as a goroutine.
func (c *wsClient) inHandler() {
out:
	for {
		// Break out of the loop once the quit channel has been closed.
		// Use a non-blocking select here so we fall through otherwise.
		select {
		case <-c.quit:
			break out
		default:
		}

		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			// Log the error if it's not due to disconnecting.
			if c.shouldLogReadError(err) {
				c.logger.Error().Str("address", c.addr).Err(err).Msg("Websocket receive error from")
			}
			break out
		}

		var request jaxjson.Request
		err = json.Unmarshal(msg, &request)
		if err != nil {
			if !c.authenticated {
				break out
			}

			jsonErr := &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCParse.Code,
				Message: "Failed to parse request: " + err.Error(),
			}

			if c.manager == nil {
				c.logger.Error().Msg("WsManager is not initialized")
				continue
			}

			reply, err := c.manager.server.createMarshalledReply(nil, nil, jsonErr)
			if err != nil {
				c.logger.Error().Err(err).Msg("Failed to marshal parse failure")
				continue
			}
			c.SendMessage(reply, nil)
			continue
		}

		// The JSON-RPC 1.0 spec defines that notifications must have their "ID"
		// set to null and states that notifications do not have a response.
		//
		// A JSON-RPC 2.0 notification is a request with "json-rpc":"2.0", and
		// without an "ID" member. The specification states that notifications
		// must not be responded to. JSON-RPC 2.0 permits the null value as a
		// valid request ID, therefore such requests are not notifications.
		//
		// Bitcoin Core serves requests with "ID":null or even an absent "ID",
		// and responds to such requests with "ID":null in the response.
		//
		// Btcd does not respond to any request without and "ID" or "ID":null,
		// regardless the indicated JSON-RPC protocol version unless RPC quirks
		// are enabled. With RPC quirks enabled, such requests will be responded
		// to if the reqeust does not indicate JSON-RPC version.
		//
		// RPC quirks can be enabled by the user to avoid compatibility issues
		// with software relying on Core's behavior.
		if request.ID == nil /*&& !(c.manager.server.cfg.Quirks && request.JsonRPC == "") */ {
			if !c.authenticated {
				break out
			}
			continue
		}

		cmd := ParseCmd(&request)
		if cmd.Err != nil {
			if !c.authenticated {
				break out
			}

			reply, err := c.manager.server.createMarshalledReply(cmd.ID, nil, cmd.Err)
			if err != nil {
				c.logger.Error().Err(err).Msg("Failed to marshal parse failure ")
				continue
			}
			c.SendMessage(reply, nil)
			continue
		}
		c.manager.logger.Debug().Msg(fmt.Sprintf("Received command <%s> from %s for shard %d", cmd.Method, c.addr, cmd.ShardID))
		// Check auth.  The client is immediately disconnected if the
		// first request of an unauthentiated websocket client is not
		// the authenticate request, an authenticate request is received
		// when the client is already authenticated, or incorrect
		// authentication credentials are provided in the request.
		switch authCmd, ok := cmd.Cmd.(*jaxjson.AuthenticateCmd); {
		case c.authenticated && ok:
			c.logger.Warn().Msg(fmt.Sprintf("Websocket client %s is already authenticated", c.addr))
			break out
		case !c.authenticated && !ok:
			c.logger.Warn().Msg("Unauthenticated websocket message received")
			break out
		case !c.authenticated:
			// Check credentials.
			login := authCmd.Username + ":" + authCmd.Passphrase
			auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(login))
			authSha := sha256.Sum256([]byte(auth))
			cmp := subtle.ConstantTimeCompare(authSha[:], c.manager.server.authSHA[:])
			limitcmp := subtle.ConstantTimeCompare(authSha[:], c.manager.server.limitAuthSHA[:])
			if cmp != 1 && limitcmp != 1 {
				c.logger.Warn().Msg("Auth failure.")
				break out
			}
			c.authenticated = true
			c.isAdmin = cmp == 1

			// Marshal and send response.
			reply, err := c.manager.server.createMarshalledReply(cmd.ID, nil, nil)
			if err != nil {
				c.logger.Error().Err(err).Msg("Failed to marshal authenticate")
				continue
			}
			c.SendMessage(reply, nil)
			continue
		}

		// Check if the client is using limited RPC credentials and
		// error when not authorized to call this RPC.
		if !c.isAdmin {
			if _, ok := rpcLimited[request.Method]; !ok {
				jsonErr := &jaxjson.RPCError{
					Code:    jaxjson.ErrRPCInvalidParams.Code,
					Message: "limited user not authorized for this Method",
				}
				// Marshal and send response.
				reply, err := c.manager.server.createMarshalledReply(request.ID, nil, jsonErr)
				if err != nil {
					c.manager.logger.Error().Msg("Failed to marshal parse failure ")
					continue
				}
				c.SendMessage(reply, nil)
				continue
			}
		}

		// Asynchronously handle the request.  A semaphore is used to
		// limit the number of concurrent requests currently being
		// serviced.  If the semaphore can not be acquired, simply wait
		// until a request finished before reading the next RPC request
		// from the websocket client.
		//
		// This could be a little fancier by timing out and erroring
		// when it takes too long to service the request, but if that is
		// done, the read of the next request should not be blocked by
		// this semaphore, otherwise the next request will be read and
		// will probably sit here for another few seconds before timing
		// out as well.  This will cause the total timeout duration for
		// later requests to be much longer than the check here would
		// imply.
		//
		// If a timeout is added, the semaphore acquiring should be
		// moved inside of the new goroutine with a select statement
		// that also reads a time.After channel.  This will unblock the
		// read of the next request from the websocket client and allow
		// many requests to be waited on concurrently.
		c.serviceRequestSem.acquire()
		go func() {
			c.serviceRequest(cmd)
			c.serviceRequestSem.release()
		}()
	}

	// Ensure the connection is closed.
	c.Disconnect()
	c.wg.Done()
	c.manager.logger.Trace().Str("address", c.addr).Msg("Websocket client input handler done for")
}

// serviceRequest services a parsed RPC request by looking up and executing the
// appropriate RPC handler.  The response is marshalled and sent to the
// websocket client.
func (c *wsClient) serviceRequest(r *ParsedRPCCmd) {
	var (
		result interface{}
		err    error
	)

	var provider *cprovider.ChainProvider
	if r.ShardID != 0 {
		shard, ok := c.manager.server.shardRPCs[r.ShardID]
		if !ok {
			jsonErr := &btcjson.RPCError{
				Code:    btcjson.ErrRPCInvalidParams.Code,
				Message: "Shard is not found by provided shard id",
			}
			// Marshal and send error response.
			reply, err := c.manager.server.createMarshalledReply(r.ID, nil, jsonErr)
			if err != nil {
				c.logger.Error().Msg("Failed to marshal parse failure ")
				return
			}
			c.SendMessage(reply, nil)
			return
		}
		provider = shard.chainProvider
	} else {
		provider = c.manager.server.beaconRPC.chainProvider
	}

	// Lookup the websocket extension for the command and if it doesn't
	// exist fallback to handling the command as a standard command.
	wsHandler, ok := c.manager.handler.handlers[r.Method]
	if ok {
		result, err = wsHandler(provider, c, r.Cmd)
	} else {
		result, err = c.handleRequestScope(r)
	}
	reply, err := c.manager.server.createMarshalledReply(r.ID, result, err)
	if err != nil {
		c.manager.logger.Error().Str("command", r.Method).Err(err).Msg("Failed to marshal reply command")
		return
	}
	c.SendMessage(reply, nil)
}

// notificationQueueHandler handles the queuing of outgoing notifications for
// the websocket client.  This runs as a muxer for various sources of input to
// ensure that queuing up notifications to be sent will not block.  Otherwise,
// slow clients could bog down the other systems (such as the mempool or block
// manager) which are queuing the data.  The data is passed on to outHandler to
// actually be written.  It must be run as a goroutine.
func (c *wsClient) notificationQueueHandler() {
	c.manager.logger.Info().Msg("Run Handler")
	ntfnSentChan := make(chan bool, 1) // nonblocking sync

	// pendingNtfns is used as a queue for notifications that are ready to
	// be sent once there are no outstanding notifications currently being
	// sent.  The waiting flag is used over simply checking for items in the
	// pending list to ensure cleanup knows what has and hasn't been sent
	// to the outHandler.  Currently no special cleanup is needed, however
	// if something like a done channel is added to notifications in the
	// future, not knowing what has and hasn't been sent to the outHandler
	// (and thus who should respond to the done channel) would be
	// problematic without using this approach.
	pendingNtfns := list.New()
	waiting := false
out:
	for {
		select {
		// This channel is notified when a message is being queued to
		// be sent across the network socket.  It will either send the
		// message immediately if a send is not already in progress, or
		// queue the message to be sent once the other pending messages
		// are sent.
		case msg := <-c.ntfnChan:
			if !waiting {
				c.SendMessage(msg, ntfnSentChan)
			} else {
				pendingNtfns.PushBack(msg)
			}
			waiting = true

		// This channel is notified when a notification has been sent
		// across the network socket.
		case <-ntfnSentChan:
			// No longer waiting if there are no more messages in
			// the pending messages queue.
			next := pendingNtfns.Front()
			if next == nil {
				waiting = false
				continue
			}

			// Notify the outHandler about the next item to
			// asynchronously send.
			msg := pendingNtfns.Remove(next).([]byte)
			c.SendMessage(msg, ntfnSentChan)

		case <-c.quit:
			break out
		}
	}

	// Drain any wait channels before exiting so nothing is left waiting
	// around to send.
cleanup:
	for {
		select {
		case <-c.ntfnChan:
		case <-ntfnSentChan:
		default:
			break cleanup
		}
	}
	c.wg.Done()
	c.manager.logger.Trace().Str("address", c.addr).Msg("Websocket client notification queue handler done")
}

// outHandler handles all outgoing messages for the websocket connection.  It
// must be run as a goroutine.  It uses a buffered channel to serialize output
// messages while allowing the sender to continue running asynchronously.  It
// must be run as a goroutine.
func (c *wsClient) outHandler() {
out:
	for {
		// Send any messages ready for send until the quit channel is
		// closed.
		select {
		case r := <-c.sendChan:
			err := c.conn.WriteMessage(websocket.TextMessage, r.msg)
			if err != nil {
				c.Disconnect()
				break out
			}
			if r.doneChan != nil {
				r.doneChan <- true
			}

		case <-c.quit:
			break out
		}
	}

	// Drain any wait channels before exiting so nothing is left waiting
	// around to send.
cleanup:
	for {
		select {
		case r := <-c.sendChan:
			if r.doneChan != nil {
				r.doneChan <- false
			}
		default:
			break cleanup
		}
	}
	c.wg.Done()
	c.manager.logger.Trace().Str("address", c.addr).Msg("Websocket client output handler done for")
}

// SendMessage sends the passed json to the websocket client.  It is backed
// by a buffered channel, so it will not block until the send channel is full.
// Note however that QueueNotification must be used for sending async
// notifications instead of the this function.  This approach allows a limit to
// the number of outstanding requests a client can make without preventing or
// blocking on async notifications.
func (c *wsClient) SendMessage(marshalledJSON []byte, doneChan chan bool) {
	// Don't send the message if disconnected.
	if c.Disconnected() {
		if doneChan != nil {
			doneChan <- false
		}
		return
	}
	c.sendChan <- wsResponse{msg: marshalledJSON, doneChan: doneChan}
}

//
// ErrClientQuit describes the error where a client send is not processed due
// to the client having already been disconnected or dropped.
var ErrClientQuit = errors.New("client quit")

// QueueNotification queues the passed notification to be sent to the websocket
// client.  This function, as the name implies, is only intended for
// notifications since it has additional logic to prevent other subsystems, such
// as the memory pool and block manager, from blocking even when the send
// channel is full.
//
// If the client is in the process of shutting down, this function returns
// ErrClientQuit.  This is intended to be checked by long-running notification
// handlers to stop processing if there is no more work needed to be done.
func (c *wsClient) QueueNotification(marshalledJSON []byte) error {
	// Don't queue the message if disconnected.
	if c.Disconnected() {
		return ErrClientQuit
	}

	c.ntfnChan <- marshalledJSON
	return nil
}

// Disconnected returns whether or not the websocket client is disconnected.
func (c *wsClient) Disconnected() bool {
	c.Lock()
	isDisconnected := c.disconnected
	c.Unlock()

	return isDisconnected
}

// Disconnect disconnects the websocket client.
func (c *wsClient) Disconnect() {
	c.Lock()
	defer c.Unlock()

	// Nothing to do if already disconnected.
	if c.disconnected {
		return
	}

	c.manager.logger.Trace().Str("address", c.addr).Msg("Disconnecting websocket client")
	close(c.quit)
	c.conn.Close()
	c.disconnected = true
}

// Start begins processing input and output messages.
func (c *wsClient) Start() {
	c.manager.logger.Trace().Str("address", c.addr).Msg("Starting websocket client")

	// Start processing input and output.
	c.wg.Add(3)

	go c.inHandler()
	go c.notificationQueueHandler()
	go c.outHandler()
}

// WaitForShutdown blocks until the websocket client goroutines are stopped
// and the connection is closed.
func (c *wsClient) WaitForShutdown() {
	c.wg.Wait()
}

func (c *wsClient) handleRequestScope(r *ParsedRPCCmd) (interface{}, error) {
	var mux Mux

	switch r.Scope {
	case "node":
		mux = c.manager.server.nodeRPC.Mux
	case "beacon", "chain":
		mux = c.manager.server.beaconRPC.Mux
	case "shard":
		shardRPC, ok := c.manager.server.shardRPCs[r.ShardID]
		if !ok {
			log.Debug().Msgf("Not existing shardID in RPC request %d", r.ShardID)
			return nil, fmt.Errorf("unknown shardID %d", r.ShardID)
		}
		mux = shardRPC.Mux
	default:
		log.Debug().Msgf("Unknown RPC request scope %s", r.Scope)
		return nil, fmt.Errorf("unknown scope %s", r.Scope)
	}

	return mux.HandleCommand(r, nil)
}
