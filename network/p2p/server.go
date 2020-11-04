// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package p2p

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/corelog"
	"gitlab.com/jaxnet/core/shard.core/database"
	"gitlab.com/jaxnet/core/shard.core/network/addrmgr"
	"gitlab.com/jaxnet/core/shard.core/network/connmgr"
	"gitlab.com/jaxnet/core/shard.core/network/netsync"
	"gitlab.com/jaxnet/core/shard.core/network/peer"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/node/mempool"
	"gitlab.com/jaxnet/core/shard.core/types"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
	"go.uber.org/zap"
)

type INodeServer interface {
	Start()
	Stop() error
	NotifyNewTransactions(txns []*mempool.TxDesc)
}

// zeroHash is the zero value hash (all zeros).  It is defined as a convenience.
var zeroHash chainhash.Hash

// broadcastMsg provides the ability to house a bitcoin message to be broadcast
// to all connected peers except specified excluded peers.
type broadcastMsg struct {
	message      wire.Message
	excludePeers []*serverPeer
}

// broadcastInventoryAdd is a type used to declare that the InvVect it contains
// needs to be added to the rebroadcast map
type broadcastInventoryAdd RelayMsg

// broadcastInventoryDel is a type used to declare that the InvVect it contains
// needs to be removed from the rebroadcast map
type broadcastInventoryDel *types.InvVect

// cfHeaderKV is a tuple of a filter header and its associated block hash. The
// struct is used to cache cfcheckpt responses.
type cfHeaderKV struct {
	blockHash    chainhash.Hash
	filterHeader chainhash.Hash
}

// server provides a bitcoin server for handling communications to and from
// bitcoin peers.
type Server struct {
	// The following variables must only be used atomically.
	// Putting the uint64s first makes them 64-bit aligned for 32-bit systems.
	bytesReceived uint64 // Total bytes received from all peers since start.
	bytesSent     uint64 // Total bytes sent by all peers since start.
	started       int32
	shutdown      int32
	shutdownSched int32
	cfg           *Config
	addrManager   *addrmgr.AddrManager
	ConnManager   *connmgr.ConnManager
	nodeServer    INodeServer

	modifyRebroadcastInv chan interface{}
	newPeers             chan *serverPeer
	donePeers            chan *serverPeer
	banPeers             chan *serverPeer
	query                chan interface{}
	relayInv             chan RelayMsg
	broadcast            chan broadcastMsg
	peerHeightsUpdate    chan UpdatePeerHeightsMsg
	wg                   sync.WaitGroup
	quit                 chan struct{}

	nat NAT

	services wire.ServiceFlag

	// cfCheckptCaches stores a cached slice of filter headers for cfcheckpt
	// messages for each filter type.
	cfCheckptCaches    map[wire.FilterType][]cfHeaderKV
	cfCheckptCachesMtx sync.RWMutex

	// agentBlacklist is a list of blacklisted substrings by which to filter
	// user agents.
	agentBlacklist []string

	// agentWhitelist is a list of whitelisted user agent substrings, no
	// whitelisting will be applied if the list is empty or nil.
	agentWhitelist []string

	logger corelog.ILogger

	chain *cprovider.ChainProvider
}

