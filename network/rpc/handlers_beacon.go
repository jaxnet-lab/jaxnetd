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
	"gitlab.com/jaxnet/jaxnetd/types/wire"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/netsync"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
)

type BeaconRPC struct {
	*CommonChainRPC
}

func NewBeaconRPC(chainProvider *cprovider.ChainProvider, connMgr netsync.P2PConnManager,
	logger zerolog.Logger) *BeaconRPC {
	rpc := &BeaconRPC{
		CommonChainRPC: NewCommonChainRPC(chainProvider, connMgr,
			logger.With().Str("ctx", "beacon_rpc").Logger()),
	}
	rpc.ComposeHandlers()
	return rpc
}

func (server *BeaconRPC) ComposeHandlers() {
	server.SetCommands(server.CommonChainRPC.OwnHandlers())
	server.SetCommands(server.Handlers())
}

func (server *BeaconRPC) Handlers() map[jaxjson.MethodName]CommandHandler {
	return map[jaxjson.MethodName]CommandHandler{
		jaxjson.ScopedMethod("beacon", "getBeaconHeaders"):               server.handleGetHeaders,
		jaxjson.ScopedMethod("beacon", "getBeaconBlock"):                 server.handleGetBlock,
		jaxjson.ScopedMethod("beacon", "getBeaconBlockHeader"):           server.handleGetBlockHeader,
		jaxjson.ScopedMethod("beacon", "getBeaconBlockBySerialNumber"):   server.handleGetBlockBySerialNumber,
		jaxjson.ScopedMethod("beacon", "listBeaconBlocksBySerialNumber"): server.handleListBlocksBySerialNumber,
		jaxjson.ScopedMethod("beacon", "getBlockHeader"):                 server.handleGetBlockHeader,
		jaxjson.ScopedMethod("beacon", "getBeaconBlockTemplate"):         server.handleGetBlockTemplate, // DEPRECATED
		jaxjson.ScopedMethod("beacon", "listEADAddresses"):               server.handleListEADAddresses,
		// jaxjson.ScopedMethod("beacon", "getBeaconBlockHash"):     server.handleGetBlockHash,
		// jaxjson.ScopedMethod("beacon", "setAllowExpansion"): server.handleSetAllowExpansion,

	}
}

// handleGetHeaders implements the getheaders command.
//
// NOTE: This is a btcsuite extension originally ported from
// github.com/decred/dcrd.
func (server *BeaconRPC) handleGetHeaders(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.GetBeaconHeadersCmd)

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
			// PrevMMRRoot: chainhash.Hash{},
			// Weight:      0,
			// Height:      0,
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
func (server *BeaconRPC) handleGetBlock(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.GetBeaconBlockCmd)

	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	return server.getBlock(hash, c.Verbosity)
}

// handleGetBlockBySerialNumber implements the getBlockBySerialNumber command.
func (server *BeaconRPC) handleGetBlockBySerialNumber(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.GetBeaconBlockBySerialNumberCmd)
	return server.getBlockBySerialID(c.Verbosity, c.SerialNumber)
}

// handleListBlocksBySerialNumber - returns transaction with specified serialNumber + number of txs which is equal to limit
// so if you specify serialNumber = 10 and limit 2, then you will receive blocks with serialNumbers 10,11,12
func (server *BeaconRPC) handleListBlocksBySerialNumber(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.ListBeaconBlocksBySerialNumberCmd)

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

func (server *BeaconRPC) getBlockBySerialID(verbosity *int, serialID int64) (interface{}, error) {
	var hash *chainhash.Hash
	err := server.chainProvider.DB.View(func(dbTx database.Tx) error {
		var err error
		hash, _, err = chaindata.DBFetchBlockHashBySerialID(dbTx, serialID)
		return err
	})
	if err != nil {
		return nil, err
	}

	return server.getBlock(hash, verbosity)
}

// handleGetBlock implements the getblock command.
func (server *BeaconRPC) getBlock(hash *chainhash.Hash, verbosity *int) (interface{}, error) {
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

	// Otherwise, generate the JSON object and return it.

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

	params := server.chainProvider.ChainParams
	blockHeader := blk.MsgBlock().Header
	dontBVerboseTx := verbosity != nil && *verbosity == 1
	blockReply := jaxjson.GetBeaconBlockVerboseResult{
		BeaconBlockHeader: rpcutli.WireHeaderToBeaconJSON(params, blockHeader, "", !dontBVerboseTx), // TODO: add actual mmr root
		Confirmations:     int64(1 + best.Height - blockHeight),
		SerialID:          serialID,
		PrevSerialID:      prevSerialID,
		Size:              int32(len(blkBytes)),
		StrippedSize:      int32(blk.MsgBlock().SerializeSizeStripped()),
		Weight:            int32(chaindata.GetBlockWeight(blk)),
		NextHash:          nextHashString,
	}

	if dontBVerboseTx {
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
func (server *BeaconRPC) handleGetBlockHeader(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.GetBeaconBlockHeaderCmd)

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

	params := server.chainProvider.ChainParams
	var serialID, prevSerialID int64
	_ = server.chainProvider.DB.View(func(tx database.Tx) error {
		serialID, prevSerialID, err = chaindata.DBFetchBlockSerialID(tx, hash)
		return err
	})

	blockHeaderReply := jaxjson.GetBeaconBlockHeaderVerboseResult{
		BeaconBlockHeader: rpcutli.WireHeaderToBeaconJSON(params, blockHeader, "", false), // TODO: add actual mmr root
		Confirmations:     int64(1 + best.Height - blockHeight),
		SerialID:          serialID,
		PrevSerialID:      prevSerialID,
		NextHash:          nextHashString,
	}
	return blockHeaderReply, nil
}
