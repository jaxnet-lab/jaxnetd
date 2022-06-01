// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
// nolint: forcetypeassert
package rpc

import (
	"bytes"
	"encoding/hex"
	"math"

	"gitlab.com/jaxnet/jaxnetd/network/rpcutli"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/netsync"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type ShardRPC struct {
	*CommonChainRPC
}

func NewShardRPC(chainProvider *cprovider.ChainProvider,
	connMgr netsync.P2PConnManager,
	logger zerolog.Logger,
) *ShardRPC {
	rpc := &ShardRPC{
		CommonChainRPC: NewCommonChainRPC(chainProvider, connMgr, logger.With().
			Uint32("id", chainProvider.ChainCtx.ShardID()).
			Str("ctx", "shard_rpc").Logger()),
	}

	rpc.ComposeHandlers()
	return rpc
}

func (server *ShardRPC) HandleCommand(cmd *ParsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
	if cmd.ShardID != server.chainProvider.ChainCtx.ShardID() {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrShardIDMismatch,
			Message: "Provided ShardID does not match with chain",
		}
	}

	return server.Mux.HandleCommand(cmd, closeChan)
}

func (server *ShardRPC) ComposeHandlers() {
	server.SetCommands(server.CommonChainRPC.OwnHandlers())
	server.SetCommands(server.OwnHandlers())
}

func (server *ShardRPC) OwnHandlers() map[jaxjson.MethodName]CommandHandler {
	return map[jaxjson.MethodName]CommandHandler{
		jaxjson.ScopedMethod("shard", "getShardHeaders"):               server.handleGetHeaders,
		jaxjson.ScopedMethod("shard", "getShardBlock"):                 server.handleGetBlock,
		jaxjson.ScopedMethod("shard", "getShardBlockHeader"):           server.handleGetBlockHeader,
		jaxjson.ScopedMethod("shard", "getShardBlockTemplate"):         server.handleGetBlockTemplate, // DEPRECATED
		jaxjson.ScopedMethod("shard", "getShardBlockBySerialNumber"):   server.handleGetBlockBySerialNumber,
		jaxjson.ScopedMethod("shard", "listShardBlocksBySerialNumber"): server.handleListBlocksBySerialNumber,
	}
}

// handleGetHeaders implements the getheaders command.
//
// NOTE: This is a btcsuite extension originally ported from
// github.com/decred/dcrd.
func (server *ShardRPC) handleGetHeaders(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.GetShardHeadersCmd)

	// Fetch the requested headers from BlockChain while respecting the provided
	// block locators and stop hash.
	blockLocators := make([]*wire.BlockLocatorMeta, len(c.BlockLocators))
	for i := range c.BlockLocators {
		blockLocator, err := chainhash.NewHashFromStr(c.BlockLocators[i])
		if err != nil {
			return nil, rpcDecodeHexError(c.BlockLocators[i])
		}
		blockLocators[i] = &wire.BlockLocatorMeta{
			Hash: *blockLocator,
		}
	}
	var hashStop chainhash.Hash
	if c.HashStop != "" {
		err := chainhash.Decode(&hashStop, c.HashStop)
		if err != nil {
			return nil, rpcDecodeHexError(c.HashStop)
		}
	}
	headers := server.chainProvider.BlockChain().LocateHeaders(blockLocators, &hashStop)

	// Return the serialized block headers as hex-encoded strings.
	hexBlockHeaders := make([]string, len(headers))
	var buf bytes.Buffer
	for i, h := range headers {
		err := h.Header.Write(&buf)
		if err != nil {
			return nil, server.InternalRPCError(err.Error(),
				"Failed to serialize block header")
		}
		hexBlockHeaders[i] = hex.EncodeToString(buf.Bytes())
		buf.Reset()
	}
	return hexBlockHeaders, nil
}

// handleGetBlock implements the getblock command.
func (server *ShardRPC) handleGetBlock(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.GetShardBlockCmd)

	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	return server.getBlock(hash, c.Verbosity)
}

// handleGetBlockBySerialNumber implements the getBlockBySerialNumber command.
func (server *ShardRPC) handleGetBlockBySerialNumber(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.GetShardBlockBySerialNumberCmd)

	return server.getBlockBySerialID(c.Verbosity, c.SerialNumber)
}

func (server *ShardRPC) handleListBlocksBySerialNumber(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.ListShardBlocksBySerialNumberCmd)

	// this variable is for showing in which direction we are heading
	// if limit < 0 then we are going downwards. This is achieved by multiplying offset
	// with sign.
	var sign int64 = 1
	if *c.Limit < 0 {
		sign = -1
	}

	// this is done to assure that offset is always positive and we just multiply it with sign
	absLimit := int64(math.Abs(float64(*c.Limit)))

	var (
		offset int64
		output []interface{}
	)

	for offset = 0; offset <= absLimit; offset++ {
		block, err := server.getBlockBySerialID(c.Verbosity, c.SerialNumber+offset*sign)
		if err != nil {
			return output, rpcutli.GetErrorBasedOnOutLength(output, err)
		}

		output = append(output, block)
	}

	return output, nil
}

func (server *ShardRPC) getBlockBySerialID(verbosity *int, serialID int64) (interface{}, error) {
	var hash *chainhash.Hash
	err := server.chainProvider.DB.View(func(dbTx database.Tx) error {
		var err error
		hash, _, err = chaindata.DBFetchBlockHashBySerialID(dbTx, serialID)
		return err
	})
	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}

	return server.getBlock(hash, verbosity)
}

