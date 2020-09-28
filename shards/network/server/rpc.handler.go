package server

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"time"

	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/btcec"
	"gitlab.com/jaxnet/core/shard.core.git/btcjson"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/database"
	"gitlab.com/jaxnet/core/shard.core.git/mempool"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chaincore"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/encoder"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
	"gitlab.com/jaxnet/core/shard.core.git/shards/types"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
	"go.uber.org/zap"
)

type ChainRPC struct {
	ToolsXt
	logger network.ILogger

	chainProvider *ChainProvider
	handlers      map[string]commandHandler
	gbtWorkState  *gbtWorkState
	helpCacher    *helpCacher
}

func NewChainRPC(chainProvider *ChainProvider, logger *zap.Logger) *ChainRPC {
	rpc := &ChainRPC{
		logger:        network.LogAdapter(logger),
		chainProvider: chainProvider,
		handlers:      nil,
		gbtWorkState:  nil,
		helpCacher:    nil,
	}
	rpc.init()

	rpc.gbtWorkState = newGbtWorkState(chainProvider.TimeSource)
	rpc.helpCacher = newHelpCacher(rpc)
	return rpc
}

// CommandsMux checks that a parsed command is a standard Bitcoin JSON-RPC
// command and runs the appropriate handler to reply to the command.  Any
// commands which are not recognized or not implemented will return an error
// suitable for use in replies.
func (server *ChainRPC) CommandsMux(cmd *parsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
	handler, ok := server.handlers[cmd.method]
	if ok {
		goto handled
	}

	_, ok = rpcAskWallet[cmd.method]
	if ok {
		handler = server.handleAskWallet
		goto handled
	}

	_, ok = rpcUnimplemented[cmd.method]
	if ok {
		handler = server.handleUnimplemented
		goto handled
	}
	return nil, btcjson.ErrRPCMethodNotFound

handled:

	return handler(cmd.cmd, closeChan)
}

func (server *ChainRPC) init() {
	server.handlers = map[string]commandHandler{
		"addnode":               server.handleAddNode,
		"createrawtransaction":  server.handleCreateRawTransaction,
		"debuglevel":            server.handleDebugLevel,
		"decoderawtransaction":  server.handleDecodeRawTransaction,
		"decodescript":          server.handleDecodeScript,
		"estimatesmartfee":      server.handleEstimateSmartFee,
		"getblockstats":         server.handleGetBlockStats,
		"getchaintxstats":       server.handleGetChaintxStats,
		"estimatefee":           server.handleEstimateFee,
		"generate":              server.handleGenerate,
		"getaddednodeinfo":      server.handleGetAddedNodeInfo,
		"getbestblock":          server.handleGetBestBlock,
		"getbestblockhash":      server.handleGetBestBlockHash,
		"getblock":              server.handleGetBlock,
		"getblockchaininfo":     server.handleGetBlockChainInfo,
		"getblockcount":         server.handleGetBlockCount,
		"getblockhash":          server.handleGetBlockHash,
		"getblockheader":        server.handleGetBlockHeader,
		"getblocktemplate":      server.handleGetBlockTemplate,
		"getcfilter":            server.handleGetCFilter,
		"getcfilterheader":      server.handleGetCFilterHeader,
		"getconnectioncount":    server.handleGetConnectionCount,
		"getcurrentnet":         server.handleGetCurrentNet,
		"getdifficulty":         server.handleGetDifficulty,
		"getheaders":            server.handleGetHeaders,
		"getinfo":               server.handleGetInfo,
		"getmempoolinfo":        server.handleGetMempoolInfo,
		"getmininginfo":         server.handleGetMiningInfo,
		"getnettotals":          server.handleGetNetTotals,
		"getnetworkhashps":      server.handleGetNetworkHashPS,
		"getpeerinfo":           server.handleGetPeerInfo,
		"getrawmempool":         server.handleGetRawMempool,
		"getrawtransaction":     server.handleGetRawTransaction,
		"gettxout":              server.handleGetTxOut,
		"help":                  server.handleHelp,
		"chainProvider":         server.handleNode,
		"ping":                  server.handlePing,
		"searchrawtransactions": server.handleSearchRawTransactions,
		"sendrawtransaction":    server.handleSendRawTransaction,
		// "setgenerate":           server.handleSetGenerate,
		"stop":            server.handleStop,
		"submitblock":     server.handleSubmitBlock,
		"uptime":          server.handleUptime,
		"validateaddress": server.handleValidateAddress,
		"verifychain":     server.handleVerifyChain,
		"verifymessage":   server.handleVerifyMessage,
		"version":         server.handleVersion,
		"getnetworkinfo":  server.handleGetnetworkinfo,

		"manageshards": server.handleManageShards,
		"listshards":   server.handleListShards,
	}
}

// internalRPCError is a convenience function to convert an internal error to
// an RPC error with the appropriate code set.  It also logs the error to the
// RPC server subsystem since internal errors really should not occur.  The
// context parameter is only used in the log message and may be empty if it's
// not needed.
func (server *ChainRPC) internalRPCError(errStr, context string) *btcjson.RPCError {
	logStr := errStr
	if context != "" {
		logStr = context + ": " + errStr
	}
	server.logger.Error(logStr)
	return btcjson.NewRPCError(btcjson.ErrRPCInternal.Code, errStr)
}

// Callback for notifications from blockchain.  It notifies clients that are
// long polling for changes or subscribed to websockets notifications.
func (server *ChainRPC) handleBlockchainNotification(notification *blockchain.Notification) {
	switch notification.Type {
	case blockchain.NTBlockAccepted:
		block, ok := notification.Data.(*btcutil.Block)
		if !ok {
			server.logger.Warnf("Chain accepted notification is not a block.")
			break
		}

		// Allow any clients performing long polling via the
		// getblocktemplate RPC to be notified when the new block causes
		// their old block template to become stale.
		server.gbtWorkState.NotifyBlockConnected(block.Hash())

	case blockchain.NTBlockConnected:
		// block, ok := notification.Data.(*btcutil.Block)
		// if !ok {
		//	server.logger.Warnf("Chain connected notification is not a block.")
		//	break
		// }

		// Notify registered websocket clients of incoming block.
		// server.ntfnMgr.NotifyBlockConnected(block)

	case blockchain.NTBlockDisconnected:
		// block, ok := notification.Data.(*btcutil.Block)
		// if !ok {
		//	server.logger.Warnf("Chain disconnected notification is not a block.")
		//	break
		// }

		// Notify registered websocket clients.
		// server.ntfnMgr.NotifyBlockDisconnected(block)
	}
}

// fetchInputTxos fetches the outpoints from all transactions referenced by the
// inputs to the passed transaction by checking the transaction mempool first
// then the transaction index for those already mined into blocks.
func (server *ChainRPC) fetchInputTxos(tx *wire.MsgTx) (map[wire.OutPoint]wire.TxOut, error) {
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
				errStr := fmt.Sprintf("unable to find output "+
					"%v referenced from transaction %s:%d",
					origin, tx.TxHash(), txInIndex)
				return nil, server.internalRPCError(errStr, "")
			}

			originOutputs[*origin] = *txOuts[origin.Index]
			continue
		}

		// Look up the location of the transaction.
		blockRegion, err := server.chainProvider.TxIndex.TxBlockRegion(&origin.Hash)
		if err != nil {
			context := "Failed to retrieve transaction location"
			return nil, server.internalRPCError(err.Error(), context)
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
			return nil, server.internalRPCError(err.Error(), context)
		}

		// Add the referenced output to the map.
		if origin.Index >= uint32(len(msgTx.TxOut)) {
			errStr := fmt.Sprintf("unable to find output %v "+
				"referenced from transaction %s:%d", origin,
				tx.TxHash(), txInIndex)
			return nil, server.internalRPCError(errStr, "")
		}
		originOutputs[*origin] = *msgTx.TxOut[origin.Index]
	}

	return originOutputs, nil
}

func (server *ChainRPC) verifyChain(level, depth int32) error {
	best := server.chainProvider.BlockChain.BestSnapshot()
	finishHeight := best.Height - depth
	if finishHeight < 0 {
		finishHeight = 0
	}
	server.logger.Infof("Verifying BlockChain for %d blocks at level %d",
		best.Height-finishHeight, level)

	for height := best.Height; height > finishHeight; height-- {
		// Level 0 just looks up the block.
		block, err := server.chainProvider.BlockChain.BlockByHeight(height)
		if err != nil {
			server.logger.Errorf("Verify is unable to fetch block at "+
				"height %d: %v", height, err)
			return err
		}

		// Level 1 does basic BlockChain sanity checks.
		if level > 0 {
			err := blockchain.CheckBlockSanity(block,
				server.chainProvider.ChainParams.PowLimit, server.chainProvider.TimeSource)
			if err != nil {
				server.logger.Errorf("Verify is unable to validate "+
					"block at hash %v height %d: %v",
					block.Hash(), height, err)
				return err
			}
		}
	}
	server.logger.Infof("Chain verify completed successfully")

	return nil
}

