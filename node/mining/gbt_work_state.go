// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/rpcutli"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const (

	// uint256Size is the number of bytes needed to represent an unsigned
	// 256-bit integer.
	uint256Size = 32

	// gbtNonceRange is two 32-bit big-endian hexadecimal integers which
	// represent the valid ranges of nonces returned by the getblocktemplate
	// RPC.
	gbtNonceRange = "00000000ffffffff"

	// gbtRegenerateSeconds is the number of seconds that must pass before
	// a new template is generated when the previous block hash has not
	// changed and there have been changes to the available transactions
	// in the memory pool.
	gbtRegenerateSeconds = 60
)

var (
	// gbtMutableFields are the manipulations the server allows to be made
	// to block templates generated by the getblocktemplate RPC.  It is
	// declared here to avoid the overhead of creating the slice on every
	// invocation for constant data.
	gbtMutableFields = []string{
		"time", "transactions/add", "prevblock", "coinbase/append",
	}

	// gbtCoinbaseAux describes additional data that miners should include
	// in the coinbase signature script.  It is declared here to avoid the
	// overhead of creating a new object on every invocation for constant
	// data.
	gbtCoinbaseAux = &jaxjson.GetBlockTemplateResultAux{
		Flags: hex.EncodeToString(builderScript(txscript.
			NewScriptBuilder().
			AddData([]byte(CoinbaseFlags)))),
	}

	// gbtCapabilities describes additional capabilities returned with a
	// block template generated by the getblocktemplate RPC.    It is
	// declared here to avoid the overhead of creating the slice on every
	// invocation for constant data.
	gbtCapabilities = []string{"proposal"}
)

// builderScript is a convenience function which is used for hard-coded scripts
// built with the script builder.   Any errors are converted to a panic since it
// is only, and must only, be used with hard-coded, and therefore, known good,
// scripts.
func builderScript(builder *txscript.ScriptBuilder) []byte {
	script, err := builder.Script()
	if err != nil {
		panic(err)
	}
	return script
}

// GBTWorkState houses state that is used in between multiple RPC invocations to
// getBlockTemplate.
type GBTWorkState struct {
	sync.Mutex
	lastTxUpdate  time.Time
	LastGenerated time.Time
	prevHash      *chainhash.Hash
	minTimestamp  time.Time
	Template      *chaindata.BlockTemplate
	notifyMap     map[chainhash.Hash]map[int64]chan struct{}
	timeSource    chaindata.MedianTimeSource
	Log           zerolog.Logger
	generator     *BlkTmplGenerator
}

// NewGbtWorkState returns a new instance of a GBTWorkState with all internal
// fields initialized and ready to use.
func NewGbtWorkState(timeSource chaindata.MedianTimeSource, generator *BlkTmplGenerator, log zerolog.Logger) *GBTWorkState {
	return &GBTWorkState{
		notifyMap:  make(map[chainhash.Hash]map[int64]chan struct{}),
		timeSource: timeSource,
		generator:  generator,
		Log:        log.With().Str("ctx", "gbtWorkState").Logger(),
	}
}

// NotifyLongPollers notifies any channels that have been registered to be
// notified when block templates are stale.
//
// This function MUST be called with the state locked.
func (state *GBTWorkState) NotifyLongPollers(latestHash *chainhash.Hash, lastGenerated time.Time) {
	// Notify anything that is waiting for a block template update from a
	// hash which is not the hash of the tip of the best BlockChain since their
	// work is now invalid.
	for hash, channels := range state.notifyMap {
		if !hash.IsEqual(latestHash) {
			for _, c := range channels {
				close(c)
			}
			delete(state.notifyMap, hash)
		}
	}

	// Return now if the provided last generated timestamp has not been
	// initialized.
	if lastGenerated.IsZero() {
		return
	}

	// Return now if there is nothing registered for updates to the current
	// best block hash.
	channels, ok := state.notifyMap[*latestHash]
	if !ok {
		return
	}

	// Notify anything that is waiting for a block template update from a
	// block template generated before the most recently generated block
	// template.
	lastGeneratedUnix := lastGenerated.Unix()
	for lastGen, c := range channels {
		if lastGen < lastGeneratedUnix {
			close(c)
			delete(channels, lastGen)
		}
	}

	// Remove the entry altogether if there are no more registered
	// channels.
	if len(channels) == 0 {
		delete(state.notifyMap, *latestHash)
	}
}