// newServer returns a new shard.core p2p server configured to listen on addr for the
// bitcoin network type specified by ChainParams.  Use start to begin accepting
// connections from peers.
func NewServer(cfg *Config, chainProvider *cprovider.ChainProvider,
	amgr *addrmgr.AddrManager, opts ListenOpts) (*Server, error) {
	logger := chainProvider.Log().With(zap.String("server", "p2p"))
	chainCfg := chainProvider.Config()

	logger.Info("Starting Server", zap.Any("Peers", cfg.Peers))
	services := defaultServices
	// if cfg.NoPeerBloomFilters {
	//	services &^= wire.SFNodeBloom
	// }

	if chainCfg.NoCFilters {
		services &^= wire.SFNodeCF
	}

	var listeners []net.Listener
	var nat NAT
	if !cfg.DisableListen {
		var err error
		listeners, nat, err = initListeners(cfg, opts.DefaultPort,
			amgr, opts.Listeners, services, logger)
		if err != nil {
			return nil, err
		}
		if len(listeners) == 0 {
			return nil, errors.New("no valid listen address")
		}
	}

	if len(cfg.AgentBlacklist) > 0 {
		logger.Info(fmt.Sprintf("User-agent blacklist %s", cfg.AgentBlacklist))
	}
	if len(cfg.AgentWhitelist) > 0 {
		logger.Info(fmt.Sprintf("User-agent whitelist %s", cfg.AgentWhitelist))
	}

	p2pServer := Server{
		chain:           chainProvider,
		cfg:             cfg,
		addrManager:     amgr,
		nat:             nat,
		services:        services,
		cfCheckptCaches: make(map[wire.FilterType][]cfHeaderKV),
		logger:          corelog.Adapter(logger),

		newPeers:             make(chan *serverPeer, chainCfg.MaxPeers),
		donePeers:            make(chan *serverPeer, chainCfg.MaxPeers),
		banPeers:             make(chan *serverPeer, chainCfg.MaxPeers),
		query:                make(chan interface{}),
		relayInv:             make(chan RelayMsg, chainCfg.MaxPeers),
		broadcast:            make(chan broadcastMsg, chainCfg.MaxPeers),
		quit:                 make(chan struct{}),
		modifyRebroadcastInv: make(chan interface{}),
		peerHeightsUpdate:    make(chan UpdatePeerHeightsMsg),
	}

	// Create a connection manager.
	targetOutbound := defaultTargetOutbound
	if chainCfg.MaxPeers < targetOutbound {
		targetOutbound = chainCfg.MaxPeers
	}
	cmgr, err := connmgr.New(&connmgr.Config{
		Listeners:      listeners,
		RetryDuration:  connectionRetryInterval,
		TargetOutbound: uint32(targetOutbound),
		Dial:           p2pServer.netDial,
		OnAccept:       p2pServer.inboundPeerConnected,
		OnConnection:   p2pServer.outboundPeerConnected,
		GetNewAddress:  p2pServer.newAddressHandler(len(cfg.ConnectPeers)),
	})
	if err != nil {
		return nil, err
	}

	p2pServer.ConnManager = cmgr

	return &p2pServer, nil
}

func (server *Server) Run(ctx context.Context) {
	// Already started?
	if atomic.AddInt32(&server.started, 1) != 1 {
		return
	}

	server.logger.Trace("Starting server")

	// Server startup time. Used for the uptime command for uptime calculation.
	server.chain.StartupTime = time.Now().Unix()

	// Start up persistent peers.
	permanentPeers := server.cfg.ConnectPeers
	if len(permanentPeers) == 0 {
		permanentPeers = server.cfg.Peers
	}

	for _, addr := range permanentPeers {
		netAddr, err := server.addrStringToNetAddr(addr)
		if err != nil {
			// return  err
		}

		go server.ConnManager.Connect(&connmgr.ConnReq{
			Addr:      netAddr,
			ShardID:   server.chain.ChainCtx.ShardID(),
			Permanent: true,
		})
	}

	// Start the peer handler which in turn starts the address and block
	// managers.
	server.wg.Add(1)
	go server.peerHandler()

	if server.nat != nil {
		server.wg.Add(1)
		go server.upnpUpdateThread()
	}

	// Start the rebroadcastHandler, which ensures user tx received by
	// the RPC server are rebroadcast until being included in a block.
	server.wg.Add(1)
	go server.rebroadcastHandler()

	<-ctx.Done()

	// Save fee estimator state in the database.
	server.chain.DB.Update(func(tx database.Tx) error {
		metadata := tx.Metadata()
		metadata.Put(mempool.EstimateFeeDatabaseKey, server.chain.FeeEstimator.Save())

		return nil
	})

	// Signal the remaining goroutines to quit.
	close(server.quit)

	server.wg.Wait()

	return
}

// Stop gracefully shuts down the Server by stopping and disconnecting all
// peers and the main listener.
func (server *Server) Stop() error {
	// Make sure this only happens once.
	if atomic.AddInt32(&server.shutdown, 1) != 1 {
		server.logger.Infof("Server is already in the process of shutting down")
		return nil
	}

	// Save fee estimator state in the database.
	server.chain.DB.Update(func(tx database.Tx) error {
		metadata := tx.Metadata()
		metadata.Put(mempool.EstimateFeeDatabaseKey, server.chain.FeeEstimator.Save())

		return nil
	})

	// Signal the remaining goroutines to quit.
	close(server.quit)
	return nil
}

