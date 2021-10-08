// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"fmt"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/blocknode"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// checkBlockHeaderContext performs several validation checks on the block header
// which depend on its position within the block chain.
//
// The flags modify the behavior of this function as follows:
//  - BFFastAdd: All checks except those involving comparing the header against
//    the checkpoints are not performed.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) checkBlockHeaderContext(header wire.BlockHeader, prevNode blocknode.IBlockNode, flags chaindata.BehaviorFlags) error {
	fastAdd := flags&chaindata.BFFastAdd == chaindata.BFFastAdd
	if !fastAdd {
		// Ensure the difficulty specified in the block header matches
		// the calculated difficulty based on the previous block and
		// difficulty retarget rules.
		expectedDifficulty, err := b.calcNextRequiredDifficulty(prevNode, header.Timestamp())
		if err != nil {
			return err
		}

		blockDifficulty := header.Bits()

		blockDifficultyLess := pow.CompactToBig(blockDifficulty).Cmp(pow.CompactToBig(expectedDifficulty)) > 0
		if (b.chain.IsBeacon() && blockDifficulty != expectedDifficulty) || (!b.chain.IsBeacon() && blockDifficultyLess) {
			str := "block difficulty of %0x is not the expected value of %0x, with time(%s)"
			str = fmt.Sprintf(str, blockDifficulty, expectedDifficulty, header.Timestamp())
			return chaindata.NewRuleError(chaindata.ErrUnexpectedDifficulty, str)
		}

		// Ensure the timestamp for the block header is after the
		// median time of the last several blocks (medianTimeBlocks).
		medianTime := prevNode.CalcPastMedianTime()
		if !header.Timestamp().After(medianTime) {
			str := "block timestamp of %v is not after expected %v"
			str = fmt.Sprintf(str, header.Timestamp, medianTime)
			return chaindata.NewRuleError(chaindata.ErrTimeTooOld, str)
		}

		if b.chain.IsBeacon() {
			// todo: validate expected K for shards also
			expectedK := b.calcNextK(prevNode)

			if expectedK != header.K() {
				str := fmt.Sprintf("block K value of %0x is not the expected value of %0x, with time(%s) at height(%v)",
					header.K(), expectedK, header.Timestamp(), prevNode.Height()+1)
				return chaindata.NewRuleError(chaindata.ErrUnexpectedKValue, str)
			}

			if prevNode != nil && prevNode.Height() > 3 {
				if err := chaindata.ValidateVoteK(header); err != nil {
					return chaindata.NewRuleError(chaindata.ErrUnexpectedKValue, err.Error())
				}
			}
		}
	}

	// The height of this block is one more than the referenced previous
	// block.
	blockHeight := prevNode.Height() + 1

	// Ensure chain matches up to predetermined checkpoints.
	blockHash := header.BlockHash()
	if !b.verifyCheckpoint(blockHeight, &blockHash) {
		str := fmt.Sprintf("block at height %d does not match "+
			"checkpoint hash", blockHeight)
		return chaindata.NewRuleError(chaindata.ErrBadCheckpoint, str)
	}

	// Find the previous checkpoint and prevent blocks which fork the main
	// chain before it.  This prevents storage of new, otherwise valid,
	// blocks which build off of old blocks that are likely at a much easier
	// difficulty and therefore could be used to waste cache and disk space.
	checkpointNode, err := b.findPreviousCheckpoint()
	if err != nil {
		return err
	}
	if checkpointNode != nil && blockHeight < checkpointNode.Height() {
		str := fmt.Sprintf("block at height %d forks the main chain "+
			"before the previous checkpoint at height %d",
			blockHeight, checkpointNode.Height())
		return chaindata.NewRuleError(chaindata.ErrForkTooOld, str)
	}

	return nil
}

