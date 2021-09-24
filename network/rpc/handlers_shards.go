// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/netsync"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
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
		jaxjson.ScopedMethod("shard", "getShardHeaders"):       server.handleGetHeaders,
		jaxjson.ScopedMethod("shard", "getShardBlock"):         server.handleGetBlock,
		jaxjson.ScopedMethod("shard", "getShardBlockHeader"):   server.handleGetBlockHeader,
		jaxjson.ScopedMethod("shard", "getShardBlockTemplate"): server.handleGetBlockTemplate,
		// jaxjson.ScopedMethod("shard", "getShardBlockHash"):     server.handleGetBlockHash,
		jaxjson.ScopedMethod("shard", "getShardBlockBySerialNumber"):   server.handleGetBlockBySerialNumber,
		jaxjson.ScopedMethod("shard", "listShardBlocksBySerialNumber"): server.handleListBlocksBySerialNumber,
	}
}

// handleGetHeaders implements the getheaders co
// mmand.
//
// NOTE: This is a btcsuite extension originally ported from
// github.com/decred/dcrd.
func (server *ShardRPC) handleGetHeaders(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetShardHeadersCmd)

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
	c := cmd.(*jaxjson.GetShardBlockCmd)

	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	return server.getBlock(hash, c.Verbosity)
}

// handleGetBlockBySerialNumber implements the getBlockBySerialNumber command.
func (server *ShardRPC) handleGetBlockBySerialNumber(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetShardBlockBySerialNumberCmd)

	return server.getBlockBySerialID(c.Verbosity, c.SerialNumber)
}

func (server *ShardRPC) handleListBlocksBySerialNumber(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.ListShardBlocksBySerialNumberCmd)

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
			return nil, err
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
	blk, err := jaxutil.NewBlockFromBytes(server.chainProvider.DB.Chain(), blkBytes)
	if err != nil {
		context := "Failed to deserialize block"
		return nil, server.InternalRPCError(err.Error(), context)
	}

	var nextHashString string
	best := server.chainProvider.BlockChain().BestSnapshot()

	// Get the block height from BlockChain.
	blockHeight, serialID, prevSerialID, err := server.chainProvider.BlockChain().BlockIDsByHash(hash)
	if err == nil {
		blk.SetHeight(blockHeight)
		// Get next block hash unless there are none.
		// TODO: resolve next block from main chain
		if blockHeight != -1 && blockHeight < best.Height {
			nextHash, err := server.chainProvider.BlockChain().BlockHashByHeight(blockHeight + 1)
			if err != nil {
				context := "No next block"
				return nil, server.InternalRPCError(err.Error(), context)
			}
			nextHashString = nextHash.String()
		}
	}
	// If verbosity is 0, return the serialized block as a hex encoded string.
	if verbosity != nil && *verbosity == 0 {
		return jaxjson.GetShardBlockResult{
			Block:        hex.EncodeToString(blkBytes),
			Height:       blockHeight,
			SerialID:     serialID,
			PrevSerialID: prevSerialID,
		}, nil
	}

	// Otherwise, generate the JSON object and return it.
	params := server.chainProvider.ChainParams
	blockHeader := blk.MsgBlock().Header.(*wire.ShardHeader)
	diff, err := server.GetDifficultyRatio(blockHeader.Bits(), params)
	if err != nil {
		return nil, err
	}

	beaconHeader := blockHeader.BeaconHeader()
	blockReply := jaxjson.GetShardBlockVerboseResult{
		Hash:          hash.String(),
		ShardHash:     blockHeader.ShardExclusiveBlockHash().String(),
		MerkleRoot:    blockHeader.MerkleRoot().String(),
		PreviousHash:  blockHeader.BlocksMerkleMountainRoot().String(), // TODO: FIX MMR ROOT
		Time:          blockHeader.Timestamp().Unix(),
		Confirmations: int64(1 + best.Height - blockHeight),
		Height:        int64(blockHeight),
		Size:          int32(len(blkBytes)),
		StrippedSize:  int32(blk.MsgBlock().SerializeSizeStripped()),
		Weight:        int32(chaindata.GetBlockWeight(blk)),
		Bits:          strconv.FormatInt(int64(blockHeader.Bits()), 16),
		K:             strconv.FormatInt(int64(blockHeader.K()), 16),
		VoteK:         strconv.FormatInt(int64(blockHeader.VoteK()), 16),
		Difficulty:    diff,
		SerialID:      serialID,
		PrevSerialID:  prevSerialID,
		NextHash:      nextHashString,
		BCBlock: jaxjson.GetBeaconBlockVerboseResult{
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
			PoWHash:             blockHeader.PoWHash().String(),
			K:                   strconv.FormatInt(int64(blockHeader.K()), 16),
			VoteK:               strconv.FormatInt(int64(blockHeader.VoteK()), 16),
		},
	}

	if verbosity != nil && *verbosity == 1 {
		transactions := blk.Transactions()
		txNames := make([]string, len(transactions))
		for i, tx := range transactions {
			txNames[i] = tx.Hash().String()
		}

		blockReply.Tx = txNames
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
		blockReply.RawTx = rawTxns
	}

	return blockReply, nil
}