// handleGetBlock implements the getblock command.
func (server *ChainRPC) handleGetBlock(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetBlockCmd)

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
		return nil, server.internalRPCError(err.Error(), context)
	}

	// Get the block height from BlockChain.
	blockHeight, err := server.chainProvider.BlockChain.BlockHeightByHash(hash)
	if err != nil {
		context := "Failed to obtain block height"
		return nil, server.internalRPCError(err.Error(), context)
	}
	blk.SetHeight(blockHeight)
	best := server.chainProvider.BlockChain.BestSnapshot()

	// Get next block hash unless there are none.
	var nextHashString string
	if blockHeight < best.Height {
		nextHash, err := server.chainProvider.BlockChain.BlockHashByHeight(blockHeight + 1)
		if err != nil {
			context := "No next block"
			return nil, server.internalRPCError(err.Error(), context)
		}
		nextHashString = nextHash.String()
	}

	params := server.chainProvider.ChainParams
	blockHeader := blk.MsgBlock().Header
	diff, err := server.GetDifficultyRatio(blockHeader.Bits(), params)
	if err != nil {
		return nil, err
	}

	blockReply := btcjson.GetBlockVerboseResult{
		Hash:                c.Hash,
		Version:             int32(blockHeader.Version()),
		VersionHex:          fmt.Sprintf("%08x", blockHeader.Version()),
		MerkleRoot:          blockHeader.MerkleRoot().String(),
		PreviousHash:        blockHeader.PrevBlock().String(),
		MerkleMountainRange: blockHeader.MergeMiningRoot().String(),
		Nonce:               blockHeader.Nonce(),
		Time:                blockHeader.Timestamp().Unix(),
		Confirmations:       int64(1 + best.Height - blockHeight),
		Height:              int64(blockHeight),
		Size:                int32(len(blkBytes)),
		StrippedSize:        int32(blk.MsgBlock().SerializeSizeStripped()),
		Weight:              int32(blockchain.GetBlockWeight(blk)),
		Bits:                strconv.FormatInt(int64(blockHeader.Bits()), 16),
		Difficulty:          diff,
		NextHash:            nextHashString,
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

// handleUnimplemented is the handler for commands that should ultimately be
// supported but are not yet implemented.
func (server *ChainRPC) handleUnimplemented(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return nil, ErrRPCUnimplemented
}

// handleAskWallet is the handler for commands that are recognized as valid, but
// are unable to answer correctly since it involves wallet state.
// These commands will be implemented in btcwallet.
func (server *ChainRPC) handleAskWallet(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return nil, ErrRPCNoWallet
}

// handleAddNode handles addnode commands.
func (server *ChainRPC) handleAddNode(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.AddNodeCmd)

	addr := normalizeAddress(c.Addr, server.chainProvider.ChainParams.DefaultPort)
	var err error
	switch c.SubCmd {
	case "add":
		err = server.chainProvider.ConnMgr.Connect(addr, true)
	case "remove":
		err = server.chainProvider.ConnMgr.RemoveByAddr(addr)
	case "onetry":
		err = server.chainProvider.ConnMgr.Connect(addr, false)
	default:
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: "invalid subcommand for addnode",
		}
	}

	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: err.Error(),
		}
	}

	// no data returned unless an error.
	return nil, nil
}

// handleNode handles chainProvider commands.
func (server *ChainRPC) handleNode(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.NodeCmd)

	var addr string
	var nodeID uint64
	var errN, err error
	params := server.chainProvider.ChainParams
	switch c.SubCmd {
	case "disconnect":
		// If we have a valid uint disconnect by chainProvider id. Otherwise,
		// attempt to disconnect by address, returning an error if a
		// valid IP address is not supplied.
		if nodeID, errN = strconv.ParseUint(c.Target, 10, 32); errN == nil {
			err = server.chainProvider.ConnMgr.DisconnectByID(int32(nodeID))
		} else {
			if _, _, errP := net.SplitHostPort(c.Target); errP == nil || net.ParseIP(c.Target) != nil {
				addr = normalizeAddress(c.Target, params.DefaultPort)
				err = server.chainProvider.ConnMgr.DisconnectByAddr(addr)
			} else {
				return nil, &btcjson.RPCError{
					Code:    btcjson.ErrRPCInvalidParameter,
					Message: "invalid address or chainProvider ID",
				}
			}
		}
		if err != nil && peerExists(server.chainProvider.ConnMgr, addr, int32(nodeID)) {

			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCMisc,
				Message: "can't disconnect a permanent Server, use remove",
			}
		}

	case "remove":
		// If we have a valid uint disconnect by chainProvider id. Otherwise,
		// attempt to disconnect by address, returning an error if a
		// valid IP address is not supplied.
		if nodeID, errN = strconv.ParseUint(c.Target, 10, 32); errN == nil {
			err = server.chainProvider.ConnMgr.RemoveByID(int32(nodeID))
		} else {
			if _, _, errP := net.SplitHostPort(c.Target); errP == nil || net.ParseIP(c.Target) != nil {
				addr = normalizeAddress(c.Target, params.DefaultPort)
				err = server.chainProvider.ConnMgr.RemoveByAddr(addr)
			} else {
				return nil, &btcjson.RPCError{
					Code:    btcjson.ErrRPCInvalidParameter,
					Message: "invalid address or chainProvider ID",
				}
			}
		}
		if err != nil && peerExists(server.chainProvider.ConnMgr, addr, int32(nodeID)) {
			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCMisc,
				Message: "can't remove a temporary Server, use disconnect",
			}
		}

	case "connect":
		addr = normalizeAddress(c.Target, params.DefaultPort)

		// Default to temporary connections.
		subCmd := "temp"
		if c.ConnectSubCmd != nil {
			subCmd = *c.ConnectSubCmd
		}

		switch subCmd {
		case "perm", "temp":
			err = server.chainProvider.ConnMgr.Connect(addr, subCmd == "perm")
		default:
			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCInvalidParameter,
				Message: "invalid subcommand for chainProvider connect",
			}
		}
	default:
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: "invalid subcommand for chainProvider",
		}
	}

	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: err.Error(),
		}
	}

	// no data returned unless an error.
	return nil, nil
}

// handleCreateRawTransaction handles createrawtransaction commands.
func (server *ChainRPC) handleCreateRawTransaction(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.CreateRawTransactionCmd)

	// Validate the locktime, if given.
	if c.LockTime != nil &&
		(*c.LockTime < 0 || *c.LockTime > int64(wire.MaxTxInSequenceNum)) {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: "Locktime out of range",
		}
	}

	// Add all transaction inputs to a new transaction after performing
	// some validity checks.
	mtx := wire.NewMsgTx(wire.TxVersion)
	for _, input := range c.Inputs {
		txHash, err := chainhash.NewHashFromStr(input.Txid)
		if err != nil {
			return nil, rpcDecodeHexError(input.Txid)
		}

		prevOut := wire.NewOutPoint(txHash, input.Vout)
		txIn := wire.NewTxIn(prevOut, []byte{}, nil)
		if c.LockTime != nil && *c.LockTime != 0 {
			txIn.Sequence = wire.MaxTxInSequenceNum - 1
		}
		mtx.AddTxIn(txIn)
	}

	// Add all transaction outputs to the transaction after performing
	// some validity checks.
	params := server.chainProvider.ChainParams
	for encodedAddr, amount := range c.Amounts {
		// Ensure amount is in the valid range for monetary amounts.
		if amount <= 0 || amount > btcutil.MaxSatoshi {
			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCType,
				Message: "Invalid amount",
			}
		}

		// Decode the provided address.
		addr, err := btcutil.DecodeAddress(encodedAddr, params)
		if err != nil {
			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCInvalidAddressOrKey,
				Message: "Invalid address or key: " + err.Error(),
			}
		}

		// Ensure the address is one of the supported types and that
		// the network encoded with the address matches the network the
		// Server is currently on.
		switch addr.(type) {
		case *btcutil.AddressPubKeyHash:
		case *btcutil.AddressScriptHash:
		default:
			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCInvalidAddressOrKey,
				Message: "Invalid address or key",
			}
		}
		if !addr.IsForNet(params) {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCInvalidAddressOrKey,
				Message: "Invalid address: " + encodedAddr +
					" is for the wrong network",
			}
		}

		// Create a new script which pays to the provided address.
		pkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			context := "Failed to generate pay-to-address script"
			return nil, server.internalRPCError(err.Error(), context)
		}

		// Convert the amount to satoshi.
		satoshi, err := btcutil.NewAmount(amount)
		if err != nil {
			context := "Failed to convert amount"
			return nil, server.internalRPCError(err.Error(), context)
		}

		txOut := wire.NewTxOut(int64(satoshi), pkScript)
		mtx.AddTxOut(txOut)
	}

	// Set the Locktime, if given.
	if c.LockTime != nil {
		mtx.LockTime = uint32(*c.LockTime)
	}

	// Return the serialized and hex-encoded transaction.  Note that this
	// is intentionally not directly returning because the first return
	// value is a string and it would result in returning an empty string to
	// the client instead of nothing (nil) in the case of an error.
	mtxHex, err := server.MessageToHex(mtx)
	if err != nil {
		return nil, err
	}
	return mtxHex, nil
}

