// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
// nolint: forcetypeassert
package rpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/websocket"
	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/rpcutli"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const (
	// websocketSendBufferSize is the number of elements the send channel
	// can queue before blocking.  Note that this only applies to requests
	// handled directly in the websocket client input handler or the async
	// handler since notifications have their own queuing mechanism
	// independent of the send channel buffer.
	websocketSendBufferSize = 50
)

// Should be singleton in the system
var wsManager *WsManager

// WebsocketHandler handles a new websocket client by creating a new wsClient,
// starting it, and blocking until the connection closes.  Since it blocks, it
// must be run in a separate goroutine.  It should be invoked from the websocket
// server handler which runs each new connection in a new goroutine thereby
// satisfying the requirement.
func (server *MultiChainRPC) WebsocketHandler(conn *websocket.Conn, remoteAddr string,
	authenticated bool, isAdmin bool) {

	// Clear the read deadline that was set before the websocket hijacked
	// the connection.
	if err := conn.SetReadDeadline(timeZeroVal); err != nil {
		log.Error().Err(err).Msg("cannot set read deadline")
	}

	// Limit max number of websocket clients.
	server.logger.Info().Str("remote", remoteAddr).Msg("New websocket client")
	server.logger.Info().Msg(fmt.Sprintf("%v", server.wsManager))

	server.logger.Info().Msg("Get num Client")
	if server.wsManager.NumClients()+1 > server.cfg.MaxWebsockets {

		server.logger.Info().Str("remote", remoteAddr).Int("MaxNumber", server.cfg.MaxWebsockets).Msg("Max websocket clients exceeded")
		if err := conn.Close(); err != nil {
			log.Error().Err(err).Msg("cannot close connection")
		}
		return
	}

	server.logger.Info().Msg("Create new client")
	// Create a new websocket client to handle the new websocket connection
	// and wait for it to shutdown.  Once it has shutdown (and hence
	// disconnected), remove it and any notifications it registered for.
	client, err := newWebsocketClient(wsManager, conn, remoteAddr, authenticated, isAdmin)
	if err != nil {
		server.logger.Error().Str("remote", remoteAddr).Err(err).Msg("Failed to serve client")
		if err := conn.Close(); err != nil {
			log.Error().Err(err).Msg("cannot close connection")
		}
		return
	}
	server.logger.Info().Msg("Register client")
	server.wsManager.AddClient(client)
	server.logger.Info().Msg("Start client")
	client.Start()
	server.logger.Info().Msg("Wait shut down")
	client.WaitForShutdown()
	server.logger.Info().Msg("Unregister client")
	server.wsManager.RemoveClient(client)
	server.logger.Info().Str("remote", remoteAddr).Msg("Disconnected websocket client")
}

type IWebsocketManager interface {
	Start(ctx context.Context)
}

// wsNotificationManager is a connection and notification manager used for
// websockets.  It allows websocket clients to register for notifications they
// are interested in.  When an event happens elsewhere in the code such as
// transactions being added to the memory pool or block connects/disconnects,
// the notification manager is provided with the relevant details needed to
// figure out which websocket clients need to be notified based on what they
// have registered for and notifies them accordingly.  It is also used to keep
// track of all connected websocket clients.
type WsManager struct {
	rpcutli.ToolsXt
	server *MultiChainRPC

	// queueNotification queues a notification for handling.
	queueNotification chan interface{}

	// notificationMsgs feeds notificationHandler with notifications
	// and client (un)registeration requests from a queue as well as
	// registeration and unregisteration requests from clients.
	notificationMsgs chan interface{}

	handler *wsHandler

	logger     zerolog.Logger
	numClients chan int
}

const notificationChanLen = 1000

// newWsNotificationManager returns a new notification manager ready for use.
// See wsNotificationManager for more details.
func WebSocketManager(server *MultiChainRPC) *WsManager {
	if wsManager != nil {
		return wsManager
	}
	wsManager = &WsManager{
		handler:           WebSocketHandlers(server),
		server:            server,
		queueNotification: make(chan interface{}, notificationChanLen),
		notificationMsgs:  make(chan interface{}),
		numClients:        make(chan int),
		logger:            log,
	}

	return wsManager
}

func (m *WsManager) Start(ctx context.Context) {
	go m.queueHandler(ctx)
	go m.notificationHandler(ctx)
}

// AddClient adds the passed websocket client to the notification manager.
func (m *WsManager) AddClient(wsc *wsClient) {
	m.queueNotification <- (*notificationRegisterClient)(wsc)
}

