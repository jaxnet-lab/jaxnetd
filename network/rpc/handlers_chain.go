// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/btcec"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/netsync"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/node/encoder"
	"gitlab.com/jaxnet/jaxnetd/node/mempool"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
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
	rpc.chainProvider.BlockChain().Subscribe(rpc.handleBlockchainNotification)
	return rpc
}

func (server *CommonChainRPC) IsBeacon() bool {
	return server.chainProvider.ChainCtx.IsBeacon()
}

func (server *CommonChainRPC) BlockGenerator(useCoinbaseValue bool, burnReward int) (mining.BlockTemplate, error) {
	return server.gbtWorkState.BlockTemplate(server.chainProvider, useCoinbaseValue, burnReward)
}

func (server *CommonChainRPC) Handlers() map[jaxjson.MethodName]CommandHandler {
	return server.handlers
}

func (server *CommonChainRPC) ComposeHandlers() {
	server.SetCommands(server.OwnHandlers())
}

func (server *CommonChainRPC) OwnHandlers() map[jaxjson.MethodName]CommandHandler {
	return map[jaxjson.MethodName]CommandHandler{
		// ---- p2p-related commands ------------------------------
		jaxjson.ScopedMethod("chain", "addNode"):            server.handleAddNode,
		jaxjson.ScopedMethod("chain", "getAddedNodeInfo"):   server.handleGetAddedNodeInfo,
		jaxjson.ScopedMethod("chain", "getConnectionCount"): server.handleGetConnectionCount,
		jaxjson.ScopedMethod("chain", "getNetTotals"):       server.handleGetNetTotals,
		jaxjson.ScopedMethod("chain", "getPeerInfo"):        server.handleGetPeerInfo,
		jaxjson.ScopedMethod("chain", "node"):               server.handleNode,
		jaxjson.ScopedMethod("chain", "ping"):               server.handlePing,
		// -------------------------------------------------------------------------------------------------------------

		jaxjson.ScopedMethod("chain", "decodeScript"):    server.handleDecodeScript,
		jaxjson.ScopedMethod("chain", "getCurrentNet"):   server.handleGetCurrentNet,
		jaxjson.ScopedMethod("chain", "validateAddress"): server.handleValidateAddress,
		jaxjson.ScopedMethod("chain", "verifyMessage"):   server.handleVerifyMessage,
		jaxjson.ScopedMethod("chain", "verifyChain"):     server.handleVerifyChain,

		// ---- tx-related commands -------------------------------
		jaxjson.ScopedMethod("chain", "createRawTransaction"):  server.handleCreateRawTransaction,
		jaxjson.ScopedMethod("chain", "decodeRawTransaction"):  server.handleDecodeRawTransaction,
		jaxjson.ScopedMethod("chain", "estimateFee"):           server.handleEstimateFee,
		jaxjson.ScopedMethod("chain", "estimateSmartFee"):      server.handleEstimateSmartFee,
		jaxjson.ScopedMethod("chain", "getExtendedFee"):        server.handleGetExtendedFee,
		jaxjson.ScopedMethod("chain", "getMempoolInfo"):        server.handleGetMempoolInfo,
		jaxjson.ScopedMethod("chain", "getRawMempool"):         server.handleGetRawMempool,
		jaxjson.ScopedMethod("chain", "getRawTransaction"):     server.handleGetRawTransaction,
		jaxjson.ScopedMethod("chain", "getTxDetails"):          server.handleGetTxDetails,
		jaxjson.ScopedMethod("chain", "getTxOut"):              server.handleGetTxOut,
		jaxjson.ScopedMethod("chain", "getTxOutsStatus"):       server.handleGetTxOutsStatus,
		jaxjson.ScopedMethod("chain", "listTxOut"):             server.handleListTxOut,
		jaxjson.ScopedMethod("chain", "getBlockTxOps"):         server.handleGetBlockTxOps,
		jaxjson.ScopedMethod("chain", "sendRawTransaction"):    server.handleSendRawTransaction,
		jaxjson.ScopedMethod("chain", "searchRawTransactions"): server.handleSearchRawTransactions,
		// -------------------------------------------------------------------------------------------------------------

		// ---- block-related commands ----------------------------
		jaxjson.ScopedMethod("chain", "getBestBlock"):             server.handleGetBestBlock,
		jaxjson.ScopedMethod("chain", "getBestBlockHash"):         server.handleGetBestBlockHash,
		jaxjson.ScopedMethod("chain", "getBlockchainInfo"):        server.handleGetBlockChainInfo,
		jaxjson.ScopedMethod("chain", "getBlockCount"):            server.handleGetBlockCount,
		jaxjson.ScopedMethod("chain", "getBlockHash"):             server.handleGetBlockHash,
		jaxjson.ScopedMethod("chain", "getCFilter"):               server.handleGetCFilter,
		jaxjson.ScopedMethod("chain", "getCFilterHeader"):         server.handleGetCFilterHeader,
		jaxjson.ScopedMethod("chain", "submitBlock"):              server.handleSubmitBlock,
		jaxjson.ScopedMethod("chain", "getLastSerialBlockNumber"): server.handleGetLastSerialBlockNumber,
		// -------------------------------------------------------------------------------------------------------------

		jaxjson.ScopedMethod("chain", "getnetworkinfo"):   server.handleGetnetworkinfo, // todo: remove
		jaxjson.ScopedMethod("chain", "getDifficulty"):    server.handleGetDifficulty,
		jaxjson.ScopedMethod("chain", "getmininginfo"):    server.handleGetMiningInfo,    // todo: remove
		jaxjson.ScopedMethod("chain", "getnetworkhashps"): server.handleGetNetworkHashPS, // todo: remove
		jaxjson.ScopedMethod("chain", "getblockstats"):    server.handleGetBlockStats,    // todo: remove
		jaxjson.ScopedMethod("chain", "getchaintxstats"):  server.handleGetChaintxStats,  // todo: remove

	}
}