// checkBlockContext peforms several validation checks on the block which depend
// on its position within the block chain.
//
// The flags modify the behavior of this function as follows:
//  - BFFastAdd: The transaction are not checked to see if they are finalized
//    and the somewhat expensive BIP0034 validation is not performed.
//
// The flags are also passed to checkBlockHeaderContext.  See its documentation
// for how the flags modify its behavior.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) checkBlockContext(block *jaxutil.Block, prevNode blocknode.IBlockNode, flags chaindata.BehaviorFlags) error {
	// Perform all block header related validation checks.
	header := block.MsgBlock().Header
	err := b.checkBlockHeaderContext(header, prevNode, flags)
	if err != nil {
		return err
	}

	fastAdd := flags&chaindata.BFFastAdd == chaindata.BFFastAdd
	if !fastAdd {
		// Obtain the latest state of the deployed CSV soft-fork in
		// order to properly guard the new validation behavior based on
		// the current BIP 9 version bits state.
		csvState, err := b.deploymentState(prevNode, chaincfg.DeploymentCSV)
		if err != nil {
			return err
		}

		// Once the CSV soft-fork is fully active, we'll switch to
		// using the current median time past of the past block's
		// timestamps for all lock-time based checks.
		blockTime := header.Timestamp()
		if csvState == ThresholdActive {
			blockTime = prevNode.CalcPastMedianTime()
		}

		// The height of this block is one more than the referenced
		// previous block.
		blockHeight := prevNode.Height() + 1

		// Ensure all transactions in the block are finalized.
		for _, tx := range block.Transactions() {
			if !chaindata.IsFinalizedTransaction(tx, blockHeight, blockTime) {

				str := fmt.Sprintf("block contains unfinalized transaction %v", tx.Hash())
				return chaindata.NewRuleError(chaindata.ErrUnfinalizedTx, str)
			}
		}

		// Ensure coinbase starts with serialized block heights for
		// blocks whose version is the serializedHeightVersion or newer
		// once a majority of the network has upgraded.  This is part of
		// BIP0034.
		if chaindata.ShouldHaveSerializedBlockHeight(header) {

			coinbaseTx := block.Transactions()[0]
			err := chaindata.CheckSerializedHeight(coinbaseTx, blockHeight)
			if err != nil {
				return err
			}
		}

		// Query for the Version Bits state for the segwit soft-fork
		// deployment. If segwit is active, we'll switch over to
		// enforcing all the new rules.
		segwitState, err := b.deploymentState(prevNode, chaincfg.DeploymentSegwit)
		if err != nil {
			return err
		}

		// If segwit is active, then we'll need to fully validate the
		// new witness commitment for adherence to the rules.
		if segwitState == ThresholdActive {
			// Validate the witness commitment (if any) within the
			// block.  This involves asserting that if the coinbase
			// contains the special commitment output, then this
			// merkle root matches a computed merkle root of all
			// the wtxid's of the transactions within the block. In
			// addition, various other checks against the
			// coinbase's witness stack.
			if err := chaindata.ValidateWitnessCommitment(block); err != nil {
				return err
			}

			// Once the witness commitment, witness nonce, and sig
			// op cost have been validated, we can finally assert
			// that the block's weight doesn't exceed the current
			// consensus parameter.
			blockWeight := chaindata.GetBlockWeight(block)
			if blockWeight > chaindata.MaxBlockWeight {
				str := fmt.Sprintf("block's weight metric is too high - got %v, max %v",
					blockWeight, chaindata.MaxBlockWeight)
				return chaindata.NewRuleError(chaindata.ErrBlockWeightTooHigh, str)
			}
		}
	}

	return nil
}