// RemoveClient removes the passed websocket client and all notifications
// registered for it.
func (m *WsManager) RemoveClient(wsc *wsClient) {
	m.queueNotification <- (*notificationUnregisterClient)(wsc)
}

// RegisterBlockUpdates requests block update notifications to the passed
// websocket client.
func (m *WsManager) RegisterBlockUpdates(wsc *wsClient, shardID uint32) {
	m.queueNotification <- &notificationRegisterBlocks{
		wsc:     wsc,
		shardID: shardID,
	}
}

// UnregisterBlockUpdates removes block update notifications for the passed
// websocket client.
func (m *WsManager) UnregisterBlockUpdates(wsc *wsClient, shardID uint32) {
	m.queueNotification <- &notificationUnregisterBlocks{
		wsc:     wsc,
		shardID: shardID,
	}
}

// UnregisterTxOutAddressRequest removes a request from the passed websocket
// client to be notified when a transaction spends to the passed address.
func (m *WsManager) UnregisterTxOutAddressRequest(chain *cprovider.ChainProvider, wsc *wsClient, addr string) {
	m.queueNotification <- &notificationUnregisterAddr{
		wsc:   wsc,
		addr:  addr,
		chain: chain,
	}
}

// RegisterTxOutAddressRequests requests notifications to the passed websocket
// client when a transaction output spends to the passed address.
func (m *WsManager) RegisterTxOutAddressRequests(chain *cprovider.ChainProvider, wsc *wsClient, addrs []string) {
	m.queueNotification <- &notificationRegisterAddr{
		wsc:   wsc,
		addrs: addrs,
		chain: chain,
	}
}

// RegisterNewMempoolTxsUpdates requests notifications to the passed websocket
// client when new transactions are added to the memory pool.
func (m *WsManager) RegisterNewMempoolTxsUpdates(wsc *wsClient) {
	m.queueNotification <- (*notificationRegisterNewMempoolTxs)(wsc)
}

// UnregisterNewMempoolTxsUpdates removes notifications to the passed websocket
// client when new transaction are added to the memory pool.
func (m *WsManager) UnregisterNewMempoolTxsUpdates(wsc *wsClient) {
	m.queueNotification <- (*notificationUnregisterNewMempoolTxs)(wsc)
}

// RegisterSpentRequests requests a notification when each of the passed
// outpoints is confirmed spent (contained in a block connected to the main
// BlockChain) for the passed websocket client.  The request is automatically
// removed once the notification has been sent.
func (m *WsManager) RegisterSpentRequests(chain *cprovider.ChainProvider, wsc *wsClient, ops []*wire.OutPoint) {
	m.queueNotification <- &notificationRegisterSpent{
		wsc:   wsc,
		ops:   ops,
		chain: chain,
	}
}

// UnregisterSpentRequest removes a request from the passed websocket client
// to be notified when the passed outpoint is confirmed spent (contained in a
// block connected to the main BlockChain).
func (m *WsManager) UnregisterSpentRequest(chain *cprovider.ChainProvider, wsc *wsClient, op *wire.OutPoint) {
	m.queueNotification <- &notificationUnregisterSpent{
		wsc:   wsc,
		op:    op,
		chain: chain,
	}
}

// queueHandler manages a queue of empty interfaces, reading from in and
// sending the oldest unsent to out.  This handler stops when either of the
// in or quit channels are closed, and closes out before returning, without
// waiting to send any variables still remaining in the queue.
func queueHandler(ctx context.Context, in chan interface{}, out chan<- interface{}) {
	var q []interface{}
	var dequeue chan<- interface{}
	skipQueue := out
	var next interface{}
out:
	for {
		select {
		case n, ok := <-in:
			if !ok {
				// Sender closed input channel.
				break out
			}

			// Either send to out immediately if skipQueue is
			// non-nil (queue is empty) and reader is ready,
			// or append to the queue and send later.
			select {
			case skipQueue <- n:
			default:
				q = append(q, n)
				dequeue = out
				skipQueue = nil
				next = q[0]
			}

		case dequeue <- next:
			copy(q, q[1:])
			q[len(q)-1] = nil // avoid leak
			q = q[:len(q)-1]
			if len(q) == 0 {
				dequeue = nil
				skipQueue = out
			} else {
				next = q[0]
			}

		case <-ctx.Done():
			break out
		}
	}
	close(out)
}

// queueHandler maintains a queue of notifications and notification handler
// control messages.
func (m *WsManager) queueHandler(ctx context.Context) {
	queueHandler(ctx, m.queueNotification, m.notificationMsgs)
}

type wsBlockNotification struct {
	Block *jaxutil.Block
	Chain *cprovider.ChainProvider
}