// NotifyBlockConnected uses the newly-connected block to notify any long poll
// clients with a new block template when their existing block template is
// stale due to the newly connected block.
func (state *GBTWorkState) NotifyBlockConnected(blockHash *chainhash.Hash) {
	go func() {
		state.Lock()
		defer state.Unlock()

		state.NotifyLongPollers(blockHash, state.lastTxUpdate)
	}()
}

// NotifyMempoolTx uses the new last updated time for the transaction memory
// pool to notify any long poll clients with a new block template when their
// existing block template is stale due to enough time passing and the contents
// of the memory pool changing.
func (state *GBTWorkState) NotifyMempoolTx(lastUpdated time.Time) {
	go func() {
		state.Lock()
		defer state.Unlock()

		// No need to notify anything if no block templates have been generated
		// yet.
		if state.prevHash == nil || state.LastGenerated.IsZero() {
			return
		}

		if time.Now().After(state.LastGenerated.Add(time.Second *
			gbtRegenerateSeconds)) {

			state.NotifyLongPollers(state.prevHash, lastUpdated)
		}
	}()
}

// TemplateUpdateChan returns a channel that will be closed once the block
// template associated with the passed previous hash and last generated time
// is stale.  The function will return existing channels for duplicate
// parameters which allows multiple clients to wait for the same block template
// without requiring a different channel for each client.
//
// This function MUST be called with the state locked.
func (state *GBTWorkState) TemplateUpdateChan(prevHash *chainhash.Hash, lastGenerated int64) chan struct{} {
	// Either get the current list of channels waiting for updates about
	// changes to block template for the previous hash or create a new one.
	channels, ok := state.notifyMap[*prevHash]
	if !ok {
		m := make(map[int64]chan struct{})
		state.notifyMap[*prevHash] = m
		channels = m
	}

	// Get the current channel associated with the time the block template
	// was last generated or create a new one.
	c, ok := channels[lastGenerated]
	if !ok {
		c = make(chan struct{})
		channels[lastGenerated] = c
	}

	return c
}

func (state *GBTWorkState) BlockTemplate(chainProvider chainProvider, useCoinbaseValue bool, burnReward int) (chaindata.BlockTemplate, error) {
	err := state.UpdateBlockTemplate(chainProvider, useCoinbaseValue, burnReward)
	if err != nil {
		return chaindata.BlockTemplate{}, err
	}
	return *state.Template, nil
}

