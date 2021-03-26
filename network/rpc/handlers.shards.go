// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package rpc

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/database"
	"gitlab.com/jaxnet/core/shard.core/network/netsync"
	"gitlab.com/jaxnet/core/shard.core/node/chaindata"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type ShardRPC struct {
	*CommonChainRPC
	shardID uint32
}

func NewShardRPC(chainProvider *cprovider.ChainProvider,
	connMgr netsync.P2PConnManager,
	logger zerolog.Logger) *ShardRPC {
	rpc := &ShardRPC{
		CommonChainRPC: NewCommonChainRPC(chainProvider, connMgr, logger.With().
			Uint32("id", chainProvider.ChainCtx.ShardID()).
			Str("ctx", "shard_rpc").Logger()),
	}

	rpc.ComposeHandlers()
	return rpc
}

func (server *ShardRPC) HandleCommand(cmd *parsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
	if cmd.shardID != server.chainProvider.ChainCtx.ShardID() {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrShardIDMismatch,
			Message: "Provided ShardID does not match with chain",
		}
	}

	return server.Mux.HandleCommand(cmd, closeChan)
}

func (server *ShardRPC) ComposeHandlers() {
	server.SetCommands(server.CommonChainRPC.OwnHandlers())
	server.SetCommands(server.OwnHandlers())
}

func (server *ShardRPC) OwnHandlers() map[btcjson.MethodName]CommandHandler {
	return map[btcjson.MethodName]CommandHandler{
		btcjson.ScopedMethod("shard", "getShardHeaders"):       server.handleGetHeaders,
		btcjson.ScopedMethod("shard", "getShardBlock"):         server.handleGetBlock,
		btcjson.ScopedMethod("shard", "getShardBlockHeader"):   server.handleGetBlockHeader,
		btcjson.ScopedMethod("shard", "getShardBlockTemplate"): server.handleGetBlockTemplate,
		// btcjson.ScopedMethod("shard", "getShardBlockHash"):     server.handleGetBlockHash,
		btcjson.ScopedMethod("shard", "getShardBlockBySerialNumber"): server.handleGetBlockBySerialNumber,
	}
}