// nolint: structcheck
type wsTransactionNotification struct {
	Client *wsClient
	isNew  bool
	tx     *jaxutil.Tx
	Chain  *cprovider.ChainProvider
}

//// Notification types
type (
	notificationBlockConnected      wsBlockNotification
	notificationBlockDisconnected   wsBlockNotification
	notificationTxAcceptedByMempool wsTransactionNotification
)

// Notification control requests
type (
	notificationRegisterClient   wsClient
	notificationUnregisterClient wsClient
	notificationRegisterBlocks   struct {
		wsc     *wsClient
		shardID uint32
	}
)

type notificationUnregisterBlocks struct {
	wsc     *wsClient
	shardID uint32
}

type (
	notificationRegisterNewMempoolTxs   wsClient
	notificationUnregisterNewMempoolTxs wsClient
	notificationRegisterSpent           struct {
		wsc   *wsClient
		ops   []*wire.OutPoint
		chain *cprovider.ChainProvider
	}
)

type notificationUnregisterSpent struct {
	chain *cprovider.ChainProvider
	wsc   *wsClient
	op    *wire.OutPoint
}

type notificationRegisterAddr struct {
	chain *cprovider.ChainProvider
	wsc   *wsClient
	addrs []string
}

type notificationUnregisterAddr struct {
	chain *cprovider.ChainProvider
	wsc   *wsClient
	addr  string
}

// notificationHandler reads notifications and control messages from the queue
// handler and processes one at a time.
func (m *WsManager) notificationHandler(ctx context.Context) {
	m.logger.Info().Msg("Run notificationHandler")
	childCtx := ctx
	// clients is a map of all currently connected websocket clients.
	clients := make(map[chan struct{}]*wsClient)

	// Maps used to hold lists of websocket clients to be notified on
	// certain events.  Each websocket client also keeps maps for the events
	// which have multiple triggers to make removal from these lists on
	// connection close less horrendously expensive.
	//
	// Where possible, the quit channel is used as the unique id for a client
	// since it is quite a bit more efficient than using the entire struct.
	blockNotifications := make(map[chan struct{}]*wsClient)

	shardIDClients := make(map[chan struct{}]map[uint32]bool)
	shardIDBlockNotifications := make(map[uint32]map[chan struct{}]*wsClient)
	txNotifications := make(map[chan struct{}]*wsClient)
	watchedOutPoints := make(map[wire.OutPoint]map[chan struct{}]*wsClient)
	watchedAddrs := make(map[string]map[chan struct{}]*wsClient)

out:
	for {
		select {
		case n, ok := <-m.notificationMsgs:
			if !ok {
				// queueHandler quit.
				break out
			}
			switch n := n.(type) {
			case *notificationBlockConnected:
				block := n.Block

				// Skip iterating through all txs if no
				// tx notification requests exist.
				if len(watchedOutPoints) != 0 || len(watchedAddrs) != 0 {
					for _, tx := range block.Transactions() {
						m.notifyForTx(n.Chain, watchedOutPoints,
							watchedAddrs, tx, block)
					}
				}
				shardID := n.Chain.ChainCtx.ShardID()
				if clientsMap, ok := shardIDBlockNotifications[shardID]; ok {
					if len(clientsMap) != 0 {
						m.notifyBlockConnected(n.Chain, clientsMap, block)
						m.notifyFilteredBlockConnected(n.Chain, clientsMap, block)
					}
				}

			case *notificationBlockDisconnected:
				block := n.Block
				shardID := n.Chain.ChainCtx.ShardID()
				if clientsMap, ok := shardIDBlockNotifications[shardID]; ok {
					if len(blockNotifications) != 0 {
						m.notifyBlockDisconnected(n.Chain, clientsMap, block)
						m.notifyFilteredBlockDisconnected(n.Chain, clientsMap, block)
					}
				}
			case *notificationTxAcceptedByMempool:
				if n.isNew && len(txNotifications) != 0 {
					m.notifyForNewTx(n.Chain, txNotifications, n.tx)
				}
				m.notifyForTx(n.Chain, watchedOutPoints, watchedAddrs, n.tx, nil)
				m.notifyRelevantTxAccepted(n.Chain, n.tx, clients)

			case *notificationRegisterBlocks:
				wsc := n.wsc
				if bnMap, ok := shardIDBlockNotifications[n.shardID]; !ok {
					shardIDBlockNotifications[n.shardID] = make(map[chan struct{}]*wsClient)
					shardIDBlockNotifications[n.shardID][wsc.quit] = wsc
				} else {
					bnMap[wsc.quit] = wsc
				}

				// this block optimises unsubscribing of the given client from all subscriptions
				// in a case when the connection is lost for any reason
				if shardMap, ok := shardIDClients[wsc.quit]; !ok {
					shardMap = make(map[uint32]bool)
					shardMap[n.shardID] = true
					shardIDClients[wsc.quit] = shardMap
				} else {
					if _, okk := shardMap[n.shardID]; !okk {
						shardMap[n.shardID] = true
					}
				}

			case *notificationUnregisterBlocks:
				wsc := n.wsc
				if bnMap, ok := shardIDBlockNotifications[n.shardID]; ok {
					delete(bnMap, wsc.quit)
				}

			case *notificationRegisterClient:
				wsc := (*wsClient)(n)
				clients[wsc.quit] = wsc

			case *notificationUnregisterClient:
				wsc := (*wsClient)(n)
				// Remove any requests made by the client as well as
				// the client itself.
				delete(blockNotifications, wsc.quit)

				if shardMap, ok := shardIDClients[wsc.quit]; ok {
					for shardID := range shardMap {
						// unsubscribe from block notifications
						if bnMap, ok := shardIDBlockNotifications[shardID]; ok {
							delete(bnMap, wsc.quit)
						}
					}
				}

				delete(txNotifications, wsc.quit)
				for k := range wsc.spentRequests {
					op := k
					m.removeSpentRequest(watchedOutPoints, wsc, &op)
				}
				for addr := range wsc.addrRequests {
					m.removeAddrRequest(watchedAddrs, wsc, addr)
				}
				delete(clients, wsc.quit)

			case *notificationRegisterSpent:
				m.addSpentRequests(n.chain, watchedOutPoints, n.wsc, n.ops)

			case *notificationUnregisterSpent:
				m.removeSpentRequest(watchedOutPoints, n.wsc, n.op)

			case *notificationRegisterAddr:
				m.addAddrRequests(watchedAddrs, n.wsc, n.addrs)

			case *notificationUnregisterAddr:
				m.removeAddrRequest(watchedAddrs, n.wsc, n.addr)

			case *notificationRegisterNewMempoolTxs:
				fmt.Println("we registered")
				wsc := (*wsClient)(n)
				txNotifications[wsc.quit] = wsc

			case *notificationUnregisterNewMempoolTxs:
				wsc := (*wsClient)(n)
				delete(txNotifications, wsc.quit)

			default:
				m.logger.Warn().Msg("Unhandled notification type")
			}

		case m.numClients <- len(clients):

		case <-childCtx.Done():
			// RPC server shutting down.
			break out
		}
	}

	for _, c := range clients {
		c.Disconnect()
	}
}