// WaitForShutdown blocks until the main listener and peer handlers are stopped.
func (server *Server) WaitForShutdown() {
	server.wg.Wait()
}

// ScheduleShutdown schedules a Server shutdown after the specified duration.
// It also dynamically adjusts how often to warn the Server is going down based
// on remaining duration.
func (server *Server) ScheduleShutdown(duration time.Duration) {
	// Don't schedule shutdown more than once.
	if atomic.AddInt32(&server.shutdownSched, 1) != 1 {
		return
	}
	server.logger.Warnf("Server shutdown in %v", duration)
	go func() {
		remaining := duration
		tickDuration := dynamicTickDuration(remaining)
		done := time.After(remaining)
		ticker := time.NewTicker(tickDuration)
	out:
		for {
			select {
			case <-done:
				ticker.Stop()
				server.Stop()
				break out
			case <-ticker.C:
				remaining = remaining - tickDuration
				if remaining < time.Second {
					continue
				}

				// Change tick duration dynamically based on remaining time.
				newDuration := dynamicTickDuration(remaining)
				if tickDuration != newDuration {
					tickDuration = newDuration
					ticker.Stop()
					ticker = time.NewTicker(tickDuration)
				}
				server.logger.Warnf("Server shutdown in %v", remaining)
			}
		}
	}()
}

// hasServices returns whether or not the provided advertised service flags have
// all of the provided desired service flags set.
func hasServices(advertised, desired wire.ServiceFlag) bool {
	return advertised&desired == desired
}

func (server *Server) P2PConnManager() netsync.P2PConnManager {
	return &ConnManager{server: server}
}

func (server *Server) Query(value interface{}) {
	server.query <- value
}

// Only setup a function to return new addresses to connect to when
// not running in connect-only mode.  The simulation network is always
// in connect-only mode since it is only intended to connect to
// specified peers and actively avoid advertising and connecting to
// discovered peers in order to prevent it from becoming a public test
// network.
func (server *Server) newAddressHandler(connectPeersCount int) func() (net.Addr, error) {
	if connectPeersCount == 0 {
		return nil
	}

	return func() (net.Addr, error) {
		for tries := 0; tries < 100; tries++ {
			addr := server.addrManager.GetAddress()
			if addr == nil {
				break
			}

			// Address will not be invalid, local or unroutable
			// because addrmanager rejects those on addition.
			// Just check that we don't already have an address
			// in the same group so that we are not connecting
			// to the same network segment at the expense of
			// others.
			key := addrmgr.GroupKey(addr.NetAddress())
			if server.OutboundGroupCount(key) != 0 {
				continue
			}

			// only allow recent nodes (10mins) after we failed 30
			// times
			if tries < 30 && time.Since(addr.LastAttempt()) < 10*time.Minute {
				continue
			}

			// allow nondefault ports after 50 failed tries.
			if tries < 50 && fmt.Sprintf("%d", addr.NetAddress().Port) !=
				server.chain.ChainParams.DefaultPort {
				continue
			}

			// Mark an attempt for the valid address.
			server.addrManager.Attempt(addr.NetAddress())

			addrString := addrmgr.NetAddressKey(addr.NetAddress())
			return server.addrStringToNetAddr(addrString)
		}

		return nil, errors.New("no valid connect address")
	}
}

// AddRebroadcastInventory adds 'iv' to the list of inventories to be
// rebroadcasted at random intervals until they show up in a block.
func (server *Server) AddRebroadcastInventory(iv *types.InvVect, data interface{}) {
	// Ignore if shutting down.
	if atomic.LoadInt32(&server.shutdown) != 0 {
		return
	}

	server.modifyRebroadcastInv <- broadcastInventoryAdd{InvVect: iv, Data: data}
}

// RemoveRebroadcastInventory removes 'iv' from the list of items to be
// rebroadcasted if present.
func (server *Server) RemoveRebroadcastInventory(iv *types.InvVect) {
	// Ignore if shutting down.
	if atomic.LoadInt32(&server.shutdown) != 0 {
		return
	}

	server.modifyRebroadcastInv <- broadcastInventoryDel(iv)
}