// checkConnectBlock performs several checks to confirm connecting the passed
// block to the chain represented by the passed view does not violate any rules.
// In addition, the passed view is updated to spend all of the referenced
// outputs and add all of the new utxos created by block.  Thus, the view will
// represent the state of the chain as if the block were actually connected and
// consequently the best hash for the view is also updated to passed block.
//
// An example of some of the checks performed are ensuring connecting the block
// would not cause any duplicate transaction hashes for old transactions that
// aren't already fully spent, double spends, exceeding the maximum allowed
// signature operations per block, invalid values in relation to the expected
// block subsidy, or fail transaction script validation.
//
// The CheckConnectBlockTemplate function makes use of this function to perform
// the bulk of its work.  The only difference is this function accepts a node
// which may or may not require reorganization to connect it to the main chain
// whereas CheckConnectBlockTemplate creates a new node which specifically
// connects to the end of the current main chain and then calls this function
// with that node.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) checkConnectBlock(node blocknode.IBlockNode, block *jaxutil.Block, view *chaindata.UtxoViewpoint, stxos *[]chaindata.SpentTxOut) error {
	// If the side chain blocks end up in the database, a call to
	// CheckBlockSanity should be done here in case a previous version
	// allowed a block that is no longer valid.  However, since the
	// implementation only currently uses memory for the side chain blocks,
	// it isn't currently necessary.

	// The coinbase for the Genesis block is not spendable, so just return
	// an error now.
	h := node.GetHash()
	if h.IsEqual(b.chain.Params().GenesisHash()) {
		str := "the coinbase for the genesis block is not spendable"
		return chaindata.NewRuleError(chaindata.ErrMissingTxOut, str)
	}

	// Ensure the view is for the node being checked.
	parentMMR := block.MsgBlock().Header.BlocksMerkleMountainRoot()
	parentHash := b.index.HashByMMR(&parentMMR)
	if !view.BestHash().IsEqual(&parentHash) {
		return chaindata.AssertError(fmt.Sprintf("inconsistent view when "+
			"checking block connection: best hash is %v instead "+
			"of expected %v", view.BestHash(), parentHash))
	}

	// Load all of the utxos referenced by the inputs for all transactions
	// in the block don't already exist in the utxo view from the database.
	//
	// These utxo entries are needed for verification of things such as
	// transaction inputs, counting pay-to-script-hashes, and scripts.
	err := view.FetchInputUtxos(b.db, block)
	if err != nil {
		return err
	}

	// BIP0016 describes a pay-to-script-hash type that is considered a
	// "standard" type.  The rules for this BIP only apply to transactions
	// after the timestamp defined by txscript.Bip16Activation.  See
	// https://en.bitcoin.it/wiki/BIP_0016 for more details.
	enforceBIP0016 := node.Timestamp() >= txscript.Bip16Activation.Unix()

	// Query for the Version Bits state for the segwit soft-fork
	// deployment. If segwit is active, we'll switch over to enforcing all
	// the new rules.
	segwitState, err := b.deploymentState(node.Parent(), chaincfg.DeploymentSegwit)
	if err != nil {
		return err
	}
	enforceSegWit := segwitState == ThresholdActive

	// The number of signature operations must be less than the maximum
	// allowed per block.  Note that the preliminary sanity checks on a
	// block also include a check similar to this one, but this check
	// expands the count to include a precise count of pay-to-script-hash
	// signature operations in each of the input transaction public key
	// scripts.
	transactions := block.Transactions()
	totalSigOpCost := 0
	for i, tx := range transactions {
		// Since the first (and only the first) transaction has
		// already been verified to be a coinbase transaction,
		// use i == 0 as an optimization for the flag to
		// countP2SHSigOps for whether or not the transaction is
		// a coinbase transaction rather than having to do a
		// full coinbase check again.
		sigOpCost, err := chaindata.GetSigOpCost(tx, i == 0, view, enforceBIP0016,
			enforceSegWit)
		if err != nil {
			return err
		}

		// Check for overflow or going over the limits.  We have to do
		// this on every loop iteration to avoid overflow.
		lastSigOpCost := totalSigOpCost
		totalSigOpCost += sigOpCost
		if totalSigOpCost < lastSigOpCost || totalSigOpCost > chaindata.MaxBlockSigOpsCost {
			str := fmt.Sprintf("block contains too many "+
				"signature operations - got %v, max %v",
				totalSigOpCost, chaindata.MaxBlockSigOpsCost)
			return chaindata.NewRuleError(chaindata.ErrTooManySigOps, str)
		}
	}

	// Perform several checks on the inputs for each transaction.  Also
	// accumulate the total fees.  This could technically be combined with
	// the loop above instead of running another loop over the transactions,
	// but by separating it we can avoid running the more expensive (though
	// still relatively cheap as compared to running the scripts) checks
	// against all the inputs when the signature operations are out of
	// bounds.
	var totalFees int64
	for _, tx := range transactions {
		txFee, err := chaindata.CheckTransactionInputs(tx, node.Height(), view, b.chain.Params())
		if err != nil {
			return err
		}

		// Sum the total fees and ensure we don't overflow the
		// accumulator.
		lastTotalFees := totalFees
		totalFees += txFee
		if totalFees < lastTotalFees {
			return chaindata.NewRuleError(chaindata.ErrBadFees, "total fees for block "+
				"overflows accumulator")
		}

		// Add all of the outputs for this transaction which are not
		// provably unspendable as available utxos.  Also, the passed
		// spent txos slice is updated to contain an entry for each
		// spent txout in the order each transaction spends them.
		err = view.ConnectTransaction(tx, node.Height(), stxos)
		if err != nil {
			return err
		}

		if tx.MsgTx().Version == wire.TxVerEADAction {
			// todo(mike)
		}
	}

	// The total output values of the coinbase transaction must not exceed
	// the expected subsidy value plus total transaction fees gained from
	// mining the block.  It is safe to ignore overflow and out of range
	// errors here because those error conditions would have already been
	// caught by checkTransactionSanity.
	var totalSatoshiOut int64
	for _, txOut := range transactions[0].MsgTx().TxOut {
		totalSatoshiOut += txOut.Value
	}

	reward := b.blockGen.CalcBlockSubsidy(node.Height(), node.Header(), b.chain.Params().Net)

	expectedSatoshiOut := reward + totalFees
	if totalSatoshiOut > expectedSatoshiOut {
		str := fmt.Sprintf("coinbase transaction for block pays %v "+
			"which is more than expected value of %v",
			totalSatoshiOut, expectedSatoshiOut)
		return chaindata.NewRuleError(chaindata.ErrBadCoinbaseValue, str)
	}

	// Don't run scripts if this node is before the latest known good
	// checkpoint since the validity is verified via the checkpoints (all
	// transactions are included in the merkle root hash and any changes
	// will therefore be detected by the next checkpoint).  This is a huge
	// optimization because running the scripts is the most time consuming
	// portion of block handling.
	checkpoint := b.LatestCheckpoint()
	runScripts := true
	if checkpoint != nil && node.Height() <= checkpoint.Height {
		runScripts = false
	}

	// Blocks created after the BIP0016 activation time need to have the
	// pay-to-script-hash checks enabled.
	var scriptFlags txscript.ScriptFlags
	if enforceBIP0016 {
		scriptFlags |= txscript.ScriptBip16
	}

	// Enforce DER signatures for block versions 3+ once the historical
	// activation threshold has been reached.  This is part of BIP0066.
	blockHeader := block.MsgBlock().Header
	if blockHeader.Version() >= 3 {
		scriptFlags |= txscript.ScriptVerifyDERSignatures
	}

	// Enforce CHECKLOCKTIMEVERIFY for block versions 4+ once the historical
	// activation threshold has been reached.  This is part of BIP0065.
	if blockHeader.Version() >= 4 {
		scriptFlags |= txscript.ScriptVerifyCheckLockTimeVerify
	}

	// Enforce CHECKSEQUENCEVERIFY during all block validation checks once
	// the soft-fork deployment is fully active.
	csvState, err := b.deploymentState(node.Parent(), chaincfg.DeploymentCSV)
	if err != nil {
		return err
	}
	if csvState == ThresholdActive {
		// If the CSV soft-fork is now active, then modify the
		// scriptFlags to ensure that the CSV op code is properly
		// validated during the script checks bleow.
		scriptFlags |= txscript.ScriptVerifyCheckSequenceVerify

		// We obtain the MTP of the *previous* block in order to
		// determine if transactions in the current block are final.
		medianTime := node.Parent().CalcPastMedianTime()

		// Additionally, if the CSV soft-fork package is now active,
		// then we also enforce the relative sequence number based
		// lock-times within the inputs of all transactions in this
		// candidate block.
		for _, tx := range block.Transactions() {
			// A transaction can only be included within a block
			// once the sequence locks of *all* its inputs are
			// active.
			sequenceLock, err := b.calcSequenceLock(node, tx, view,
				false)
			if err != nil {
				return err
			}
			switch tx.MsgTx().Version {
			case wire.TxVerTimeLock:
				if !chaindata.SequenceLockActive(sequenceLock, node.Height(), medianTime) {
					str := fmt.Sprintf(
						"block contains transaction whose input sequence locks are not met")
					return chaindata.NewRuleError(chaindata.ErrUnfinalizedTx, str)
				}
			}

		}
	}

	// Enforce the segwit soft-fork package once the soft-fork has shifted
	// into the "active" version bits state.
	if enforceSegWit {
		scriptFlags |= txscript.ScriptVerifyWitness
		scriptFlags |= txscript.ScriptStrictMultiSig
	}

	// Now that the inexpensive checks are done and have passed, verify the
	// transactions are actually allowed to spend the coins by running the
	// expensive ECDSA signature check scripts.  Doing this last helps
	// prevent CPU exhaustion attacks.
	if runScripts {
		err := chaindata.CheckBlockScripts(block, view, scriptFlags, b.SigCache, b.HashCache)
		if err != nil {
			return err
		}
	}

	// Update the best hash for view to include this block since all of its
	// transactions have been connected.
	h = node.GetHash()
	view.SetBestHash(&h)

	return nil
}