// notifyForTx examines the inputs and outputs of the passed transaction,
// notifying websocket clients of outputs spending to a watched address
// and inputs spending a watched outpoint.
func (m *WsManager) notifyForTx(chain *cprovider.ChainProvider, ops map[wire.OutPoint]map[chan struct{}]*wsClient,
	addrs map[string]map[chan struct{}]*wsClient, tx *jaxutil.Tx, block *jaxutil.Block) {

	if len(ops) != 0 {
		m.notifyForTxIns(chain, ops, tx, block)
	}
	if len(addrs) != 0 {
		m.notifyForTxOuts(chain, ops, addrs, tx, block)
	}
}

// notifyForTxOuts examines each transaction output, notifying interested
// websocket clients of the transaction if an output spends to a watched
// address.  A spent notification request is automatically registered for
// the client for each matching output.
// nolint: staticcheck
func (m *WsManager) notifyForTxOuts(chain *cprovider.ChainProvider, ops map[wire.OutPoint]map[chan struct{}]*wsClient, addrs map[string]map[chan struct{}]*wsClient, tx *jaxutil.Tx, block *jaxutil.Block) {
	// Nothing to do if nobody is listening for address notifications.
	if len(addrs) == 0 {
		return
	}

	txHex := ""
	wscNotified := make(map[chan struct{}]struct{})
	for i, txOut := range tx.MsgTx().TxOut {
		_, txAddrs, _, err := txscript.ExtractPkScriptAddrs(
			txOut.PkScript, chain.ChainParams)
		if err != nil {
			continue
		}

		for _, txAddr := range txAddrs {
			cmap, ok := addrs[txAddr.EncodeAddress()]
			if !ok {
				continue
			}

			if txHex == "" {
				txHex = txHexString(tx.MsgTx())
			}
			ntfn := jaxjson.NewRecvTxNtfn(txHex, blockDetails(block,
				tx.Index()))

			marshalledJSON, err := jaxjson.MarshalCmd(nil, chain.ChainCtx.ShardID(), ntfn)
			if err != nil {
				m.logger.Error().Err(err).Msg("Failed to marshal processedtx notification")
				continue
			}

			op := []*wire.OutPoint{wire.NewOutPoint(tx.Hash(), uint32(i))}
			for wscQuit, wsc := range cmap {
				m.addSpentRequests(chain, ops, wsc, op)

				if _, ok := wscNotified[wscQuit]; !ok {
					wscNotified[wscQuit] = struct{}{}
					if err := wsc.QueueNotification(marshalledJSON); err != nil {
						log.Error().Err(err).Msg("cannot queue notification")
					}
				}
			}
		}
	}
}