// RelayTransactions generates and relays inventory vectors for all of the
// passed transactions to all connected peers.
func (server *Server) RelayTransactions(txns []*mempool.TxDesc) {
	for _, txD := range txns {
		iv := types.NewInvVect(types.InvTypeTx, txD.Tx.Hash())
		server.RelayInventory(iv, txD)
	}
}

// AnnounceNewTransactions generates and relays inventory vectors and notifies
// both websocket and getblocktemplate long poll clients of the passed
// transactions.  This function should be called whenever new transactions
// are added to the mempool.
func (server *Server) AnnounceNewTransactions(txns []*mempool.TxDesc) {
	// Generate and relay inventory vectors for all newly accepted
	// transactions.
	server.RelayTransactions(txns)

	// Notify both websocket and getblocktemplate long poll clients of all
	// newly accepted transactions.
	if server.nodeServer != nil {
		server.nodeServer.NotifyNewTransactions(txns)
	}
}

// Transaction has one confirmation on the main BlockChain. Now we can mark it as no
// longer needing rebroadcasting.
func (server *Server) TransactionConfirmed(tx *btcutil.Tx) {
	// Rebroadcasting is only necessary when the RPC server is active.
	if server.nodeServer == nil {
		return
	}

	iv := types.NewInvVect(types.InvTypeTx, tx.Hash())
	server.RemoveRebroadcastInventory(iv)
}

// handleQuery is the central handler for all queries and commands from other
// goroutines related to peer state.
func (server *Server) handleQuery(state *peerState, querymsg interface{}) {
	switch msg := querymsg.(type) {
	case GetConnCountMsg:
		nconnected := int32(0)
		state.forAllPeers(func(sp *serverPeer) {
			if sp.Connected() {
				nconnected++
			}
		})
		msg.Reply <- nconnected

	case GetPeersMsg:
		peers := make([]*serverPeer, 0, state.Count())
		state.forAllPeers(func(sp *serverPeer) {
			if !sp.Connected() {
				return
			}
			peers = append(peers, sp)
		})
		msg.Reply <- peers

	case ConnectNodeMsg:
		// TODO: duplicate oneshots?
		// Limit max number of total peers.
		if state.Count() >= server.chain.Config().MaxPeers {
			msg.Reply <- errors.New("max peers reached")
			return
		}
		for _, peer := range state.persistentPeers {
			if peer.Addr() == msg.Addr {
				if msg.Permanent {
					msg.Reply <- errors.New("peer already connected")
				} else {
					msg.Reply <- errors.New("peer exists as a permanent peer")
				}
				return
			}
		}

		netAddr, err := server.addrStringToNetAddr(msg.Addr)
		if err != nil {
			msg.Reply <- err
			return
		}

		// TODO: if too many, nuke a non-perm peer.
		go server.ConnManager.Connect(&connmgr.ConnReq{
			Addr:      netAddr,
			ShardID:   server.chain.ChainCtx.ShardID(),
			Permanent: msg.Permanent,
		})
		msg.Reply <- nil

	case RemoveNodeMsg:
		found := disconnectPeer(state.persistentPeers, msg.Cmp, func(sp *serverPeer) {
			// Keep group counts ok since we remove from
			// the list now.
			state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
		})

		if found {
			msg.Reply <- nil
		} else {
			msg.Reply <- errors.New("peer not found")
		}

	case GetOutboundGroup:
		count, ok := state.outboundGroups[msg.Key]
		if ok {
			msg.Reply <- count
		} else {
			msg.Reply <- 0
		}

	// Request a list of the persistent (added) peers.
	case GetAddedNodesMsg:
		// Respond with a slice of the relevant peers.
		peers := make([]*serverPeer, 0, len(state.persistentPeers))
		for _, sp := range state.persistentPeers {
			peers = append(peers, sp)
		}
		msg.Reply <- peers

	case DisconnectNodeMsg:
		// Check inbound peers. We pass a nil callback since we don't
		// require any additional actions on disconnect for inbound peers.
		found := disconnectPeer(state.inboundPeers, msg.Cmp, nil)
		if found {
			msg.Reply <- nil
			return
		}

		// Check outbound peers.
		found = disconnectPeer(state.outboundPeers, msg.Cmp, func(sp *serverPeer) {
			// Keep group counts ok since we remove from
			// the list now.
			state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
		})
		if found {
			// If there are multiple outbound connections to the same
			// ip:port, continue disconnecting them all until no such
			// peers are found.
			for found {
				found = disconnectPeer(state.outboundPeers, msg.Cmp, func(sp *serverPeer) {
					state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
				})
			}
			msg.Reply <- nil
			return
		}

		msg.Reply <- errors.New("peer not found")
	}
}