// Callback for notifications from blockchain.  It notifies clients that are
// long polling for changes or subscribed to websockets notifications.
func (server *CommonChainRPC) handleBlockchainNotification(notification *blockchain.Notification) {
	switch notification.Type {
	case blockchain.NTBlockAccepted:
		block, ok := notification.Data.(*jaxutil.Block)
		if !ok {
			server.Log.Warn().Msgf("Chain accepted notification is not a block.")
			break
		}

		// Allow any clients performing long polling via the
		// getblocktemplate RPC to be notified when the new block causes
		// their old block template to become stale.
		server.gbtWorkState.NotifyBlockConnected(block.Hash())

	case blockchain.NTBlockConnected:
		block, ok := notification.Data.(*jaxutil.Block)
		if !ok {
			server.Log.Warn().Msg("Chain connected notification is not a block.")
			break
		}

		if server.ntfnMgr != nil {
			// Notify registered websocket clients of incoming block.
			server.ntfnMgr.NotifyBlockConnected(server.chainProvider, block)
		}

	case blockchain.NTBlockDisconnected:
		block, ok := notification.Data.(*jaxutil.Block)
		if !ok {
			server.Log.Warn().Msg("Chain disconnected notification is not a block.")
			break
		}
		if server.ntfnMgr != nil {
			// Notify registered websocket clients.
			server.ntfnMgr.NotifyBlockDisconnected(server.chainProvider, block)
		}
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
	c := cmd.(*jaxjson.DecodeScriptCmd)

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
	p2sh, err := jaxutil.NewAddressScriptHash(script, server.chainProvider.ChainParams)
	if err != nil {
		context := "Failed to convert script to pay-to-script-hash"
		return nil, server.InternalRPCError(err.Error(), context)
	}

	// Generate and return the reply.
	reply := jaxjson.DecodeScriptResult{
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
	result := &jaxjson.GetBestBlockResult{
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

	shards, err := blockChain.ShardCount()
	if err != nil {
		return nil, err
	}

	chainInfo := &jaxjson.GetBlockChainInfoResult{
		Chain:         params.Name,
		Blocks:        chainSnapshot.Height,
		Headers:       chainSnapshot.Height,
		BestBlockHash: chainSnapshot.Hash.String(),
		Difficulty:    diff,
		MedianTime:    chainSnapshot.MedianTime.Unix(),
		Pruned:        false,
		Shards:        shards,
		SoftForks: &jaxjson.SoftForks{
			Bip9SoftForks: make(map[string]*jaxjson.Bip9SoftForkDescription),
		},
	}

	// Next, populate the response with information describing the current
	// status of soft-forks deployed via the super-majority block
	// signalling mechanism.
	height := chainSnapshot.Height
	chainInfo.SoftForks.SoftForks = []*jaxjson.SoftForkDescription{
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
			return nil, &jaxjson.RPCError{
				Code: jaxjson.ErrRPCInternal.Code,
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
			return nil, &jaxjson.RPCError{
				Code: jaxjson.ErrRPCInternal.Code,
				Message: fmt.Sprintf("unknown deployment status: %v",
					deploymentStatus),
			}
		}

		// Finally, populate the soft-fork description with all the
		// information gathered above.
		chainInfo.SoftForks.Bip9SoftForks[forkName] = &jaxjson.Bip9SoftForkDescription{
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
	c := cmd.(*jaxjson.GetBlockHashCmd)
	hash, err := server.chainProvider.BlockChain().BlockHashByHeight(int32(c.Index))
	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCOutOfRange,
			Message: "Block number out of range",
		}
	}

	return hash.String(), nil
}

// handleGetCFilter implements the getcfilter command.
func (server *CommonChainRPC) handleGetCFilter(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	if server.chainProvider.CfIndex == nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCNoCFIndex,
			Message: "The CF index must be enabled for this command",
		}
	}

	c := cmd.(*jaxjson.GetCFilterCmd)
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}

	filterBytes, err := server.chainProvider.CfIndex.FilterByBlockHash(hash, c.FilterType)
	if err != nil {
		server.Log.Debug().Msgf("Could not find committed filter for %v %v", hash, err)
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}

	server.Log.Debug().Msgf("Found committed filter for %s", hash.String())
	return hex.EncodeToString(filterBytes), nil
}