// notifyForTxIns examines the inputs of the passed transaction and sends
// interested websocket clients a redeemingtx notification if any inputs
// spend a watched output.  If block is non-nil, any matching spent
// requests are removed.
func (m *WsManager) notifyForTxIns(chain *cprovider.ChainProvider, ops map[wire.OutPoint]map[chan struct{}]*wsClient, tx *jaxutil.Tx, block *jaxutil.Block) {
	// Nothing to do if nobody is watching outpoints.
	if len(ops) == 0 {
		return
	}

	txHex := ""
	wscNotified := make(map[chan struct{}]struct{})
	for _, txIn := range tx.MsgTx().TxIn {
		prevOut := &txIn.PreviousOutPoint
		if cmap, ok := ops[*prevOut]; ok {
			if txHex == "" {
				txHex = txHexString(tx.MsgTx())
			}
			marshalledJSON, err := newRedeemingTxNotification(chain, txHex, tx.Index(), block)
			if err != nil {
				m.logger.Warn().Err(err).Msg("Failed to marshal redeemingtx notification")
				continue
			}
			for wscQuit, wsc := range cmap {
				if block != nil {
					m.removeSpentRequest(ops, wsc, prevOut)
				}

				if _, ok := wscNotified[wscQuit]; !ok {
					wscNotified[wscQuit] = struct{}{}
					if err := wsc.QueueNotification(marshalledJSON); err != nil {
						log.Error().Err(err).Msg("cannot queue notification")
					}
				}
			}
		}
	}
}

// txHexString returns the serialized transaction encoded in hexadecimal.
func txHexString(tx *wire.MsgTx) string {
	buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
	// Ignore Serialize's error, as writing to a bytes.buffer cannot fail.
	if err := tx.Serialize(buf); err != nil {
		log.Error().Err(err).Msg("cannot serialize tx")
	}
	return hex.EncodeToString(buf.Bytes())
}

// newRedeemingTxNotification returns a new marshalled redeemingtx notification
// with the passed parameters.
// nolint: staticcheck
func newRedeemingTxNotification(chain *cprovider.ChainProvider, txHex string, index int, block *jaxutil.Block) ([]byte, error) {
	// Create and marshal the notification.
	ntfn := jaxjson.NewRedeemingTxNtfn(txHex, blockDetails(block, index))
	return jaxjson.MarshalCmd(nil, chain.ChainCtx.ShardID(), ntfn)
}

// blockDetails creates a BlockDetails struct to include in btcws notifications
// from a block and a transaction's block index.
func blockDetails(block *jaxutil.Block, txIndex int) *jaxjson.BlockDetails {
	if block == nil {
		return nil
	}
	return &jaxjson.BlockDetails{
		Height: block.Height(),
		Hash:   block.Hash().String(),
		Index:  txIndex,
		Time:   block.MsgBlock().Header.Timestamp().Unix(),
	}
}

// removeSpentRequest modifies a map of watched outpoints to remove the
// websocket client wsc from the set of clients to be notified when a
// watched outpoint is spent.  If wsc is the last client, the outpoint
// key is removed from the map.
func (m *WsManager) removeSpentRequest(ops map[wire.OutPoint]map[chan struct{}]*wsClient,
	wsc *wsClient, op *wire.OutPoint) {

	// Remove the request tracking from the client.
	delete(wsc.spentRequests, *op)

	// Remove the client from the list to notify.
	notifyMap, ok := ops[*op]
	if !ok {
		m.logger.Warn().Str("address", wsc.addr).Msg("Attempt to remove nonexistent spent request for websocket client")
		return
	}
	delete(notifyMap, wsc.quit)

	// Remove the map entry altogether if there are
	// no more clients interested in it.
	if len(notifyMap) == 0 {
		delete(ops, *op)
	}
}

