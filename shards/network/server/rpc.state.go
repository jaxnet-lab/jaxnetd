package server

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/btcjson"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/mining"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
)

// gbtWorkState houses state that is used in between multiple RPC invocations to
// getblocktemplate.
type gbtWorkState struct {
	sync.Mutex
	lastTxUpdate  time.Time
	lastGenerated time.Time
	prevHash      *chainhash.Hash
	minTimestamp  time.Time
	template      *mining.BlockTemplate
	notifyMap     map[chainhash.Hash]map[int64]chan struct{}
	timeSource    blockchain.MedianTimeSource
}

// newGbtWorkState returns a new instance of a gbtWorkState with all internal
// fields initialized and ready to use.
func newGbtWorkState(timeSource blockchain.MedianTimeSource) *gbtWorkState {
	return &gbtWorkState{
		notifyMap:  make(map[chainhash.Hash]map[int64]chan struct{}),
		timeSource: timeSource,
	}
}

// notifyLongPollers notifies any channels that have been registered to be
// notified when block templates are stale.
//
// This function MUST be called with the state locked.
func (state *gbtWorkState) notifyLongPollers(latestHash *chainhash.Hash, lastGenerated time.Time) {
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
func (state *gbtWorkState) NotifyBlockConnected(blockHash *chainhash.Hash) {
	go func() {
		state.Lock()
		defer state.Unlock()

		state.notifyLongPollers(blockHash, state.lastTxUpdate)
	}()
}

// NotifyMempoolTx uses the new last updated time for the transaction memory
// pool to notify any long poll clients with a new block template when their
// existing block template is stale due to enough time passing and the contents
// of the memory pool changing.
func (state *gbtWorkState) NotifyMempoolTx(lastUpdated time.Time) {
	go func() {
		state.Lock()
		defer state.Unlock()

		// No need to notify anything if no block templates have been generated
		// yet.
		if state.prevHash == nil || state.lastGenerated.IsZero() {
			return
		}

		if time.Now().After(state.lastGenerated.Add(time.Second *
			gbtRegenerateSeconds)) {

			state.notifyLongPollers(state.prevHash, lastUpdated)
		}
	}()
}

// templateUpdateChan returns a channel that will be closed once the block
// template associated with the passed previous hash and last generated time
// is stale.  The function will return existing channels for duplicate
// parameters which allows multiple clients to wait for the same block template
// without requiring a different channel for each client.
//
// This function MUST be called with the state locked.
func (state *gbtWorkState) templateUpdateChan(prevHash *chainhash.Hash, lastGenerated int64) chan struct{} {
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

// updateBlockTemplate creates or updates a block template for the work state.
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
func (state *gbtWorkState) updateBlockTemplate(s *ChainRPC, useCoinbaseValue bool) error {
	generator := s.chainProvider.Generator
	lastTxUpdate := generator.TxSource().LastUpdated()
	if lastTxUpdate.IsZero() {
		lastTxUpdate = time.Now()
	}

	// Generate a new block template when the current best block has
	// changed or the transactions in the memory pool have been updated and
	// it has been at least gbtRegenerateSecond since the last template was
	// generated.
	var msgBlock *wire.MsgBlock
	var targetDifficulty string
	latestHash := &s.chainProvider.BlockChain.BestSnapshot().Hash
	template := state.template
	if template == nil || state.prevHash == nil ||
		!state.prevHash.IsEqual(latestHash) ||
		(state.lastTxUpdate != lastTxUpdate &&
			time.Now().After(state.lastGenerated.Add(time.Second*
				gbtRegenerateSeconds))) {

		// Reset the previous best hash the block template was generated
		// against so any errors below cause the next invocation to try
		// again.
		state.prevHash = nil

		// Choose a payment address at random if the caller requests a
		// full coinbase as opposed to only the pertinent details needed
		// to create their own coinbase.
		var payAddr btcutil.Address
		if !useCoinbaseValue && len(s.chainProvider.MiningAddrs) > 0 {
			payAddr = s.chainProvider.MiningAddrs[rand.Intn(len(s.chainProvider.MiningAddrs))]
		}

		// Create a new block template that has a coinbase which anyone
		// can redeem.  This is only acceptable because the returned
		// block template doesn't include the coinbase, so the caller
		// will ultimately create their own coinbase which pays to the
		// appropriate address(es).
		blkTemplate, err := generator.NewBlockTemplate(payAddr)
		if err != nil {
			return btcjson.NewRPCError(btcjson.ErrRPCInternal.Code,
				"ailed to create new block template:"+err.Error())
		}

		template = blkTemplate
		msgBlock = template.Block
		targetDifficulty = fmt.Sprintf("%064x",
			chain.CompactToBig(msgBlock.Header.Bits()))

		// Get the minimum allowed timestamp for the block based on the
		// median timestamp of the last several blocks per the BlockChain
		// consensus rules.
		best := s.chainProvider.BlockChain.BestSnapshot()
		minTimestamp := mining.MinimumMedianTime(best)

		// Update work state to ensure another block template isn't
		// generated until needed.
		state.template = template
		state.lastGenerated = time.Now()
		state.lastTxUpdate = lastTxUpdate
		state.prevHash = latestHash
		state.minTimestamp = minTimestamp

		s.logger.Debugf("Generated block template (timestamp %v, "+
			"target %s, merkle root %s, mmr %s)",
			msgBlock.Header.Timestamp, targetDifficulty,
			msgBlock.Header.MerkleRoot, msgBlock.Header.MergeMiningRoot().String())

		// Notify any clients that are long polling about the new
		// template.
		state.notifyLongPollers(latestHash, lastTxUpdate)
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
			payToAddr := s.chainProvider.MiningAddrs[rand.Intn(len(s.chainProvider.MiningAddrs))]

			// Update the block coinbase output of the template to
			// pay to the randomly selected payment address.
			pkScript, err := txscript.PayToAddrScript(payToAddr)
			if err != nil {
				return btcjson.NewRPCError(btcjson.ErrRPCInternal.Code,
					"Failed to create pay-to-addr script:"+err.Error())
			}
			template.Block.Transactions[0].TxOut[0].PkScript = pkScript
			template.ValidPayAddress = true

			// Update the merkle root.
			block := btcutil.NewBlock(template.Block)
			merkles := blockchain.BuildMerkleTreeStore(block.Transactions(), false)
			template.Block.Header.SetMerkleRoot(*merkles[len(merkles)-1])
		}

		// Set locals for convenience.
		msgBlock = template.Block
		targetDifficulty = fmt.Sprintf("%064x",
			chain.CompactToBig(msgBlock.Header.Bits()))

		// Update the time of the block template to the current time
		// while accounting for the median time of the past several
		// blocks per the BlockChain consensus rules.
		generator.UpdateBlockTime(msgBlock)
		msgBlock.Header.SetNonce(0)

		s.logger.Debugf("Updated block template (timestamp %v, "+
			"target %s)", msgBlock.Header.Timestamp,
			targetDifficulty)
	}

	return nil
}

// blockTemplateResult returns the current block template associated with the
// state as a btcjson.GetBlockTemplateResult that is ready to be encoded to JSON
// and returned to the caller.
//
// This function MUST be called with the state locked.
func (state *gbtWorkState) blockTemplateResult(useCoinbaseValue bool, submitOld *bool) (*btcjson.GetBlockTemplateResult, error) {
	// Ensure the timestamps are still in valid range for the template.
	// This should really only ever happen if the local clock is changed
	// after the template is generated, but it's important to avoid serving
	// invalid block templates.
	template := state.template
	msgBlock := template.Block
	header := msgBlock.Header
	adjustedTime := state.timeSource.AdjustedTime()
	maxTime := adjustedTime.Add(time.Second * blockchain.MaxTimeOffsetSeconds)
	if header.Timestamp().After(maxTime) {
		return nil, &btcjson.RPCError{
			Code: btcjson.ErrRPCOutOfRange,
			Message: fmt.Sprintf("The template time is after the "+
				"maximum allowed time for a block - template "+
				"time %v, maximum time %v", adjustedTime,
				maxTime),
		}
	}

	// Convert each transaction in the block template to a template result
	// transaction.  The result does not include the coinbase, so notice
	// the adjustments to the various lengths and indices.
	numTx := len(msgBlock.Transactions)
	transactions := make([]btcjson.GetBlockTemplateResultTx, 0, numTx-1)
	txIndex := make(map[chainhash.Hash]int64, numTx)
	for i, tx := range msgBlock.Transactions {
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
			return nil, btcjson.NewRPCError(btcjson.ErrRPCInternal.Code, context)
		}

		bTx := btcutil.NewTx(tx)
		resultTx := btcjson.GetBlockTemplateResultTx{
			Data:    hex.EncodeToString(txBuf.Bytes()),
			Hash:    txHash.String(),
			Depends: depends,
			Fee:     template.Fees[i],
			SigOps:  template.SigOpCosts[i],
			Weight:  blockchain.GetTransactionWeight(bTx),
		}
		transactions = append(transactions, resultTx)
	}

	// Generate the block template reply.  Note that following mutations are
	// implied by the included or omission of fields:
	//  Including MinTime -> time/decrement
	//  Omitting CoinbaseTxn -> coinbase, generation
	targetDifficulty := fmt.Sprintf("%064x", chain.CompactToBig(header.Bits()))
	templateID := ToolsXt{}.EncodeTemplateID(state.prevHash, state.lastGenerated)
	reply := btcjson.GetBlockTemplateResult{
		Bits:         strconv.FormatInt(int64(header.Bits()), 16),
		CurTime:      header.Timestamp().Unix(),
		Height:       int64(template.Height),
		PreviousHash: header.PrevBlock().String(),
		WeightLimit:  blockchain.MaxBlockWeight,
		SigOpLimit:   blockchain.MaxBlockSigOpsCost,
		SizeLimit:    wire.MaxBlockPayload,
		Transactions: transactions,
		Version:      int32(header.Version()),
		LongPollID:   templateID,
		SubmitOld:    submitOld,
		Target:       targetDifficulty,
		MinTime:      state.minTimestamp.Unix(),
		MaxTime:      maxTime.Unix(),
		Mutable:      gbtMutableFields,
		NonceRange:   gbtNonceRange,
		Capabilities: gbtCapabilities,
	}
	// If the generated block template includes transactions with witness
	// data, then include the witness commitment in the GBT result.
	if template.WitnessCommitment != nil {
		reply.DefaultWitnessCommitment = hex.EncodeToString(template.WitnessCommitment)
	}

	if useCoinbaseValue {
		reply.CoinbaseAux = gbtCoinbaseAux
		reply.CoinbaseValue = &msgBlock.Transactions[0].TxOut[0].Value
	} else {
		// Ensure the template has a valid payment address associated
		// with it when a full coinbase is requested.
		if !template.ValidPayAddress {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCInternal.Code,
				Message: "A coinbase transaction has been " +
					"requested, but the Server has not " +
					"been configured with any payment " +
					"addresses via --miningaddr",
			}
		}

		// Serialize the transaction for conversion to hex.
		tx := msgBlock.Transactions[0]
		txBuf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		if err := tx.Serialize(txBuf); err != nil {
			err := errors.Wrap(err, "Failed to serialize transaction")
			return nil, btcjson.NewRPCError(btcjson.ErrRPCInternal.Code, err.Error())
		}

		resultTx := btcjson.GetBlockTemplateResultTx{
			Data:    hex.EncodeToString(txBuf.Bytes()),
			Hash:    tx.TxHash().String(),
			Depends: []int64{},
			Fee:     template.Fees[0],
			SigOps:  template.SigOpCosts[0],
		}

		reply.CoinbaseTxn = &resultTx
	}

	return &reply, nil
}