// handleGetBlock implements the getblock command.
func (server *ShardRPC) getBlock(hash *chainhash.Hash, verbosity *int) (interface{}, error) {
	var blkBytes []byte
	err := server.chainProvider.DB.View(func(dbTx database.Tx) error {
		var err error
		blkBytes, err = dbTx.FetchBlock(hash)
		return err
	})
	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}

	// Deserialize the block.
	blk, err := jaxutil.NewBlockFromBytes(blkBytes)
	if err != nil {
		return nil, server.InternalRPCError(err.Error(), deserializeBlockErrorString)
	}

	var nextHashString string
	best := server.chainProvider.BlockChain().BestSnapshot()

	// Get the block height from BlockChain.
	blockHeight, serialID, prevSerialID, err := server.chainProvider.BlockChain().BlockIDsByHash(hash)
	if err == nil {
		// Get next block hash unless there are none.
		// TODO: resolve next block from main chain
		if blockHeight != -1 && blockHeight < best.Height {
			nextHash, err := server.chainProvider.BlockChain().BlockHashByHeight(blockHeight + 1)
			if err != nil {
				return nil, server.InternalRPCError(err.Error(), noNextBlock)
			}
			nextHashString = nextHash.String()
		}
	}
	// If verbosity is 0, return the serialized block as a hex encoded string.
	if verbosity != nil && *verbosity == 0 {
		return jaxjson.GetBlockResult{
			Block:        hex.EncodeToString(blkBytes),
			Height:       blockHeight,
			SerialID:     serialID,
			PrevSerialID: prevSerialID,
		}, nil
	}

	// Otherwise, generate the JSON object and return it.
	params := server.chainProvider.ChainParams
	blockHeader := blk.MsgBlock().Header

	// prevHash, _ := server.chainProvider.BlockChain().MMRTree().LookupNodeByRoot(blockHeader.PrevBlocksMMRRoot())
	dontVerboseTx := verbosity != nil && *verbosity == 1
	blockReply := jaxjson.GetShardBlockVerboseResult{
		ShardBlockHeader: rpcutli.WireHeaderToShardJSON(params, blockHeader, "", !dontVerboseTx), // todo: add actual MMR root
		Confirmations:    int64(1 + best.Height - blockHeight),
		Size:             int32(len(blkBytes)),
		StrippedSize:     int32(blk.MsgBlock().SerializeSizeStripped()),
		Weight:           int32(chaindata.GetBlockWeight(blk)),
		SerialID:         serialID,
		PrevSerialID:     prevSerialID,
		NextHash:         nextHashString,
	}

	if dontVerboseTx {
		transactions := blk.Transactions()
		txNames := make([]string, len(transactions))
		for i, tx := range transactions {
			txNames[i] = tx.Hash().String()
		}

		blockReply.TxHashes = txNames
	} else {
		txns := blk.Transactions()
		rawTxns := make([]jaxjson.TxRawResult, len(txns))
		for i, tx := range txns {
			rawTxn, err := server.CreateTxRawResult(server.chainProvider.ChainCtx.Params(), tx.MsgTx(),
				tx.Hash().String(), blockHeader, hash.String(),
				blockHeight, best.Height)
			if err != nil {
				return nil, err
			}
			rawTxns[i] = *rawTxn
		}
		blockReply.Tx = rawTxns
	}

	return blockReply, nil
}

// handleGetBlockHeader implements the getblockheader command.
func (server *ShardRPC) handleGetBlockHeader(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.GetShardBlockHeaderCmd)

	// Fetch the header from BlockChain.
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	blockHeader, err := server.chainProvider.BlockChain().HeaderByHash(hash)
	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}

	// When the verbose flag isn't set, simply return the serialized block
	// header as a hex-encoded string.
	if c.Verbose != nil && !*c.Verbose {
		var headerBuf bytes.Buffer
		err := blockHeader.Write(&headerBuf)
		if err != nil {
			context := "Failed to serialize block header"
			return nil, server.InternalRPCError(err.Error(), context)
		}
		return hex.EncodeToString(headerBuf.Bytes()), nil
	}

	// The verbose flag is set, so generate the JSON object and return it.

	// Get the block height from BlockChain.
	blockHeight, err := server.chainProvider.BlockChain().BlockHeightByHash(hash)
	if err != nil {
		return nil, server.InternalRPCError(err.Error(), obtainBlockHeightErrorString)
	}
	best := server.chainProvider.BlockChain().BestSnapshot()

	// Get next block hash unless there are none.
	var nextHashString string
	if blockHeight < best.Height {
		nextHash, err := server.chainProvider.BlockChain().BlockHashByHeight(blockHeight + 1)
		if err != nil {
			return nil, server.InternalRPCError(err.Error(), noNextBlock)
		}
		nextHashString = nextHash.String()
	}

	var serialID, prevSerialID int64
	_ = server.chainProvider.DB.View(func(tx database.Tx) error {
		serialID, prevSerialID, err = chaindata.DBFetchBlockSerialID(tx, hash)
		return err
	})

	params := server.chainProvider.ChainParams
	blockHeaderReply := jaxjson.GetShardBlockHeaderVerboseResult{
		ShardBlockHeader: rpcutli.WireHeaderToShardJSON(params, blockHeader, "", false), // todo: add actual MMR root
		Confirmations:    int64(1 + best.Height - blockHeight),
		SerialID:         serialID,
		PrevSerialID:     prevSerialID,
		NextHash:         nextHashString,
	}
	return blockHeaderReply, nil
}
