// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package p2p

import (
	"bytes"
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
	"gitlab.com/jaxnet/core/shard.core/btcutil/bloom"
	"gitlab.com/jaxnet/core/shard.core/corelog"
	"gitlab.com/jaxnet/core/shard.core/database"
	"gitlab.com/jaxnet/core/shard.core/network/addrmgr"
	"gitlab.com/jaxnet/core/shard.core/network/connmgr"
	"gitlab.com/jaxnet/core/shard.core/network/netsync"
	"gitlab.com/jaxnet/core/shard.core/network/peer"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/node/encoder"
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
	excludePeers []*ServerPeer
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
	newPeers             chan *ServerPeer
	donePeers            chan *ServerPeer
	banPeers             chan *ServerPeer
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

		newPeers:             make(chan *ServerPeer, chainCfg.MaxPeers),
		donePeers:            make(chan *ServerPeer, chainCfg.MaxPeers),
		banPeers:             make(chan *ServerPeer, chainCfg.MaxPeers),
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
		OnAccept:       p2pServer.inboundPeerConnected,
		RetryDuration:  connectionRetryInterval,
		TargetOutbound: uint32(targetOutbound),
		Dial:           p2pServer.btcdDial,
		OnConnection:   p2pServer.outboundPeerConnected,
		GetNewAddress:  p2pServer.newAddressHandler(len(cfg.ConnectPeers)),
	})
	if err != nil {
		return nil, err
	}

	p2pServer.ConnManager = cmgr

	// Start up persistent peers.
	permanentPeers := cfg.ConnectPeers
	if len(permanentPeers) == 0 {
		permanentPeers = cfg.Peers
	}

	for _, addr := range permanentPeers {
		netAddr, err := p2pServer.addrStringToNetAddr(addr)
		if err != nil {
			return nil, err
		}

		go p2pServer.ConnManager.Connect(&connmgr.ConnReq{
			Addr:      netAddr,
			ShardID:   chainProvider.ChainCtx.ShardID(),
			Permanent: true,
		})
	}

	return &p2pServer, nil
}

// hasServices returns whether or not the provided advertised service flags have
// all of the provided desired service flags set.
func hasServices(advertised, desired wire.ServiceFlag) bool {
	return advertised&desired == desired
}

func (s *Server) P2PConnManager() netsync.P2PConnManager {
	return &ConnManager{server: s}
}

func (s *Server) Query(value interface{}) {
	s.query <- value
}

// Only setup a function to return new addresses to connect to when
// not running in connect-only mode.  The simulation network is always
// in connect-only mode since it is only intended to connect to
// specified peers and actively avoid advertising and connecting to
// discovered peers in order to prevent it from becoming a public test
// network.
func (s *Server) newAddressHandler(connectPeersCount int) func() (net.Addr, error) {
	if connectPeersCount == 0 {
		return nil
	}

	return func() (net.Addr, error) {
		for tries := 0; tries < 100; tries++ {
			addr := s.addrManager.GetAddress()
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
			if s.OutboundGroupCount(key) != 0 {
				continue
			}

			// only allow recent nodes (10mins) after we failed 30
			// times
			if tries < 30 && time.Since(addr.LastAttempt()) < 10*time.Minute {
				continue
			}

			// allow nondefault ports after 50 failed tries.
			if tries < 50 && fmt.Sprintf("%d", addr.NetAddress().Port) !=
				s.chain.ChainParams.DefaultPort {
				continue
			}

			// Mark an attempt for the valid address.
			s.addrManager.Attempt(addr.NetAddress())

			addrString := addrmgr.NetAddressKey(addr.NetAddress())
			return s.addrStringToNetAddr(addrString)
		}

		return nil, errors.New("no valid connect address")
	}
}