// UpdateBlockTemplate creates or updates a block template for the work state.
// A new block template will be generated when the current best block has
// changed or the transactions in the memory pool have been updated and it has
// been long enough since the last template was generated.  Otherwise, the
// timestamp for the existing block template is updated (and possibly the
// difficulty on testnet per the consesus rules).  Finally, if the
// useCoinbaseValue flag is false and the existing block template does not
// already contain a valid payment address, the block template will be updated
// with a randomly selected payment address from the list of configured
// addresses.
//
// This function MUST be called with the state locked.
func (state *GBTWorkState) UpdateBlockTemplate(chainProvider chainProvider, useCoinbaseValue bool, burnRewardFlags int) error {
	lastTxUpdate := state.generator.TxSource().LastUpdated()
	if lastTxUpdate.IsZero() {
		lastTxUpdate = time.Now()
	}

	// Generate a new block template when the current best block has
	// changed or the transactions in the memory pool have been updated and
	// it has been at least gbtRegenerateSecond since the last template was
	// generated.
	var msgBlock = state.generator.chainCtx.EmptyBlock()
	var targetDifficulty string

	miningAddrs := chainProvider.MiningAddresses()
	latestHash := &chainProvider.BlockChain().BestSnapshot().Hash
	template := state.Template

	if template == nil || state.prevHash == nil ||
		!state.prevHash.IsEqual(latestHash) ||
		(state.lastTxUpdate != lastTxUpdate &&
			time.Now().After(state.LastGenerated.Add(time.Second*
				gbtRegenerateSeconds))) {

		// Reset the previous best hash the block template was generated
		// against so any errors below cause the next invocation to try
		// again.
		state.prevHash = nil

		// Choose a payment address at random if the caller requests a
		// full coinbase as opposed to only the pertinent details needed
		// to create their own coinbase.
		var payAddr jaxutil.Address
		if !useCoinbaseValue && len(miningAddrs) <= 1 {
			payAddr = miningAddrs[0]
		}
		if !useCoinbaseValue && len(miningAddrs) > 0 {
			payAddr = miningAddrs[rand.Intn(len(miningAddrs))]
		}

		// Create a new block template that has a coinbase which anyone
		// can redeem.  This is only acceptable because the returned
		// block template doesn't include the coinbase, so the caller
		// will ultimately create their own coinbase which pays to the
		// appropriate address(es).
		blkTemplate, err := state.generator.NewBlockTemplate(payAddr, burnRewardFlags)
		if err != nil {
			return jaxjson.NewRPCError(jaxjson.ErrRPCInternal.Code,
				"failed to create new block template: "+err.Error())
		}

		template = blkTemplate
		msgBlock = *template.Block
		targetDifficulty = fmt.Sprintf("%064x",
			pow.CompactToBig(msgBlock.Header.Bits()))

		// Get the minimum allowed timestamp for the block based on the
		// median timestamp of the last several blocks per the BlockChain
		// consensus rules.
		best := chainProvider.BlockChain().BestSnapshot()
		minTimestamp := MinimumMedianTime(best)

		// Update work state to ensure another block template isn't
		// generated until needed.
		state.Template = template
		state.LastGenerated = time.Now()
		state.lastTxUpdate = lastTxUpdate
		state.prevHash = latestHash
		state.minTimestamp = minTimestamp

		state.Log.Debug().
			Time("block_time", msgBlock.Header.Timestamp()).
			Str("target_difficulty", targetDifficulty).
			Stringer("merkle_root", msgBlock.Header.MerkleRoot()).
			Msg("Generated block template")

		// Notify any clients that are long polling about the new
		// template.
		state.NotifyLongPollers(latestHash, lastTxUpdate)
	} else {
		// At this point, there is a saved block template and another
		// request for a template was made, but either the available
		// transactions haven't change or it hasn't been long enough to
		// trigger a new block template to be generated.  So, update the
		// existing block template.

		// When the caller requires a full coinbase as opposed to only
		// the pertinent details needed to create their own coinbase,
		// add a payment address to the output of the coinbase of the
		// template if it doesn't already have one.  Since this requires
		// mining addresses to be specified via the config, an error is
		// returned if none have been specified.
		if !useCoinbaseValue && !template.ValidPayAddress {
			// Choose a payment address at random.
			payToAddr := miningAddrs[rand.Intn(len(miningAddrs))]

			// Update the block coinbase output of the template to
			// pay to the randomly selected payment address.
			pkScript, err := txscript.PayToAddrScript(payToAddr)
			if err != nil {
				return jaxjson.NewRPCError(jaxjson.ErrRPCInternal.Code,
					"Failed to create pay-to-addr script:"+err.Error())
			}
			burnReward := false
			switch state.generator.chainCtx.IsBeacon() {
			case true:
				burnReward = burnRewardFlags&types.BurnJaxNetReward == types.BurnJaxNetReward
			case false:
				burnReward = burnRewardFlags&types.BurnJaxReward == types.BurnJaxReward
			}
			// rare case, impossible with current rules, remove it
			if len(template.Block.Transactions[0].TxOut) < 2 && !burnReward {
				template.Block.Transactions[0].TxOut[0].PkScript = pkScript
			}

			template.ValidPayAddress = true

			// Update the merkle root.
			block := jaxutil.NewBlock(template.Block)
			merkles := chaindata.BuildMerkleTreeStore(block.Transactions(), false)
			template.Block.Header.SetMerkleRoot(*merkles[len(merkles)-1])
		}

		// Set locals for convenience.
		msgBlock = *template.Block
		targetDifficulty = fmt.Sprintf("%064x",
			pow.CompactToBig(msgBlock.Header.Bits()))

		// Update the time of the block template to the current time
		// while accounting for the median time of the past several
		// blocks per the BlockChain consensus rules.
		err := state.generator.UpdateBlockTime(&msgBlock)
		if err != nil {
			return jaxjson.NewRPCError(jaxjson.ErrRPCInternal.Code,
				"Failed to update block time:"+err.Error())
		}
		msgBlock.Header.SetNonce(0)

		state.Log.Debug().
			Time("block_time", msgBlock.Header.Timestamp()).
			Str("target_difficulty", targetDifficulty).
			Msg("Updated block template")
	}

	return nil
}