// handleGetCFilterHeader implements the getcfilterheader command.
func (server *CommonChainRPC) handleGetCFilterHeader(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	if server.chainProvider.CfIndex == nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCNoCFIndex,
			Message: "The CF index must be enabled for this command",
		}
	}

	c := cmd.(*jaxjson.GetCFilterHeaderCmd)
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}

	headerBytes, err := server.chainProvider.CfIndex.FilterHeaderByBlockHash(hash, c.FilterType)
	if len(headerBytes) > 0 {
		server.Log.Debug().Msgf("Found header of committed filter for %s", hash.String())
	} else {
		server.Log.Debug().Msgf("Could not find header of committed filter for %v %v", hash, err)
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCBlockNotFound,
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
func (server *CommonChainRPC) createVinListPrevOut(mtx *wire.MsgTx, chainParams *chaincfg.Params, vinExtra bool, filterAddrMap map[string]struct{}) ([]jaxjson.VinPrevOut, error) {
	// Coinbase transactions only have a single txin by definition.
	if chaindata.IsCoinBaseTx(mtx) {
		// Only include the transaction if the filter map is empty
		// because a coinbase input has no addresses and so would never
		// match a non-empty filter.
		if len(filterAddrMap) != 0 {
			return nil, nil
		}

		txIn := mtx.TxIn[0]
		vinList := make([]jaxjson.VinPrevOut, 1)
		vinList[0].Coinbase = hex.EncodeToString(txIn.SignatureScript)
		vinList[0].Sequence = txIn.Sequence
		return vinList, nil
	}

	// Use a dynamically sized list to accommodate the address filter.
	vinList := make([]jaxjson.VinPrevOut, 0, len(mtx.TxIn))

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
		vinEntry := jaxjson.VinPrevOut{
			Txid:     prevOut.Hash.String(),
			Vout:     prevOut.Index,
			Sequence: txIn.Sequence,
			ScriptSig: &jaxjson.ScriptSig{
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
			vinListEntry.PrevOut = &jaxjson.PrevOut{
				Addresses:    encodedAddrs,
				Value:        jaxutil.Amount(originTxOut.Value).ToCoin(server.IsBeacon()),
				PreciseValue: originTxOut.Value,
			}
		}
	}

	return vinList, nil
}

// fetchMempoolTxnsForAddress queries the address index for all unconfirmed
// transactions that involve the provided address.  The results will be limited
// by the number to skip and the number requested.
func (server *CommonChainRPC) fetchMempoolTxnsForAddress(addr jaxutil.Address, numToSkip, numRequested uint32) ([]*jaxutil.Tx, uint32) {
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
	if server.ntfnMgr == nil {
		return
	}
	for _, txD := range txns {
		// Notify websocket clients about mempool transactions.
		server.ntfnMgr.NotifyMempoolTx(txD.Tx, true)
	}
}

// handleSubmitBlock implements the submitblock command.
func (server *CommonChainRPC) handleSubmitBlock(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.SubmitBlockCmd)

	// Deserialize the submitted block.
	hexStr := c.HexBlock
	if len(hexStr)%2 != 0 {
		hexStr = "0" + c.HexBlock
	}

	serializedBlock, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, rpcDecodeHexError(hexStr)
	}

	block, err := jaxutil.NewBlockFromBytes(server.chainProvider.DB.Chain(), serializedBlock)
	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCDeserialization,
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
	c := cmd.(*jaxjson.ValidateAddressCmd)

	result := jaxjson.ValidateAddressChainResult{}
	addr, err := jaxutil.DecodeAddress(c.Address, server.chainProvider.ChainParams)
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
	c := cmd.(*jaxjson.VerifyChainCmd)

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
	c := cmd.(*jaxjson.VerifyMessageCmd)

	// Decode the provided address.
	params := server.chainProvider.ChainParams
	addr, err := jaxutil.DecodeAddress(c.Address, params)
	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInvalidAddressOrKey,
			Message: "Invalid address or key: " + err.Error(),
		}
	}

	// Only P2PKH addresses are valid for signing.
	if _, ok := addr.(*jaxutil.AddressPubKeyHash); !ok {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCType,
			Message: "Address is not a pay-to-pubkey-hash address",
		}
	}

	// Decode base64 signature.
	sig, err := base64.StdEncoding.DecodeString(c.Signature)
	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCParse.Code,
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
	address, err := jaxutil.NewAddressPubKey(serializedPK, params)
	if err != nil {
		// Again mirror Bitcoin Core behavior, which treats error in public key
		// reconstruction as invalid signature.
		return false, nil
	}

	// Return boolean if addresses match.
	return address.EncodeAddress() == c.Address, nil
}

