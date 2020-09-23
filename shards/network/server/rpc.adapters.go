// Copyright (c) 2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package server

import (
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"sync/atomic"

	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/netsync"
	"gitlab.com/jaxnet/core/shard.core.git/peer"
)

// rpcPeer provides a peer for use with the RPC server and implements the
// rpcserverPeer interface.
type rpcPeer serverPeer

// Ensure rpcPeer implements the rpcserverPeer interface.
var _ rpcserverPeer = (*rpcPeer)(nil)

// ToPeer returns the underlying peer instance.
//
// This function is safe for concurrent access and is part of the rpcserverPeer
// interface implementation.
func (p *rpcPeer) ToPeer() *peer.Peer {
	if p == nil {
		return nil
	}
	return (*serverPeer)(p).Peer
}

// IsTxRelayDisabled returns whether or not the peer has disabled transaction
// relay.
//
// This function is safe for concurrent access and is part of the rpcserverPeer
// interface implementation.
func (p *rpcPeer) IsTxRelayDisabled() bool {
	return (*serverPeer)(p).disableRelayTx
}

// BanScore returns the current integer value that represents how close the peer
// is to being banned.
//
// This function is safe for concurrent access and is part of the rpcserverPeer
// interface implementation.
func (p *rpcPeer) BanScore() uint32 {
	return (*serverPeer)(p).banScore.Int()
}

// FeeFilter returns the requested current minimum fee rate for which
// transactions should be announced.
//
// This function is safe for concurrent access and is part of the rpcserverPeer
// interface implementation.
func (p *rpcPeer) FeeFilter() int64 {
	return atomic.LoadInt64(&(*serverPeer)(p).feeFilter)
}

// RPCSyncMgr provides a block manager for use with the RPC Server and
// implements the rpcserverSyncManager interface.
type RPCSyncMgr struct {
	Server  *server
	SyncMgr *netsync.SyncManager
}

// Ensure RPCSyncMgr implements the rpcserverSyncManager interface.
var _ rpcserverSyncManager = (*RPCSyncMgr)(nil)

// IsCurrent returns whether or not the sync manager believes the chain is
// current as compared to the rest of the network.
//
// This function is safe for concurrent access and is part of the
// rpcserverSyncManager interface implementation.
func (b *RPCSyncMgr) IsCurrent() bool {
	return b.SyncMgr.IsCurrent()
}

// SubmitBlock submits the provided block to the network after processing it
// locally.
//
// This function is safe for concurrent access and is part of the
// rpcserverSyncManager interface implementation.
func (b *RPCSyncMgr) SubmitBlock(block *btcutil.Block, flags blockchain.BehaviorFlags) (bool, error) {
	return b.SyncMgr.ProcessBlock(block, flags)
}

// Pause pauses the sync manager until the returned channel is closed.
//
// This function is safe for concurrent access and is part of the
// rpcserverSyncManager interface implementation.
func (b *RPCSyncMgr) Pause() chan<- struct{} {
	return b.SyncMgr.Pause()
}

// SyncPeerID returns the peer that is currently the peer being used to sync
// from.
//
// This function is safe for concurrent access and is part of the
// rpcserverSyncManager interface implementation.
func (b *RPCSyncMgr) SyncPeerID() int32 {
	return b.SyncMgr.SyncPeerID()
}

// LocateBlocks returns the hashes of the blocks after the first known block in
// the provided locators until the provided stop hash or the current tip is
// reached, up to a max of wire.MaxBlockHeadersPerMsg hashes.
//
// This function is safe for concurrent access and is part of the
// rpcserverSyncManager interface implementation.
func (b *RPCSyncMgr) LocateHeaders(locators []*chainhash.Hash, hashStop *chainhash.Hash) []chain.BlockHeader {
	return b.Server.chain.LocateHeaders(locators, hashStop)
}