// BeaconBlockTemplateResult returns the current block template associated with the
// state as a jaxjson.GetBeaconBlockTemplateResult that is ready to be encoded to JSON
// and returned to the caller.
//
// This function MUST be called with the state locked.
func (state *GBTWorkState) BeaconBlockTemplateResult(useCoinbaseValue bool, submitOld *bool) (*jaxjson.GetBeaconBlockTemplateResult, error) {
	// Ensure the timestamps are still in valid range for the template.
	// This should really only ever happen if the local clock is changed
	// after the template is generated, but it's important to avoid serving
	// invalid block templates.
	template := state.Template
	msgBlock := template.Block
	header := msgBlock.Header
	adjustedTime := state.timeSource.AdjustedTime()
	maxTime := adjustedTime.Add(time.Second * chaindata.MaxTimeOffsetSeconds)

	if header.Timestamp().After(maxTime) {
		return nil, &jaxjson.RPCError{
			Code: jaxjson.ErrRPCOutOfRange,
			Message: fmt.Sprintf("The template time is after the maximum allowed time for a block - template "+
				"time %v, maximum time %v", adjustedTime,
				maxTime),
		}
	}

	transactions, err := state.TransformTxs(template)
	if err != nil {
		return nil, err
	}

	mmrRoot := header.BlocksMerkleMountainRoot()
	prevHash, _ := state.generator.blockChain.MMRTree().LookupNodeByRoot(mmrRoot)
	prevSerialID, _, err := state.generator.blockChain.BlockSerialIDByHash(&prevHash.Hash)
	if err != nil {
		return nil, err
	}

	btcAuxBuf := bytes.NewBuffer(nil)
	err = header.BeaconHeader().BTCAux().Serialize(btcAuxBuf)
	if err != nil {
		return nil, err
	}

	// Generate the block template reply.  Note that following mutations are
	// implied by the included or omission of fields:
	//  Including MinTime -> time/decrement
	//  Omitting CoinbaseTxn -> coinbase, generation
	targetDifficulty := fmt.Sprintf("%064x", pow.CompactToBig(header.Bits()))
	templateID := rpcutli.ToolsXt{}.EncodeTemplateID(state.prevHash, state.LastGenerated)
	reply := jaxjson.GetBeaconBlockTemplateResult{
		Bits:          strconv.FormatInt(int64(header.Bits()), 16),
		CurTime:       header.Timestamp().Unix(),
		PreviousHash:  prevHash.Hash.String(),
		BlocksMMRRoot: header.BlocksMerkleMountainRoot().String(),
		Height:        int64(template.Height),
		SerialID:      prevSerialID + 1,
		PrevSerialID:  prevSerialID,
		Version:       int32(header.Version()),
		Shards:        header.BeaconHeader().Shards(),
		WeightLimit:   chaindata.MaxBlockWeight,
		SigOpLimit:    chaindata.MaxBlockSigOpsCost,
		SizeLimit:     wire.MaxBlockPayload,
		Transactions:  transactions,
		LongPollID:    templateID,
		SubmitOld:     submitOld,
		Target:        targetDifficulty,
		MinTime:       state.minTimestamp.Unix(),
		MaxTime:       maxTime.Unix(),
		Mutable:       gbtMutableFields,
		NonceRange:    gbtNonceRange,
		Capabilities:  gbtCapabilities,

		K:      header.K(),
		VoteK:  header.VoteK(),
		BTCAux: hex.EncodeToString(btcAuxBuf.Bytes()),
	}

	// If the generated block template includes transactions with witness
	// data, then include the witness commitment in the GBT result.
	if template.WitnessCommitment != nil {
		reply.DefaultWitnessCommitment = hex.EncodeToString(template.WitnessCommitment)
	}

	cData, err := state.CoinbaseData(template, useCoinbaseValue)
	if err != nil {
		return nil, err
	}

	reply.CoinbaseAux = cData.CoinbaseAux
	reply.CoinbaseValue = cData.CoinbaseValue
	reply.CoinbaseTxn = cData.CoinbaseTxn

	return &reply, nil
}