// handleGetBlockHeader implements the getblockheader command.
func (server *ShardRPC) handleGetBlockHeader(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetShardBlockHeaderCmd)

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
	var serialID, prevSerialID int64
	_ = server.chainProvider.DB.View(func(tx database.Tx) error {
		serialID, prevSerialID, err = chaindata.DBFetchBlockSerialID(tx, hash)
		return err
	})
	beaconHeader := blockHeader.BeaconHeader()
	blockHeaderReply := jaxjson.GetShardBlockHeaderVerboseResult{
		Hash:          c.Hash,
		ShardHash:     shardHeader.ShardExclusiveBlockHash().String(),
		Confirmations: int64(1 + best.Height - blockHeight),
		Height:        blockHeight,
		SerialID:      serialID,
		PrevSerialID:  prevSerialID,
		NextHash:      nextHashString,
		PreviousHash:  blockHeader.BlocksMerkleMountainRoot().String(), // TODO: FIX MMR ROOT
		MerkleRoot:    blockHeader.MerkleRoot().String(),
		Bits:          strconv.FormatInt(int64(blockHeader.Bits()), 16),
		K:             strconv.FormatInt(int64(blockHeader.K()), 16),
		VoteK:         strconv.FormatInt(int64(blockHeader.VoteK()), 16),
		Difficulty:    diff,
		Time:          blockHeader.Timestamp().Unix(),
		BCHeader: jaxjson.GetBeaconBlockHeaderVerboseResult{
			Confirmations: 0,
			Height:        0,
			Difficulty:    0,
			NextHash:      "",

			Hash:                beaconHeader.BlockHash().String(),
			MerkleRoot:          beaconHeader.MerkleRoot().String(),
			MerkleMountainRange: beaconHeader.MergeMiningRoot().String(),
			Time:                beaconHeader.Timestamp().Unix(),
			Bits:                strconv.FormatInt(int64(beaconHeader.Bits()), 16),
			PreviousHash:        beaconHeader.BlocksMerkleMountainRoot().String(), // TODO: FIX MMR ROOT
			Version:             int32(beaconHeader.Version()),
			VersionHex:          fmt.Sprintf("%08x", beaconHeader.Version()),
			Nonce:               uint64(beaconHeader.Nonce()),
			K:                   strconv.FormatInt(int64(blockHeader.K()), 16),
			VoteK:               strconv.FormatInt(int64(blockHeader.VoteK()), 16),
		},
	}
	return blockHeaderReply, nil
}