// handleGetLastSerialBlockNumber implements the getLastSerialBlockNumber command.
func (server *CommonChainRPC) handleGetLastSerialBlockNumber(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	var lastSerial int64
	err := server.chainProvider.DB.View(func(dbTx database.Tx) error {
		var err error
		lastSerial, err = chaindata.DBFetchLastSerialID(dbTx)
		return err
	})

	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCBlockNotFound,
			Message: err.Error(),
		}
	}

	result := &jaxjson.GetLastSerialBlockNumberResult{
		LastSerial: lastSerial,
	}

	return result, nil
}

// directionString is a helper function that returns a string that represents
// the direction of a connection (inbound or outbound).
func directionString(inbound bool) string {
	if inbound {
		return "inbound"
	}
	return "outbound"
}

// handleGetMiningInfo implements the getmininginfo command. We only return the
// fields that are not related to wallet functionality.
func (server *CommonChainRPC) handleGetMiningInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Create a default getnetworkhashps command to use defaults and make
	// use of the existing getnetworkhashps handler.
	gnhpsCmd := jaxjson.NewGetNetworkHashPSCmd(nil, nil)
	networkHashesPerSecIface, err := server.handleGetNetworkHashPS(gnhpsCmd, closeChan)
	if err != nil {
		return nil, err
	}
	networkHashesPerSec, ok := networkHashesPerSecIface.(int64)
	if !ok {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInternal.Code,
			Message: "networkHashesPerSec is not an int64",
		}
	}

	best := server.chainProvider.BlockChain().BestSnapshot()
	diff, err := server.GetDifficultyRatio(best.Bits, server.chainProvider.ChainParams)
	if err != nil {
		return nil, err
	}
	result := jaxjson.GetMiningInfoResult{
		Blocks:             int64(best.Height),
		CurrentBlockSize:   best.BlockSize,
		CurrentBlockWeight: best.BlockWeight,
		CurrentBlockTx:     best.NumTxns,
		Difficulty:         diff,
		NetworkHashPS:      networkHashesPerSec,
		PooledTx:           uint64(server.chainProvider.TxMemPool.Count()),
		// TestNet:            server.cfg.TestNet3,
	}
	return &result, nil
}