// addSpentRequests modifies a map of watched outpoints to sets of websocket
// clients to add a new request watch all of the outpoints in ops and create
// and send a notification when spent to the websocket client wsc.
func (m *WsManager) addSpentRequests(chain *cprovider.ChainProvider, opMap map[wire.OutPoint]map[chan struct{}]*wsClient,
	wsc *wsClient, ops []*wire.OutPoint) {

	for _, op := range ops {
		// Track the request in the client as well so it can be quickly
		// be removed on disconnect.
		wsc.spentRequests[*op] = struct{}{}

		// Add the client to the list to notify when the outpoint is seen.
		// Create the list as needed.
		cmap, ok := opMap[*op]
		if !ok {
			cmap = make(map[chan struct{}]*wsClient)
			opMap[*op] = cmap
		}
		cmap[wsc.quit] = wsc
	}

	// Check if any transactions spending these outputs already exists in
	// the mempool, if so send the notification immediately.
	spends := make(map[chainhash.Hash]*jaxutil.Tx)
	for _, op := range ops {
		spend := chain.TxMemPool.CheckSpend(*op)
		if spend != nil {
			m.logger.Debug().Msg(fmt.Sprintf("Found existing mempool spend for outpoint<%v>: %v", op, spend.Hash()))
			spends[*spend.Hash()] = spend
		}
	}

	for _, spend := range spends {
		m.notifyForTx(chain, opMap, nil, spend, nil)
	}
}

// notifyForNewTx notifies websocket clients that have registered for updates
// when a new transaction is added to the memory pool.
func (m *WsManager) notifyForNewTx(chain *cprovider.ChainProvider, clients map[chan struct{}]*wsClient, tx *jaxutil.Tx) {
	txHashStr := tx.Hash().String()
	mtx := tx.MsgTx()

	var amount int64
	for _, txOut := range mtx.TxOut {
		amount += txOut.Value
	}

	ntfn := jaxjson.NewTxAcceptedNtfn(txHashStr, jaxutil.Amount(amount).ToCoin(chain.ChainCtx.IsBeacon()))
	marshalledJSON, err := jaxjson.MarshalCmd(nil, chain.ChainCtx.ShardID(), ntfn)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to marshal tx notification")
		return
	}

	var verboseNtfn *jaxjson.TxAcceptedVerboseNtfn
	var marshalledJSONVerbose []byte
	for _, wsc := range clients {
		if wsc.verboseTxUpdates {
			if marshalledJSONVerbose != nil {
				if err := wsc.QueueNotification(marshalledJSONVerbose); err != nil {
					log.Error().Err(err).Msg("cannot queue notification")
				}
				continue
			}

			rawTx, err := m.CreateTxRawResult(chain.ChainCtx.Params(), mtx, txHashStr, nil,
				"", 0, 0)
			if err != nil {
				return
			}

			verboseNtfn = jaxjson.NewTxAcceptedVerboseNtfn(*rawTx)
			marshalledJSONVerbose, err = jaxjson.MarshalCmd(nil, chain.ChainCtx.ShardID(), verboseNtfn)
			if err != nil {
				m.logger.Error().Err(err).Msg("Failed to marshal verbose tx notification")
				return
			}
			if err := wsc.QueueNotification(marshalledJSONVerbose); err != nil {
				log.Error().Err(err).Msg("cannot queue notification")
			}
		} else {
			if err := wsc.QueueNotification(marshalledJSON); err != nil {
				log.Error().Err(err).Msg("cannot queue notification")
			}
		}
	}
}

// notifyBlockConnected notifies websocket clients that have registered for
// block updates when a block is connected to the main BlockChain.
// nolint: staticcheck
func (m *WsManager) notifyBlockConnected(chain *cprovider.ChainProvider, clients map[chan struct{}]*wsClient,
	block *jaxutil.Block) {

	// Notify interested websocket clients about the connected block.
	ntfn := jaxjson.NewBlockConnectedNtfn(block.Hash().String(), block.Height(),
		block.MsgBlock().Header.Timestamp().Unix())
	marshalledJSON, err := jaxjson.MarshalCmd(nil, chain.ChainCtx.ShardID(), ntfn)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to marshal block connected notification")
		return
	}
	for _, wsc := range clients {
		if err := wsc.QueueNotification(marshalledJSON); err != nil {
			log.Error().Err(err).Msg("cannot queue notification")
		}
	}
}