// handleDebugLevel handles debuglevel commands.
func (server *ChainRPC) handleDebugLevel(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// c := cmd.(*btcjson.DebugLevelCmd)

	// Special show command to list supported subsystems.
	// if c.LevelSpec == "show" {
	//	return fmt.Sprintf("Supported subsystems %v",
	//		supportedSubsystems()), nil
	// }

	// err := parseAndSetDebugLevels(c.LevelSpec)
	// if err != nil {
	//	return nil, &btcjson.RPCError{
	//		Code:    btcjson.ErrRPCInvalidParams.Code,
	//		Message: err.Error(),
	//	}
	// }

	return "Done.", nil
}

// handleDecodeRawTransaction handles decoderawtransaction commands.
func (server *ChainRPC) handleDecodeRawTransaction(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.DecodeRawTransactionCmd)

	// Deserialize the transaction.
	hexStr := c.HexTx
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	serializedTx, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, rpcDecodeHexError(hexStr)
	}
	var mtx wire.MsgTx
	err = mtx.Deserialize(bytes.NewReader(serializedTx))
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCDeserialization,
			Message: "TX decode failed: " + err.Error(),
		}
	}

	// Create and return the result.
	txReply := btcjson.TxRawDecodeResult{
		Txid:     mtx.TxHash().String(),
		Version:  mtx.Version,
		Locktime: mtx.LockTime,
		Vin:      server.CreateVinList(&mtx),
		Vout:     server.CreateVoutList(&mtx, server.chainProvider.ChainParams, nil),
	}
	return txReply, nil
}

// handleDecodeScript handles decodescript commands.
func (server *ChainRPC) handleDecodeScript(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
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
		return nil, server.internalRPCError(err.Error(), context)
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

func (server *ChainRPC) handleGetBlockStats(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// c := cmd.(*btcjson.GetBlockStatsCmd)
	res := btcjson.GetBlockStatsResult{}
	return res, nil
}

func (server *ChainRPC) handleGetChaintxStats(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	_ = cmd.(*btcjson.GetChainStatsCmd)
	res := btcjson.GetChainStatsResult{}
	return res, nil
}

// estimatesmartfee
func (server *ChainRPC) handleEstimateSmartFee(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {

	// c := cmd.(*btcjson.EstimateSmartFeeResult)
	rate := 1.0
	res := btcjson.EstimateSmartFeeResult{
		FeeRate: &rate,
	}
	// if server.cfg.FeeEstimator == nil {
	//	return nil, errors.New("Fee estimation disabled")
	// }

	// if c.NumBlocks <= 0 {
	//	return -1.0, errors.New("Parameter NumBlocks must be positive")
	// }
	//
	// feeRate, err := server.cfg.FeeEstimator.EstimateFee(uint32(c.NumBlocks))

	// if err != nil {
	//	return -1.0, err
	// }

	// Convert to satoshis per kb.
	return res, nil
}

// handleEstimateFee handles estimatefee commands.
func (server *ChainRPC) handleEstimateFee(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.EstimateFeeCmd)

	if server.chainProvider.FeeEstimator == nil {
		return nil, errors.New("Fee estimation disabled")
	}

	if c.NumBlocks <= 0 {
		return -1.0, errors.New("Parameter NumBlocks must be positive")
	}

	feeRate, err := server.chainProvider.FeeEstimator.EstimateFee(uint32(c.NumBlocks))

	if err != nil {
		return -1.0, err
	}

	// Convert to satoshis per kb.
	return float64(feeRate), nil
}

// handleGenerate handles generate commands.
func (server *ChainRPC) handleGenerate(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Respond with an error if there are no addresses to pay the
	// created blocks to.
	if len(server.chainProvider.MiningAddrs) == 0 {
		return nil, &btcjson.RPCError{
			Code: btcjson.ErrRPCInternal.Code,
			Message: "No payment addresses specified " +
				"via --miningaddr",
		}
	}

	// Respond with an error if there's virtually 0 chance of mining a block
	// with the CPU.
	// if !s.cfg.ChainParams.GenerateSupported {
	// 	return nil, &btcjson.RPCError{
	// 		Code: btcjson.ErrRPCDifficulty,
	// 		Message: fmt.Sprintf("No support for `generate` on "+
	// 			"the current network, %s, as it's unlikely to "+
	// 			"be possible to mine a block with the CPU.",
	// 			s.cfg.ChainParams.Net),
	// 	}
	// }

	c := cmd.(*btcjson.GenerateCmd)

	// Respond with an error if the client is requesting 0 blocks to be generated.
	if c.NumBlocks == 0 {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInternal.Code,
			Message: "Please request a nonzero number of blocks to generate.",
		}
	}

	// Create a reply
	reply := make([]string, c.NumBlocks)

	blockHashes, err := server.chainProvider.CPUMiner.GenerateNBlocks(c.NumBlocks)
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInternal.Code,
			Message: err.Error(),
		}
	}

	// Mine the correct number of blocks, assigning the hex representation of the
	// hash of each one to its place in the reply.
	for i, hash := range blockHashes {
		reply[i] = hash.String()
	}

	return reply, nil
}

// handleGetAddedNodeInfo handles getaddednodeinfo commands.
func (server *ChainRPC) handleGetAddedNodeInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetAddedNodeInfoCmd)

	// Retrieve a list of persistent (added) peers from the Server and
	// filter the list of peers per the specified address (if any).
	peers := server.chainProvider.ConnMgr.PersistentPeers()
	if c.Node != nil {
		node := *c.Node
		found := false
		for i, peer := range peers {
			if peer.ToPeer().Addr() == node {
				peers = peers[i : i+1]
				found = true
			}
		}
		if !found {
			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCClientNodeNotAdded,
				Message: "Node has not been added",
			}
		}
	}

	// Without the dns flag, the result is just a slice of the addresses as
	// strings.
	if !c.DNS {
		results := make([]string, 0, len(peers))
		for _, peer := range peers {
			results = append(results, peer.ToPeer().Addr())
		}
		return results, nil
	}

	// With the dns flag, the result is an array of JSON objects which
	// include the result of DNS lookups for each Server.
	results := make([]*btcjson.GetAddedNodeInfoResult, 0, len(peers))
	for _, rpcPeer := range peers {
		// Set the "address" of the Server which could be an ip address
		// or a domain name.
		peer := rpcPeer.ToPeer()
		var result btcjson.GetAddedNodeInfoResult
		result.AddedNode = peer.Addr()
		result.Connected = btcjson.Bool(peer.Connected())

		// Split the address into host and port portions so we can do
		// a DNS lookup against the host.  When no port is specified in
		// the address, just use the address as the host.
		host, _, err := net.SplitHostPort(peer.Addr())
		if err != nil {
			host = peer.Addr()
		}

		var ipList []string
		switch {
		case net.ParseIP(host) != nil, strings.HasSuffix(host, ".onion"):
			ipList = make([]string, 1)
			ipList[0] = host
		default:
			// Do a DNS lookup for the address.  If the lookup fails, just
			// use the host.
			// ips, err := btcdLookup(host)
			// if err != nil {
			//	ipList = make([]string, 1)
			//	ipList[0] = host
			//	break
			// }
			// ipList = make([]string, 0, len(ips))
			// for _, ip := range ips {
			//	ipList = append(ipList, ip.String())
			// }
		}

		// Add the addresses and connection info to the result.
		addrs := make([]btcjson.GetAddedNodeInfoResultAddr, 0, len(ipList))
		for _, ip := range ipList {
			var addr btcjson.GetAddedNodeInfoResultAddr
			addr.Address = ip
			addr.Connected = "false"
			if ip == host && peer.Connected() {
				addr.Connected = directionString(peer.Inbound())
			}
			addrs = append(addrs, addr)
		}
		result.Addresses = &addrs
		results = append(results, &result)
	}
	return results, nil
}