// disconnectPeer attempts to drop the connection of a targeted peer in the
// passed peer list. Targets are identified via usage of the passed
// `compareFunc`, which should return `true` if the passed peer is the target
// peer. This function returns true on success and false if the peer is unable
// to be located. If the peer is found, and the passed callback: `whenFound'
// isn't nil, we call it with the peer as the argument before it is removed
// from the peerList, and is disconnected from the server.
func disconnectPeer(peerList map[int32]*serverPeer, compareFunc func(*serverPeer) bool, whenFound func(*serverPeer)) bool {
	for addr, peer := range peerList {
		if compareFunc(peer) {
			if whenFound != nil {
				whenFound(peer)
			}

			// This is ok because we are not continuing
			// to iterate so won't corrupt the loop.
			delete(peerList, addr)
			peer.Disconnect()
			return true
		}
	}
	return false
}

func (server *Server) handlePeerRedirect(peerAddress, newAddress *wire.NetAddress) {
	netAddr := &net.TCPAddr{
		IP:   newAddress.IP,
		Port: int(newAddress.Port),
	}
	server.addrManager.Replace(peerAddress, newAddress)
	go server.ConnManager.Connect(&connmgr.ConnReq{
		Addr:      netAddr,
		ShardID:   server.chain.ChainCtx.ShardID(),
		Permanent: true,
	})
}

// inboundPeerConnected is invoked by the connection manager when a new inbound
// connection is established.  It initializes a new inbound server peer
// instance, associates it with the connection, and starts a goroutine to wait
// for disconnection.
func (server *Server) inboundPeerConnected(conn net.Conn) {
	sp := newServerPeer(newServerPeerHandler(server), false)

	sp.isWhitelisted = server.isWhitelisted(conn.RemoteAddr())
	sp.Peer = peer.NewInboundPeer(sp.newPeerConfig(), server.chain.BlockChain().Chain())
	sp.AssociateConnection(conn)
	go server.peerDoneHandler(sp)
}

// outboundPeerConnected is invoked by the connection manager when a new
// outbound connection is established.  It initializes a new outbound server
// peer instance, associates it with the relevant state such as the connection
// request instance and the connection itself, and finally notifies the address
// manager of the attempt.
func (server *Server) outboundPeerConnected(connReq *connmgr.ConnReq, conn net.Conn) {
	if server.cfg.DisableOutbound {
		server.logger.Debugf("Outbound Conn disabled: can't create outbound peer %s: %v", connReq.Addr)
		return
	}
	sp := newServerPeer(newServerPeerHandler(server), connReq.Permanent)

	p, err := peer.NewOutboundPeer(sp.newPeerConfig(), connReq.Addr.String(), server.chain.BlockChain().Chain())
	if err != nil {
		server.logger.Debugf("Cannot create outbound peer %s: %v", connReq.Addr, err)
		if connReq.Permanent {
			// server.ConnManager.Disconnect(connReq.ID())
			// } else {
			server.ConnManager.Remove(connReq.ID())
			// go server.ConnManager.NewConnReq()
		}
		return
	}

	sp.Peer = p
	sp.connReq = connReq
	sp.isWhitelisted = server.isWhitelisted(conn.RemoteAddr())
	sp.AssociateConnection(conn)
	go server.peerDoneHandler(sp)
}

// peerDoneHandler handles peer disconnects by notifiying the server that it's
// done along with other performing other desirable cleanup.
func (server *Server) peerDoneHandler(sp *serverPeer) {
	sp.WaitForDisconnect()
	server.donePeers <- sp

	// Only tell sync manager we are gone if we ever told it we existed.
	if sp.VerAckReceived() {
		server.chain.SyncManager.DonePeer(sp.Peer)

		// Evict any remaining orphans that were sent by the peer.
		numEvicted := server.chain.TxMemPool.RemoveOrphansByTag(mempool.Tag(sp.ID()))
		if numEvicted > 0 {
			server.logger.Debugf("Evicted %d %s from peer %v (id %d)",
				numEvicted, pickNoun(numEvicted, "orphan",
					"orphans"), sp, sp.ID())
		}
	}
	close(sp.quit)
}