// ShardBlockTemplateResult returns the current block template associated with the
// state as a jaxjson.GetShardBlockTemplateResult that is ready to be encoded to JSON
// and returned to the caller.
//
// This function MUST be called with the state locked.
func (state *GBTWorkState) ShardBlockTemplateResult(useCoinbaseValue bool, submitOld *bool) (*jaxjson.GetShardBlockTemplateResult, error) {
	// Ensure the timestamps are still in valid range for the template.
	// This should really only ever happen if the local clock is changed
	// after the template is generated, but it's important to avoid serving
	// invalid block templates.
	template := state.Template
	msgBlock := template.Block
	header := msgBlock.Header
	adjustedTime := state.timeSource.AdjustedTime()
	maxTime := adjustedTime.Add(time.Second * chaindata.MaxTimeOffsetSeconds)

	if header.Timestamp().After(maxTime) {
		return nil, &jaxjson.RPCError{
			Code: jaxjson.ErrRPCOutOfRange,
			Message: fmt.Sprintf("The template time is after the maximum allowed time for a block - template "+
				"time %v, maximum time %v", adjustedTime,
				maxTime),
		}
	}

	transactions, err := state.TransformTxs(template)
	if err != nil {
		return nil, err
	}
	mmrRoot := header.BlocksMerkleMountainRoot()
	prevHash, _ := state.generator.blockChain.MMRTree().LookupNodeByRoot(mmrRoot)
	prevSerialID, _, err := state.generator.blockChain.BlockSerialIDByHash(&prevHash.Hash)
	if err != nil {
		return nil, err
	}

	btcAuxBuf := bytes.NewBuffer(nil)
	err = header.BeaconHeader().BTCAux().Serialize(btcAuxBuf)
	if err != nil {
		return nil, err
	}

	// Generate the block template reply.  Note that following mutations are
	// implied by the included or omission of fields:
	//  Including MinTime -> time/decrement
	//  Omitting CoinbaseTxn -> coinbase, generation
	targetDifficulty := fmt.Sprintf("%064x", pow.CompactToBig(header.Bits()))
	templateID := rpcutli.ToolsXt{}.EncodeTemplateID(state.prevHash, state.LastGenerated)
	reply := jaxjson.GetShardBlockTemplateResult{
		Bits:          strconv.FormatInt(int64(header.Bits()), 16),
		CurTime:       header.Timestamp().Unix(),
		PreviousHash:  prevHash.Hash.String(),
		BlocksMMRRoot: header.BlocksMerkleMountainRoot().String(),
		Height:        int64(template.Height),
		SerialID:      prevSerialID + 1,
		PrevSerialID:  prevSerialID,
		Version:       int32(header.Version()),
		WeightLimit:   chaindata.MaxBlockWeight,
		SigOpLimit:    chaindata.MaxBlockSigOpsCost,
		SizeLimit:     wire.MaxBlockPayload,
		Transactions:  transactions,
		LongPollID:    templateID,
		SubmitOld:     submitOld,
		Target:        targetDifficulty,
		MinTime:       state.minTimestamp.Unix(),
		MaxTime:       maxTime.Unix(),
		Mutable:       gbtMutableFields,
		NonceRange:    gbtNonceRange,
		Capabilities:  gbtCapabilities,

		K:      header.K(),
		VoteK:  header.VoteK(),
		BTCAux: hex.EncodeToString(btcAuxBuf.Bytes()),
	}

	// If the generated block template includes transactions with witness
	// data, then include the witness commitment in the GBT result.
	if template.WitnessCommitment != nil {
		reply.DefaultWitnessCommitment = hex.EncodeToString(template.WitnessCommitment)
	}

	cData, err := state.CoinbaseData(template, useCoinbaseValue)
	if err != nil {
		return nil, err
	}

	reply.CoinbaseAux = cData.CoinbaseAux
	reply.CoinbaseValue = cData.CoinbaseValue
	reply.CoinbaseTxn = cData.CoinbaseTxn

	return &reply, nil
}

