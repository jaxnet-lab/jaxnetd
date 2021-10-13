// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/mempool"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

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
	tx      *jaxutil.Tx
}

// handleEstimateFee handles estimatefee commands.
func (server *CommonChainRPC) handleEstimateFee(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.EstimateFeeCmd)

	if server.chainProvider.FeeEstimator == nil {
		return nil, errors.New("Fee estimation disabled")
	}

	if c.NumBlocks <= 0 {
		return -1.0, errors.New("Parameter NumBlocks must be positive")
	}

	feeRate, err := server.chainProvider.FeeEstimator.EstimateFee(uint32(c.NumBlocks), server.IsBeacon())
	if err != nil {
		return -1.0, err
	}

	// Convert to satoshis per kb.
	return float64(feeRate), nil
}

// estimatesmartfee
func (server *CommonChainRPC) handleEstimateSmartFee(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.EstimateSmartFeeCmd)
	if server.chainProvider.FeeEstimator == nil {
		return nil, errors.New("Fee estimation disabled")
	}

	if c.ConfTarget <= 0 {
		return -1.0, errors.New("Parameter NumBlocks must be positive")
	}

	feeRate, err := server.chainProvider.FeeEstimator.EstimateFee(uint32(c.ConfTarget), server.IsBeacon())
	if err != nil {
		return -1.0, err
	}

	btcPerKB := float64(feeRate)
	satoshiPerB := float64(feeRate.ToSatoshiPerByte(server.IsBeacon()))

	res := jaxjson.EstimateSmartFeeResult{
		BtcPerKB:    &btcPerKB,
		SatoshiPerB: &satoshiPerB,
		Blocks:      c.ConfTarget}
	return res, nil
}

func (server *CommonChainRPC) handleGetExtendedFee(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	var err error
	result := jaxjson.ExtendedFeeFeeResult{}

	fast, moderate, slow := 2, 64, 128
	if server.chainProvider.ChainCtx.IsBeacon() {
		fast, moderate, slow = 2, 4, 8
	}

	result.Fast, err = server.estimateFeeForTarget(int64(fast))
	if err != nil {
		result.Fast.SatoshiPerB = mempool.DefaultMinRelayTxFeeSatoshiPerByte
		result.Fast.CoinsPerKB = mempool.MinRelayFee(server.IsBeacon())
	}

	result.Moderate, err = server.estimateFeeForTarget(int64(moderate))
	if err != nil {
		result.Fast.SatoshiPerB = mempool.DefaultMinRelayTxFeeSatoshiPerByte
		result.Fast.CoinsPerKB = mempool.MinRelayFee(server.IsBeacon())
	}

	result.Slow, err = server.estimateFeeForTarget(int64(slow))
	if err != nil {
		result.Fast.SatoshiPerB = mempool.DefaultMinRelayTxFeeSatoshiPerByte
		result.Fast.CoinsPerKB = mempool.MinRelayFee(server.IsBeacon())
	}

	return result, nil
}

func (server *CommonChainRPC) estimateFeeForTarget(target int64) (jaxjson.Fee, error) {
	if server.chainProvider.FeeEstimator == nil {
		return jaxjson.Fee{}, errors.New("Fee estimation disabled")
	}

	feeRate, err := server.chainProvider.FeeEstimator.EstimateFee(uint32(target), server.IsBeacon())
	if err != nil {
		return jaxjson.Fee{}, err
	}

	btcPerKB := float64(feeRate)
	satoshiPerB := float64(feeRate.ToSatoshiPerByte(server.IsBeacon()))

	res := jaxjson.Fee{
		CoinsPerKB:  btcPerKB,
		SatoshiPerB: satoshiPerB,
		Blocks:      target,
		Estimated:   true,
	}

	return res, nil
}

// handleDecodeRawTransaction handles decoderawtransaction commands.
func (server *CommonChainRPC) handleDecodeRawTransaction(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.DecodeRawTransactionCmd)

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
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCDeserialization,
			Message: "TX decode failed: " + err.Error(),
		}
	}

	// Create and return the result.
	txReply := jaxjson.TxRawDecodeResult{
		Txid:     mtx.TxHash().String(),
		Version:  mtx.Version,
		Locktime: mtx.LockTime,
		Vin:      server.CreateVinList(&mtx, 0),
		Vout:     server.CreateVoutList(&mtx, server.chainProvider.ChainParams, nil),
	}
	return txReply, nil
}