// peerHandler is used to handle peer operations such as adding and removing
// peers to and from the server, banning peers, and broadcasting messages to
// peers.  It must be run in a goroutine.
func (server *Server) peerHandler() {
	// Start the address manager and sync manager, both of which are needed
	// by peers.  This is done here since their lifecycle is closely tied
	// to this handler and rather than adding more channels to sychronize
	// things, it's easier and slightly faster to simply start and stop them
	// in this handler.
	server.addrManager.Start()
	server.chain.SyncManager.Start()

	server.logger.Tracef("Starting peer handler")

	state := &peerState{
		inboundPeers:    make(map[int32]*serverPeer),
		persistentPeers: make(map[int32]*serverPeer),
		outboundPeers:   make(map[int32]*serverPeer),
		banned:          make(map[string]time.Time),
		outboundGroups:  make(map[string]int),
	}

	if !server.cfg.DisableDNSSeed {
		// Add peers discovered through DNS to the address manager.
		connmgr.SeedFromDNS(server.chain.ChainParams, defaultRequiredServices,
			server.netLookup, func(addrs []*wire.NetAddress) {
				// Bitcoind uses a lookup of the dns seeder here. This
				// is rather strange since the values looked up by the
				// DNS seed lookups will vary quite a lot.
				// to replicate this behaviour we put all addresses as
				// having come from the first one.
				server.addrManager.AddAddresses(addrs, addrs[0])
			})
	}
	go server.ConnManager.Start()

out:
	for {
		select {
		// New peers connected to the Server.
		case p := <-server.newPeers:
			server.handleAddPeerMsg(state, p)

		// Disconnected peers.
		case p := <-server.donePeers:
			server.handleDonePeerMsg(state, p)

		// Block accepted in mainchain or orphan, update peer height.
		case umsg := <-server.peerHeightsUpdate:
			server.handleUpdatePeerHeights(state, umsg)

		// Peer to ban.
		case p := <-server.banPeers:
			server.handleBanPeerMsg(state, p)

		// New inventory to potentially be relayed to other peers.
		case invMsg := <-server.relayInv:
			server.handleRelayInvMsg(state, invMsg)

		// Message to broadcast to all connected peers except those
		// which are excluded by the message.
		case bmsg := <-server.broadcast:
			server.handleBroadcastMsg(state, &bmsg)

		case qmsg := <-server.query:
			server.handleQuery(state, qmsg)

		case <-server.quit:
			// Disconnect all peers on Server shutdown.
			state.forAllPeers(func(sp *serverPeer) {
				server.logger.Tracef("Shutdown peer %s", sp)
				sp.Disconnect()
			})
			break out
		}
	}

	server.ConnManager.Stop()
	server.chain.SyncManager.Stop()
	server.addrManager.Stop()

	// Drain channels before exiting so nothing is left waiting around
	// to send.
cleanup:
	for {
		select {
		case <-server.newPeers:
		case <-server.donePeers:
		case <-server.peerHeightsUpdate:
		case <-server.relayInv:
		case <-server.broadcast:
		case <-server.query:
		default:
			break cleanup
		}
	}
	server.wg.Done()
	server.logger.Tracef("Peer handler done")
}

// RelayInventory relays the passed inventory vector to all connected peers
// that are not already known to have it.
func (server *Server) RelayInventory(invVect *types.InvVect, data interface{}) {
	server.relayInv <- RelayMsg{InvVect: invVect, Data: data}
}

// BroadcastMessage sends msg to all peers currently connected to the Server
// except those in the passed peers to exclude.
func (server *Server) BroadcastMessage(msg wire.Message, exclPeers ...*serverPeer) {
	// XXX: Need to determine if this is an alert that has already been
	// broadcast and refrain from broadcasting again.
	bmsg := broadcastMsg{message: msg, excludePeers: exclPeers}
	server.broadcast <- bmsg
}