// handleGetHeaders implements the getheaders co
// mmand.
//
// NOTE: This is a btcsuite extension originally ported from
// github.com/decred/dcrd.
func (server *ShardRPC) handleGetHeaders(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetShardHeadersCmd)

	// Fetch the requested headers from BlockChain while respecting the provided
	// block locators and stop hash.
	blockLocators := make([]*chainhash.Hash, len(c.BlockLocators))
	for i := range c.BlockLocators {
		blockLocator, err := chainhash.NewHashFromStr(c.BlockLocators[i])
		if err != nil {
			return nil, rpcDecodeHexError(c.BlockLocators[i])
		}
		blockLocators[i] = blockLocator
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
		err := h.Write(&buf)
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
func (server *ShardRPC) handleGetBlock(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetShardBlockCmd)

	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	var blkBytes []byte
	err = server.chainProvider.DB.View(func(dbTx database.Tx) error {
		var err error
		blkBytes, err = dbTx.FetchBlock(hash)
		return err
	})
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}
	// If verbosity is 0, return the serialized block as a hex encoded string.
	if c.Verbosity != nil && *c.Verbosity == 0 {
		return hex.EncodeToString(blkBytes), nil
	}

	// Otherwise, generate the JSON object and return it.

	// Deserialize the block.
	blk, err := btcutil.NewBlockFromBytes(server.chainProvider.DB.Chain(), blkBytes)
	if err != nil {
		context := "Failed to deserialize block"
		return nil, server.InternalRPCError(err.Error(), context)
	}

	// Get the block height from BlockChain.
	blockHeight, err := server.chainProvider.BlockChain().BlockHeightByHash(hash)
	if err != nil {
		context := "Failed to obtain block height"
		return nil, server.InternalRPCError(err.Error(), context)
	}
	blk.SetHeight(blockHeight)
	best := server.chainProvider.BlockChain().BestSnapshot()

	// Get next block hash unless there are none.
	var nextHashString string
	if blockHeight < best.Height {
		nextHash, err := server.chainProvider.BlockChain().BlockHashByHeight(blockHeight + 1)
		if err != nil {
			context := "No next block"
			return nil, server.InternalRPCError(err.Error(), context)
		}
		nextHashString = nextHash.String()
	}

	params := server.chainProvider.ChainParams
	blockHeader := blk.MsgBlock().Header.(*wire.ShardHeader)
	diff, err := server.GetDifficultyRatio(blockHeader.Bits(), params)
	if err != nil {
		return nil, err
	}

	beaconHeader := blockHeader.BeaconHeader()
	blockReply := btcjson.GetShardBlockVerboseResult{
		Hash:          c.Hash,
		ShardHash:     blockHeader.ShardBlockHash().String(),
		MerkleRoot:    blockHeader.MerkleRoot().String(),
		PreviousHash:  blockHeader.PrevBlock().String(),
		Time:          blockHeader.Timestamp().Unix(),
		Confirmations: int64(1 + best.Height - blockHeight),
		Height:        int64(blockHeight),
		Size:          int32(len(blkBytes)),
		StrippedSize:  int32(blk.MsgBlock().SerializeSizeStripped()),
		Weight:        int32(chaindata.GetBlockWeight(blk)),
		Bits:          strconv.FormatInt(int64(blockHeader.Bits()), 16),
		Difficulty:    diff,
		NextHash:      nextHashString,
		BCBlock: btcjson.GetBeaconBlockVerboseResult{
			Confirmations: 0,
			StrippedSize:  0,
			Size:          0,
			Weight:        0,
			Height:        0,
			Tx:            nil,
			RawTx:         nil,
			Time:          0,
			Difficulty:    0,
			PreviousHash:  "",
			NextHash:      "",

			Bits:                strconv.FormatInt(int64(beaconHeader.Bits()), 16),
			Hash:                beaconHeader.BlockHash().String(),
			MerkleRoot:          beaconHeader.MerkleRoot().String(),
			MerkleMountainRange: beaconHeader.MergeMiningRoot().String(),
			Version:             int32(blockHeader.Version()),
			VersionHex:          fmt.Sprintf("%08x", blockHeader.Version()),
			Nonce:               blockHeader.Nonce(),
		},
	}

	if *c.Verbosity == 1 {
		transactions := blk.Transactions()
		txNames := make([]string, len(transactions))
		for i, tx := range transactions {
			txNames[i] = tx.Hash().String()
		}

		blockReply.Tx = txNames
	} else {
		txns := blk.Transactions()
		rawTxns := make([]btcjson.TxRawResult, len(txns))
		for i, tx := range txns {
			rawTxn, err := server.CreateTxRawResult(params, tx.MsgTx(),
				tx.Hash().String(), blockHeader, hash.String(),
				blockHeight, best.Height)
			if err != nil {
				return nil, err
			}
			rawTxns[i] = *rawTxn
		}
		blockReply.RawTx = rawTxns
	}

	return blockReply, nil
}