// handleCreateRawTransaction handles createrawtransaction commands.
func (server *CommonChainRPC) handleCreateRawTransaction(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.CreateRawTransactionCmd)

	// Validate the locktime, if given.
	if c.LockTime != nil &&
		(*c.LockTime < 0 || *c.LockTime > int64(wire.MaxTxInSequenceNum)) {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInvalidParameter,
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
		if amount <= 0 || amount > chaincfg.MaxSatoshi {
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCType,
				Message: "Invalid amount",
			}
		}

		// Decode the provided address.
		addr, err := jaxutil.DecodeAddress(encodedAddr, params)
		if err != nil {
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCInvalidAddressOrKey,
				Message: "Invalid address or key: " + err.Error(),
			}
		}

		// Ensure the address is one of the supported types and that
		// the network encoded with the address matches the network the
		// Server is currently on.
		switch addr.(type) {
		case *jaxutil.AddressPubKeyHash:
		case *jaxutil.AddressScriptHash:
		default:
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCInvalidAddressOrKey,
				Message: "Invalid address or key",
			}
		}
		if !addr.IsForNet(params) {
			return nil, &jaxjson.RPCError{
				Code: jaxjson.ErrRPCInvalidAddressOrKey,
				Message: "Invalid address: " + encodedAddr +
					" is for the wrong network",
			}
		}

		// Create a new script which pays to the provided address.
		pkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			context := "Failed to generate pay-to-address script"
			return nil, server.InternalRPCError(err.Error(), context)
		}

		// Convert the amount to satoshi.
		satoshi, err := jaxutil.NewAmount(amount)
		if err != nil {
			context := "Failed to convert amount"
			return nil, server.InternalRPCError(err.Error(), context)
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

// handleGetRawTransaction implements the getrawtransaction command.
func (server *CommonChainRPC) handleGetRawTransaction(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetRawTransactionCmd)

	// Convert the provided transaction hash hex to a Hash.
	txHash, err := chainhash.NewHashFromStr(c.Txid)
	if err != nil {
		return nil, rpcDecodeHexError(c.Txid)
	}

	verbose := false
	if c.Verbose != nil {
		verbose = *c.Verbose != 0
	}
	includeOrphan := false
	if c.IncludeOrphan != nil {
		includeOrphan = *c.IncludeOrphan
	}

	rawTxn, mtx, err := server.getTxVerbose(txHash, false, includeOrphan)
	if err != nil {
		return nil, err
	}

	// When the verbose flag isn't set, simply return the
	// network-serialized transaction as a hex-encoded string.
	if !verbose {
		// Note that this is intentionally not directly
		// returning because the first return value is a
		// string and it would result in returning an empty
		// string to the client instead of nothing (nil) in the
		// case of an error.
		mtxHex, err := server.MessageToHex(mtx)
		if err != nil {
			return nil, err
		}
		return mtxHex, nil
	}

	return *rawTxn, nil
}

func (server *CommonChainRPC) handleGetTxDetails(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetTxDetailsCmd)

	// Convert the provided transaction hash hex to a Hash.
	txHash, err := chainhash.NewHashFromStr(c.Txid)
	if err != nil {
		return nil, rpcDecodeHexError(c.Txid)
	}
	includeOrphan := false
	if c.IncludeOrphan != nil {
		includeOrphan = *c.IncludeOrphan
	}

	rawTxn, _, err := server.getTxVerbose(txHash, true, includeOrphan)
	if err != nil {
		return nil, err
	}

	return *rawTxn, nil
}

func (server *CommonChainRPC) getTx(txHash *chainhash.Hash, mempool, orphan bool) (*txInfo, error) {
	if mempool {
		tx, err := server.chainProvider.TxMemPool.FetchTransaction(txHash)
		if err == nil {
			return &txInfo{
				tx:        tx.MsgTx(),
				blkHash:   &chainhash.ZeroHash,
				blkHeight: 0,
			}, err
		}
	}

	// return jaxjson.TxRawResult{}, nil
	if server.chainProvider.TxIndex == nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCNoTxInfo,
			Message: "The transaction index must be enabled to query the blockchain (specify --txindex)",
		}
	}

	// Look up the location of the transaction.
	blockRegion, err := server.chainProvider.TxIndex.TxBlockRegion(txHash)
	switch {
	case err != nil:
		context := "Failed to retrieve transaction location"
		return nil, server.InternalRPCError(err.Error(), context)
	case blockRegion == nil && !orphan:
		return nil, rpcNoTxInfoError(txHash)
	case blockRegion == nil && orphan:
		blockRegion, err = server.chainProvider.OrphanTxIndex.TxBlockRegion(txHash)
		switch {
		case err != nil:
			context := "Failed to retrieve orphan transaction location"
			return nil, server.InternalRPCError(err.Error(), context)
		case blockRegion == nil:
			return nil, rpcNoTxInfoError(txHash)
		}
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

	// Grab the block height.
	blkHash := blockRegion.Hash
	blkHeight, err := server.chainProvider.BlockChain().BlockHeightByHash(blkHash)
	switch {
	case err != nil && !orphan:
		context := "Failed to retrieve block height"
		return nil, server.InternalRPCError(err.Error(), context)
	case err != nil && orphan:
		blkHeight = -1
	}

	// Deserialize the transaction
	var msgTx wire.MsgTx
	err = msgTx.Deserialize(bytes.NewReader(txBytes))
	if err != nil {
		context := "Failed to deserialize transaction"
		return nil, server.InternalRPCError(err.Error(), context)
	}

	return &txInfo{
		tx:        &msgTx,
		blkHash:   blkHash,
		blkHeight: blkHeight,
		isOrphan:  blkHeight == -1,
	}, nil
}