// handleGetBestBlock implements the getbestblock command.
func (server *ChainRPC) handleGetBestBlock(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// All other "get block" commands give either the height, the
	// hash, or both but require the block SHA.  This gets both for
	// the best block.
	best := server.chainProvider.BlockChain.BestSnapshot()
	result := &btcjson.GetBestBlockResult{
		Hash:   best.Hash.String(),
		Height: best.Height,
	}
	return result, nil
}

// handleGetBestBlockHash implements the getbestblockhash command.
func (server *ChainRPC) handleGetBestBlockHash(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	best := server.chainProvider.BlockChain.BestSnapshot()
	return best.Hash.String(), nil
}

// handleGetBlockChainInfo implements the getblockchaininfo command.
func (server *ChainRPC) handleGetBlockChainInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Obtain a snapshot of the current best known blockchain state. We'll
	// populate the response to this call primarily from this snapshot.
	params := server.chainProvider.ChainParams
	blockChain := server.chainProvider.BlockChain
	chainSnapshot := blockChain.BestSnapshot()
	diff, err := server.GetDifficultyRatio(chainSnapshot.Bits, params)
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
		case chaincore.DeploymentTestDummy:
			forkName = "dummy"

		case chaincore.DeploymentCSV:
			forkName = "csv"

		case chaincore.DeploymentSegwit:
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
			return nil, server.internalRPCError(err.Error(), context)
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

	return chainInfo, nil
}

// handleGetBlockCount implements the getblockcount command.
func (server *ChainRPC) handleGetBlockCount(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	best := server.chainProvider.BlockChain.BestSnapshot()
	return int64(best.Height), nil
}

// handleGetBlockHash implements the getblockhash command.
func (server *ChainRPC) handleGetBlockHash(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetBlockHashCmd)
	hash, err := server.chainProvider.BlockChain.BlockHashByHeight(int32(c.Index))
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCOutOfRange,
			Message: "Block number out of range",
		}
	}

	return hash.String(), nil
}

// handleGetBlockHeader implements the getblockheader command.
func (server *ChainRPC) handleGetBlockHeader(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetBlockHeaderCmd)

	// Fetch the header from BlockChain.
	hash, err := chainhash.NewHashFromStr(c.Hash)
	if err != nil {
		return nil, rpcDecodeHexError(c.Hash)
	}
	blockHeader, err := server.chainProvider.BlockChain.HeaderByHash(hash)
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
			return nil, server.internalRPCError(err.Error(), context)
		}
		return hex.EncodeToString(headerBuf.Bytes()), nil
	}

	// The verbose flag is set, so generate the JSON object and return it.

	// Get the block height from BlockChain.
	blockHeight, err := server.chainProvider.BlockChain.BlockHeightByHash(hash)
	if err != nil {
		context := "Failed to obtain block height"
		return nil, server.internalRPCError(err.Error(), context)
	}
	best := server.chainProvider.BlockChain.BestSnapshot()

	// Get next block hash unless there are none.
	var nextHashString string
	if blockHeight < best.Height {
		nextHash, err := server.chainProvider.BlockChain.BlockHashByHeight(blockHeight + 1)
		if err != nil {
			context := "No next block"
			return nil, server.internalRPCError(err.Error(), context)
		}
		nextHashString = nextHash.String()
	}

	params := server.chainProvider.ChainParams
	diff, err := server.GetDifficultyRatio(blockHeader.Bits(), params)
	if err != nil {
		return nil, err
	}
	blockHeaderReply := btcjson.GetBlockHeaderVerboseResult{
		Hash:                c.Hash,
		Confirmations:       int64(1 + best.Height - blockHeight),
		Height:              blockHeight,
		Version:             int32(blockHeader.Version()),
		VersionHex:          fmt.Sprintf("%08x", blockHeader.Version()),
		MerkleRoot:          blockHeader.MerkleRoot().String(),
		MerkleMountainRange: blockHeader.MergeMiningRoot().String(),
		NextHash:            nextHashString,
		PreviousHash:        blockHeader.PrevBlock().String(),
		Nonce:               uint64(blockHeader.Nonce()),
		Time:                blockHeader.Timestamp().Unix(),
		Bits:                strconv.FormatInt(int64(blockHeader.Bits()), 16),
		Difficulty:          diff,
	}
	return blockHeaderReply, nil
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
func (server *ChainRPC) handleGetBlockTemplateLongPoll(longPollID string, useCoinbaseValue bool, closeChan <-chan struct{}) (interface{}, error) {
	state := server.gbtWorkState
	state.Lock()
	// The state unlock is intentionally not deferred here since it needs to
	// be manually unlocked before waiting for a notification about block
	// template changes.

	if err := state.updateBlockTemplate(server, useCoinbaseValue); err != nil {
		state.Unlock()
		return nil, err
	}

	// Just return the current block template if the long poll ID provided by
	// the caller is invalid.
	prevHash, lastGenerated, err := server.DecodeTemplateID(longPollID)
	if err != nil {
		result, err := state.blockTemplateResult(useCoinbaseValue, nil)
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
	prevTemplateHash := state.template.Block.Header.PrevBlock()
	if !prevHash.IsEqual(&prevTemplateHash) ||
		lastGenerated != state.lastGenerated.Unix() {

		// Include whether or not it is valid to submit work against the
		// old block template depending on whether or not a solution has
		// already been found and added to the block BlockChain.
		submitOld := prevHash.IsEqual(&prevTemplateHash)
		result, err := state.blockTemplateResult(useCoinbaseValue, &submitOld)
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
	longPollChan := state.templateUpdateChan(prevHash, lastGenerated)
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

	if err := state.updateBlockTemplate(server, useCoinbaseValue); err != nil {
		return nil, err
	}

	// Include whether or not it is valid to submit work against the old
	// block template depending on whether or not a solution has already
	// been found and added to the block BlockChain.
	h := state.template.Block.Header.PrevBlock()
	submitOld := prevHash.IsEqual(&h)
	result, err := state.blockTemplateResult(useCoinbaseValue, &submitOld)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// handleGetBlockTemplateRequest is a helper for handleGetBlockTemplate which
// deals with generating and returning block templates to the caller.  It
// handles both long poll requests as specified by BIP 0022 as well as regular
// requests.  In addition, it detects the capabilities reported by the caller
// in regards to whether or not it supports creating its own coinbase (the
// coinbasetxn and coinbasevalue capabilities) and modifies the returned block
// template accordingly.
func (server *ChainRPC) handleGetBlockTemplateRequest(request *btcjson.TemplateRequest, closeChan <-chan struct{}) (interface{}, error) {
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

	// // Return an error if there are no peers connected since there is no
	// // way to relay a found block or receive transactions to work on.
	// // However, allow this state when running in the regression test or
	// // simulation test mode.
	// if !(cfg.RegressionTest || cfg.SimNet) &&
	//	server.cfg.ConnMgr.ConnectedCount() == 0 {
	//
	//	return nil, &btcjson.RPCError{
	//		Code:    btcjson.ErrRPCClientNotConnected,
	//		Message: "Bitcoin is not connected",
	//	}
	// }

	// No point in generating or accepting work before the BlockChain is synced.
	currentHeight := server.chainProvider.BlockChain.BestSnapshot().Height
	if currentHeight != 0 && !server.chainProvider.SyncMgr.IsCurrent() {
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
	if err := state.updateBlockTemplate(server, useCoinbaseValue); err != nil {
		return nil, err
	}
	return state.blockTemplateResult(useCoinbaseValue, nil)
}

// handleGetBlockTemplateProposal is a helper for handleGetBlockTemplate which
// deals with block proposals.
//
// See https://en.bitcoin.it/wiki/BIP_0023 for more details.
func (server *ChainRPC) handleGetBlockTemplateProposal(request *btcjson.TemplateRequest) (interface{}, error) {
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
	var msgBlock wire.MsgBlock
	if err := msgBlock.Deserialize(bytes.NewReader(dataBytes)); err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCDeserialization,
			Message: "Block decode failed: " + err.Error(),
		}
	}
	block := btcutil.NewBlock(&msgBlock)

	// Ensure the block is building from the expected previous block.
	expectedPrevHash := server.chainProvider.BlockChain.BestSnapshot().Hash
	prevHash := block.MsgBlock().Header.PrevBlock()
	if !expectedPrevHash.IsEqual(&prevHash) {
		return "bad-prevblk", nil
	}

	if err := server.chainProvider.BlockChain.CheckConnectBlockTemplate(block); err != nil {
		if _, ok := err.(blockchain.RuleError); !ok {
			errStr := fmt.Sprintf("Failed to process block proposal: %v", err)
			server.logger.Error(errStr)
			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCVerify,
				Message: errStr,
			}
		}

		server.logger.Infof("Rejected block proposal. %s", err.Error())
		return server.ChainErrToGBTErrString(err), nil
	}

	return nil, nil
}

// handleGetBlockTemplate implements the getblocktemplate command.
//
// See https://en.bitcoin.it/wiki/BIP_0022 and
// https://en.bitcoin.it/wiki/BIP_0023 for more details.
func (server *ChainRPC) handleGetBlockTemplate(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetBlockTemplateCmd)
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

// handleGetCFilter implements the getcfilter command.
func (server *ChainRPC) handleGetCFilter(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
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
		server.logger.Debugf("Could not find committed filter for %v %v", hash, err)
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}

	server.logger.Debugf("Found committed filter for %s", hash.String())
	return hex.EncodeToString(filterBytes), nil
}