// CheckConnectBlockTemplate fully validates that connecting the passed block to
// the main chain does not violate any consensus rules, aside from the proof of
// work requirement. The block must connect to the current tip of the main chain.
//
// This function is safe for concurrent access.
func (b *BlockChain) CheckConnectBlockTemplate(block *jaxutil.Block) error {
	b.chainLock.Lock()
	defer b.chainLock.Unlock()

	// Skip the proof of work check as this is just a block template.
	flags := chaindata.BFNoPoWCheck

	// This only checks whether the block can be connected to the tip of the
	// current chain.
	tip := b.bestChain.Tip()
	header := block.MsgBlock().Header
	prevMMRRoot := header.BlocksMerkleMountainRoot()
	prevHash := b.index.HashByMMR(&prevMMRRoot)
	if tip.GetHash() != prevHash {
		str := fmt.Sprintf("previous block must be the current chain tip %v, "+
			"instead got %v", tip.GetHash(), prevHash)
		return chaindata.NewRuleError(chaindata.ErrPrevBlockNotBest, str)
	}

	err := chaindata.CheckBlockSanityWF(block, b.chain.Params(), b.TimeSource, flags)
	if err != nil {
		return err
	}
	//
	// // Perform checks of the coinbase tx structure according to merge mining spec.
	// err = b.blockGen.ValidateCoinbaseTx(block.MsgBlock(), block.Height())
	// if err != nil {
	// 	return err
	// }

	err = b.checkBlockContext(block, tip, flags)
	if err != nil {
		return err
	}

	// Leave the spent txouts entry nil in the state since the information
	// is not needed and thus extra work can be avoided.
	view := chaindata.NewUtxoViewpoint(b.chain.IsBeacon())
	h := tip.GetHash()
	view.SetBestHash(&h)
	newNode := b.chain.NewNode(header, tip)
	return b.checkConnectBlock(newNode, block, view, nil)
}