type txInfo struct {
	tx        *wire.MsgTx
	blkHash   *chainhash.Hash
	blkHeight int32
	isOrphan  bool
}

func (server *CommonChainRPC) getTxVerbose(txHash *chainhash.Hash, detailedIn bool,
	includeOrphan bool) (*jaxjson.TxRawResult, *wire.MsgTx, error) {
	txInfo, err := server.getTx(txHash, true, includeOrphan)
	if err != nil {
		return nil, nil, err
	}

	// The verbose flag is set, so generate the JSON object and return it.
	var blkHeader wire.BlockHeader
	var blkHashStr string
	var chainHeight int32
	if txInfo.blkHash != nil && *txInfo.blkHash != chainhash.ZeroHash && !txInfo.isOrphan {
		// Fetch the header from BlockChain.
		header, err := server.chainProvider.BlockChain().HeaderByHash(txInfo.blkHash)
		if err != nil {
			context := "Failed to fetch block header"
			return nil, nil, server.InternalRPCError(err.Error(), context)
		}

		blkHeader = header
		blkHashStr = txInfo.blkHash.String()
		chainHeight = server.chainProvider.BlockChain().BestSnapshot().Height
	}

	rawTxn, err := server.CreateTxRawResult(server.chainProvider.ChainCtx.Params(), txInfo.tx, txHash.String(),
		blkHeader, blkHashStr, txInfo.blkHeight, chainHeight)
	if err != nil {
		context := "Failed to create TxRawResult"
		return nil, nil, server.InternalRPCError(err.Error(), context)
	}

	var inputsFromAnotherChain []int
	if detailedIn && !rawTxn.CoinbaseTx {
		for i, in := range rawTxn.Vin {
			hashStr := in.Coinbase
			if in.Txid != "" {
				hashStr = in.Txid
			}
			hash, _ := chainhash.NewHashFromStr(hashStr)
			parentTx, err := server.getTx(hash, true, includeOrphan)
			switch {
			case err != nil && !txInfo.tx.SwapTx():
				context := fmt.Sprintf("missing details of parent for tx(%s)", txInfo.tx.TxHash().String())
				return nil, nil, server.InternalRPCError(err.Error(), context)
			case err != nil && txInfo.tx.SwapTx():
				// ignore missed parent for for swap tx
				inputsFromAnotherChain = append(inputsFromAnotherChain, i)
				continue
			}

			out := parentTx.tx.TxOut[in.Vout]
			if out == nil {
				continue
			}

			rawTxn.InAmount += out.Value
			rawTxn.Vin[i].Amount = out.Value
		}
		if txInfo.tx.SwapTx() {
			// in swap tx we have fixed order of inputs and outputs,
			// so if input X from chain A, then output X in chain A also
			for i := range inputsFromAnotherChain {
				rawTxn.OutAmount -= rawTxn.Vout[i].PreciseValue
			}
		}

		rawTxn.Fee = rawTxn.InAmount - rawTxn.OutAmount
	}
	rawTxn.OrphanTx = txInfo.isOrphan

	return rawTxn, txInfo.tx, err
}