// handleGetCFilterHeader implements the getcfilterheader command.
func (server *ChainRPC) handleGetCFilterHeader(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
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
		server.logger.Debugf("Found header of committed filter for %s", hash.String())
	} else {
		server.logger.Debugf("Could not find header of committed filter for %v %v", hash, err)
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCBlockNotFound,
			Message: "Block not found",
		}
	}

	hash.SetBytes(headerBytes)
	return hash.String(), nil
}

// handleGetConnectionCount implements the getconnectioncount command.
func (server *ChainRPC) handleGetConnectionCount(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return server.chainProvider.ConnMgr.ConnectedCount(), nil
}

// handleGetCurrentNet implements the getcurrentnet command.
func (server *ChainRPC) handleGetCurrentNet(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return server.chainProvider.ChainParams.Net, nil
}

// handleGetDifficulty implements the getdifficulty command.
func (server *ChainRPC) handleGetDifficulty(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	best := server.chainProvider.BlockChain.BestSnapshot()
	return server.GetDifficultyRatio(best.Bits, server.chainProvider.ChainParams)
}

// handleGetHeaders implements the getheaders command.
//
// NOTE: This is a btcsuite extension originally ported from
// github.com/decred/dcrd.
func (server *ChainRPC) handleGetHeaders(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetHeadersCmd)

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
	headers := server.chainProvider.SyncMgr.LocateHeaders(blockLocators, &hashStop)

	// Return the serialized block headers as hex-encoded strings.
	hexBlockHeaders := make([]string, len(headers))
	var buf bytes.Buffer
	for i, h := range headers {
		err := h.Write(&buf)
		if err != nil {
			return nil, server.internalRPCError(err.Error(),
				"Failed to serialize block header")
		}
		hexBlockHeaders[i] = hex.EncodeToString(buf.Bytes())
		buf.Reset()
	}
	return hexBlockHeaders, nil
}

// handleGetInfo implements the getinfo command. We only return the fields
// that are not related to wallet functionality.
func (server *ChainRPC) handleGetInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	best := server.chainProvider.BlockChain.BestSnapshot()
	ret := &btcjson.InfoChainResult{
		// Version:         int32(1000000*appMajor + 10000*appMinor + 100*appPatch),
		ProtocolVersion: int32(maxProtocolVersion),
		Blocks:          best.Height,
		TimeOffset:      int64(server.chainProvider.TimeSource.Offset().Seconds()),
		Connections:     server.chainProvider.ConnMgr.ConnectedCount(),
		// Proxy:           cfg.Proxy,
		// Difficulty:      GetDifficultyRatio(best.Bits, server.cfg.ChainParams),
		// TestNet:         cfg.TestNet3,
		// RelayFee:        cfg.MinRelayTxFeeValues.ToBTC(),
	}

	return ret, nil
}

// handleGetMempoolInfo implements the getmempoolinfo command.
func (server *ChainRPC) handleGetMempoolInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	mempoolTxns := server.chainProvider.TxMemPool.TxDescs()

	var numBytes int64
	for _, txD := range mempoolTxns {
		numBytes += int64(txD.Tx.MsgTx().SerializeSize())
	}

	ret := &btcjson.GetMempoolInfoResult{
		Size:  int64(len(mempoolTxns)),
		Bytes: numBytes,
	}

	return ret, nil
}