// notifyFilteredBlockConnected notifies websocket clients that have registered for
// block updates when a block is connected to the main BlockChain.
func (m *WsManager) notifyFilteredBlockConnected(chain *cprovider.ChainProvider, clients map[chan struct{}]*wsClient,
	block *jaxutil.Block) {

	// Create the common portion of the notification that is the same for
	// every client.
	var w bytes.Buffer
	err := block.MsgBlock().Header.Write(&w)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to serialize header for filtered block")
		return
	}
	ntfn := jaxjson.NewFilteredBlockConnectedNtfn(block.Height(),
		hex.EncodeToString(w.Bytes()), nil)

	// Search for relevant transactions for each client and save them
	// serialized in hex encoding for the notification.
	subscribedTxs := make(map[chan struct{}][]string)
	for _, tx := range block.Transactions() {
		var txHex string
		for quitChan := range m.subscribedClients(chain, tx, clients) {
			if txHex == "" {
				txHex = txHexString(tx.MsgTx())
			}
			subscribedTxs[quitChan] = append(subscribedTxs[quitChan], txHex)
		}
	}
	for quitChan, wsc := range clients {
		// Add all discovered transactions for this client. For clients
		// that have no new-style filter, add the empty string slice.
		ntfn.SubscribedTxs = subscribedTxs[quitChan]

		// Marshal and queue notification.
		marshalledJSON, err := jaxjson.MarshalCmd(nil, chain.ChainCtx.ShardID(), ntfn)
		if err != nil {
			m.logger.Error().Err(err).Msg("Failed to marshal filtered block connected notification")
			return
		}
		if err := wsc.QueueNotification(marshalledJSON); err != nil {
			log.Error().Err(err).Msg("cannot queue notification")
		}
	}
}

// nolint: staticcheck
func (m *WsManager) notifyBlockDisconnected(chain *cprovider.ChainProvider, clients map[chan struct{}]*wsClient, block *jaxutil.Block) {
	// Skip notification creation if no clients have requested block
	// connected/disconnected notifications.
	if len(clients) == 0 {
		return
	}

	// Notify interested websocket clients about the disconnected block.
	ntfn := jaxjson.NewBlockDisconnectedNtfn(block.Hash().String(),
		block.Height(), block.MsgBlock().Header.Timestamp().Unix())
	marshalledJSON, err := jaxjson.MarshalCmd(nil, chain.ChainCtx.ShardID(), ntfn)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to marshal block disconnected notification")
		return
	}
	for _, wsc := range clients {
		if err := wsc.QueueNotification(marshalledJSON); err != nil {
			log.Error().Err(err).Msg("cannot queue notification")
		}
	}
}

// NumClients returns the number of clients actively being served.
func (m *WsManager) NumClients() (n int) {
	return <-m.numClients
}

//
// subscribedClients returns the set of all websocket client quit channels that
// are registered to receive notifications regarding tx, either due to tx
// spending a watched output or outputting to a watched address.  Matching
// client's filters are updated based on this transaction's outputs and output
// addresses that may be relevant for a client.
func (m *WsManager) subscribedClients(chain *cprovider.ChainProvider, tx *jaxutil.Tx, clients map[chan struct{}]*wsClient) map[chan struct{}]struct{} {
	// Use a map of client quit channels as keys to prevent duplicates when
	// multiple inputs and/or outputs are relevant to the client.
	subscribed := make(map[chan struct{}]struct{})

	msgTx := tx.MsgTx()
	for _, input := range msgTx.TxIn {
		for quitChan, wsc := range clients {
			wsc.Lock()
			filter := wsc.filterData
			wsc.Unlock()
			if filter == nil {
				continue
			}
			filter.mu.Lock()
			if filter.existsUnspentOutPoint(&input.PreviousOutPoint) {
				subscribed[quitChan] = struct{}{}
			}
			filter.mu.Unlock()
		}
	}

	for i, output := range msgTx.TxOut {
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(
			output.PkScript, chain.ChainParams)
		if err != nil {
			// Clients are not able to subscribe to
			// nonstandard or non-address outputs.
			continue
		}
		for quitChan, wsc := range clients {
			wsc.Lock()
			filter := wsc.filterData
			wsc.Unlock()
			if filter == nil {
				continue
			}
			filter.mu.Lock()
			for _, a := range addrs {
				if filter.existsAddress(a) {
					subscribed[quitChan] = struct{}{}
					op := wire.OutPoint{
						Hash:  *tx.Hash(),
						Index: uint32(i),
					}
					filter.addUnspentOutPoint(&op)
				}
			}
			filter.mu.Unlock()
		}
	}

	return subscribed
}

//

