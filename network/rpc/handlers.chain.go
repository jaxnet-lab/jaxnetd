// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package rpc

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/core/shard.core/btcec"
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/database"
	"gitlab.com/jaxnet/core/shard.core/network/netsync"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain"
	"gitlab.com/jaxnet/core/shard.core/node/chaindata"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/node/encoder"
	"gitlab.com/jaxnet/core/shard.core/node/mempool"
	"gitlab.com/jaxnet/core/shard.core/node/mining"
	"gitlab.com/jaxnet/core/shard.core/txscript"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type CommonChainRPC struct {
	Mux

	// connMgr defines the connection manager for the RPC Server to use.  It
	// provides the RPC Server with a means to do things such as add,
	// remove, connect, disconnect, and query peers as well as other
	// connection-related data and tasks.
	connMgr netsync.P2PConnManager

	chainProvider *cprovider.ChainProvider
	gbtWorkState  *mining.GBTWorkState
	ntfnMgr       *wsChainManager
	helpCache     *helpCacher
}

func NewCommonChainRPC(chainProvider *cprovider.ChainProvider, connMgr netsync.P2PConnManager,
	logger zerolog.Logger) *CommonChainRPC {
	rpc := &CommonChainRPC{
		Mux:           NewRPCMux(logger),
		connMgr:       connMgr,
		chainProvider: chainProvider,
		gbtWorkState:  nil,
		helpCache:     nil,
	}
	rpc.ComposeHandlers()

	rpc.gbtWorkState = chainProvider.GbtWorkState()
	rpc.helpCache = newHelpCacher(rpc)
	return rpc
}

func (server *CommonChainRPC) BlockGenerator(useCoinbaseValue bool) (mining.BlockTemplate, error) {
	return server.gbtWorkState.BlockTemplate(server.chainProvider, useCoinbaseValue)
}

func (server *CommonChainRPC) Handlers() map[btcjson.MethodName]CommandHandler {
	return server.handlers
}

func (server *CommonChainRPC) ComposeHandlers() {
	server.SetCommands(server.OwnHandlers())
}

func (server *CommonChainRPC) OwnHandlers() map[btcjson.MethodName]CommandHandler {
	return map[btcjson.MethodName]CommandHandler{
		// ---- p2p-related commands ------------------------------
		btcjson.ScopedMethod("chain", "addNode"):            server.handleAddNode,
		btcjson.ScopedMethod("chain", "getAddedNodeInfo"):   server.handleGetAddedNodeInfo,
		btcjson.ScopedMethod("chain", "getConnectionCount"): server.handleGetConnectionCount,
		btcjson.ScopedMethod("chain", "getNetTotals"):       server.handleGetNetTotals,
		btcjson.ScopedMethod("chain", "getPeerInfo"):        server.handleGetPeerInfo,
		btcjson.ScopedMethod("chain", "node"):               server.handleNode,
		btcjson.ScopedMethod("chain", "ping"):               server.handlePing,
		// -------------------------------------------------------------------------------------------------------------

		btcjson.ScopedMethod("chain", "decodeScript"):    server.handleDecodeScript,
		btcjson.ScopedMethod("chain", "getCurrentNet"):   server.handleGetCurrentNet,
		btcjson.ScopedMethod("chain", "validateAddress"): server.handleValidateAddress,
		btcjson.ScopedMethod("chain", "verifyMessage"):   server.handleVerifyMessage,
		btcjson.ScopedMethod("chain", "verifyChain"):     server.handleVerifyChain,

		// ---- tx-related commands -------------------------------
		btcjson.ScopedMethod("chain", "createRawTransaction"):  server.handleCreateRawTransaction,
		btcjson.ScopedMethod("chain", "decodeRawTransaction"):  server.handleDecodeRawTransaction,
		btcjson.ScopedMethod("chain", "estimateFee"):           server.handleEstimateFee,
		btcjson.ScopedMethod("chain", "estimateSmartFee"):      server.handleEstimateSmartFee,
		btcjson.ScopedMethod("chain", "getMempoolInfo"):        server.handleGetMempoolInfo,
		btcjson.ScopedMethod("chain", "getRawMempool"):         server.handleGetRawMempool,
		btcjson.ScopedMethod("chain", "getRawTransaction"):     server.handleGetRawTransaction,
		btcjson.ScopedMethod("chain", "getTxOut"):              server.handleGetTxOut,
		btcjson.ScopedMethod("chain", "sendRawTransaction"):    server.handleSendRawTransaction,
		btcjson.ScopedMethod("chain", "searchRawTransactions"): server.handleSearchRawTransactions,
		// -------------------------------------------------------------------------------------------------------------

		// ---- block-related commands ----------------------------
		btcjson.ScopedMethod("chain", "getBestBlock"):      server.handleGetBestBlock,
		btcjson.ScopedMethod("chain", "getBestBlockHash"):  server.handleGetBestBlockHash,
		btcjson.ScopedMethod("chain", "getBlockchainInfo"): server.handleGetBlockChainInfo,
		btcjson.ScopedMethod("chain", "getBlockCount"):     server.handleGetBlockCount,
		btcjson.ScopedMethod("chain", "getBlockHash"):      server.handleGetBlockHash,
		btcjson.ScopedMethod("chain", "getCFilter"):        server.handleGetCFilter,
		btcjson.ScopedMethod("chain", "getCFilterHeader"):  server.handleGetCFilterHeader,
		btcjson.ScopedMethod("chain", "submitBlock"):       server.handleSubmitBlock,
		// -------------------------------------------------------------------------------------------------------------
	}
}