// handleGetNetworkHashPS implements the getnetworkhashps command.
func (server *CommonChainRPC) handleGetNetworkHashPS(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Note: All valid error return paths should return an int64.
	// Literal zeros are inferred as int, and won't coerce to int64
	// because the return value is an interface{}.

	c := cmd.(*jaxjson.GetNetworkHashPSCmd)
	if server.chainProvider.BlockChain() == nil {
		return int64(0), nil
	}

	// When the passed height is too high or zero, just return 0 now
	// since we can't reasonably calculate the number of network hashes
	// per second from invalid values.  When it'server negative, use the current
	// best block height.
	best := server.chainProvider.BlockChain().BestSnapshot()
	endHeight := int32(-1)
	if c.Height != nil {
		endHeight = int32(*c.Height)
	}
	if endHeight > best.Height || endHeight == 0 {
		return int64(0), nil
	}
	if endHeight < 0 {
		endHeight = best.Height
	}

	// Calculate the number of blocks per retarget interval based on the
	// BlockChain parameters.
	blocksPerRetarget := int32(server.chainProvider.ChainParams.TargetTimespan /
		server.chainProvider.ChainParams.TargetTimePerBlock)

	// Calculate the starting block height based on the passed number of
	// blocks.  When the passed value is negative, use the last block the
	// difficulty changed as the starting height.  Also make sure the
	// starting height is not before the beginning of the BlockChain.
	numBlocks := int32(120)
	if c.Blocks != nil {
		numBlocks = int32(*c.Blocks)
	}
	var startHeight int32
	if numBlocks <= 0 {
		startHeight = endHeight - ((endHeight % blocksPerRetarget) + 1)
	} else {
		startHeight = endHeight - numBlocks
	}
	if startHeight < 0 {
		startHeight = 0
	}

	// Find the min and max block timestamps as well as calculate the total
	// amount of work that happened between the start and end blocks.
	var minTimestamp, maxTimestamp time.Time
	totalWork := big.NewInt(0)
	for curHeight := startHeight; curHeight <= endHeight; curHeight++ {
		hash, err := server.chainProvider.BlockChain().BlockHashByHeight(curHeight)
		if err != nil {
			context := "Failed to fetch block hash"
			return int64(0), server.InternalRPCError(err.Error(), context)
		}

		// Fetch the header from BlockChain.
		header, err := server.chainProvider.BlockChain().HeaderByHash(hash)
		if err != nil {
			context := "Failed to fetch block header"
			return int64(0), server.InternalRPCError(err.Error(), context)
		}

		if curHeight == startHeight {
			minTimestamp = header.Timestamp()
			maxTimestamp = minTimestamp
		} else {
			totalWork.Add(totalWork, pow.CalcWork(header.Bits()))

			if minTimestamp.After(header.Timestamp()) {
				minTimestamp = header.Timestamp()
			}
			if maxTimestamp.Before(header.Timestamp()) {
				maxTimestamp = header.Timestamp()
			}
		}
	}

	// Calculate the difference in seconds between the min and max block
	// timestamps and avoid division by zero in the case where there is no
	// time difference.
	timeDiff := int64(maxTimestamp.Sub(minTimestamp) / time.Second)
	if timeDiff == 0 {
		return int64(0), nil
	}

	hashesPerSec := new(big.Int).Div(totalWork, big.NewInt(timeDiff))
	return hashesPerSec.Int64(), nil
}

func (server *CommonChainRPC) handleGetBlockStats(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// c := cmd.(*jaxjson.GetBlockStatsCmd)
	res := jaxjson.GetBlockStatsResult{}
	return res, nil
}

func (server *CommonChainRPC) handleGetChaintxStats(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	_ = cmd.(*jaxjson.GetChainStatsCmd)
	res := jaxjson.GetChainStatsResult{}
	return res, nil
}

func (server *CommonChainRPC) handleGetnetworkinfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return struct {
		Subversion string `json:"subversion"`
	}{
		Subversion: "/Satoshi:0.18.0/",
	}, nil
}

// handleGetDifficulty implements the getdifficulty command.
func (server *CommonChainRPC) handleGetDifficulty(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	best := server.chainProvider.BlockChain().BestSnapshot()
	return server.GetDifficultyRatio(best.Bits, server.chainProvider.ChainParams)
}