// AddRebroadcastInventory adds 'iv' to the list of inventories to be
// rebroadcasted at random intervals until they show up in a block.
func (s *Server) AddRebroadcastInventory(iv *types.InvVect, data interface{}) {
	// Ignore if shutting down.
	if atomic.LoadInt32(&s.shutdown) != 0 {
		return
	}

	s.modifyRebroadcastInv <- broadcastInventoryAdd{InvVect: iv, Data: data}
}

// RemoveRebroadcastInventory removes 'iv' from the list of items to be
// rebroadcasted if present.
func (s *Server) RemoveRebroadcastInventory(iv *types.InvVect) {
	// Ignore if shutting down.
	if atomic.LoadInt32(&s.shutdown) != 0 {
		return
	}

	s.modifyRebroadcastInv <- broadcastInventoryDel(iv)
}

// RelayTransactions generates and relays inventory vectors for all of the
// passed transactions to all connected peers.
func (s *Server) RelayTransactions(txns []*mempool.TxDesc) {
	for _, txD := range txns {
		iv := types.NewInvVect(types.InvTypeTx, txD.Tx.Hash())
		s.RelayInventory(iv, txD)
	}
}

// AnnounceNewTransactions generates and relays inventory vectors and notifies
// both websocket and getblocktemplate long poll clients of the passed
// transactions.  This function should be called whenever new transactions
// are added to the mempool.
func (s *Server) AnnounceNewTransactions(txns []*mempool.TxDesc) {
	// Generate and relay inventory vectors for all newly accepted
	// transactions.
	s.RelayTransactions(txns)

	// Notify both websocket and getblocktemplate long poll clients of all
	// newly accepted transactions.
	if s.nodeServer != nil {
		s.nodeServer.NotifyNewTransactions(txns)
	}
}

// Transaction has one confirmation on the main BlockChain. Now we can mark it as no
// longer needing rebroadcasting.
func (s *Server) TransactionConfirmed(tx *btcutil.Tx) {
	// Rebroadcasting is only necessary when the RPC server is active.
	if s.nodeServer == nil {
		return
	}

	iv := types.NewInvVect(types.InvTypeTx, tx.Hash())
	s.RemoveRebroadcastInventory(iv)
}

// pushTxMsg sends a tx message for the provided transaction hash to the
// connected peer.  An error is returned if the transaction hash is not known.
func (s *Server) pushTxMsg(sp *ServerPeer, hash *chainhash.Hash, doneChan chan<- struct{},
	waitChan <-chan struct{}, encoding encoder.MessageEncoding) error {

	// Attempt to fetch the requested transaction from the pool.  A
	// call could be made to check for existence first, but simply trying
	// to fetch a missing transaction results in the same behavior.
	tx, err := s.chain.TxMemPool.FetchTransaction(hash)
	if err != nil {
		s.logger.Tracef("Unable to fetch tx %v from transaction "+
			"pool: %v", hash, err)

		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}

	// Once we have fetched data wait for any previous operation to finish.
	if waitChan != nil {
		<-waitChan
	}

	sp.QueueMessageWithEncoding(tx.MsgTx(), doneChan, encoding)

	return nil
}