// Callback for notifications from blockchain.  It notifies clients that are
// long polling for changes or subscribed to websockets notifications.
func (server *CommonChainRPC) handleBlockchainNotification(notification *blockchain.Notification) {
	switch notification.Type {
	case blockchain.NTBlockAccepted:
		block, ok := notification.Data.(*btcutil.Block)
		if !ok {
			server.Log.Warn().Msgf("Chain accepted notification is not a block.")
			break
		}

		// Allow any clients performing long polling via the
		// getblocktemplate RPC to be notified when the new block causes
		// their old block template to become stale.
		server.gbtWorkState.NotifyBlockConnected(block.Hash())

	case blockchain.NTBlockConnected:
		block, ok := notification.Data.(*btcutil.Block)
		if !ok {
			server.Log.Warn().Msg("Chain connected notification is not a block.")
			break
		}
		//Notify registered websocket clients of incoming block.
		server.ntfnMgr.NotifyBlockConnected(server.chainProvider, block)

	case blockchain.NTBlockDisconnected:
		block, ok := notification.Data.(*btcutil.Block)
		if !ok {
			server.Log.Warn().Msg("Chain disconnected notification is not a block.")
			break
		}
		//
		//Notify registered websocket clients.
		server.ntfnMgr.NotifyBlockDisconnected(server.chainProvider, block)
	}
}