// handleGetBlockHeader implements the getblockheader command.
func (server *ShardRPC) handleGetBlockHeader(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetShardBlockHeaderCmd)

	// Fetch the header from BlockChain.
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	blockHeader, err := server.chainProvider.BlockChain().HeaderByHash(hash)
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}

	// When the verbose flag isn't set, simply return the serialized block
	// header as a hex-encoded string.
	if c.Verbose != nil && !*c.Verbose {
		var headerBuf bytes.Buffer
		err := blockHeader.Read(&headerBuf)
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
		context := "Failed to obtain block height"
		return nil, server.InternalRPCError(err.Error(), context)
	}
	best := server.chainProvider.BlockChain().BestSnapshot()

	// Get next block hash unless there are none.
	var nextHashString string
	if blockHeight < best.Height {
		nextHash, err := server.chainProvider.BlockChain().BlockHashByHeight(blockHeight + 1)
		if err != nil {
			context := "No next block"
			return nil, server.InternalRPCError(err.Error(), context)
		}
		nextHashString = nextHash.String()
	}

	params := server.chainProvider.ChainParams
	diff, err := server.GetDifficultyRatio(blockHeader.Bits(), params)
	if err != nil {
		return nil, err
	}
	shardHeader, ok := blockHeader.(*wire.ShardHeader)
	if !ok {
		return nil, errors.New("header cast failed")
	}

	beaconHeader := blockHeader.BeaconHeader()
	blockHeaderReply := btcjson.GetShardBlockHeaderVerboseResult{
		Hash:          c.Hash,
		ShardHash:     shardHeader.ShardBlockHash().String(),
		Confirmations: int64(1 + best.Height - blockHeight),
		Height:        blockHeight,
		NextHash:      nextHashString,
		PreviousHash:  blockHeader.PrevBlock().String(),
		MerkleRoot:    blockHeader.MerkleRoot().String(),
		Bits:          strconv.FormatInt(int64(blockHeader.Bits()), 16),
		Difficulty:    diff,
		Time:          blockHeader.Timestamp().Unix(),
		BCHeader: btcjson.GetBeaconBlockHeaderVerboseResult{
			Confirmations: 0,
			Height:        0,
			Difficulty:    0,
			NextHash:      "",

			Hash:                beaconHeader.BlockHash().String(),
			MerkleRoot:          beaconHeader.MerkleRoot().String(),
			MerkleMountainRange: beaconHeader.MergeMiningRoot().String(),
			Time:                beaconHeader.Timestamp().Unix(),
			Bits:                strconv.FormatInt(int64(beaconHeader.Bits()), 16),
			PreviousHash:        beaconHeader.PrevBlock().String(),
			Version:             int32(beaconHeader.Version()),
			VersionHex:          fmt.Sprintf("%08x", beaconHeader.Version()),
			Nonce:               uint64(beaconHeader.Nonce()),
		},
	}
	return blockHeaderReply, nil
}

// handleGetBlockTemplate implements the getShardBlockTemplate command.
//
// See https://en.bitcoin.it/wiki/BIP_0022 and
// https://en.bitcoin.it/wiki/BIP_0023 for more details.
func (server *ShardRPC) handleGetBlockTemplate(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetShardBlockTemplateCmd)
	request := c.Request

	// Set the default mode and override it if supplied.
	mode := "template"
	if request != nil && request.Mode != "" {
		mode = request.Mode
	}

	switch mode {
	case "template":
		return server.handleGetBlockTemplateRequest(request, closeChan)
	case "proposal":
		return server.handleGetBlockTemplateProposal(request)
	}

	return nil, &btcjson.RPCError{
		Code:    btcjson.ErrRPCInvalidParameter,
		Message: "Invalid mode",
	}
}