// handleGetMiningInfo implements the getmininginfo command. We only return the
// fields that are not related to wallet functionality.
func (server *ChainRPC) handleGetMiningInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Create a default getnetworkhashps command to use defaults and make
	// use of the existing getnetworkhashps handler.
	gnhpsCmd := btcjson.NewGetNetworkHashPSCmd(nil, nil)
	networkHashesPerSecIface, err := server.handleGetNetworkHashPS(gnhpsCmd, closeChan)
	if err != nil {
		return nil, err
	}
	networkHashesPerSec, ok := networkHashesPerSecIface.(int64)
	if !ok {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInternal.Code,
			Message: "networkHashesPerSec is not an int64",
		}
	}

	best := server.chainProvider.BlockChain.BestSnapshot()
	diff, err := server.GetDifficultyRatio(best.Bits, server.chainProvider.ChainParams)
	if err != nil {
		return nil, err
	}
	result := btcjson.GetMiningInfoResult{
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

// handleGetNetTotals implements the getnettotals command.
func (server *ChainRPC) handleGetNetTotals(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	totalBytesRecv, totalBytesSent := server.chainProvider.ConnMgr.NetTotals()
	reply := &btcjson.GetNetTotalsResult{
		TotalBytesRecv: totalBytesRecv,
		TotalBytesSent: totalBytesSent,
		TimeMillis:     time.Now().UTC().UnixNano() / int64(time.Millisecond),
	}
	return reply, nil
}

// handleGetNetworkHashPS implements the getnetworkhashps command.
func (server *ChainRPC) handleGetNetworkHashPS(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Note: All valid error return paths should return an int64.
	// Literal zeros are inferred as int, and won't coerce to int64
	// because the return value is an interface{}.

	c := cmd.(*btcjson.GetNetworkHashPSCmd)

	// When the passed height is too high or zero, just return 0 now
	// since we can't reasonably calculate the number of network hashes
	// per second from invalid values.  When it'server negative, use the current
	// best block height.
	best := server.chainProvider.BlockChain.BestSnapshot()
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
	server.logger.Debugf("Calculating network hashes per second %v %v", startHeight, endHeight)

	// Find the min and max block timestamps as well as calculate the total
	// amount of work that happened between the start and end blocks.
	var minTimestamp, maxTimestamp time.Time
	totalWork := big.NewInt(0)
	for curHeight := startHeight; curHeight <= endHeight; curHeight++ {
		hash, err := server.chainProvider.BlockChain.BlockHashByHeight(curHeight)
		if err != nil {
			context := "Failed to fetch block hash"
			return nil, server.internalRPCError(err.Error(), context)
		}

		// Fetch the header from BlockChain.
		header, err := server.chainProvider.BlockChain.HeaderByHash(hash)
		if err != nil {
			context := "Failed to fetch block header"
			return nil, server.internalRPCError(err.Error(), context)
		}

		if curHeight == startHeight {
			minTimestamp = header.Timestamp()
			maxTimestamp = minTimestamp
		} else {
			totalWork.Add(totalWork, blockchain.CalcWork(header.Bits()))

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

// handleGetPeerInfo implements the getpeerinfo command.
func (server *ChainRPC) handleGetPeerInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	peers := server.chainProvider.ConnMgr.ConnectedPeers()
	syncPeerID := server.chainProvider.SyncMgr.SyncPeerID()
	infos := make([]*btcjson.GetPeerInfoResult, 0, len(peers))
	for _, p := range peers {
		statsSnap := p.ToPeer().StatsSnapshot()
		info := &btcjson.GetPeerInfoResult{
			ID:             statsSnap.ID,
			Addr:           statsSnap.Addr,
			AddrLocal:      p.ToPeer().LocalAddr().String(),
			Services:       fmt.Sprintf("%08d", uint64(statsSnap.Services)),
			RelayTxes:      !p.IsTxRelayDisabled(),
			LastSend:       statsSnap.LastSend.Unix(),
			LastRecv:       statsSnap.LastRecv.Unix(),
			BytesSent:      statsSnap.BytesSent,
			BytesRecv:      statsSnap.BytesRecv,
			ConnTime:       statsSnap.ConnTime.Unix(),
			PingTime:       float64(statsSnap.LastPingMicros),
			TimeOffset:     statsSnap.TimeOffset,
			Version:        statsSnap.Version,
			SubVer:         statsSnap.UserAgent,
			Inbound:        statsSnap.Inbound,
			StartingHeight: statsSnap.StartingHeight,
			CurrentHeight:  statsSnap.LastBlock,
			BanScore:       int32(p.BanScore()),
			FeeFilter:      p.FeeFilter(),
			SyncNode:       statsSnap.ID == syncPeerID,
		}
		if p.ToPeer().LastPingNonce() != 0 {
			wait := float64(time.Since(statsSnap.LastPingTime).Nanoseconds())
			// We actually want microseconds.
			info.PingWait = wait / 1000
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// handleGetRawMempool implements the getrawmempool command.
func (server *ChainRPC) handleGetRawMempool(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetRawMempoolCmd)
	mp := server.chainProvider.TxMemPool

	if c.Verbose != nil && *c.Verbose {
		return mp.RawMempoolVerbose(), nil
	}

	// The response is simply an array of the transaction hashes if the
	// verbose flag is not set.
	descs := mp.TxDescs()
	hashStrings := make([]string, len(descs))
	for i := range hashStrings {
		hashStrings[i] = descs[i].Tx.Hash().String()
	}

	return hashStrings, nil
}

// handleGetRawTransaction implements the getrawtransaction command.
func (server *ChainRPC) handleGetRawTransaction(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetRawTransactionCmd)

	// Convert the provided transaction hash hex to a Hash.
	txHash, err := chainhash.NewHashFromStr(c.Txid)
	if err != nil {
		return nil, rpcDecodeHexError(c.Txid)
	}

	verbose := false
	if c.Verbose != nil {
		verbose = *c.Verbose != 0
	}

	// Try to fetch the transaction from the memory pool and if that fails,
	// try the block database.
	var mtx *wire.MsgTx
	var blkHash *chainhash.Hash
	var blkHeight int32
	tx, err := server.chainProvider.TxMemPool.FetchTransaction(txHash)
	if err != nil {
		if server.chainProvider.TxIndex == nil {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCNoTxInfo,
				Message: "The transaction index must be " +
					"enabled to query the blockchain " +
					"(specify --txindex)",
			}
		}

		// Look up the location of the transaction.
		blockRegion, err := server.chainProvider.TxIndex.TxBlockRegion(txHash)
		if err != nil {
			context := "Failed to retrieve transaction location"
			return nil, server.internalRPCError(err.Error(), context)
		}
		if blockRegion == nil {
			return nil, rpcNoTxInfoError(txHash)
		}

		// Load the raw transaction bytes from the database.
		var txBytes []byte
		err = server.chainProvider.DB.View(func(dbTx database.Tx) error {
			var err error
			txBytes, err = dbTx.FetchBlockRegion(blockRegion)
			return err
		})
		if err != nil {
			return nil, rpcNoTxInfoError(txHash)
		}

		// When the verbose flag isn't set, simply return the serialized
		// transaction as a hex-encoded string.  This is done here to
		// avoid deserializing it only to reserialize it again later.
		if !verbose {
			return hex.EncodeToString(txBytes), nil
		}

		// Grab the block height.
		blkHash = blockRegion.Hash
		blkHeight, err = server.chainProvider.BlockChain.BlockHeightByHash(blkHash)
		if err != nil {
			context := "Failed to retrieve block height"
			return nil, server.internalRPCError(err.Error(), context)
		}

		// Deserialize the transaction
		var msgTx wire.MsgTx
		err = msgTx.Deserialize(bytes.NewReader(txBytes))
		if err != nil {
			context := "Failed to deserialize transaction"
			return nil, server.internalRPCError(err.Error(), context)
		}
		mtx = &msgTx
	} else {
		// When the verbose flag isn't set, simply return the
		// network-serialized transaction as a hex-encoded string.
		if !verbose {
			// Note that this is intentionally not directly
			// returning because the first return value is a
			// string and it would result in returning an empty
			// string to the client instead of nothing (nil) in the
			// case of an error.
			mtxHex, err := server.MessageToHex(tx.MsgTx())
			if err != nil {
				return nil, err
			}
			return mtxHex, nil
		}

		mtx = tx.MsgTx()
	}

	// The verbose flag is set, so generate the JSON object and return it.
	var blkHeader chain.BlockHeader
	var blkHashStr string
	var chainHeight int32
	if blkHash != nil {
		// Fetch the header from BlockChain.
		header, err := server.chainProvider.BlockChain.HeaderByHash(blkHash)
		if err != nil {
			context := "Failed to fetch block header"
			return nil, server.internalRPCError(err.Error(), context)
		}

		blkHeader = header
		blkHashStr = blkHash.String()
		chainHeight = server.chainProvider.BlockChain.BestSnapshot().Height
	}

	rawTxn, err := server.CreateTxRawResult(server.chainProvider.ChainParams, mtx, txHash.String(),
		blkHeader, blkHashStr, blkHeight, chainHeight)
	if err != nil {
		return nil, err
	}
	return *rawTxn, nil
}

// handleGetTxOut handles gettxout commands.
func (server *ChainRPC) handleGetTxOut(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetTxOutCmd)

	// Convert the provided transaction hash hex to a Hash.
	txHash, err := chainhash.NewHashFromStr(c.Txid)
	if err != nil {
		return nil, rpcDecodeHexError(c.Txid)
	}

	// If requested and the tx is available in the mempool try to fetch it
	// from there, otherwise attempt to fetch from the block database.
	var bestBlockHash string
	var confirmations int32
	var value int64
	var pkScript []byte
	var isCoinbase bool
	includeMempool := true
	if c.IncludeMempool != nil {
		includeMempool = *c.IncludeMempool
	}
	// TODO: This is racy.  It should attempt to fetch it directly and check
	// the error.
	if includeMempool && server.chainProvider.TxMemPool.HaveTransaction(txHash) {
		tx, err := server.chainProvider.TxMemPool.FetchTransaction(txHash)
		if err != nil {
			return nil, rpcNoTxInfoError(txHash)
		}

		mtx := tx.MsgTx()
		if c.Vout > uint32(len(mtx.TxOut)-1) {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCInvalidTxVout,
				Message: "Output index number (vout) does not " +
					"exist for transaction.",
			}
		}

		txOut := mtx.TxOut[c.Vout]
		if txOut == nil {
			errStr := fmt.Sprintf("Output index: %d for txid: %s "+
				"does not exist", c.Vout, txHash)
			return nil, server.internalRPCError(errStr, "")
		}

		best := server.chainProvider.BlockChain.BestSnapshot()
		bestBlockHash = best.Hash.String()
		confirmations = 0
		value = txOut.Value
		pkScript = txOut.PkScript
		isCoinbase = blockchain.IsCoinBaseTx(mtx)
	} else {
		out := wire.OutPoint{Hash: *txHash, Index: c.Vout}
		entry, err := server.chainProvider.BlockChain.FetchUtxoEntry(out)
		if err != nil {
			return nil, rpcNoTxInfoError(txHash)
		}

		// To match the behavior of the reference client, return nil
		// (JSON null) if the transaction output is spent by another
		// transaction already in the main BlockChain.  Mined transactions
		// that are spent by a mempool transaction are not affected by
		// this.
		if entry == nil || entry.IsSpent() {
			return nil, nil
		}

		best := server.chainProvider.BlockChain.BestSnapshot()
		bestBlockHash = best.Hash.String()
		confirmations = 1 + best.Height - entry.BlockHeight()
		value = entry.Amount()
		pkScript = entry.PkScript()
		isCoinbase = entry.IsCoinBase()
	}

	// Disassemble script into single line printable format.
	// The disassembled string will contain [error] inline if the script
	// doesn't fully parse, so ignore the error here.
	disbuf, _ := txscript.DisasmString(pkScript)

	// Get further info about the script.
	// Ignore the error here since an error means the script couldn't parse
	// and there is no additional information about it anyways.
	scriptClass, addrs, reqSigs, _ := txscript.ExtractPkScriptAddrs(pkScript,
		server.chainProvider.ChainParams)
	addresses := make([]string, len(addrs))
	for i, addr := range addrs {
		addresses[i] = addr.EncodeAddress()
	}

	txOutReply := &btcjson.GetTxOutResult{
		BestBlock:     bestBlockHash,
		Confirmations: int64(confirmations),
		Value:         btcutil.Amount(value).ToBTC(),
		ScriptPubKey: btcjson.ScriptPubKeyResult{
			Asm:       disbuf,
			Hex:       hex.EncodeToString(pkScript),
			ReqSigs:   int32(reqSigs),
			Type:      scriptClass.String(),
			Addresses: addresses,
		},
		Coinbase: isCoinbase,
	}
	return txOutReply, nil
}

// handleHelp implements the help command.
func (server *ChainRPC) handleHelp(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.HelpCmd)

	// Provide a usage overview of all commands when no specific command
	// was specified.
	var command string
	if c.Command != nil {
		command = *c.Command
	}
	if command == "" {
		usage, err := server.helpCacher.rpcUsage(false)
		if err != nil {
			context := "Failed to generate RPC usage"
			return nil, server.internalRPCError(err.Error(), context)
		}
		return usage, nil
	}

	// Check that the command asked for is supported and implemented.  Only
	// search the main list of handlers since help should not be provided
	// for commands that are unimplemented or related to wallet
	// functionality.
	if _, ok := server.handlers[command]; !ok {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: "Unknown command: " + command,
		}
	}

	// Get the help for the command.
	help, err := server.helpCacher.rpcMethodHelp(command)
	if err != nil {
		context := "Failed to generate help"
		return nil, server.internalRPCError(err.Error(), context)
	}
	return help, nil
}