// fetchInputTxos fetches the outpoints from all transactions referenced by the
// inputs to the passed transaction by checking the transaction mempool first
// then the transaction index for those already mined into blocks.
func (server *CommonChainRPC) fetchInputTxos(tx *wire.MsgTx) (map[wire.OutPoint]wire.TxOut, error) {
	mp := server.chainProvider.TxMemPool
	originOutputs := make(map[wire.OutPoint]wire.TxOut)
	for txInIndex, txIn := range tx.TxIn {
		// Attempt to fetch and use the referenced transaction from the
		// memory pool.
		origin := &txIn.PreviousOutPoint
		originTx, err := mp.FetchTransaction(&origin.Hash)
		if err == nil {
			txOuts := originTx.MsgTx().TxOut
			if origin.Index >= uint32(len(txOuts)) {
				errStr := fmt.Sprintf("unable to find output %v referenced from transaction %s:%d",
					origin, tx.TxHash(), txInIndex)
				return nil, server.InternalRPCError(errStr, "")
			}

			originOutputs[*origin] = *txOuts[origin.Index]
			continue
		}

		// Look up the location of the transaction.
		blockRegion, err := server.chainProvider.TxIndex.TxBlockRegion(&origin.Hash)
		if err != nil {
			context := "Failed to retrieve transaction location"
			return nil, server.InternalRPCError(err.Error(), context)
		}
		if blockRegion == nil {
			return nil, rpcNoTxInfoError(&origin.Hash)
		}

		// Load the raw transaction bytes from the database.
		var txBytes []byte
		err = server.chainProvider.DB.View(func(dbTx database.Tx) error {
			var err error
			txBytes, err = dbTx.FetchBlockRegion(blockRegion)
			return err
		})
		if err != nil {
			return nil, rpcNoTxInfoError(&origin.Hash)
		}

		// Deserialize the transaction
		var msgTx wire.MsgTx
		err = msgTx.Deserialize(bytes.NewReader(txBytes))
		if err != nil {
			context := "Failed to deserialize transaction"
			return nil, server.InternalRPCError(err.Error(), context)
		}

		// Add the referenced output to the map.
		if origin.Index >= uint32(len(msgTx.TxOut)) {
			errStr := fmt.Sprintf("unable to find output %v referenced from transaction %s:%d", origin,
				tx.TxHash(), txInIndex)
			return nil, server.InternalRPCError(errStr, "")
		}
		originOutputs[*origin] = *msgTx.TxOut[origin.Index]
	}

	return originOutputs, nil
}

func (server *CommonChainRPC) verifyChain(level, depth int32) error {
	best := server.chainProvider.BlockChain().BestSnapshot()
	finishHeight := best.Height - depth
	if finishHeight < 0 {
		finishHeight = 0
	}
	server.Log.Info().Msgf("Verifying BlockChain for %d blocks at level %d",
		best.Height-finishHeight, level)

	for height := best.Height; height > finishHeight; height-- {
		// Level 0 just looks up the block.
		block, err := server.chainProvider.BlockChain().BlockByHeight(height)
		if err != nil {
			server.Log.Error().Msgf("Verify is unable to fetch block at "+
				"height %d: %v", height, err)
			return err
		}

		// Level 1 does basic BlockChain sanity checks.
		if level > 0 {
			err := chaindata.CheckBlockSanity(block,
				server.chainProvider.ChainParams.PowLimit, server.chainProvider.TimeSource)
			if err != nil {
				server.Log.Error().Msgf("Verify is unable to validate "+
					"block at hash %v height %d: %v",
					block.Hash(), height, err)
				return err
			}
		}
	}
	server.Log.Info().Msgf("Chain verify completed successfully")

	return nil
}

// handleUnimplemented is the handler for commands that should ultimately be
// supported but are not yet implemented.
func (server *CommonChainRPC) handleUnimplemented(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return nil, ErrRPCUnimplemented
}

// handleAskWallet is the handler for commands that are recognized as valid, but
// are unable to answer correctly since it involves wallet state.
// These commands will be implemented in btcwallet.
func (server *CommonChainRPC) handleAskWallet(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return nil, ErrRPCNoWallet
}

// handleDecodeScript handles decodescript commands.
func (server *CommonChainRPC) handleDecodeScript(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.DecodeScriptCmd)

	// Convert the hex script to bytes.
	hexStr := c.HexScript
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	script, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, rpcDecodeHexError(hexStr)
	}

	// The disassembled string will contain [error] inline if the script
	// doesn't fully parse, so ignore the error here.
	disbuf, _ := txscript.DisasmString(script)

	// Get information about the script.
	// Ignore the error here since an error means the script couldn't parse
	// and there is no additinal information about it anyways.
	scriptClass, addrs, reqSigs, _ := txscript.ExtractPkScriptAddrs(script,
		server.chainProvider.ChainParams)
	addresses := make([]string, len(addrs))
	for i, addr := range addrs {
		addresses[i] = addr.EncodeAddress()
	}

	// Convert the script itself to a pay-to-script-hash address.
	p2sh, err := btcutil.NewAddressScriptHash(script, server.chainProvider.ChainParams)
	if err != nil {
		context := "Failed to convert script to pay-to-script-hash"
		return nil, server.InternalRPCError(err.Error(), context)
	}

	// Generate and return the reply.
	reply := btcjson.DecodeScriptResult{
		Asm:       disbuf,
		ReqSigs:   int32(reqSigs),
		Type:      scriptClass.String(),
		Addresses: addresses,
	}
	if scriptClass != txscript.ScriptHashTy {
		reply.P2sh = p2sh.EncodeAddress()
	}
	return reply, nil
}