// handleGetBlockTemplateRequest is a helper for handleGetBlockTemplate which
// deals with generating and returning block templates to the caller.  It
// handles both long poll requests as specified by BIP 0022 as well as regular
// requests.  In addition, it detects the capabilities reported by the caller
// in regards to whether or not it supports creating its own coinbase (the
// coinbasetxn and coinbasevalue capabilities) and modifies the returned block
// template accordingly.
func (server *ShardRPC) handleGetBlockTemplateRequest(request *btcjson.TemplateRequest, closeChan <-chan struct{}) (interface{}, error) {
	// Extract the relevant passed capabilities and restrict the result to
	// either a coinbase value or a coinbase transaction object depending on
	// the request.  Default to only providing a coinbase value.
	useCoinbaseValue := true
	if request != nil {
		var hasCoinbaseValue, hasCoinbaseTxn bool
		for _, capability := range request.Capabilities {
			switch capability {
			case "coinbasetxn":
				hasCoinbaseTxn = true
			case "coinbasevalue":
				hasCoinbaseValue = true
			}
		}

		if hasCoinbaseTxn && !hasCoinbaseValue {
			useCoinbaseValue = false
		}
	}

	// todo: repair this logic
	// // When a coinbase transaction has been requested, respond with an error
	// // if there are no addresses to pay the created block template to.
	// if !useCoinbaseValue && len(server.cfg.MiningAddrs) == 0 {
	//	return nil, &btcjson.RPCError{
	//		Code: btcjson.ErrRPCInternal.Code,
	//		Message: "A coinbase transaction has been requested, " +
	//			"but the Server has not been configured with " +
	//			"any payment addresses via --miningaddr",
	//	}
	// }

	// todo: redo this
	// // Return an error if there are no peers connected since there is no
	// // way to relay a found block or receive transactions to work on.
	// // However, allow this state when running in the regression test or
	// // simulation test mode.
	// if !(cfg.RegressionTest || cfg.SimNet) &&
	//	server.cfg.connMgr.ConnectedCount() == 0 {
	//
	//	return nil, &btcjson.RPCError{
	//		Code:    btcjson.ErrRPCClientNotConnected,
	//		Message: "Bitcoin is not connected",
	//	}
	// }

	// No point in generating or accepting work before the BlockChain is synced.
	currentHeight := server.chainProvider.BlockChain().BestSnapshot().Height
	if currentHeight != 0 && !server.chainProvider.SyncManager.IsCurrent() {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCClientInInitialDownload,
			Message: "Bitcoin is downloading blocks...",
		}
	}

	// When a long poll ID was provided, this is a long poll request by the
	// client to be notified when block template referenced by the ID should
	// be replaced with a new one.
	if request != nil && request.LongPollID != "" {
		return server.handleGetBlockTemplateLongPoll(request.LongPollID, useCoinbaseValue, closeChan)
	}

	// Protect concurrent access when updating block templates.
	state := server.gbtWorkState
	state.Lock()
	defer state.Unlock()

	// Get and return a block template.  A new block template will be
	// generated when the current best block has changed or the transactions
	// in the memory pool have been updated and it has been at least five
	// seconds since the last template was generated.  Otherwise, the
	// timestamp for the existing block template is updated (and possibly
	// the difficulty on testnet per the consesus rules).
	if err := state.UpdateBlockTemplate(server.chainProvider, useCoinbaseValue); err != nil {
		return nil, err
	}
	return state.ShardBlockTemplateResult(useCoinbaseValue, nil)
}

// handleGetBlockTemplateProposal is a helper for handleGetBlockTemplate which
// deals with block proposals.
//
// See https://en.bitcoin.it/wiki/BIP_0023 for more details.
func (server *ShardRPC) handleGetBlockTemplateProposal(request *btcjson.TemplateRequest) (interface{}, error) {
	hexData := request.Data
	if hexData == "" {
		return false, &btcjson.RPCError{
			Code: btcjson.ErrRPCType,
			Message: fmt.Sprintf("Data must contain the " +
				"hex-encoded serialized block that is being " +
				"proposed"),
		}
	}

	// Ensure the provided data is sane and deserialize the proposed block.
	if len(hexData)%2 != 0 {
		hexData = "0" + hexData
	}
	dataBytes, err := hex.DecodeString(hexData)
	if err != nil {
		return false, &btcjson.RPCError{
			Code: btcjson.ErrRPCDeserialization,
			Message: fmt.Sprintf("Data must be "+
				"hexadecimal string (not %q)", hexData),
		}
	}
	var msgBlock = server.chainProvider.ChainCtx.EmptyBlock()
	if err := msgBlock.Deserialize(bytes.NewReader(dataBytes)); err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCDeserialization,
			Message: "Block decode failed: " + err.Error(),
		}
	}
	block := btcutil.NewBlock(&msgBlock)

	// Ensure the block is building from the expected previous block.
	expectedPrevHash := server.chainProvider.BlockChain().BestSnapshot().Hash
	prevHash := block.MsgBlock().Header.PrevBlock()
	if !expectedPrevHash.IsEqual(&prevHash) {
		return "bad-prevblk", nil
	}

	if err := server.chainProvider.BlockChain().CheckConnectBlockTemplate(block); err != nil {
		if _, ok := err.(chaindata.RuleError); !ok {
			errStr := fmt.Sprintf("Failed to process block proposal: %v", err)
			server.Log.Error().Msg(errStr)
			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCVerify,
				Message: errStr,
			}
		}

		server.Log.Info().Msgf("Rejected block proposal. %s", err.Error())
		return server.ChainErrToGBTErrString(err), nil
	}

	return nil, nil
}