// handlePing implements the ping command.
func (server *ChainRPC) handlePing(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Ask server to ping \o_
	nonce, err := encoder.RandomUint64()
	if err != nil {
		return nil, server.internalRPCError("Not sending ping - failed to "+
			"generate nonce: "+err.Error(), "")
	}
	server.chainProvider.ConnMgr.BroadcastMessage(wire.NewMsgPing(nonce))

	return nil, nil
}

// retrievedTx represents a transaction that was either loaded from the
// transaction memory pool or from the database.  When a transaction is loaded
// from the database, it is loaded with the raw serialized bytes while the
// mempool has the fully deserialized structure.  This structure therefore will
// have one of the two fields set depending on where is was retrieved from.
// This is mainly done for efficiency to avoid extra serialization steps when
// possible.
type retrievedTx struct {
	txBytes []byte
	blkHash *chainhash.Hash // Only set when transaction is in a block.
	tx      *btcutil.Tx
}

// createVinListPrevOut returns a slice of JSON objects for the inputs of the
// passed transaction.
func (server *ChainRPC) createVinListPrevOut(mtx *wire.MsgTx, chainParams *chaincore.Params, vinExtra bool, filterAddrMap map[string]struct{}) ([]btcjson.VinPrevOut, error) {
	// Coinbase transactions only have a single txin by definition.
	if blockchain.IsCoinBaseTx(mtx) {
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
func (server *ChainRPC) fetchMempoolTxnsForAddress(addr btcutil.Address, numToSkip, numRequested uint32) ([]*btcutil.Tx, uint32) {
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

// handleSearchRawTransactions implements the searchrawtransactions command.
func (server *ChainRPC) handleSearchRawTransactions(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Respond with an error if the address index is not enabled.
	addrIndex := server.chainProvider.AddrIndex
	if addrIndex == nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCMisc,
			Message: "Address index must be enabled (--addrindex)",
		}
	}

	// Override the flag for including extra previous output information in
	// each input if needed.
	c := cmd.(*btcjson.SearchRawTransactionsCmd)
	vinExtra := false
	if c.VinExtra != nil {
		vinExtra = *c.VinExtra != 0
	}

	// Including the extra previous output information requires the
	// transaction index.  Currently the address index relies on the
	// transaction index, so this check is redundant, but it'server better to be
	// safe in case the address index is ever changed to not rely on it.
	if vinExtra && server.chainProvider.TxIndex == nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCMisc,
			Message: "Transaction index must be enabled (--txindex)",
		}
	}

	// Attempt to decode the supplied address.
	params := server.chainProvider.ChainParams
	addr, err := btcutil.DecodeAddress(c.Address, params)
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidAddressOrKey,
			Message: "Invalid address or key: " + err.Error(),
		}
	}

	// Override the default number of requested entries if needed.  Also,
	// just return now if the number of requested entries is zero to avoid
	// extra work.
	numRequested := 100
	if c.Count != nil {
		numRequested = *c.Count
		if numRequested < 0 {
			numRequested = 1
		}
	}
	if numRequested == 0 {
		return nil, nil
	}

	// Override the default number of entries to skip if needed.
	var numToSkip int
	if c.Skip != nil {
		numToSkip = *c.Skip
		if numToSkip < 0 {
			numToSkip = 0
		}
	}

	// Override the reverse flag if needed.
	var reverse bool
	if c.Reverse != nil {
		reverse = *c.Reverse
	}

	// Add transactions from mempool first if client asked for reverse
	// order.  Otherwise, they will be added last (as needed depending on
	// the requested counts).
	//
	// NOTE: This code doesn't sort by dependency.  This might be something
	// to do in the future for the client'server convenience, or leave it to the
	// client.
	numSkipped := uint32(0)
	addressTxns := make([]retrievedTx, 0, numRequested)
	if reverse {
		// Transactions in the mempool are not in a block header yet,
		// so the block header field in the retieved transaction struct
		// is left nil.
		mpTxns, mpSkipped := server.fetchMempoolTxnsForAddress(addr, uint32(numToSkip), uint32(numRequested))
		numSkipped += mpSkipped
		for _, tx := range mpTxns {
			addressTxns = append(addressTxns, retrievedTx{tx: tx})
		}
	}

	// Fetch transactions from the database in the desired order if more are
	// needed.
	if len(addressTxns) < numRequested {
		err = server.chainProvider.DB.View(func(dbTx database.Tx) error {
			regions, dbSkipped, err := addrIndex.TxRegionsForAddress(
				dbTx, addr, uint32(numToSkip)-numSkipped,
				uint32(numRequested-len(addressTxns)), reverse)
			if err != nil {
				return err
			}

			// Load the raw transaction bytes from the database.
			serializedTxns, err := dbTx.FetchBlockRegions(regions)
			if err != nil {
				return err
			}

			// Add the transaction and the hash of the block it is
			// contained in to the list.  Note that the transaction
			// is left serialized here since the caller might have
			// requested non-verbose output and hence there would be
			// no point in deserializing it just to reserialize it
			// later.
			for i, serializedTx := range serializedTxns {
				addressTxns = append(addressTxns, retrievedTx{
					txBytes: serializedTx,
					blkHash: regions[i].Hash,
				})
			}
			numSkipped += dbSkipped

			return nil
		})
		if err != nil {
			context := "Failed to load address index entries"
			return nil, server.internalRPCError(err.Error(), context)
		}

	}

	// Add transactions from mempool last if client did not request reverse
	// order and the number of results is still under the number requested.
	if !reverse && len(addressTxns) < numRequested {
		// Transactions in the mempool are not in a block header yet,
		// so the block header field in the retieved transaction struct
		// is left nil.
		mpTxns, mpSkipped := server.fetchMempoolTxnsForAddress(addr,
			uint32(numToSkip)-numSkipped, uint32(numRequested-
				len(addressTxns)))
		numSkipped += mpSkipped
		for _, tx := range mpTxns {
			addressTxns = append(addressTxns, retrievedTx{tx: tx})
		}
	}

	// Address has never been used if neither source yielded any results.
	if len(addressTxns) == 0 {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCNoTxInfo,
			Message: "No information available about address",
		}
	}

	// Serialize all of the transactions to hex.
	hexTxns := make([]string, len(addressTxns))
	for i := range addressTxns {
		// Simply encode the raw bytes to hex when the retrieved
		// transaction is already in serialized form.
		rtx := &addressTxns[i]
		if rtx.txBytes != nil {
			hexTxns[i] = hex.EncodeToString(rtx.txBytes)
			continue
		}

		// Serialize the transaction first and convert to hex when the
		// retrieved transaction is the deserialized structure.
		hexTxns[i], err = server.MessageToHex(rtx.tx.MsgTx())
		if err != nil {
			return nil, err
		}
	}

	// When not in verbose mode, simply return a list of serialized txns.
	if c.Verbose != nil && *c.Verbose == 0 {
		return hexTxns, nil
	}

	// Normalize the provided filter addresses (if any) to ensure there are
	// no duplicates.
	filterAddrMap := make(map[string]struct{})
	if c.FilterAddrs != nil && len(*c.FilterAddrs) > 0 {
		for _, addr := range *c.FilterAddrs {
			filterAddrMap[addr] = struct{}{}
		}
	}

	// The verbose flag is set, so generate the JSON object and return it.
	best := server.chainProvider.BlockChain.BestSnapshot()
	srtList := make([]btcjson.SearchRawTransactionsResult, len(addressTxns))
	for i := range addressTxns {
		// The deserialized transaction is needed, so deserialize the
		// retrieved transaction if it'server in serialized form (which will
		// be the case when it was lookup up from the database).
		// Otherwise, use the existing deserialized transaction.
		rtx := &addressTxns[i]
		var mtx *wire.MsgTx
		if rtx.tx == nil {
			// Deserialize the transaction.
			mtx = new(wire.MsgTx)
			err := mtx.Deserialize(bytes.NewReader(rtx.txBytes))
			if err != nil {
				context := "Failed to deserialize transaction"
				return nil, server.internalRPCError(err.Error(),
					context)
			}
		} else {
			mtx = rtx.tx.MsgTx()
		}

		result := &srtList[i]
		result.Hex = hexTxns[i]
		result.Txid = mtx.TxHash().String()
		result.Vin, err = server.createVinListPrevOut(mtx, params, vinExtra,
			filterAddrMap)
		if err != nil {
			return nil, err
		}
		result.Vout = server.CreateVoutList(mtx, params, filterAddrMap)
		result.Version = mtx.Version
		result.LockTime = mtx.LockTime

		// Transactions grabbed from the mempool aren't yet in a block,
		// so conditionally fetch block details here.  This will be
		// reflected in the final JSON output (mempool won't have
		// confirmations or block information).
		var blkHeader chain.BlockHeader
		var blkHashStr string
		var blkHeight int32
		if blkHash := rtx.blkHash; blkHash != nil {
			// Fetch the header from BlockChain.
			header, err := server.chainProvider.BlockChain.HeaderByHash(blkHash)
			if err != nil {
				return nil, &btcjson.RPCError{
					Code:    btcjson.ErrRPCBlockNotFound,
					Message: "Block not found",
				}
			}

			// Get the block height from BlockChain.
			height, err := server.chainProvider.BlockChain.BlockHeightByHash(blkHash)
			if err != nil {
				context := "Failed to obtain block height"
				return nil, server.internalRPCError(err.Error(), context)
			}

			blkHeader = header
			blkHashStr = blkHash.String()
			blkHeight = height
		}

		// Add the block information to the result if there is any.
		if blkHeader != nil {
			// This is not a typo, they are identical in Bitcoin
			// Core as well.
			result.Time = blkHeader.Timestamp().Unix()
			result.Blocktime = blkHeader.Timestamp().Unix()
			result.BlockHash = blkHashStr
			result.Confirmations = uint64(1 + best.Height - blkHeight)
		}
	}

	return srtList, nil
}