// handleGetBestBlock implements the getbestblock command.
func (server *CommonChainRPC) handleGetBestBlock(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// All other "get block" commands give either the height, the
	// hash, or both but require the block SHA.  This gets both for
	// the best block.
	best := server.chainProvider.BlockChain().BestSnapshot()
	result := &btcjson.GetBestBlockResult{
		Hash:   best.Hash.String(),
		Height: best.Height,
	}
	return result, nil
}

// handleGetBestBlockHash implements the getbestblockhash command.
func (server *CommonChainRPC) handleGetBestBlockHash(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	best := server.chainProvider.BlockChain().BestSnapshot()
	return best.Hash.String(), nil
}

// handleGetBlockChainInfo implements the getblockchaininfo command.
func (server *CommonChainRPC) handleGetBlockChainInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Obtain a snapshot of the current best known blockchain state. We'll
	// populate the response to this call primarily from this snapshot.
	params := server.chainProvider.ChainParams
	blockChain := server.chainProvider.BlockChain()
	chainSnapshot := blockChain.BestSnapshot()
	diff, err := server.GetDifficultyRatio(chainSnapshot.Bits, params)
	if err != nil {
		return nil, err
	}

	shards, err := server.chainProvider.ShardCount()
	if err != nil {
		return nil, err
	}

	chainInfo := &btcjson.GetBlockChainInfoResult{
		Chain:         params.Name,
		Blocks:        chainSnapshot.Height,
		Headers:       chainSnapshot.Height,
		BestBlockHash: chainSnapshot.Hash.String(),
		Difficulty:    diff,
		MedianTime:    chainSnapshot.MedianTime.Unix(),
		Pruned:        false,
		Shards:        shards,
		SoftForks: &btcjson.SoftForks{
			Bip9SoftForks: make(map[string]*btcjson.Bip9SoftForkDescription),
		},
	}

	// Next, populate the response with information describing the current
	// status of soft-forks deployed via the super-majority block
	// signalling mechanism.
	height := chainSnapshot.Height
	chainInfo.SoftForks.SoftForks = []*btcjson.SoftForkDescription{
		{
			ID:      "bip34",
			Version: 2,
			Reject: struct {
				Status bool `json:"status"`
			}{
				Status: height >= params.BIP0034Height,
			},
		},
		{
			ID:      "bip66",
			Version: 3,
			Reject: struct {
				Status bool `json:"status"`
			}{
				Status: height >= params.BIP0066Height,
			},
		},
		{
			ID:      "bip65",
			Version: 4,
			Reject: struct {
				Status bool `json:"status"`
			}{
				Status: height >= params.BIP0065Height,
			},
		},
	}

	// Finally, query the BIP0009 version bits state for all currently
	// defined BIP0009 soft-fork deployments.
	for deployment, deploymentDetails := range params.Deployments {
		// Map the integer deployment ID into a human readable
		// fork-name.
		var forkName string
		switch deployment {
		case chaincfg.DeploymentCSV:
			forkName = "csv"

		case chaincfg.DeploymentSegwit:
			forkName = "segwit"

		default:
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCInternal.Code,
				Message: fmt.Sprintf("Unknown deployment %v "+
					"detected", deployment),
			}
		}

		// Query the BlockChain for the current status of the deployment as
		// identified by its deployment ID.
		deploymentStatus, err := blockChain.ThresholdState(uint32(deployment))
		if err != nil {
			context := "Failed to obtain deployment status"
			return nil, server.InternalRPCError(err.Error(), context)
		}

		// Attempt to convert the current deployment status into a
		// human readable string. If the status is unrecognized, then a
		// non-nil error is returned.
		statusString, err := server.SoftForkStatus(deploymentStatus)
		if err != nil {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCInternal.Code,
				Message: fmt.Sprintf("unknown deployment status: %v",
					deploymentStatus),
			}
		}

		// Finally, populate the soft-fork description with all the
		// information gathered above.
		chainInfo.SoftForks.Bip9SoftForks[forkName] = &btcjson.Bip9SoftForkDescription{
			Status:     strings.ToLower(statusString),
			Bit:        deploymentDetails.BitNumber,
			StartTime2: int64(deploymentDetails.StartTime),
			Timeout:    int64(deploymentDetails.ExpireTime),
		}
	}

	fmt.Printf("%+v\n", chainInfo)
	return chainInfo, nil
}

