/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package p2p

import (
	"bytes"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/core/shard.core/btcutil/bloom"
	"gitlab.com/jaxnet/core/shard.core/database"
	"gitlab.com/jaxnet/core/shard.core/network/addrmgr"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/node/encoder"
	"gitlab.com/jaxnet/core/shard.core/types"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type PeersConfig struct {
	DisableBanning  bool          `yaml:"disable_banning" long:"nobanning" description:"Disable banning of misbehaving peers"`
	BanThreshold    uint32        `yaml:"ban_threshold" long:"banthreshold" description:"Maximum allowed ban score before disconnecting and banning misbehaving peers."`
	BlocksOnly      bool          `yaml:"blocks_only" long:"blocksonly" description:"Do not accept transactions from remote peers."`
	Proxy           string        `yaml:"proxy" long:"proxy" description:"Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
	TrickleInterval time.Duration `yaml:"trickle_interval" long:"trickleinterval" description:"Minimum time between attempts to send new inventory to a connected Server"`
}

type serverPeerHandler struct {
	cfg         PeersConfig
	services    wire.ServiceFlag
	chain       *cprovider.ChainProvider
	addrManager *addrmgr.AddrManager

	// cfCheckptCaches stores a cached slice of filter headers for cfcheckpt
	// messages for each filter type.
	cfCheckptCaches    map[wire.FilterType][]cfHeaderKV
	cfCheckptCachesMtx sync.RWMutex

	newPeers chan<- *serverPeer
	banPeers chan<- *serverPeer

	logger       zerolog.Logger
	getChainPort func(shardID uint32) (int, bool)
}

func newServerPeerHandler(server *Server) *serverPeerHandler {
	return &serverPeerHandler{
		cfg: PeersConfig{
			DisableBanning:  server.cfg.DisableBanning,
			BanThreshold:    server.cfg.BanThreshold,
			BlocksOnly:      server.cfg.BlocksOnly,
			Proxy:           server.cfg.Proxy,
			TrickleInterval: server.cfg.TrickleInterval,
		},
		services:    server.services,
		chain:       server.chain,
		addrManager: server.addrManager,

		cfCheckptCaches:    make(map[wire.FilterType][]cfHeaderKV),
		cfCheckptCachesMtx: sync.RWMutex{},

		getChainPort: server.cfg.GetChainPort,

		newPeers: server.newPeers,
		banPeers: server.banPeers,
		logger:   server.logger,
	}
}

// AddPeer adds a new peer that has already been connected to the server.
func (server *serverPeerHandler) AddPeer(sp *serverPeer) {
	server.newPeers <- sp
}

// BanPeer bans a peer that has already been connected to the server by ip.
func (server *serverPeerHandler) BanPeer(sp *serverPeer) {
	server.banPeers <- sp
}

// pushTxMsg sends a tx message for the provided transaction hash to the
// connected peer.  An error is returned if the transaction hash is not known.
func (server *serverPeerHandler) pushTxMsg(sp *serverPeer, hash *chainhash.Hash, doneChan chan<- struct{},
	waitChan <-chan struct{}, encoding encoder.MessageEncoding) error {

	// Attempt to fetch the requested transaction from the pool.  A
	// call could be made to check for existence first, but simply trying
	// to fetch a missing transaction results in the same behavior.
	tx, err := server.chain.TxMemPool.FetchTransaction(hash)
	if err != nil {
		server.logger.Trace().Msgf("Unable to fetch tx %v from transaction "+
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
func (server *serverPeerHandler) pushBlockMsg(sp *serverPeer, hash *chainhash.Hash, doneChan chan<- struct{},
	waitChan <-chan struct{}, encoding encoder.MessageEncoding) error {

	// Fetch the raw block bytes from the database.
	var blockBytes []byte
	err := sp.serverPeerHandler.chain.DB.View(func(dbTx database.Tx) error {
		var err error
		blockBytes, err = dbTx.FetchBlock(hash)
		return err
	})
	if err != nil {
		server.logger.Trace().Msgf("Unable to fetch requested block hash %v: %v",
			hash, err)

		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return err
	}

	// Deserialize the block.
	var msgBlock = server.chain.ChainCtx.EmptyBlock()

	err = msgBlock.Deserialize(bytes.NewReader(blockBytes))
	if err != nil {
		server.logger.Trace().Msgf("Unable to deserialize requested block hash "+
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
		best := sp.serverPeerHandler.chain.BlockChain().BestSnapshot()
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
func (server *serverPeerHandler) pushMerkleBlockMsg(sp *serverPeer, hash *chainhash.Hash,
	doneChan chan<- struct{}, waitChan <-chan struct{}, encoding encoder.MessageEncoding) error {

	// Do not send a response if the peer doesn't have a filter loaded.
	if !sp.filter.IsLoaded() {
		if doneChan != nil {
			doneChan <- struct{}{}
		}
		return nil
	}

	// Fetch the raw block bytes from the database.
	blk, err := sp.serverPeerHandler.chain.BlockChain().BlockByHash(hash)
	if err != nil {
		server.logger.Trace().Msgf("Unable to fetch requested block hash %v: %v",
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

// AddBytesSent adds the passed number of bytes to the total bytes sent counter
// for the server.  It is safe for concurrent access.
func (server *serverPeerHandler) AddBytesSent(bytesSent uint64) {
	// TODO(mike): incr server runtime stats
}

// AddBytesReceived adds the passed number of bytes to the total bytes received
// counter for the server.  It is safe for concurrent access.
func (server *serverPeerHandler) AddBytesReceived(bytesReceived uint64) {
	// TODO(mike): incr server runtime stats
}