// notifyFilteredBlockDisconnected notifies websocket clients that have registered for
// block updates when a block is disconnected from the main BlockChain (due to a
// reorganize).
func (m *WsManager) notifyFilteredBlockDisconnected(chain *cprovider.ChainProvider, clients map[chan struct{}]*wsClient, block *jaxutil.Block) {
	// Skip notification creation if no clients have requested block
	// connected/disconnected notifications.
	if len(clients) == 0 {
		return
	}

	// Notify interested websocket clients about the disconnected block.
	var w bytes.Buffer
	err := block.MsgBlock().Header.Write(&w)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to serialize header for filtered block disconnected notification")
		return
	}
	ntfn := jaxjson.NewFilteredBlockDisconnectedNtfn(block.Height(),
		hex.EncodeToString(w.Bytes()))
	marshalledJSON, err := jaxjson.MarshalCmd(nil, chain.ChainCtx.ShardID(), ntfn)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to marshal filtered block disconnected notification")
		return
	}
	for _, wsc := range clients {
		if err := wsc.QueueNotification(marshalledJSON); err != nil {
			log.Error().Err(err).Msg("cannot queue notification")
		}
	}
}

//
// notifyRelevantTxAccepted examines the inputs and outputs of the passed
// transaction, notifying websocket clients of outputs spending to a watched
// address and inputs spending a watched outpoint.  Any outputs paying to a
// watched address result in the output being watched as well for future
// notifications.
func (m *WsManager) notifyRelevantTxAccepted(chain *cprovider.ChainProvider, tx *jaxutil.Tx, clients map[chan struct{}]*wsClient) {
	clientsToNotify := m.subscribedClients(chain, tx, clients)

	if len(clientsToNotify) != 0 {
		n := jaxjson.NewRelevantTxAcceptedNtfn(txHexString(tx.MsgTx()))
		marshalled, err := jaxjson.MarshalCmd(nil, chain.ChainCtx.ShardID(), n)
		if err != nil {
			m.logger.Error().Err(err).Msg("Failed to marshal notification")
			return
		}
		for quitChan := range clientsToNotify {
			if err := clients[quitChan].QueueNotification(marshalled); err != nil {
				log.Error().Err(err).Msg("cannot queue notification")
			}
		}
	}
}

//
// addAddrRequests adds the websocket client wsc to the address to client set
// addrMap so wsc will be notified for any mempool or block transaction outputs
// spending to any of the addresses in addrs.
func (m *WsManager) addAddrRequests(addrMap map[string]map[chan struct{}]*wsClient,
	wsc *wsClient, addrs []string) {

	for _, addr := range addrs {
		// Track the request in the client as well so it can be quickly be
		// removed on disconnect.
		wsc.addrRequests[addr] = struct{}{}

		// Add the client to the set of clients to notify when the
		// outpoint is seen.  Create map as needed.
		cmap, ok := addrMap[addr]
		if !ok {
			cmap = make(map[chan struct{}]*wsClient)
			addrMap[addr] = cmap
		}
		cmap[wsc.quit] = wsc
	}
}

//
// removeAddrRequest removes the websocket client wsc from the address to
// client set addrs so it will no longer receive notification updates for
// any transaction outputs send to addr.
func (m *WsManager) removeAddrRequest(addrs map[string]map[chan struct{}]*wsClient,
	wsc *wsClient, addr string) {

	// Remove the request tracking from the client.
	delete(wsc.addrRequests, addr)

	// Remove the client from the list to notify.
	cmap, ok := addrs[addr]
	if !ok {
		m.logger.Warn().Msg(fmt.Sprintf("Attempt to remove nonexistent addr request <%s> for websocket client %s", addr, wsc.addr))
		return
	}
	delete(cmap, wsc.quit)

	// Remove the map entry altogether if there are no more clients
	// interested in it.
	if len(cmap) == 0 {
		delete(addrs, addr)
	}
}

// notifyMempoolTx passes a transaction accepted by mempool to the
// notification manager for transaction notification processing.  If
// isNew is true, the tx is is a new transaction, rather than one
// added to the mempool during a reorg.
func (m *WsManager) notifyMempoolTx(tx *jaxutil.Tx, isNew bool, provider *cprovider.ChainProvider) {
	n := &notificationTxAcceptedByMempool{
		isNew: isNew,
		tx:    tx,
		Chain: provider, // todo
	}

	// As notifyMempoolTx will be called by mempool and the RPC server
	// may no longer be running, use a select statement to unblock
	// enqueuing the notification once the RPC server has begun
	// shutting down.
	m.queueNotification <- n
}

// wsResponse houses a message to send to a connected websocket client as
// well as a channel to reply on when the message is sent.
type wsResponse struct {
	msg      []byte
	doneChan chan bool
}

// ErrRescanReorg defines the error that is returned when an unrecoverable
// reorganize is detected during a rescan.
var ErrRescanReorg = jaxjson.RPCError{
	Code:    jaxjson.ErrRPCDatabase,
	Message: "Reorganize",
}