func (server *CommonChainRPC) handleSendRawTransaction(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.SendRawTransactionCmd)

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
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCDeserialization,
			Message: "TX decode failed: " + err.Error(),
		}
	}

	if server.chainProvider.ChainCtx.IsBeacon() && msgTx.SwapTx() {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCTxError,
			Message: "Beacon does not support ShardSwapTx",
		}
	}

	if !server.chainProvider.ChainCtx.IsBeacon() && msgTx.Version == wire.TxVerEADAction {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCTxError,
			Message: "ShardChain does not support TxVerEADAction",
		}
	}

	// Use 0 for the tag to represent local beaconChain.
	tx := jaxutil.NewTx(&msgTx)
	acceptedTxs, err := server.chainProvider.TxMemPool.ProcessTransaction(tx, false, false, 0)
	if err != nil {
		// When the error is a rule error, it means the transaction was
		// simply rejected as opposed to something actually going wrong,
		// so log it as such. Otherwise, something really did go wrong,
		// so log it as an actual error and return.
		ruleErr, ok := err.(mempool.RuleError)
		if !ok {
			server.Log.Error().Msgf("Failed to process transaction %v %v", tx.Hash(), err)

			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCTxError,
				Message: "TX rejected: " + err.Error(),
			}
		}

		server.Log.Debug().Msgf("Rejected transaction %v: %v", tx.Hash(), err)

		// We'll then map the rule error to the appropriate RPC error,
		// matching jaxnetd'server behavior.
		code := jaxjson.ErrRPCTxError
		if txRuleErr, ok := ruleErr.Err.(mempool.TxRuleError); ok {
			errDesc := txRuleErr.Description
			switch {
			case strings.Contains(
				strings.ToLower(errDesc), "orphan transaction",
			):
				code = jaxjson.ErrRPCTxError

			case strings.Contains(
				strings.ToLower(errDesc), "transaction already exists",
			):
				code = jaxjson.ErrRPCTxAlreadyInChain

			default:
				code = jaxjson.ErrRPCTxRejected
			}
		}

		return nil, &jaxjson.RPCError{
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
		return nil, server.InternalRPCError(errStr, "")
	}

	// Generate and relay inventory vectors for all newly accepted
	// transactions into the memory pool due to the original being
	// accepted.
	server.connMgr.RelayTransactions(acceptedTxs)

	// Notify both websocket and getblocktemplate long poll clients of all
	// newly accepted transactions.
	server.NotifyNewTransactions(acceptedTxs)

	// Keep track of all the sendrawtransaction request txns so that they
	// can be rebroadcast if they don't make their way into a block.
	txD := acceptedTxs[0]
	iv := types.NewInvVect(types.InvTypeTx, txD.Tx.Hash())
	server.connMgr.AddRebroadcastInventory(iv, txD)

	return tx.Hash().String(), nil
}

// handleGetTxOut handles gettxout commands.
func (server *CommonChainRPC) handleGetTx(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetTxCmd)

	// Convert the provided transaction hash hex to a Hash.
	txHash, err := chainhash.NewHashFromStr(c.Txid)
	if err != nil {
		return nil, rpcDecodeHexError(c.Txid)
	}

	includeMempool := true
	if c.IncludeMempool != nil {
		includeMempool = *c.IncludeMempool
	}

	includeOrphan := true
	if c.IncludeOrphan != nil {
		includeOrphan = *c.IncludeOrphan
	}

	txInfo, err := server.getTx(txHash, includeMempool, includeOrphan)
	if err != nil {
		return nil, err
	}

	best := server.chainProvider.BlockChain().BestSnapshot()
	rawTx, err := txInfo.tx.SerializeToHex()
	if err != nil {
		return nil, server.InternalRPCError(err.Error(), "unable to marshal tx")
	}

	return jaxjson.GetTxResult{
		Block:         txInfo.blkHash.String(),
		Height:        int64(txInfo.blkHeight),
		Confirmations: int64(best.Height - txInfo.blkHeight),
		BestBlock:     best.Hash.String(),
		Coinbase:      chaindata.IsCoinBaseTx(txInfo.tx),
		RawTx:         rawTx,
	}, nil
}

// handleGetTxOut handles gettxout commands.
func (server *CommonChainRPC) handleGetTxOut(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetTxOutCmd)

	// Convert the provided transaction hash hex to a Hash.
	txHash, err := chainhash.NewHashFromStr(c.Txid)
	if err != nil {
		return nil, rpcDecodeHexError(c.Txid)
	}

	includeMempool := true
	if c.IncludeMempool != nil {
		includeMempool = *c.IncludeMempool
	}

	return server.getTxOut(txHash, c.Vout, includeMempool)
}