// handleGetBlockTemplateLongPoll is a helper for handleGetBlockTemplateRequest
// which deals with handling long polling for block templates.  When a caller
// sends a request with a long poll ID that was previously returned, a response
// is not sent until the caller should stop working on the previous block
// template in favor of the new one.  In particular, this is the case when the
// old block template is no longer valid due to a solution already being found
// and added to the block BlockChain, or new transactions have shown up and some time
// has passed without finding a solution.
//
// See https://en.bitcoin.it/wiki/BIP_0022 for more details.
func (server *ShardRPC) handleGetBlockTemplateLongPoll(longPollID string, useCoinbaseValue bool, closeChan <-chan struct{}) (interface{}, error) {
	state := server.gbtWorkState
	state.Lock()
	// The state unlock is intentionally not deferred here since it needs to
	// be manually unlocked before waiting for a notification about block
	// template changes.

	if err := state.UpdateBlockTemplate(server.chainProvider, useCoinbaseValue); err != nil {
		state.Unlock()
		return nil, err
	}

	// Just return the current block template if the long poll ID provided by
	// the caller is invalid.
	prevHash, lastGenerated, err := server.DecodeTemplateID(longPollID)
	if err != nil {
		result, err := state.ShardBlockTemplateResult(useCoinbaseValue, nil)
		if err != nil {
			state.Unlock()
			return nil, err
		}

		state.Unlock()
		return result, nil
	}

	// Return the block template now if the specific block template
	// identified by the long poll ID no longer matches the current block
	// template as this means the provided template is stale.
	prevTemplateHash := state.Template.Block.Header.PrevBlock()
	if !prevHash.IsEqual(&prevTemplateHash) ||
		lastGenerated != state.LastGenerated.Unix() {

		// Include whether or not it is valid to submit work against the
		// old block template depending on whether or not a solution has
		// already been found and added to the block BlockChain.
		submitOld := prevHash.IsEqual(&prevTemplateHash)
		result, err := state.ShardBlockTemplateResult(useCoinbaseValue, &submitOld)
		if err != nil {
			state.Unlock()
			return nil, err
		}

		state.Unlock()
		return result, nil
	}

	// Register the previous hash and last generated time for notifications
	// Get a channel that will be notified when the template associated with
	// the provided ID is stale and a new block template should be returned to
	// the caller.
	longPollChan := state.TemplateUpdateChan(prevHash, lastGenerated)
	state.Unlock()

	select {
	// When the client closes before it'server time to send a reply, just return
	// now so the goroutine doesn't hang around.
	case <-closeChan:
		return nil, ErrClientQuit

	// Wait until signal received to send the reply.
	case <-longPollChan:
		// Fallthrough
	}

	// Get the lastest block template
	state.Lock()
	defer state.Unlock()

	if err := state.UpdateBlockTemplate(server.chainProvider, useCoinbaseValue); err != nil {
		return nil, err
	}

	// Include whether or not it is valid to submit work against the old
	// block template depending on whether or not a solution has already
	// been found and added to the block BlockChain.
	h := state.Template.Block.Header.PrevBlock()
	submitOld := prevHash.IsEqual(&h)
	result, err := state.ShardBlockTemplateResult(useCoinbaseValue, &submitOld)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// handleGetBlockBySerialNumber implements the getBlockBySerialNumber command.
func (server *ShardRPC) handleGetBlockBySerialNumber(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {

	c := cmd.(*btcjson.GetBlockBySerialNumberCmd)
	serialNumber := c.SerialNumber

	var (
		hash     *chainhash.Hash
		blkBytes []byte
		prevID   int
	)
	err := server.chainProvider.DB.View(func(dbTx database.Tx) error {
		var err error
		hash, prevID, err = chaindata.DBFetchBlockHashPrevBySerialID(dbTx, serialNumber)
		if err != nil {
			return err
		}

		blkBytes, err = dbTx.FetchBlock(hash)
		return err
	})

	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}

	// If verbosity is 0, return the serialized block as a hex encoded string.
	if c.Verbosity != nil && *c.Verbosity == 0 {
		return hex.EncodeToString(blkBytes), nil
	}

	// Otherwise, generate the JSON object and return it.

	// Deserialize the block.
	blk, err := btcutil.NewBlockFromBytes(server.chainProvider.DB.Chain(), blkBytes)
	if err != nil {
		context := "Failed to deserialize block"
		return nil, server.InternalRPCError(err.Error(), context)
	}

	// Get the block height from BlockChain.
	blockHeight, err := server.chainProvider.BlockChain().BlockHeightByHash(hash)
	if err != nil {
		context := "Failed to obtain block height"
		return nil, server.InternalRPCError(err.Error(), context)
	}
	blk.SetHeight(blockHeight)
	best := server.chainProvider.BlockChain().BestSnapshot()

	// Get next block hash unless there are none.
	var nextHashString string
	if blockHeight < best.Height {
		nextHash, err := server.chainProvider.BlockChain().BlockHashByHeight(blockHeight + 1)
		if err != nil {
			context := "No next block"
			return nil, server.InternalRPCError(err.Error(), context)
		}
		nextHashString = nextHash.String()
	}

	params := server.chainProvider.ChainParams
	blockHeader := blk.MsgBlock().Header.(*wire.ShardHeader)
	diff, err := server.GetDifficultyRatio(blockHeader.Bits(), params)
	if err != nil {
		return nil, err
	}

	beaconHeader := blockHeader.BeaconHeader()
	blockReply := btcjson.GetShardBlockBySerialNumberVerboseResult{
		Hash:          hash.String(),
		ShardHash:     blockHeader.ShardBlockHash().String(),
		MerkleRoot:    blockHeader.MerkleRoot().String(),
		PreviousHash:  blockHeader.PrevBlock().String(),
		Time:          blockHeader.Timestamp().Unix(),
		Confirmations: int64(1 + best.Height - blockHeight),
		Height:        int64(blockHeight),
		Size:          int32(len(blkBytes)),
		StrippedSize:  int32(blk.MsgBlock().SerializeSizeStripped()),
		Weight:        int32(chaindata.GetBlockWeight(blk)),
		Bits:          strconv.FormatInt(int64(blockHeader.Bits()), 16),
		Difficulty:    diff,
		NextHash:      nextHashString,
		BCBlock: btcjson.GetBeaconBlockVerboseResult{
			Confirmations: 0,
			StrippedSize:  0,
			Size:          0,
			Weight:        0,
			Height:        0,
			Tx:            nil,
			RawTx:         nil,
			Time:          0,
			Difficulty:    0,
			PreviousHash:  "",
			NextHash:      "",

			Bits:                strconv.FormatInt(int64(beaconHeader.Bits()), 16),
			Hash:                beaconHeader.BlockHash().String(),
			MerkleRoot:          beaconHeader.MerkleRoot().String(),
			MerkleMountainRange: beaconHeader.MergeMiningRoot().String(),
			Version:             int32(blockHeader.Version()),
			VersionHex:          fmt.Sprintf("%08x", blockHeader.Version()),
			Nonce:               blockHeader.Nonce(),
		},
		SerialID:     serialNumber,
		PrevSerialID: prevID,
	}

	if *c.Verbosity == 1 {
		transactions := blk.Transactions()
		txNames := make([]string, len(transactions))
		for i, tx := range transactions {
			txNames[i] = tx.Hash().String()
		}

		blockReply.Tx = txNames
	} else {
		txns := blk.Transactions()
		rawTxns := make([]btcjson.TxRawResult, len(txns))
		for i, tx := range txns {
			rawTxn, err := server.CreateTxRawResult(params, tx.MsgTx(),
				tx.Hash().String(), blockHeader, hash.String(),
				blockHeight, best.Height)
			if err != nil {
				return nil, err
			}
			rawTxns[i] = *rawTxn
		}
		blockReply.RawTx = rawTxns
	}

	return blockReply, nil
}