// handleGetBlockCount implements the getblockcount command.
func (server *CommonChainRPC) handleGetBlockCount(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	best := server.chainProvider.BlockChain().BestSnapshot()
	return int64(best.Height), nil
}

// handleGetBlockHash implements the getblockhash command.
func (server *CommonChainRPC) handleGetBlockHash(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetBlockHashCmd)
	hash, err := server.chainProvider.BlockChain().BlockHashByHeight(int32(c.Index))
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCOutOfRange,
			Message: "Block number out of range",
		}
	}

	return hash.String(), nil
}

// handleGetCFilter implements the getcfilter command.
func (server *CommonChainRPC) handleGetCFilter(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	if server.chainProvider.CfIndex == nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCNoCFIndex,
			Message: "The CF index must be enabled for this command",
		}
	}

	c := cmd.(*btcjson.GetCFilterCmd)
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}

	filterBytes, err := server.chainProvider.CfIndex.FilterByBlockHash(hash, c.FilterType)
	if err != nil {
		server.Log.Debug().Msgf("Could not find committed filter for %v %v", hash, err)
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}

	server.Log.Debug().Msgf("Found committed filter for %s", hash.String())
	return hex.EncodeToString(filterBytes), nil
}

// handleGetCFilterHeader implements the getcfilterheader command.
func (server *CommonChainRPC) handleGetCFilterHeader(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	if server.chainProvider.CfIndex == nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCNoCFIndex,
			Message: "The CF index must be enabled for this command",
		}
	}

	c := cmd.(*btcjson.GetCFilterHeaderCmd)
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}

	headerBytes, err := server.chainProvider.CfIndex.FilterHeaderByBlockHash(hash, c.FilterType)
	if len(headerBytes) > 0 {
		server.Log.Debug().Msgf("Found header of committed filter for %s", hash.String())
	} else {
		server.Log.Debug().Msgf("Could not find header of committed filter for %v %v", hash, err)
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}

	hash.SetBytes(headerBytes)
	return hash.String(), nil
}

// handleGetCurrentNet implements the getcurrentnet command.
func (server *CommonChainRPC) handleGetCurrentNet(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return server.chainProvider.ChainParams.Net, nil
}