// handleGetTxOut handles gettxout commands.
func (server *CommonChainRPC) getTxOut(txHash *chainhash.Hash, vout uint32, includeMempool bool) (*jaxjson.GetTxOutResult, error) {
	// If requested and the tx is available in the mempool try to fetch it
	// from there, otherwise attempt to fetch from the block database.
	var bestBlockHash string
	var confirmations int32
	var value int64
	var pkScript []byte
	var isCoinbase bool
	var isSpent bool

	// TODO: This is racy.  It should attempt to fetch it directly and check the error.
	if includeMempool && server.chainProvider.TxMemPool.HaveTransaction(txHash) {
		tx, err := server.chainProvider.TxMemPool.FetchTransaction(txHash)
		if err != nil {
			return nil, rpcNoTxInfoError(txHash)
		}

		mtx := tx.MsgTx()
		if vout > uint32(len(mtx.TxOut)-1) {
			return nil, &jaxjson.RPCError{
				Code: jaxjson.ErrRPCInvalidTxVout,
				Message: "Output index number (vout) does not " +
					"exist for transaction.",
			}
		}

		txOut := mtx.TxOut[vout]
		if txOut == nil {
			errStr := fmt.Sprintf("Output index: %d for txid: %s "+
				"does not exist", vout, txHash)
			return nil, server.InternalRPCError(errStr, "")
		}

		best := server.chainProvider.BlockChain().BestSnapshot()
		bestBlockHash = best.Hash.String()
		confirmations = 0
		value = txOut.Value
		pkScript = txOut.PkScript
		isCoinbase = chaindata.IsCoinBaseTx(mtx)
	} else {
		out := wire.OutPoint{Hash: *txHash, Index: vout}
		entry, err := server.chainProvider.BlockChain().FetchUtxoEntry(out)
		if err != nil {
			return nil, rpcNoTxInfoError(txHash)
		}

		// To match the behavior of the reference client, return nil
		// (JSON null) if the transaction output is spent by another
		// transaction already in the main BlockChain.  Mined transactions
		// that are spent by a mempool transaction are not affected by
		// this.
		if entry == nil {
			return nil, nil
		}

		best := server.chainProvider.BlockChain().BestSnapshot()
		bestBlockHash = best.Hash.String()
		confirmations = 1 + best.Height - entry.BlockHeight()
		value = entry.Amount()
		pkScript = entry.PkScript()
		isCoinbase = entry.IsCoinBase()
		isSpent = entry.IsSpent()
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

	txOutReply := &jaxjson.GetTxOutResult{
		BestBlock:     bestBlockHash,
		Confirmations: int64(confirmations),
		Value:         jaxutil.Amount(value).ToCoin(server.IsBeacon()),
		PreciseValue:  value,
		ScriptPubKey: jaxjson.ScriptPubKeyResult{
			Asm:       disbuf,
			Hex:       hex.EncodeToString(pkScript),
			ReqSigs:   int32(reqSigs),
			Type:      scriptClass.String(),
			Addresses: addresses,
		},
		Coinbase: isCoinbase,
		IsSpent:  isSpent,
	}

	return txOutReply, nil
}

type txOut struct {
	txHash chainhash.Hash
	vout   uint32
}

// handleGetTxOut handles gettxout commands.
func (server *CommonChainRPC) handleGetTxOutsStatus(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetTxOutStatus)
	onlyMempool := true
	if c.OnlyMempool != nil {
		onlyMempool = *c.OnlyMempool
	}

	filter := map[txOut]bool{}
	for _, out := range c.Outs {
		// Convert the provided transaction hash hex to a Hash.
		txHash, err := chainhash.NewHashFromStr(out.TxHash)
		if err != nil {
			return nil, rpcDecodeHexError(out.TxHash)
		}

		filter[txOut{txHash: *txHash, vout: out.Index}] = false
	}

	return server.getTxOutStatus(filter, onlyMempool)
}

// getTxOutStatus handles getTxOutStatus commands.
func (server *CommonChainRPC) getTxOutStatus(filter map[txOut]bool, onlyMempool bool) ([]jaxjson.TxOutStatus, error) {
	result := make([]jaxjson.TxOutStatus, 0, len(filter))
	for _, txDesc := range server.chainProvider.TxMemPool.MiningDescs() {
		for _, in := range txDesc.Tx.MsgTx().TxIn {
			_, ok := filter[txOut{
				txHash: in.PreviousOutPoint.Hash,
				vout:   in.PreviousOutPoint.Index,
			}]
			if !ok {
				continue
			}

			result = append(result, jaxjson.TxOutStatus{
				OutTxHash: in.PreviousOutPoint.Hash.String(),
				OutIndex:  in.PreviousOutPoint.Index,
				IsSpent:   true,
				InMempool: true,
			})

			filter[txOut{txHash: in.PreviousOutPoint.Hash, vout: in.PreviousOutPoint.Index}] = true
		}
	}

	if onlyMempool {
		for out, found := range filter {
			if found {
				continue
			}

			result = append(result, jaxjson.TxOutStatus{
				OutTxHash: out.txHash.String(),
				OutIndex:  out.vout,
				Found:     false,
				IsSpent:   false,
				InMempool: false,
			})
		}
		return result, nil
	}

	for out, found := range filter {
		if found {
			continue
		}

		entry, err := server.chainProvider.BlockChain().
			FetchUtxoEntry(wire.OutPoint{Hash: out.txHash, Index: out.vout})
		if err != nil || entry == nil {
			result = append(result, jaxjson.TxOutStatus{
				OutTxHash: out.txHash.String(),
				OutIndex:  out.vout,
				Found:     false,
				IsSpent:   true,
				InMempool: false,
			})
			continue
		}

		result = append(result, jaxjson.TxOutStatus{
			OutTxHash: out.txHash.String(),
			OutIndex:  out.vout,
			Found:     true,
			InMempool: false,
			IsSpent:   entry.IsSpent(),
		})
	}

	return result, nil
}