// pushBlockMsg sends a block message for the provided block hash to the
// connected peer.  An error is returned if the block hash is not known.
func (s *Server) pushBlockMsg(sp *ServerPeer, hash *chainhash.Hash, doneChan chan<- struct{},
	waitChan <-chan struct{}, encoding encoder.MessageEncoding) error {

	// Fetch the raw block bytes from the database.
	var blockBytes []byte
	err := sp.server.chain.DB.View(func(dbTx database.Tx) error {
		var err error
		blockBytes, err = dbTx.FetchBlock(hash)
		return err
	})
	if err != nil {
		s.logger.Tracef("Unable to fetch requested block hash %v: %v",
			hash, err)

		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}

	// Deserialize the block.
	var msgBlock = s.chain.ChainCtx.EmptyBlock()

	err = msgBlock.Deserialize(bytes.NewReader(blockBytes))
	if err != nil {
		s.logger.Tracef("Unable to deserialize requested block hash "+
			"%v: %v", hash, err)

		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}

	// Once we have fetched data wait for any previous operation to finish.
	if waitChan != nil {
		<-waitChan
	}

	// We only send the channel for this message if we aren't sending
	// an inv straight after.
	var dc chan<- struct{}
	continueHash := sp.continueHash
	sendInv := continueHash != nil && continueHash.IsEqual(hash)
	if !sendInv {
		dc = doneChan
	}
	sp.QueueMessageWithEncoding(&msgBlock, dc, encoding)

	// When the peer requests the final block that was advertised in
	// response to a getblocks message which requested more blocks than
	// would fit into a single message, send it a new inventory message
	// to trigger it to issue another getblocks message for the next
	// batch of inventory.
	if sendInv {
		best := sp.server.chain.BlockChain().BestSnapshot()
		invMsg := wire.NewMsgInvSizeHint(1)
		iv := types.NewInvVect(types.InvTypeBlock, &best.Hash)
		invMsg.AddInvVect(iv)
		sp.QueueMessage(invMsg, doneChan)
		sp.continueHash = nil
	}
	return nil
}

// pushMerkleBlockMsg sends a merkleblock message for the provided block hash to
// the connected peer.  Since a merkle block requires the peer to have a filter
// loaded, this call will simply be ignored if there is no filter loaded.  An
// error is returned if the block hash is not known.
func (s *Server) pushMerkleBlockMsg(sp *ServerPeer, hash *chainhash.Hash,
	doneChan chan<- struct{}, waitChan <-chan struct{}, encoding encoder.MessageEncoding) error {

	// Do not send a response if the peer doesn't have a filter loaded.
	if !sp.filter.IsLoaded() {
		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return nil
	}

	// Fetch the raw block bytes from the database.
	blk, err := sp.server.chain.BlockChain().BlockByHash(hash)
	if err != nil {
		s.logger.Tracef("Unable to fetch requested block hash %v: %v",
			hash, err)

		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}

	// Generate a merkle block by filtering the requested block according
	// to the filter for the peer.
	merkle, matchedTxIndices := bloom.NewMerkleBlock(blk, sp.filter)

	// Once we have fetched data wait for any previous operation to finish.
	if waitChan != nil {
		<-waitChan
	}

	// Send the merkleblock.  Only send the done channel with this message
	// if no transactions will be sent afterwards.
	var dc chan<- struct{}
	if len(matchedTxIndices) == 0 {
		dc = doneChan
	}
	sp.QueueMessage(merkle, dc)

	// Finally, send any matched transactions.
	blkTransactions := blk.MsgBlock().Transactions
	for i, txIndex := range matchedTxIndices {
		// Only send the done channel on the final transaction.
		var dc chan<- struct{}
		if i == len(matchedTxIndices)-1 {
			dc = doneChan
		}
		if txIndex < uint32(len(blkTransactions)) {
			sp.QueueMessageWithEncoding(blkTransactions[txIndex], dc,
				encoding)
		}
	}

	return nil
}