// handleSendRawTransaction implements the sendrawtransaction command.
func (server *ChainRPC) handleSendRawTransaction(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.SendRawTransactionCmd)
	// Deserialize and send off to tx relay
	hexStr := c.HexTx
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	serializedTx, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, rpcDecodeHexError(hexStr)
	}
	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(serializedTx))
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCDeserialization,
			Message: "TX decode failed: " + err.Error(),
		}
	}

	// Use 0 for the tag to represent local chainProvider.
	tx := btcutil.NewTx(&msgTx)
	acceptedTxs, err := server.chainProvider.TxMemPool.ProcessTransaction(tx, false, false, 0)
	if err != nil {
		// When the error is a rule error, it means the transaction was
		// simply rejected as opposed to something actually going wrong,
		// so log it as such. Otherwise, something really did go wrong,
		// so log it as an actual error and return.
		ruleErr, ok := err.(mempool.RuleError)
		if !ok {
			server.logger.Errorf("Failed to process transaction %v %v", tx.Hash(), err)

			return nil, &btcjson.RPCError{
				Code:    btcjson.ErrRPCTxError,
				Message: "TX rejected: " + err.Error(),
			}
		}

		server.logger.Debugf("Rejected transaction %v: %v", tx.Hash(), err)

		// We'll then map the rule error to the appropriate RPC error,
		// matching bitcoind'server behavior.
		code := btcjson.ErrRPCTxError
		if txRuleErr, ok := ruleErr.Err.(mempool.TxRuleError); ok {
			errDesc := txRuleErr.Description
			switch {
			case strings.Contains(
				strings.ToLower(errDesc), "orphan transaction",
			):
				code = btcjson.ErrRPCTxError

			case strings.Contains(
				strings.ToLower(errDesc), "transaction already exists",
			):
				code = btcjson.ErrRPCTxAlreadyInChain

			default:
				code = btcjson.ErrRPCTxRejected
			}
		}

		return nil, &btcjson.RPCError{
			Code:    code,
			Message: "TX rejected: " + err.Error(),
		}
	}

	// When the transaction was accepted it should be the first item in the
	// returned array of accepted transactions.  The only way this will not
	// be true is if the API for ProcessTransaction changes and this code is
	// not properly updated, but ensure the condition holds as a safeguard.
	//
	// Also, since an error is being returned to the caller, ensure the
	// transaction is removed from the memory pool.
	if len(acceptedTxs) == 0 || !acceptedTxs[0].Tx.Hash().IsEqual(tx.Hash()) {
		server.chainProvider.TxMemPool.RemoveTransaction(tx, true)

		errStr := fmt.Sprintf("transaction %v is not in accepted list",
			tx.Hash())
		return nil, server.internalRPCError(errStr, "")
	}

	// Generate and relay inventory vectors for all newly accepted
	// transactions into the memory pool due to the original being
	// accepted.
	server.chainProvider.ConnMgr.RelayTransactions(acceptedTxs)

	// Notify both websocket and getblocktemplate long poll clients of all
	// newly accepted transactions.
	server.NotifyNewTransactions(acceptedTxs)

	// Keep track of all the sendrawtransaction request txns so that they
	// can be rebroadcast if they don't make their way into a block.
	txD := acceptedTxs[0]
	iv := types.NewInvVect(types.InvTypeTx, txD.Tx.Hash())
	server.chainProvider.ConnMgr.AddRebroadcastInventory(iv, txD)

	return tx.Hash().String(), nil
}

// NotifyNewTransactions notifies both websocket and getblocktemplate long
// poll clients of the passed transactions.  This function should be called
// whenever new transactions are added to the mempool.
func (server *ChainRPC) NotifyNewTransactions(txns []*mempool.TxDesc) {
	// for _, txD := range txns {
	//	// Notify websocket clients about mempool transactions.
	//	//server.ntfnMgr.NotifyMempoolTx(txD.Tx, true)
	//
	//	// Potentially notify any getblocktemplate long poll clients
	//	// about stale block templates due to the new transaction.
	//	server.gbtWorkState.NotifyMempoolTx(server.cfg.TxMemPool.LastUpdated())
	// }
}

// handleSetGenerate implements the setgenerate command.
// func (s *ChainRPC) handleSetGenerate(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
//	c := cmd.(*btcjson.SetGenerateCmd)
//
//	// Disable generation regardless of the provided generate flag if the
//	// maximum number of threads (goroutines for our purposes) is 0.
//	// Otherwise enable or disable it depending on the provided flag.
//	generate := c.Generate
//	genProcLimit := -1
//	if c.GenProcLimit != nil {
//		genProcLimit = *c.GenProcLimit
//	}
//	if genProcLimit == 0 {
//		generate = false
//	}
//
//	if !generate {
//		s.chainProvider.CPUMiner.Stop()
//	} else {
//		// Respond with an error if there are no addresses to pay the
//		// created blocks to.
//		if len(s.cfg.MiningAddrs) == 0 {
//			return nil, &btcjson.RPCError{
//				Code: btcjson.ErrRPCInternal.Code,
//				Message: "No payment addresses specified " +
//					"via --miningaddr",
//			}
//		}
//
//		// It's safe to call start even if it's already started.
//		s.chainProvider.CPUMiner.SetNumWorkers(int32(genProcLimit))
//		s.chainProvider.CPUMiner.Start()
//	}
//	return nil, nil
// }

// handleStop implements the stop command.
func (server *ChainRPC) handleStop(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// todo
	// select {
	// case server.requestProcessShutdown <- struct{}{}:
	// default:
	// }
	return "btcd stopping.", nil
}

// handleSubmitBlock implements the submitblock command.
func (server *ChainRPC) handleSubmitBlock(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
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
	_, err = server.chainProvider.SyncMgr.SubmitBlock(block, blockchain.BFNone)
	if err != nil {
		return fmt.Sprintf("rejected: %s", err.Error()), nil
	}

	server.logger.Infof(fmt.Sprintf("Accepted block %s via submitblock", block.Hash().String()))
	return nil, nil
}

// handleUptime implements the uptime command.
func (server *ChainRPC) handleUptime(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return time.Now().Unix() - server.chainProvider.StartupTime, nil
}

// handleValidateAddress implements the validateaddress command.
func (server *ChainRPC) handleValidateAddress(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
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
func (server *ChainRPC) handleVerifyChain(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
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
func (server *ChainRPC) handleVerifyMessage(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
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

// handleVersion implements the version command.
//
// NOTE: This is a btcsuite extension ported from github.com/decred/dcrd.
func (server *ChainRPC) handleVersion(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	result := map[string]btcjson.VersionResult{
		"btcdjsonrpcapi": {
			VersionString: jsonrpcSemverString,
			Major:         jsonrpcSemverMajor,
			Minor:         jsonrpcSemverMinor,
			Patch:         jsonrpcSemverPatch,
		},
	}
	return result, nil
}

func (server *ChainRPC) handleGetnetworkinfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// result := btcjson.NewGetNetworkInfoCmd()
	// fmt.Println("NetworkInfo: ", result)
	return struct {
		Subversion string `json:"subversion"`
	}{
		Subversion: "/Satoshi:0.18.0/",
	}, nil
}