// ConnectedCount returns the number of currently connected peers.
func (server *Server) ConnectedCount() int32 {
	replyChan := make(chan int32)

	server.query <- GetConnCountMsg{Reply: replyChan}

	return <-replyChan
}

// OutboundGroupCount returns the number of peers connected to the given
// outbound group key.
func (server *Server) OutboundGroupCount(key string) int {
	replyChan := make(chan int)
	server.query <- GetOutboundGroup{Key: key, Reply: replyChan}
	return <-replyChan
}

// AddBytesSent adds the passed number of bytes to the total bytes sent counter
// for the server.  It is safe for concurrent access.
func (server *Server) AddBytesSent(bytesSent uint64) {
	atomic.AddUint64(&server.bytesSent, bytesSent)
}

// AddBytesReceived adds the passed number of bytes to the total bytes received
// counter for the server.  It is safe for concurrent access.
func (server *Server) AddBytesReceived(bytesReceived uint64) {
	atomic.AddUint64(&server.bytesReceived, bytesReceived)
}

// NetTotals returns the sum of all bytes received and sent across the network
// for all peers.  It is safe for concurrent access.
func (server *Server) NetTotals() (uint64, uint64) {
	return atomic.LoadUint64(&server.bytesReceived),
		atomic.LoadUint64(&server.bytesSent)
}

// UpdatePeerHeights updates the heights of all peers who have have announced
// the latest connected main BlockChain block, or a recognized orphan. These height
// updates allow us to dynamically refresh peer heights, ensuring sync peer
// selection has access to the latest block heights for each peer.
func (server *Server) UpdatePeerHeights(latestBlkHash *chainhash.Hash, latestHeight int32, updateSource *peer.Peer) {
	server.peerHeightsUpdate <- UpdatePeerHeightsMsg{
		NewHash:    latestBlkHash,
		NewHeight:  latestHeight,
		OriginPeer: updateSource,
	}
}

// rebroadcastHandler keeps track of user submitted inventories that we have
// sent out but have not yet made it into a block. We periodically rebroadcast
// them in case our peers restarted or otherwise lost track of them.
func (server *Server) rebroadcastHandler() {
	// Wait 5 min before first tx rebroadcast.
	timer := time.NewTimer(5 * time.Minute)
	pendingInvs := make(map[types.InvVect]interface{})

out:
	for {
		select {
		case riv := <-server.modifyRebroadcastInv:
			switch msg := riv.(type) {
			// Incoming InvVects are added to our map of RPC txs.
			case broadcastInventoryAdd:
				pendingInvs[*msg.InvVect] = msg.Data

			// When an InvVect has been added to a block, we can
			// now remove it, if it was present.
			case broadcastInventoryDel:
				delete(pendingInvs, *msg)
			}

		case <-timer.C:
			// Any inventory we have has not made it into a block
			// yet. We periodically resubmit them until they have.
			for iv, data := range pendingInvs {
				ivCopy := iv
				server.RelayInventory(&ivCopy, data)
			}

			// Process at a random time up to 30mins (in seconds)
			// in the future.
			timer.Reset(time.Second *
				time.Duration(randomUint16Number(1800)))

		case <-server.quit:
			break out
		}
	}

	timer.Stop()

	// Drain channels before exiting so nothing is left waiting around
	// to send.
cleanup:
	for {
		select {
		case <-server.modifyRebroadcastInv:
		default:
			break cleanup
		}
	}
	server.wg.Done()
}