// handleQuery is the central handler for all queries and commands from other
// goroutines related to peer state.
func (s *Server) handleQuery(state *peerState, querymsg interface{}) {
	switch msg := querymsg.(type) {
	case GetConnCountMsg:
		nconnected := int32(0)
		state.forAllPeers(func(sp *ServerPeer) {
			if sp.Connected() {
				nconnected++
			}
		})
		msg.Reply <- nconnected

	case GetPeersMsg:
		peers := make([]*ServerPeer, 0, state.Count())
		state.forAllPeers(func(sp *ServerPeer) {
			if !sp.Connected() {
				return
			}
			peers = append(peers, sp)
		})
		msg.Reply <- peers

	case ConnectNodeMsg:
		// TODO: duplicate oneshots?
		// Limit max number of total peers.
		if state.Count() >= s.chain.Config().MaxPeers {
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

		netAddr, err := s.addrStringToNetAddr(msg.Addr)
		if err != nil {
			msg.Reply <- err
			return
		}

		// TODO: if too many, nuke a non-perm peer.
		go s.ConnManager.Connect(&connmgr.ConnReq{
			Addr:      netAddr,
			ShardID:   s.chain.ChainCtx.ShardID(),
			Permanent: msg.Permanent,
		})
		msg.Reply <- nil

	case RemoveNodeMsg:
		found := disconnectPeer(state.persistentPeers, msg.Cmp, func(sp *ServerPeer) {
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
		peers := make([]*ServerPeer, 0, len(state.persistentPeers))
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
		found = disconnectPeer(state.outboundPeers, msg.Cmp, func(sp *ServerPeer) {
			// Keep group counts ok since we remove from
			// the list now.
			state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
		})
		if found {
			// If there are multiple outbound connections to the same
			// ip:port, continue disconnecting them all until no such
			// peers are found.
			for found {
				found = disconnectPeer(state.outboundPeers, msg.Cmp, func(sp *ServerPeer) {
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
func disconnectPeer(peerList map[int32]*ServerPeer, compareFunc func(*ServerPeer) bool, whenFound func(*ServerPeer)) bool {
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

// newPeerConfig returns the configuration for the given ServerPeer.
func (s *Server) newPeerConfig(sp *ServerPeer) *peer.Config {
	return &peer.Config{
		Listeners: peer.MessageListeners{
			OnVersion:      sp.OnVersion,
			OnVerAck:       sp.OnVerAck,
			OnMemPool:      sp.OnMemPool,
			OnTx:           sp.OnTx,
			OnBlock:        sp.OnBlock,
			OnInv:          sp.OnInv,
			OnHeaders:      sp.OnHeaders,
			OnGetData:      sp.OnGetData,
			OnGetBlocks:    sp.OnGetBlocks,
			OnGetHeaders:   sp.OnGetHeaders,
			OnGetCFilters:  sp.OnGetCFilters,
			OnGetCFHeaders: sp.OnGetCFHeaders,
			OnGetCFCheckpt: sp.OnGetCFCheckpt,
			OnFeeFilter:    sp.OnFeeFilter,
			OnFilterAdd:    sp.OnFilterAdd,
			OnFilterClear:  sp.OnFilterClear,
			OnFilterLoad:   sp.OnFilterLoad,
			OnGetAddr:      sp.OnGetAddr,
			OnAddr:         sp.OnAddr,
			OnRead:         sp.OnRead,
			OnWrite:        sp.OnWrite,

			// Note: The reference client currently bans peers that send alerts
			// not signed with its key.  We could verify against their key, but
			// since the reference client is currently unwilling to support
			// other implementations' alert messages, we will not relay theirs.
			OnAlert: nil,
		},
		NewestBlock:      sp.newestBlock,
		HostToNetAddress: sp.server.addrManager.HostToNetAddress,
		Proxy:            s.cfg.Proxy,
		UserAgentName:    userAgentName,
		UserAgentVersion: userAgentVersion,
		// UserAgentComments: s.cfg.UserAgentComments,
		ChainParams:         sp.server.chain.ChainParams,
		Services:            sp.server.services,
		DisableRelayTx:      s.cfg.BlocksOnly,
		ProtocolVersion:     peer.MaxProtocolVersion,
		TrickleInterval:     s.cfg.TrickleInterval,
		ChainsPortsProvider: s.cfg.GetChainPort,
	}
}

// inboundPeerConnected is invoked by the connection manager when a new inbound
// connection is established.  It initializes a new inbound server peer
// instance, associates it with the connection, and starts a goroutine to wait
// for disconnection.
func (s *Server) inboundPeerConnected(conn net.Conn) {
	sp := newServerPeer(s, false)
	sp.isWhitelisted = s.isWhitelisted(conn.RemoteAddr())
	sp.Peer = peer.NewInboundPeer(s.newPeerConfig(sp), s.chain.BlockChain().Chain())
	sp.AssociateConnection(conn)
	go s.peerDoneHandler(sp)
}

// outboundPeerConnected is invoked by the connection manager when a new
// outbound connection is established.  It initializes a new outbound server
// peer instance, associates it with the relevant state such as the connection
// request instance and the connection itself, and finally notifies the address
// manager of the attempt.
func (s *Server) outboundPeerConnected(c *connmgr.ConnReq, conn net.Conn) {
	sp := newServerPeer(s, c.Permanent)
	p, err := peer.NewOutboundPeer(s.newPeerConfig(sp), c.Addr.String(), s.chain.BlockChain().Chain())
	if err != nil {
		s.logger.Debugf("Cannot create outbound peer %s: %v", c.Addr, err)
		if c.Permanent {
			s.ConnManager.Disconnect(c.ID())
		} else {
			s.ConnManager.Remove(c.ID())
			go s.ConnManager.NewConnReq()
		}
		return
	}
	sp.Peer = p
	sp.connReq = c
	sp.isWhitelisted = s.isWhitelisted(conn.RemoteAddr())
	sp.AssociateConnection(conn)
	go s.peerDoneHandler(sp)
}

// peerDoneHandler handles peer disconnects by notifiying the server that it's
// done along with other performing other desirable cleanup.
func (s *Server) peerDoneHandler(sp *ServerPeer) {
	sp.WaitForDisconnect()
	s.donePeers <- sp

	// Only tell sync manager we are gone if we ever told it we existed.
	if sp.VerAckReceived() {
		s.chain.SyncManager.DonePeer(sp.Peer)

		// Evict any remaining orphans that were sent by the peer.
		numEvicted := s.chain.TxMemPool.RemoveOrphansByTag(mempool.Tag(sp.ID()))
		if numEvicted > 0 {
			s.logger.Debugf("Evicted %d %s from peer %v (id %d)",
				numEvicted, pickNoun(numEvicted, "orphan",
					"orphans"), sp, sp.ID())
		}
	}
	close(sp.quit)
}

// peerHandler is used to handle peer operations such as adding and removing
// peers to and from the server, banning peers, and broadcasting messages to
// peers.  It must be run in a goroutine.
func (s *Server) peerHandler() {
	// Start the address manager and sync manager, both of which are needed
	// by peers.  This is done here since their lifecycle is closely tied
	// to this handler and rather than adding more channels to sychronize
	// things, it's easier and slightly faster to simply start and stop them
	// in this handler.
	s.addrManager.Start()
	s.chain.SyncManager.Start()

	s.logger.Tracef("Starting peer handler")

	state := &peerState{
		inboundPeers:    make(map[int32]*ServerPeer),
		persistentPeers: make(map[int32]*ServerPeer),
		outboundPeers:   make(map[int32]*ServerPeer),
		banned:          make(map[string]time.Time),
		outboundGroups:  make(map[string]int),
	}

	if !s.cfg.DisableDNSSeed {
		// Add peers discovered through DNS to the address manager.
		connmgr.SeedFromDNS(s.chain.ChainParams, defaultRequiredServices,
			s.btcdLookup, func(addrs []*wire.NetAddress) {
				// Bitcoind uses a lookup of the dns seeder here. This
				// is rather strange since the values looked up by the
				// DNS seed lookups will vary quite a lot.
				// to replicate this behaviour we put all addresses as
				// having come from the first one.
				s.addrManager.AddAddresses(addrs, addrs[0])
			})
	}
	go s.ConnManager.Start()

out:
	for {
		select {
		// New peers connected to the Server.
		case p := <-s.newPeers:
			s.handleAddPeerMsg(state, p)

		// Disconnected peers.
		case p := <-s.donePeers:
			s.handleDonePeerMsg(state, p)

		// Block accepted in mainchain or orphan, update peer height.
		case umsg := <-s.peerHeightsUpdate:
			s.handleUpdatePeerHeights(state, umsg)

		// Peer to ban.
		case p := <-s.banPeers:
			s.handleBanPeerMsg(state, p)

		// New inventory to potentially be relayed to other peers.
		case invMsg := <-s.relayInv:
			s.handleRelayInvMsg(state, invMsg)

		// Message to broadcast to all connected peers except those
		// which are excluded by the message.
		case bmsg := <-s.broadcast:
			s.handleBroadcastMsg(state, &bmsg)

		case qmsg := <-s.query:
			s.handleQuery(state, qmsg)

		case <-s.quit:
			// Disconnect all peers on Server shutdown.
			state.forAllPeers(func(sp *ServerPeer) {
				s.logger.Tracef("Shutdown peer %s", sp)
				sp.Disconnect()
			})
			break out
		}
	}

	s.ConnManager.Stop()
	s.chain.SyncManager.Stop()
	s.addrManager.Stop()

	// Drain channels before exiting so nothing is left waiting around
	// to send.
cleanup:
	for {
		select {
		case <-s.newPeers:
		case <-s.donePeers:
		case <-s.peerHeightsUpdate:
		case <-s.relayInv:
		case <-s.broadcast:
		case <-s.query:
		default:
			break cleanup
		}
	}
	s.wg.Done()
	s.logger.Tracef("Peer handler done")
}

// AddPeer adds a new peer that has already been connected to the server.
func (s *Server) AddPeer(sp *ServerPeer) {
	s.newPeers <- sp
}

// BanPeer bans a peer that has already been connected to the server by ip.
func (s *Server) BanPeer(sp *ServerPeer) {
	s.banPeers <- sp
}

// RelayInventory relays the passed inventory vector to all connected peers
// that are not already known to have it.
func (s *Server) RelayInventory(invVect *types.InvVect, data interface{}) {
	s.relayInv <- RelayMsg{InvVect: invVect, Data: data}
}

// BroadcastMessage sends msg to all peers currently connected to the Server
// except those in the passed peers to exclude.
func (s *Server) BroadcastMessage(msg wire.Message, exclPeers ...*ServerPeer) {
	// XXX: Need to determine if this is an alert that has already been
	// broadcast and refrain from broadcasting again.
	bmsg := broadcastMsg{message: msg, excludePeers: exclPeers}
	s.broadcast <- bmsg
}

// ConnectedCount returns the number of currently connected peers.
func (s *Server) ConnectedCount() int32 {
	replyChan := make(chan int32)

	s.query <- GetConnCountMsg{Reply: replyChan}

	return <-replyChan
}

// OutboundGroupCount returns the number of peers connected to the given
// outbound group key.
func (s *Server) OutboundGroupCount(key string) int {
	replyChan := make(chan int)
	s.query <- GetOutboundGroup{Key: key, Reply: replyChan}
	return <-replyChan
}

// AddBytesSent adds the passed number of bytes to the total bytes sent counter
// for the server.  It is safe for concurrent access.
func (s *Server) AddBytesSent(bytesSent uint64) {
	atomic.AddUint64(&s.bytesSent, bytesSent)
}

// AddBytesReceived adds the passed number of bytes to the total bytes received
// counter for the server.  It is safe for concurrent access.
func (s *Server) AddBytesReceived(bytesReceived uint64) {
	atomic.AddUint64(&s.bytesReceived, bytesReceived)
}

// NetTotals returns the sum of all bytes received and sent across the network
// for all peers.  It is safe for concurrent access.
func (s *Server) NetTotals() (uint64, uint64) {
	return atomic.LoadUint64(&s.bytesReceived),
		atomic.LoadUint64(&s.bytesSent)
}

// UpdatePeerHeights updates the heights of all peers who have have announced
// the latest connected main BlockChain block, or a recognized orphan. These height
// updates allow us to dynamically refresh peer heights, ensuring sync peer
// selection has access to the latest block heights for each peer.
func (s *Server) UpdatePeerHeights(latestBlkHash *chainhash.Hash, latestHeight int32, updateSource *peer.Peer) {
	s.peerHeightsUpdate <- UpdatePeerHeightsMsg{
		NewHash:    latestBlkHash,
		NewHeight:  latestHeight,
		OriginPeer: updateSource,
	}
}

// rebroadcastHandler keeps track of user submitted inventories that we have
// sent out but have not yet made it into a block. We periodically rebroadcast
// them in case our peers restarted or otherwise lost track of them.
func (s *Server) rebroadcastHandler() {
	// Wait 5 min before first tx rebroadcast.
	timer := time.NewTimer(5 * time.Minute)
	pendingInvs := make(map[types.InvVect]interface{})

out:
	for {
		select {
		case riv := <-s.modifyRebroadcastInv:
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
				s.RelayInventory(&ivCopy, data)
			}

			// Process at a random time up to 30mins (in seconds)
			// in the future.
			timer.Reset(time.Second *
				time.Duration(randomUint16Number(1800)))

		case <-s.quit:
			break out
		}
	}

	timer.Stop()

	// Drain channels before exiting so nothing is left waiting around
	// to send.
cleanup:
	for {
		select {
		case <-s.modifyRebroadcastInv:
		default:
			break cleanup
		}
	}
	s.wg.Done()
}

func (s *Server) Run(ctx context.Context) {
	// Already started?
	if atomic.AddInt32(&s.started, 1) != 1 {
		return
	}

	s.logger.Trace("Starting server")

	// Server startup time. Used for the uptime command for uptime calculation.
	s.chain.StartupTime = time.Now().Unix()

	// Start the peer handler which in turn starts the address and block
	// managers.
	s.wg.Add(1)
	go s.peerHandler()

	if s.nat != nil {
		s.wg.Add(1)
		go s.upnpUpdateThread()
	}
	<-ctx.Done()

	// Save fee estimator state in the database.
	s.chain.DB.Update(func(tx database.Tx) error {
		metadata := tx.Metadata()
		metadata.Put(mempool.EstimateFeeDatabaseKey, s.chain.FeeEstimator.Save())

		return nil
	})

	// Signal the remaining goroutines to quit.
	close(s.quit)

	s.wg.Wait()

	return
}

// Start begins accepting connections from peers.
func (s *Server) Start() {
	// Already started?
	if atomic.AddInt32(&s.started, 1) != 1 {
		return
	}

	s.logger.Trace("Starting Server")

	// Server startup time. Used for the uptime command for uptime calculation.
	s.chain.StartupTime = time.Now().Unix()

	// Start the peer handler which in turn starts the address and block
	// managers.
	s.wg.Add(1)
	go s.peerHandler()

	if s.nat != nil {
		s.wg.Add(1)
		go s.upnpUpdateThread()
	}

}

// Stop gracefully shuts down the Server by stopping and disconnecting all
// peers and the main listener.
func (s *Server) Stop() error {
	// Make sure this only happens once.
	if atomic.AddInt32(&s.shutdown, 1) != 1 {
		s.logger.Infof("Server is already in the process of shutting down")
		return nil
	}

	// Stop the CPU miner if needed
	// s.cpuMiner.Stop()

	// Shutdown the RPC Server if it's not disabled.
	// if !s.rpcCfg.Disable {
	//	s.RPCServer.Stop()
	// }

	// Save fee estimator state in the database.
	s.chain.DB.Update(func(tx database.Tx) error {
		metadata := tx.Metadata()
		metadata.Put(mempool.EstimateFeeDatabaseKey, s.chain.FeeEstimator.Save())

		return nil
	})

	// Signal the remaining goroutines to quit.
	close(s.quit)
	return nil
}

// WaitForShutdown blocks until the main listener and peer handlers are stopped.
func (s *Server) WaitForShutdown() {
	s.wg.Wait()
}

// ScheduleShutdown schedules a Server shutdown after the specified duration.
// It also dynamically adjusts how often to warn the Server is going down based
// on remaining duration.
func (s *Server) ScheduleShutdown(duration time.Duration) {
	// Don't schedule shutdown more than once.
	if atomic.AddInt32(&s.shutdownSched, 1) != 1 {
		return
	}
	s.logger.Warnf("Server shutdown in %v", duration)
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
				s.Stop()
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
				s.logger.Warnf("Server shutdown in %v", remaining)
			}
		}
	}()
}

func (s *Server) upnpUpdateThread() {
	// Go off immediately to prevent code duplication, thereafter we renew
	// lease every 15 minutes.
	timer := time.NewTimer(0 * time.Second)
	lport, _ := strconv.ParseInt(s.chain.ChainParams.DefaultPort, 10, 16)
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
			listenPort, err := s.nat.AddPortMapping("tcp", int(lport), int(lport),
				"shard BlockChain p2p listen port", 20*60)
			if err != nil {
				s.logger.Warnf("can't add UPnP port mapping: %v", err)
			}
			if first && err == nil {
				// TODO: look this up periodically to see if upnp domain changed
				// and so did ip.
				externalIP, err := s.nat.GetExternalAddress()
				if err != nil {
					s.logger.Warnf("UPnP can't get external address: %v", err)
					continue out
				}
				na := wire.NewNetAddressIPPort(externalIP, uint16(listenPort),
					s.services)
				err = s.addrManager.AddLocalAddress(na, addrmgr.UpnpPrio)
				if err != nil {
					// XXX DeletePortMapping?
				}
				s.logger.Warnf("Successfully bound via UPnP to %s", addrmgr.NetAddressKey(na))
				first = false
			}
			timer.Reset(time.Minute * 15)
		case <-s.quit:
			break out
		}
	}

	timer.Stop()

	if err := s.nat.DeletePortMapping("tcp", int(lport), int(lport)); err != nil {
		s.logger.Warnf("unable to remove UPnP port mapping: %v", err)
	} else {
		s.logger.Debugf("successfully disestablished UPnP port mapping")
	}

	s.wg.Done()
}

// addrStringToNetAddr takes an address in the form of 'host:port' and returns
// a net.Addr which maps to the original address with any host names resolved
// to IP addresses.  It also handles tor addresses properly by returning a
// net.Addr that encapsulates the address.
func (s *Server) addrStringToNetAddr(addr string) (net.Addr, error) {
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
		if s.cfg.NoOnion {
			return nil, errors.New("tor has been disabled")
		}

		return &onionAddr{addr: addr}, nil
	}

	// Attempt to look up an IP address associated with the parsed host.
	ips, err := s.btcdLookup(host)
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

// isWhitelisted returns whether the IP address is included in the whitelisted
// networks and IPs.
func (s *Server) isWhitelisted(addr net.Addr) bool {
	// if len(s.cfg.Whitelists) == 0 {
	//	return false
	// }

	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		s.logger.Warnf("Unable to SplitHostPort on '%s': %v", addr, err)
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		s.logger.Warnf("Unable to parse IP '%s'", addr)
		return false
	}

	// for _, ipnet := range s.cfg.whitelists {
	//	if ipnet.Contains(ip) {
	//		return true
	//	}
	// }
	return false
}

func (s *Server) btcdLookup(host string) ([]net.IP, error) {
	if strings.HasSuffix(host, ".onion") {
		return nil, fmt.Errorf("attempt to resolve tor address %s", host)
	}

	return s.cfg.Lookup(host)
}

// btcdDial connects to the address on the named network using the appropriate
// dial function depending on the address and configuration options.  For
// example, .onion addresses will be dialed using the onion specific proxy if
// one was specified, but will otherwise use the normal dial function (which
// could itself use a proxy or not).
func (s *Server) btcdDial(addr net.Addr) (net.Conn, error) {
	if strings.Contains(addr.String(), ".onion:") {
		return s.cfg.Oniondial(addr.Network(), addr.String(),
			defaultConnectTimeout)
	}
	return s.cfg.Dial(addr.Network(), addr.String(), defaultConnectTimeout)
}