type coinbaseData struct {
	CoinbaseAux   *jaxjson.GetBlockTemplateResultAux `json:"coinbaseaux,omitempty"`
	CoinbaseTxn   *jaxjson.GetBlockTemplateResultTx  `json:"coinbasetxn,omitempty"`
	CoinbaseValue *int64                             `json:"coinbasevalue,omitempty"`
}

func (state *GBTWorkState) CoinbaseData(template *chaindata.BlockTemplate, useCoinbaseValue bool) (*coinbaseData, error) {
	reply := new(coinbaseData)

	reply.CoinbaseAux = gbtCoinbaseAux
	if state.generator.blockChain.Chain().IsBeacon() {
		value := template.Block.Transactions[0].TxOut[1].Value + template.Block.Transactions[0].TxOut[2].Value
		reply.CoinbaseValue = &value
	} else {
		reply.CoinbaseValue = &template.Block.Transactions[0].TxOut[1].Value
	}

	if useCoinbaseValue {
		return reply, nil
	}

	// Ensure the template has a valid payment address associated
	// with it when a full coinbase is requested.
	if !template.ValidPayAddress {
		return nil, &jaxjson.RPCError{
			Code: jaxjson.ErrRPCInternal.Code,
			Message: "A coinbase transaction has been " +
				"requested, but the Server has not " +
				"been configured with any payment " +
				"addresses via --miningaddr",
		}
	}

	// Serialize the transaction for conversion to hex.
	tx := template.Block.Transactions[0]
	txBuf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
	if err := tx.Serialize(txBuf); err != nil {
		err := errors.Wrap(err, "Failed to serialize transaction")
		return nil, jaxjson.NewRPCError(jaxjson.ErrRPCInternal.Code, err.Error())
	}

	resultTx := jaxjson.GetBlockTemplateResultTx{
		Data:    hex.EncodeToString(txBuf.Bytes()),
		Hash:    tx.TxHash().String(),
		Depends: []int64{},
		Fee:     template.Fees[0],
		SigOps:  template.SigOpCosts[0],
	}

	reply.CoinbaseTxn = &resultTx
	return reply, nil
}

func (state *GBTWorkState) TransformTxs(template *chaindata.BlockTemplate) ([]jaxjson.GetBlockTemplateResultTx, error) {

	// Convert each transaction in the block template to a template result
	// transaction.  The result does not include the coinbase, so notice
	// the adjustments to the various lengths and indices.
	numTx := len(template.Block.Transactions)
	transactions := make([]jaxjson.GetBlockTemplateResultTx, 0, numTx-1)
	txIndex := make(map[chainhash.Hash]int64, numTx)
	for i, tx := range template.Block.Transactions {
		txHash := tx.TxHash()
		txIndex[txHash] = int64(i)

		// Skip the coinbase transaction.
		if i == 0 {
			continue
		}

		// Create an array of 1-based indices to transactions that come
		// before this one in the transactions list which this one
		// depends on.  This is necessary since the created block must
		// ensure proper ordering of the dependencies.  A map is used
		// before creating the final array to prevent duplicate entries
		// when multiple inputs reference the same transaction.
		dependsMap := make(map[int64]struct{})
		for _, txIn := range tx.TxIn {
			if idx, ok := txIndex[txIn.PreviousOutPoint.Hash]; ok {
				dependsMap[idx] = struct{}{}
			}
		}
		depends := make([]int64, 0, len(dependsMap))
		for idx := range dependsMap {
			depends = append(depends, idx)
		}

		// Serialize the transaction for later conversion to hex.
		txBuf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		if err := tx.Serialize(txBuf); err != nil {
			context := errors.Wrap(err, "Failed to serialize transaction").Error()
			return nil, jaxjson.NewRPCError(jaxjson.ErrRPCInternal.Code, context)
		}

		bTx := jaxutil.NewTx(tx)
		resultTx := jaxjson.GetBlockTemplateResultTx{
			Data:    hex.EncodeToString(txBuf.Bytes()),
			Hash:    txHash.String(),
			Depends: depends,
			Fee:     template.Fees[i],
			SigOps:  template.SigOpCosts[i],
			Weight:  chaindata.GetTransactionWeight(bTx),
		}
		transactions = append(transactions, resultTx)
	}
	return transactions, nil

}