func (server *Server) upnpUpdateThread() {
	// Go off immediately to prevent code duplication, thereafter we renew
	// lease every 15 minutes.
	timer := time.NewTimer(0 * time.Second)
	lport, _ := strconv.ParseInt(server.chain.ChainParams.DefaultPort, 10, 16)
	first := true
out:
	for {
		select {
		case <-timer.C:
			// TODO: pick external port  more cleverly
			// TODO: know which ports we are listening to on an external net.
			// TODO: if specific listen port doesn't work then ask for wildcard
			// listen port?
			// XXX this assumes timeout is in seconds.
			listenPort, err := server.nat.AddPortMapping("tcp", int(lport), int(lport),
				"shard BlockChain p2p listen port", 20*60)
			if err != nil {
				server.logger.Warnf("can't add UPnP port mapping: %v", err)
			}
			if first && err == nil {
				// TODO: look this up periodically to see if upnp domain changed
				// and so did ip.
				externalIP, err := server.nat.GetExternalAddress()
				if err != nil {
					server.logger.Warnf("UPnP can't get external address: %v", err)
					continue out
				}
				na := wire.NewNetAddressIPPort(externalIP, uint16(listenPort),
					server.services)
				err = server.addrManager.AddLocalAddress(na, addrmgr.UpnpPrio)
				if err != nil {
					// XXX DeletePortMapping?
				}
				server.logger.Warnf("Successfully bound via UPnP to %s", addrmgr.NetAddressKey(na))
				first = false
			}
			timer.Reset(time.Minute * 15)
		case <-server.quit:
			break out
		}
	}

	timer.Stop()

	if err := server.nat.DeletePortMapping("tcp", int(lport), int(lport)); err != nil {
		server.logger.Warnf("unable to remove UPnP port mapping: %v", err)
	} else {
		server.logger.Debugf("successfully disestablished UPnP port mapping")
	}

	server.wg.Done()
}

// addrStringToNetAddr takes an address in the form of 'host:port' and returns
// a net.Addr which maps to the original address with any host names resolved
// to IP addresses.  It also handles tor addresses properly by returning a
// net.Addr that encapsulates the address.
func (server *Server) addrStringToNetAddr(addr string) (net.Addr, error) {
	host, strPort, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(strPort)
	if err != nil {
		return nil, err
	}

	// Skip if host is already an IP address.
	if ip := net.ParseIP(host); ip != nil {
		return &net.TCPAddr{
			IP:   ip,
			Port: port,
		}, nil
	}

	// Tor addresses cannot be resolved to an IP, so just return an onion
	// address instead.
	if strings.HasSuffix(host, ".onion") {
		if server.cfg.NoOnion {
			return nil, errors.New("tor has been disabled")
		}

		return &onionAddr{addr: addr}, nil
	}

	// Attempt to look up an IP address associated with the parsed host.
	ips, err := server.netLookup(host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %s", host)
	}

	return &net.TCPAddr{
		IP:   ips[0],
		Port: port,
	}, nil
}

// isWhitelisted returns whether the IP address is included in the whitelisted
// networks and IPs.
func (server *Server) isWhitelisted(addr net.Addr) bool {
	// TODO(mike): return whitelist
	// if len(server.cfg.Whitelists) == 0 {
	//	return false
	// }

	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		server.logger.Warnf("Unable to SplitHostPort on '%s': %v", addr, err)
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		server.logger.Warnf("Unable to parse IP '%s'", addr)
		return false
	}

	// for _, ipnet := range server.cfg.whitelists {
	//	if ipnet.Contains(ip) {
	//		return true
	//	}
	// }
	return false
}

func (server *Server) netLookup(host string) ([]net.IP, error) {
	if strings.HasSuffix(host, ".onion") {
		return nil, fmt.Errorf("attempt to resolve tor address %s", host)
	}

	return server.cfg.Lookup(host)
}

// netDial connects to the address on the named network using the appropriate
// dial function depending on the address and configuration options.  For
// example, .onion addresses will be dialed using the onion specific proxy if
// one was specified, but will otherwise use the normal dial function (which
// could itself use a proxy or not).
func (server *Server) netDial(addr net.Addr) (net.Conn, error) {
	if strings.Contains(addr.String(), ".onion:") {
		return server.cfg.Oniondial(addr.Network(), addr.String(),
			defaultConnectTimeout)
	}
	return server.cfg.Dial(addr.Network(), addr.String(), defaultConnectTimeout)
}

// dynamicTickDuration is a convenience function used to dynamically choose a
// tick duration based on remaining time.  It is primarily used during
// server shutdown to make shutdown warnings more frequent as the shutdown time
// approaches.
func dynamicTickDuration(remaining time.Duration) time.Duration {
	switch {
	case remaining <= time.Second*5:
		return time.Second
	case remaining <= time.Second*15:
		return time.Second * 5
	case remaining <= time.Minute:
		return time.Second * 15
	case remaining <= time.Minute*5:
		return time.Minute
	case remaining <= time.Minute*15:
		return time.Minute * 5
	case remaining <= time.Hour:
		return time.Minute * 15
	}
	return time.Hour
}
