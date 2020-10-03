package netsync

import (
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/network/peer"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain"
	"gitlab.com/jaxnet/core/shard.core/node/mempool"
	"gitlab.com/jaxnet/core/shard.core/types"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

// ServerSyncManager represents a sync manager for use with the RPC server.
//
// The interface contract requires that all of these methods are safe for
// concurrent access.
type ServerSyncManager interface {
	// IsCurrent returns whether or not the sync manager believes the BlockChain
	// is current as compared to the rest of the network.
	IsCurrent() bool

	// SubmitBlock submits the provided block to the network after
	// processing it locally.
	SubmitBlock(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error)

	// Pause pauses the sync manager until the returned channel is closed.
	Pause() chan<- struct{}

	// SyncPeerID returns the ID of the peer that is currently the peer being
	// used to sync from or 0 if there is none.
	SyncPeerID() int32

	// LocateHeaders returns the headers of the blocks after the first known
	// block in the provided locators until the provided stop hash or the
	// current tip is reached, up to a max of wire.MaxBlockHeadersPerMsg
	// hashes.
	LocateHeaders(locators []*chainhash.Hash, hashStop *chainhash.Hash) []wire.BlockHeader
}

// ServerPeer represents a peer for use with the RPC server.
//
// The interface contract requires that all of these methods are safe for
// concurrent access.
type ServerPeer interface {
	// ToPeer returns the underlying peer instance.
	ToPeer() *peer.Peer

	// IsTxRelayDisabled returns whether or not the peer has disabled
	// transaction relay.
	IsTxRelayDisabled() bool

	// GetBanScore returns the current integer value that represents how close
	// the peer is to being banned.
	GetBanScore() uint32

	// FeeFilter returns the requested current minimum fee rate for which
	// transactions should be announced.
	GetFeeFilter() int64
}

// P2PConnManager represents a connection manager for use with the RPC
// server.
//
// The interface contract requires that all of these methods are safe for
// concurrent access.
type P2PConnManager interface {
	// Connect adds the provided address as a new outbound peer.  The
	// permanent flag indicates whether or not to make the peer persistent
	// and reconnect if the connection is lost.  Attempting to connect to an
	// already existing peer will return an error.
	Connect(addr string, permanent bool) error

	// RemoveByID removes the peer associated with the provided id from the
	// list of persistent peers.  Attempting to remove an id that does not
	// exist will return an error.
	RemoveByID(id int32) error

	// RemoveByAddr removes the peer associated with the provided address
	// from the list of persistent peers.  Attempting to remove an address
	// that does not exist will return an error.
	RemoveByAddr(addr string) error

	// DisconnectByID disconnects the peer associated with the provided id.
	// This applies to both inbound and outbound peers.  Attempting to
	// remove an id that does not exist will return an error.
	DisconnectByID(id int32) error

	// DisconnectByAddr disconnects the peer associated with the provided
	// address.  This applies to both inbound and outbound peers.
	// Attempting to remove an address that does not exist will return an
	// error.
	DisconnectByAddr(addr string) error

	// ConnectedCount returns the number of currently connected peers.
	ConnectedCount() int32

	// NetTotals returns the sum of all bytes received and sent across the
	// network for all peers.
	NetTotals() (uint64, uint64)

	// ConnectedPeers returns an array consisting of all connected peers.
	ConnectedPeers() []ServerPeer

	// PersistentPeers returns an array consisting of all the persistent
	// peers.
	PersistentPeers() []ServerPeer

	// BroadcastMessage sends the provided message to all currently
	// connected peers.
	BroadcastMessage(msg wire.Message)

	// AddRebroadcastInventory adds the provided inventory to the list of
	// inventories to be rebroadcast at random intervals until they show up
	// in a block.
	AddRebroadcastInventory(iv *types.InvVect, data interface{})

	// RelayTransactions generates and relays inventory vectors for all of
	// the passed transactions to all connected peers.
	RelayTransactions(txns []*mempool.TxDesc)
}

// RPCSyncMgr provides a block manager for use with the RPC Server and
// implements the ServerSyncManager interface.
type RPCSyncMgr struct {
	SyncMgr        *SyncManager
	HeadersLocator func(locators []*chainhash.Hash, hashStop *chainhash.Hash) []wire.BlockHeader
}

// Ensure RPCSyncMgr implements the ServerSyncManager interface.
var _ ServerSyncManager = (*RPCSyncMgr)(nil)

// IsCurrent returns whether or not the sync manager believes the BlockChain is
// current as compared to the rest of the network.
//
// This function is safe for concurrent access and is part of the
// ServerSyncManager interface implementation.
func (b *RPCSyncMgr) IsCurrent() bool {
	return b.SyncMgr.IsCurrent()
}

// SubmitBlock submits the provided block to the network after processing it
// locally.
//
// This function is safe for concurrent access and is part of the
// ServerSyncManager interface implementation.
func (b *RPCSyncMgr) SubmitBlock(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
	return b.SyncMgr.ProcessBlock(block, flags)
}

// Pause pauses the sync manager until the returned channel is closed.
//
// This function is safe for concurrent access and is part of the
// ServerSyncManager interface implementation.
func (b *RPCSyncMgr) Pause() chan<- struct{} {
	return b.SyncMgr.Pause()
}

// SyncPeerID returns the peer that is currently the peer being used to sync
// from.
//
// This function is safe for concurrent access and is part of the
// ServerSyncManager interface implementation.
func (b *RPCSyncMgr) SyncPeerID() int32 {
	return b.SyncMgr.SyncPeerID()
}

// LocateBlocks returns the hashes of the blocks after the first known block in
// the provided locators until the provided stop hash or the current tip is
// reached, up to a max of wire.MaxBlockHeadersPerMsg hashes.
//
// This function is safe for concurrent access and is part of the
// ServerSyncManager interface implementation.
func (b *RPCSyncMgr) LocateHeaders(locators []*chainhash.Hash, hashStop *chainhash.Hash) []wire.BlockHeader {
	return b.HeadersLocator(locators, hashStop)
}