// createVinListPrevOut returns a slice of JSON objects for the inputs of the
// passed transaction.
func (server *CommonChainRPC) createVinListPrevOut(mtx *wire.MsgTx, chainParams *chaincfg.Params, vinExtra bool, filterAddrMap map[string]struct{}) ([]btcjson.VinPrevOut, error) {
	// Coinbase transactions only have a single txin by definition.
	if chaindata.IsCoinBaseTx(mtx) {
		// Only include the transaction if the filter map is empty
		// because a coinbase input has no addresses and so would never
		// match a non-empty filter.
		if len(filterAddrMap) != 0 {
			return nil, nil
		}

		txIn := mtx.TxIn[0]
		vinList := make([]btcjson.VinPrevOut, 1)
		vinList[0].Coinbase = hex.EncodeToString(txIn.SignatureScript)
		vinList[0].Sequence = txIn.Sequence
		return vinList, nil
	}

	// Use a dynamically sized list to accommodate the address filter.
	vinList := make([]btcjson.VinPrevOut, 0, len(mtx.TxIn))

	// Lookup all of the referenced transaction outputs needed to populate
	// the previous output information if requested.
	var originOutputs map[wire.OutPoint]wire.TxOut
	if vinExtra || len(filterAddrMap) > 0 {
		var err error
		originOutputs, err = server.fetchInputTxos(mtx)
		if err != nil {
			return nil, err
		}
	}

	for _, txIn := range mtx.TxIn {
		// The disassembled string will contain [error] inline
		// if the script doesn't fully parse, so ignore the
		// error here.
		disbuf, _ := txscript.DisasmString(txIn.SignatureScript)

		// Create the basic input entry without the additional optional
		// previous output details which will be added later if
		// requested and available.
		prevOut := &txIn.PreviousOutPoint
		vinEntry := btcjson.VinPrevOut{
			Txid:     prevOut.Hash.String(),
			Vout:     prevOut.Index,
			Sequence: txIn.Sequence,
			ScriptSig: &btcjson.ScriptSig{
				Asm: disbuf,
				Hex: hex.EncodeToString(txIn.SignatureScript),
			},
		}

		if len(txIn.Witness) != 0 {
			vinEntry.Witness = server.WitnessToHex(txIn.Witness)
		}

		// Add the entry to the list now if it already passed the filter
		// since the previous output might not be available.
		passesFilter := len(filterAddrMap) == 0
		if passesFilter {
			vinList = append(vinList, vinEntry)
		}

		// Only populate previous output information if requested and
		// available.
		if len(originOutputs) == 0 {
			continue
		}
		originTxOut, ok := originOutputs[*prevOut]
		if !ok {
			continue
		}

		// Ignore the error here since an error means the script
		// couldn't parse and there is no additional information about
		// it anyways.
		_, addrs, _, _ := txscript.ExtractPkScriptAddrs(
			originTxOut.PkScript, chainParams)

		// Encode the addresses while checking if the address passes the
		// filter when needed.
		encodedAddrs := make([]string, len(addrs))
		for j, addr := range addrs {
			encodedAddr := addr.EncodeAddress()
			encodedAddrs[j] = encodedAddr

			// No need to check the map again if the filter already
			// passes.
			if passesFilter {
				continue
			}
			if _, exists := filterAddrMap[encodedAddr]; exists {
				passesFilter = true
			}
		}

		// Ignore the entry if it doesn't pass the filter.
		if !passesFilter {
			continue
		}

		// Add entry to the list if it wasn't already done above.
		if len(filterAddrMap) != 0 {
			vinList = append(vinList, vinEntry)
		}

		// Update the entry with previous output information if
		// requested.
		if vinExtra {
			vinListEntry := &vinList[len(vinList)-1]
			vinListEntry.PrevOut = &btcjson.PrevOut{
				Addresses: encodedAddrs,
				Value:     btcutil.Amount(originTxOut.Value).ToBTC(),
			}
		}
	}

	return vinList, nil
}

// fetchMempoolTxnsForAddress queries the address index for all unconfirmed
// transactions that involve the provided address.  The results will be limited
// by the number to skip and the number requested.
func (server *CommonChainRPC) fetchMempoolTxnsForAddress(addr btcutil.Address, numToSkip, numRequested uint32) ([]*btcutil.Tx, uint32) {
	// There are no entries to return when there are less available than the
	// number being skipped.
	mpTxns := server.chainProvider.AddrIndex.UnconfirmedTxnsForAddress(addr)
	numAvailable := uint32(len(mpTxns))
	if numToSkip > numAvailable {
		return nil, numAvailable
	}

	// Filter the available entries based on the number to skip and number
	// requested.
	rangeEnd := numToSkip + numRequested
	if rangeEnd > numAvailable {
		rangeEnd = numAvailable
	}
	return mpTxns[numToSkip:rangeEnd], numToSkip
}

// NotifyNewTransactions notifies both websocket and getblocktemplate long
// poll clients of the passed transactions.  This function should be called
// whenever new transactions are added to the mempool.
func (server *CommonChainRPC) NotifyNewTransactions(txns []*mempool.TxDesc) {
	for _, txD := range txns {
		// Notify websocket clients about mempool transactions.
		server.ntfnMgr.NotifyMempoolTx(txD.Tx, true)
	}
}