// handleGetBlockTxOps handles getblocktxops commands.
func (server *CommonChainRPC) handleGetBlockTxOps(cmd interface{}, _ <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetBlockTxOpsCmd)

	// Convert the provided block hash hex to a Hash.
	hash, err := chainhash.NewHashFromStr(c.BlockHash)
	if err != nil {
		return nil, rpcDecodeHexError(c.BlockHash)
	}

	var blkBytes []byte
	err = server.chainProvider.DB.View(func(dbTx database.Tx) error {
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
	blk, err := jaxutil.NewBlockFromBytes(server.chainProvider.ChainCtx, blkBytes)
	if err != nil {
		context := "Failed to deserialize block"
		return nil, server.InternalRPCError(err.Error(), context)
	}

	best := server.chainProvider.BlockChain().BestSnapshot()
	var confirmations int64
	// Get the block height from BlockChain.
	blockHeight, err := server.chainProvider.BlockChain().BlockHeightByHash(hash)
	if err != nil {
		blockHeight = -1
		confirmations = -1
	} else {
		confirmations = int64(best.Height - blockHeight)
	}

	blk.SetHeight(blockHeight)
	txs := blk.Transactions()
	result := jaxjson.BlockTxOperations{
		BlockHash:     c.BlockHash,
		BlockHeight:   int64(blockHeight),
		Confirmations: confirmations,
		Ops:           make([]jaxjson.TxOperation, 0, len(txs)*2),
	}

	for txId, tx := range txs {
		coinbase := chaindata.IsCoinBase(tx)
		var missedInputs = map[int]struct{}{}

		if chaindata.IsCoinBase(tx) {
			goto outsAnalysis
		}

		for inId, in := range tx.MsgTx().TxIn {
			out, found, err := server.getParentOut(tx.MsgTx().SwapTx(), in, inId, tx.Hash())
			if err != nil {
				return nil, err
			}

			if !found {
				missedInputs[inId] = struct{}{}
				continue
			}

			_, addrr, _, _ := txscript.ExtractPkScriptAddrs(out.PkScript, server.chainProvider.ChainParams)
			addresses := make([]string, 0, len(addrr))
			for _, adr := range addrr {
				addresses = append(addresses, adr.EncodeAddress())
			}

			op := jaxjson.TxOperation{
				Input:        true,
				PkScript:     hex.EncodeToString(out.PkScript),
				Addresses:    addresses,
				Idx:          uint32(inId),
				Amount:       out.Value,
				TxHash:       tx.Hash().String(),
				TxIndex:      uint32(txId),
				Coinbase:     false,
				OriginTxHash: in.PreviousOutPoint.Hash.String(),
				OriginIdx:    in.PreviousOutPoint.Index,
				CSTx:         tx.MsgTx().SwapTx(),
				ShardID:      server.chainProvider.ChainCtx.ShardID(),
			}

			result.Ops = append(result.Ops, op)
		}

	outsAnalysis:
		for outId, out := range tx.MsgTx().TxOut {
			_, missedInput := missedInputs[outId]
			if tx.MsgTx().SwapTx() && missedInput {
				continue
			}

			_, adr, _, _ := txscript.ExtractPkScriptAddrs(out.PkScript, server.chainProvider.ChainParams)
			addresses := make([]string, 0, len(adr))
			for _, adr := range adr {
				addresses = append(addresses, adr.EncodeAddress())
			}

			op := jaxjson.TxOperation{
				Input:        false,
				PkScript:     hex.EncodeToString(out.PkScript),
				Addresses:    addresses,
				Idx:          uint32(outId),
				Amount:       out.Value,
				TxHash:       tx.Hash().String(),
				TxIndex:      uint32(txId),
				Coinbase:     coinbase,
				OriginTxHash: tx.Hash().String(),
				OriginIdx:    uint32(outId),
				CSTx:         tx.MsgTx().SwapTx(),
				ShardID:      server.chainProvider.ChainCtx.ShardID(),
			}

			result.Ops = append(result.Ops, op)
		}

	}

	return result, nil
}

func (server *CommonChainRPC) getParentOut(swapTx bool, in *wire.TxIn, inId int, txHash *chainhash.Hash) (out *wire.TxOut, found bool, err error) {
	parentTx, err := server.getTx(&in.PreviousOutPoint.Hash, true, true)
	switch {
	case err != nil && !swapTx:
		context := fmt.Sprintf("unable to fetch input(%d) details for tx(%s)", inId, txHash.String())
		err = rpcNoTxInfoError(&in.PreviousOutPoint.Hash)
		return nil, false, server.InternalRPCError(err.Error(), context)
	case err != nil && swapTx:
		// ignore missed parent for for swap tx
		return nil, false, nil
	}

	if swapTx && parentTx.tx.SwapTx() {
		grandParentIn := parentTx.tx.TxIn[in.PreviousOutPoint.Index]
		_, found, err := server.getParentOut(true, grandParentIn, int(in.PreviousOutPoint.Index), txHash)
		if !found || err != nil {
			return nil, false, nil
		}
	}

	return parentTx.tx.TxOut[in.PreviousOutPoint.Index], true, nil
}

// handleListTxOut handles listtxout commands.
func (server *CommonChainRPC) handleListTxOut(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	_ = cmd.(*jaxjson.ListTxOutCmd)

	entries, err := server.chainProvider.BlockChain().ListUtxoEntry(1000)
	if err != nil {
		return nil, rpcNoTxInfoError(nil)
	}

	best := server.chainProvider.BlockChain().BestSnapshot()

	var reply = make([]jaxjson.ExtendedTxOutResult, 0, len(entries))
	for out, entry := range entries {
		var confirmations int32
		var pkScript []byte
		var isCoinbase bool

		// To match the behavior of the reference client, return nil
		// (JSON null) if the transaction output is spent by another
		// transaction already in the main BlockChain.  Mined transactions
		// that are spent by a mempool transaction are not affected by
		// this.
		// if entry == nil || entry.IsSpent() {
		// 	continue
		// }

		confirmations = 1 + best.Height - entry.BlockHeight()
		pkScript = entry.PkScript()
		isCoinbase = entry.IsCoinBase()

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

		reply = append(reply, jaxjson.ExtendedTxOutResult{
			TxHash:        out.Hash.String(),
			Index:         out.Index,
			Used:          entry.IsSpent(),
			Confirmations: int64(confirmations),
			Value:         entry.Amount(),
			ScriptPubKey: jaxjson.ScriptPubKeyResult{
				Asm:       disbuf,
				Hex:       hex.EncodeToString(pkScript),
				ReqSigs:   int32(reqSigs),
				Type:      scriptClass.String(),
				Addresses: addresses,
			},
			Coinbase: isCoinbase,
		})
	}

	return jaxjson.ListTxOutResult{List: reply}, nil
}

// handleListEADAddresses handles listEADAddresses commands.
func (server *CommonChainRPC) handleListEADAddresses(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	opts := cmd.(*jaxjson.ListEADAddressesCmd)

	listEADAddresses, err := server.chainProvider.BlockChain().ListEADAddresses()
	if err != nil {
		return nil, rpcNoTxInfoError(nil)
	}

	var reply = make(map[string]jaxjson.EADAddresses, len(listEADAddresses))
	for pubKey, eadAddresses := range listEADAddresses {
		if opts.EadPublicKey != nil && pubKey != *opts.EadPublicKey {
			continue
		}

		ips := make([]jaxjson.EADAddress, 0, len(eadAddresses.Addresses))
		presentShards := map[uint32]struct{}{}
		for _, p := range eadAddresses.Addresses {
			has := p.HasShard(opts.Shards...)
			if len(opts.Shards) > 0 && !has {
				continue
			}

			ip := ""
			if p.IP != nil {
				ip = p.IP.String()
			}
			ips = append(ips, jaxjson.EADAddress{
				IP:         ip,
				Port:       p.Port,
				URL:        p.URL,
				ExpiresAt:  p.ExpiresAt,
				Shard:      p.Shard,
				TxHash:     p.TxHash.String(),
				TxOutIndex: p.TxOutIndex,
			})
			presentShards[p.Shard] = struct{}{}
		}

		if len(ips) == 0 {
			continue
		}

		if len(opts.Shards) > 0 && len(presentShards) < len(opts.Shards) {
			continue
		}

		reply[pubKey] = jaxjson.EADAddresses{
			PublicKey: string(eadAddresses.OwnerPubKey),
			ID:        eadAddresses.ID,
			IPs:       ips,
		}

	}

	return jaxjson.ListEADAddresses{Agents: reply}, nil
}

// handleSearchRawTransactions implements the searchrawtransactions command.
func (server *CommonChainRPC) handleSearchRawTransactions(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Respond with an error if the address index is not enabled.
	addrIndex := server.chainProvider.AddrIndex
	if addrIndex == nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCMisc,
			Message: "Address index must be enabled (--addrindex)",
		}
	}

	// Override the flag for including extra previous output information in
	// each input if needed.
	c := cmd.(*jaxjson.SearchRawTransactionsCmd)
	vinExtra := false
	if c.VinExtra != nil {
		vinExtra = *c.VinExtra != 0
	}

	// Including the extra previous output information requires the
	// transaction index.  Currently the address index relies on the
	// transaction index, so this check is redundant, but it'server better to be
	// safe in case the address index is ever changed to not rely on it.
	if vinExtra && server.chainProvider.TxIndex == nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCMisc,
			Message: "Transaction index must be enabled (--txindex)",
		}
	}

	// Attempt to decode the supplied address.
	params := server.chainProvider.ChainParams
	addr, err := jaxutil.DecodeAddress(c.Address, params)
	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInvalidAddressOrKey,
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
			return nil, server.InternalRPCError(err.Error(), context)
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
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCNoTxInfo,
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
	best := server.chainProvider.BlockChain().BestSnapshot()
	srtList := make([]jaxjson.SearchRawTransactionsResult, len(addressTxns))
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
				return nil, server.InternalRPCError(err.Error(),
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
		result.Vout = server.CreateVoutList(mtx, server.chainProvider.ChainCtx.Params(), filterAddrMap)
		result.Version = mtx.Version
		result.LockTime = mtx.LockTime

		// Transactions grabbed from the mempool aren't yet in a block,
		// so conditionally fetch block details here.  This will be
		// reflected in the final JSON output (mempool won't have
		// confirmations or block information).
		var blkHeader wire.BlockHeader
		var blkHashStr string
		var blkHeight int32
		if blkHash := rtx.blkHash; blkHash != nil {
			// Fetch the header from BlockChain.
			header, err := server.chainProvider.BlockChain().HeaderByHash(blkHash)
			if err != nil {
				return nil, &jaxjson.RPCError{
					Code:    jaxjson.ErrRPCBlockNotFound,
					Message: "Block not found",
				}
			}

			// Get the block height from BlockChain.
			height, err := server.chainProvider.BlockChain().BlockHeightByHash(blkHash)
			if err != nil {
				context := "Failed to obtain block height"
				return nil, server.InternalRPCError(err.Error(), context)
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

// handleGetMempoolInfo implements the getmempoolinfo command.
func (server *CommonChainRPC) handleGetMempoolInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	mempoolTxns := server.chainProvider.TxMemPool.TxDescs()

	var numBytes int64
	for _, txD := range mempoolTxns {
		numBytes += int64(txD.Tx.MsgTx().SerializeSize())
	}

	ret := &jaxjson.GetMempoolInfoResult{
		Size:  int64(len(mempoolTxns)),
		Bytes: numBytes,
	}

	return ret, nil
}

// handleGetRawMempool implements the getrawmempool command.
func (server *CommonChainRPC) handleGetRawMempool(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.GetRawMempoolCmd)
	mp := server.chainProvider.TxMemPool

	if c.Verbose != nil && *c.Verbose {
		return mp.RawMempoolVerbose(server.IsBeacon()), nil
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

// handleGetMempoolUTXOs implements the gGetMempoolUTXOs command.
// return list of UTXO used by txs in mempool
func (server *CommonChainRPC) handleGetMempoolUTXOs(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	_ = cmd.(*jaxjson.GetMempoolUTXOs)
	mp := server.chainProvider.TxMemPool

	result := make([]jaxjson.MempoolUTXO, 0, len(mp.TxDescs()))

	for _, txDesc := range mp.TxDescs() {
		for i, in := range txDesc.Tx.MsgTx().TxIn {
			result = append(result, jaxjson.MempoolUTXO{
				UTXOHash:      in.PreviousOutPoint.Hash.String(),
				UTXOIndex:     in.PreviousOutPoint.Index,
				UsedByTxHash:  txDesc.Tx.Hash().String(),
				UsedByTxIndex: uint32(i),
			})
		}
	}

	return result, nil
}