// handleGetBlockTemplate implements the getShardBlockTemplate command.
//
// See https://en.bitcoin.it/wiki/BIP_0022 and
// https://en.bitcoin.it/wiki/BIP_0023 for more details.
func (server *ShardRPC) handleGetBlockTemplate(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetShardBlockTemplateCmd)
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

	return nil, &jaxjson.RPCError{
		Code:    jaxjson.ErrRPCInvalidParameter,
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
func (server *ShardRPC) handleGetBlockTemplateRequest(request *jaxjson.TemplateRequest, closeChan <-chan struct{}) (interface{}, error) {
	// Extract the relevant passed capabilities and restrict the result to
	// either a coinbase value or a coinbase transaction object depending on
	// the request.  Default to only providing a coinbase value.
	useCoinbaseValue := true
	burnReward := 0
	if request != nil {
		var hasCoinbaseValue, hasCoinbaseTxn bool
		for _, capability := range request.Capabilities {
			switch capability {
			case "coinbasetxn":
				hasCoinbaseTxn = true
			case "coinbasevalue":
				hasCoinbaseValue = true
			case "burnbtcreward":
				burnReward |= types.BurnBtcReward
			case "burnjaxnetreward":
				burnReward |= types.BurnJaxNetReward
			case "burnjaxreward":
				burnReward |= types.BurnJaxReward
			}
		}

		if hasCoinbaseTxn && !hasCoinbaseValue {
			useCoinbaseValue = false
		}
	}

	// When a coinbase transaction has been requested, respond with an error
	// if there are no addresses to pay the created block template to.
	if !useCoinbaseValue && len(server.chainProvider.MiningAddrs) == 0 {
		return nil, &jaxjson.RPCError{
			Code: jaxjson.ErrRPCInternal.Code,
			Message: "A coinbase transaction has been requested, " +
				"but the Server has not been configured with " +
				"any payment addresses via --miningaddr",
		}
	}

	// Return an error if there are no peers connected since there is no
	// way to relay a found block or receive transactions to work on.
	// However, allow this state when running in the regression test or
	// simulation test mode.
	netType := server.chainProvider.ChainParams.Net
	if !(netType == types.FastTestNet || netType == types.SimNet) &&
		server.connMgr.ConnectedCount() == 0 {

		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCClientNotConnected,
			Message: "JaxNetD Shard Chain is not connected",
		}
	}

	// No point in generating or accepting work before the BlockChain is synced.
	currentHeight := server.chainProvider.BlockChain().BestSnapshot().Height
	if currentHeight != 0 && !server.chainProvider.SyncManager.IsCurrent() {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCClientInInitialDownload,
			Message: "JaxNetD Shard Chain is downloading blocks...",
		}
	}

	// When a long poll ID was provided, this is a long poll request by the
	// client to be notified when block template referenced by the ID should
	// be replaced with a new one.
	if request != nil && request.LongPollID != "" {
		return server.handleGetBlockTemplateLongPoll(request.LongPollID, useCoinbaseValue, burnReward, closeChan)
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
	if err := state.UpdateBlockTemplate(server.chainProvider, useCoinbaseValue, burnReward); err != nil {
		return nil, err
	}
	return state.ShardBlockTemplateResult(useCoinbaseValue, nil)
}

// handleGetBlockTemplateProposal is a helper for handleGetBlockTemplate which
// deals with block proposals.
//
// See https://en.bitcoin.it/wiki/BIP_0023 for more details.
func (server *ShardRPC) handleGetBlockTemplateProposal(request *jaxjson.TemplateRequest) (interface{}, error) {
	hexData := request.Data
	if hexData == "" {
		return false, &jaxjson.RPCError{
			Code: jaxjson.ErrRPCType,
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
		return false, &jaxjson.RPCError{
			Code: jaxjson.ErrRPCDeserialization,
			Message: fmt.Sprintf("Data must be "+
				"hexadecimal string (not %q)", hexData),
		}
	}
	var msgBlock = server.chainProvider.ChainCtx.EmptyBlock()
	if err := msgBlock.Deserialize(bytes.NewReader(dataBytes)); err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCDeserialization,
			Message: "Block decode failed: " + err.Error(),
		}
	}
	block := jaxutil.NewBlock(&msgBlock)

	// Ensure the block is building from the expected previous block.
	expectedPrevHash := server.chainProvider.BlockChain().BestSnapshot().Hash
	prevHash := block.MsgBlock().Header.BlocksMerkleMountainRoot() // TODO: FIX MMR ROOT
	if !expectedPrevHash.IsEqual(&prevHash) {
		return "bad-prevblk", nil
	}

	if err := server.chainProvider.BlockChain().CheckConnectBlockTemplate(block); err != nil {
		if _, ok := err.(chaindata.RuleError); !ok {
			errStr := fmt.Sprintf("Failed to process block proposal: %v", err)
			server.Log.Error().Msg(errStr)
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCVerify,
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
func (server *ShardRPC) handleGetBlockTemplateLongPoll(longPollID string, useCoinbaseValue bool, burnReward int, closeChan <-chan struct{}) (interface{}, error) {
	state := server.gbtWorkState
	state.Lock()
	// The state unlock is intentionally not deferred here since it needs to
	// be manually unlocked before waiting for a notification about block
	// template changes.

	if err := state.UpdateBlockTemplate(server.chainProvider, useCoinbaseValue, burnReward); err != nil {
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
	prevTemplateHash := state.Template.Block.Header.BlocksMerkleMountainRoot() // TODO: FIX MMR ROOT
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

	if err := state.UpdateBlockTemplate(server.chainProvider, useCoinbaseValue, burnReward); err != nil {
		return nil, err
	}

	// Include whether or not it is valid to submit work against the old
	// block template depending on whether or not a solution has already
	// been found and added to the block BlockChain.
	h := state.Template.Block.Header.BlocksMerkleMountainRoot() // TODO: FIX MMR ROOT
	submitOld := prevHash.IsEqual(&h)
	result, err := state.ShardBlockTemplateResult(useCoinbaseValue, &submitOld)
	if err != nil {
		return nil, err
	}

	return result, nil
}