// handleSubmitBlock implements the submitblock command.
func (server *CommonChainRPC) handleSubmitBlock(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.SubmitBlockCmd)

	// Deserialize the submitted block.
	hexStr := c.HexBlock
	if len(hexStr)%2 != 0 {
		hexStr = "0" + c.HexBlock
	}

	serializedBlock, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, rpcDecodeHexError(hexStr)
	}

	block, err := btcutil.NewBlockFromBytes(server.chainProvider.DB.Chain(), serializedBlock)
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCDeserialization,
			Message: "Block decode failed: " + err.Error(),
		}
	}

	// Process this block using the same rules as blocks coming from other
	// nodes.  This will in turn relay it to the network like normal.
	_, err = server.chainProvider.SyncManager.ProcessBlock(block, chaindata.BFNone)
	if err != nil {
		return fmt.Sprintf("rejected: %s", err.Error()), nil
	}

	server.Log.Info().Msgf("Accepted block %s via submitblock", block.Hash().String())
	return nil, nil
}

// handleValidateAddress implements the validateaddress command.
func (server *CommonChainRPC) handleValidateAddress(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.ValidateAddressCmd)

	result := btcjson.ValidateAddressChainResult{}
	addr, err := btcutil.DecodeAddress(c.Address, server.chainProvider.ChainParams)
	if err != nil {
		// Return the default value (false) for IsValid.
		return result, nil
	}

	result.Address = addr.EncodeAddress()
	result.IsValid = true

	return result, nil
}

// handleVerifyChain implements the verifychain command.
func (server *CommonChainRPC) handleVerifyChain(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.VerifyChainCmd)

	var checkLevel, checkDepth int32
	if c.CheckLevel != nil {
		checkLevel = *c.CheckLevel
	}
	if c.CheckDepth != nil {
		checkDepth = *c.CheckDepth
	}

	err := server.verifyChain(checkLevel, checkDepth)
	return err == nil, nil
}

// handleVerifyMessage implements the verifymessage command.
func (server *CommonChainRPC) handleVerifyMessage(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.VerifyMessageCmd)

	// Decode the provided address.
	params := server.chainProvider.ChainParams
	addr, err := btcutil.DecodeAddress(c.Address, params)
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidAddressOrKey,
			Message: "Invalid address or key: " + err.Error(),
		}
	}

	// Only P2PKH addresses are valid for signing.
	if _, ok := addr.(*btcutil.AddressPubKeyHash); !ok {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCType,
			Message: "Address is not a pay-to-pubkey-hash address",
		}
	}

	// Decode base64 signature.
	sig, err := base64.StdEncoding.DecodeString(c.Signature)
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCParse.Code,
			Message: "Malformed base64 encoding: " + err.Error(),
		}
	}

	// Validate the signature - this just shows that it was valid at all.
	// we will compare it with the key next.
	var buf bytes.Buffer
	encoder.WriteVarString(&buf, 0, "Bitcoin Signed Message:\n")
	encoder.WriteVarString(&buf, 0, c.Message)
	expectedMessageHash := chainhash.DoubleHashB(buf.Bytes())
	pk, wasCompressed, err := btcec.RecoverCompact(btcec.S256(), sig,
		expectedMessageHash)
	if err != nil {
		// Mirror Bitcoin Core behavior, which treats error in
		// RecoverCompact as invalid signature.
		return false, nil
	}

	// Reconstruct the pubkey hash.
	var serializedPK []byte
	if wasCompressed {
		serializedPK = pk.SerializeCompressed()
	} else {
		serializedPK = pk.SerializeUncompressed()
	}
	address, err := btcutil.NewAddressPubKey(serializedPK, params)
	if err != nil {
		// Again mirror Bitcoin Core behavior, which treats error in public key
		// reconstruction as invalid signature.
		return false, nil
	}

	// Return boolean if addresses match.
	return address.EncodeAddress() == c.Address, nil
}

// directionString is a helper function that returns a string that represents
// the direction of a connection (inbound or outbound).
func directionString(inbound bool) string {
	if inbound {
		return "inbound"
	}
	return "outbound"
}
