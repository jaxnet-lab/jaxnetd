// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package chaindata

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/blocknode"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const (
	// MaxTimeOffsetSeconds is the maximum number of seconds a block time
	// is allowed to be ahead of the current time.  This is currently 2
	// hours.
	MaxTimeOffsetSeconds = 2 * 60 * 60

	// MinCoinbaseScriptLen is the minimum length a coinbase script can be.
	MinCoinbaseScriptLen = 2

	// MaxCoinbaseScriptLen is the maximum length a coinbase script can be.
	MaxCoinbaseScriptLen = 100

	// serializedHeightVersion is the block version which changed block
	// coinbases to start with the serialized block height.
	serializedHeightVersion = 2

	// baseSubsidy is the starting subsidy amount for mined blocks.  This
	// value is halved every SubsidyHalvingInterval blocks.
	baseSubsidy = 50 * jaxutil.SatoshiPerBitcoin
)

var (
	// block91842Hash is one of the two nodes which violate the rules
	// set forth in BIP0030.  It is defined as a package level variable to
	// avoid the need to create a new instance every time a check is needed.
	block91842Hash = newHashFromStr("00000000000a4d0a398161ffc163c503763b1f4360639393e0e4c8e300e0caec")

	// block91880Hash is one of the two nodes which violate the rules
	// set forth in BIP0030.  It is defined as a package level variable to
	// avoid the need to create a new instance every time a check is needed.
	block91880Hash = newHashFromStr("00000000000743f190a18c5577a3c2d2a1f610ae9601ac046a38084ccb7cd721")
)

// isNullOutpoint determines whether or not a previous transaction output point
// is set.
func isNullOutpoint(outpoint *wire.OutPoint) bool {
	if outpoint.Index == math.MaxUint32 && outpoint.Hash == chainhash.ZeroHash {
		return true
	}
	return false
}

// newHashFromStr converts the passed big-endian hex string into a
// chainhash.Hash.  It only differs from the one available in chainhash in that
// it ignores the error since it will only (and must only) be called with
// hard-coded, and therefore known good, hashes.
func newHashFromStr(hexStr string) *chainhash.Hash {
	hash, _ := chainhash.NewHashFromStr(hexStr)
	return hash
}

// ShouldHaveSerializedBlockHeight determines if a block should have a
// serialized block height embedded within the scriptSig of its
// coinbase transaction. Judgement is based on the block version in the block
// header. Blocks with version 2 and above satisfy this criteria. See BIP0034
// for further information.
func ShouldHaveSerializedBlockHeight(header wire.BlockHeader) bool {
	return header.Version() >= serializedHeightVersion
}

// IsCoinBaseTx determines whether or not a transaction is a coinbase.  A coinbase
// is a special transaction created by miners that has no inputs.  This is
// represented in the block chain by a transaction with a single input that has
// a previous output transaction index set to the maximum value along with a
// zero Hash.
//
// This function only differs from IsCoinBase in that it works with a raw wire
// transaction as opposed to a higher level util transaction.
func IsCoinBaseTx(msgTx *wire.MsgTx) bool {
	// A coin base must only have one transaction input.
	if len(msgTx.TxIn) != 1 {
		return false
	}

	// The previous output of a coin base must have a max value index and
	// a zero Hash.
	prevOut := &msgTx.TxIn[0].PreviousOutPoint
	if prevOut.Index != math.MaxUint32 || prevOut.Hash != chainhash.ZeroHash {
		return false
	}

	return true
}

// IsCoinBase determines whether or not a transaction is a coinbase.  A coinbase
// is a special transaction created by miners that has no inputs.  This is
// represented in the block chain by a transaction with a single input that has
// a previous output transaction index set to the maximum value along with a
// zero Hash.
//
// This function only differs from IsCoinBaseTx in that it works with a higher
// level util transaction as opposed to a raw wire transaction.
func IsCoinBase(tx *jaxutil.Tx) bool {
	return IsCoinBaseTx(tx.MsgTx())
}

// SequenceLockActive determines if a transaction's sequence locks have been
// met, meaning that all the inputs of a given transaction have reached a
// height or time sufficient for their relative lock-time maturity.
func SequenceLockActive(sequenceLock *SequenceLock, blockHeight int32,
	medianTimePast time.Time) bool {

	// If either the seconds, or height relative-lock time has not yet
	// reached, then the transaction is not yet mature according to its
	// sequence locks.
	if sequenceLock.Seconds >= medianTimePast.Unix() ||
		sequenceLock.BlockHeight >= blockHeight {
		return false
	}

	return true
}

func ValidMoneyBackAfterExpiration(tx *jaxutil.Tx, view *UtxoViewpoint) bool {
	if len(tx.MsgTx().TxOut) > len(tx.MsgTx().TxIn) {
		return false
	}
	inputAddresses := map[string]struct{}{}
	for _, in := range tx.MsgTx().TxIn {
		origUTXO := view.LookupEntry(in.PreviousOutPoint)
		if origUTXO == nil {
			continue
		}
		inputAddresses[hex.EncodeToString(origUTXO.PkScript())] = struct{}{}
	}

	for _, out := range tx.MsgTx().TxOut {
		if _, ok := inputAddresses[hex.EncodeToString(out.PkScript)]; !ok {
			return false
		}
	}

	return true
}

// IsFinalizedTransaction determines whether or not a transaction is finalized.
func IsFinalizedTransaction(tx *jaxutil.Tx, blockHeight int32, blockTime time.Time) bool {
	msgTx := tx.MsgTx()

	// Lock time of zero means the transaction is finalized.
	// wire.TxVerRefundableTimeLock is always finalized, like a wire.TxVerRegular.
	lockTime := msgTx.LockTime
	if lockTime == 0 {
		return true
	}

	// The lock time field of a transaction is either a block height at
	// which the transaction is finalized or a timestamp depending on if the
	// value is before the txscript.LockTimeThreshold.  When it is under the
	// threshold it is a block height.
	blockTimeOrHeight := int64(0)
	if lockTime < txscript.LockTimeThreshold {
		blockTimeOrHeight = int64(blockHeight)
	} else {
		blockTimeOrHeight = blockTime.Unix()
	}

	timeLockTx := tx.MsgTx().Version == wire.TxVerTimeLock
	if int64(lockTime) < blockTimeOrHeight {
		return timeLockTx
	}
	// At this point, the transaction's lock time hasn't occurred yet, but
	// the transaction might still be finalized if the sequence number
	// for all transaction inputs is maxed out.
	for _, txIn := range msgTx.TxIn {
		if txIn.Sequence != math.MaxUint32 {
			return !timeLockTx
		}
	}

	return true
}

// isBIP0030Node returns whether or not the passed node represents one of the
// two blocks that violate the BIP0030 rule which prevents transactions from
// overwriting old ones.
func IsBIP0030Node(node blocknode.IBlockNode) bool {
	h := node.GetHash()
	if node.Height() == 91842 && h.IsEqual(block91842Hash) {
		return true
	}

	if node.Height() == 91880 && h.IsEqual(block91880Hash) {
		return true
	}

	return false
}

// CalcBlockSubsidy returns the subsidy amount a block at the provided height
// should have. This is mainly used for determining how much the coinbase for
// newly generated blocks awards as well as validating the coinbase for blocks
// has the expected value.
//
// The subsidy is halved every SubsidyReductionInterval blocks.  Mathematically
// this is: baseSubsidy / 2^(height/SubsidyReductionInterval)
//
// At the target block generation rate for the main network, this is
// approximately every 4 years.
func CalcBlockSubsidy(height int32, chainParams *chaincfg.Params) int64 {
	if chainParams.SubsidyReductionInterval == 0 {
		return baseSubsidy
	}

	// Equivalent to: baseSubsidy / 2^(height/subsidyHalvingInterval)
	return baseSubsidy >> uint(height/chainParams.SubsidyReductionInterval)
}

// CheckTransactionSanity performs some preliminary checks on a transaction to
// ensure it is sane.  These checks are context free.
func CheckTransactionSanity(tx *jaxutil.Tx) error {
	// A transaction must have at least one input.
	msgTx := tx.MsgTx()
	if len(msgTx.TxIn) == 0 {
		return NewRuleError(ErrNoTxInputs, "transaction has no inputs")
	}

	// A transaction must have at least one output.
	if len(msgTx.TxOut) == 0 {
		return NewRuleError(ErrNoTxOutputs, "transaction has no outputs")
	}

	if msgTx.SwapTx() {
		err := ValidateSwapTxStructure(msgTx, -1)
		if err != nil {
			return err
		}
	}

	// A transaction must not exceed the maximum allowed block payload when
	// serialized.
	serializedTxSize := tx.MsgTx().SerializeSizeStripped()
	if serializedTxSize > MaxBlockBaseSize {
		str := fmt.Sprintf("serialized transaction is too big - got "+
			"%d, max %d", serializedTxSize, MaxBlockBaseSize)
		return NewRuleError(ErrTxTooBig, str)
	}

	// Ensure the transaction amounts are in range.  Each transaction
	// output must not be negative or more than the max allowed per
	// transaction.  Also, the total of all outputs must abide by the same
	// restrictions.  All amounts in a transaction are in a unit value known
	// as a satoshi.  One bitcoin is a quantity of satoshi as defined by the
	// SatoshiPerBitcoin constant.
	var totalSatoshi int64
	for _, txOut := range msgTx.TxOut {
		satoshi := txOut.Value
		if satoshi < 0 {
			str := fmt.Sprintf("transaction output has negative "+
				"value of %v", satoshi)
			return NewRuleError(ErrBadTxOutValue, str)
		}
		if satoshi > jaxutil.MaxSatoshi {
			str := fmt.Sprintf("transaction output value of %v is "+
				"higher than max allowed value of %v", satoshi,
				jaxutil.MaxSatoshi)
			return NewRuleError(ErrBadTxOutValue, str)
		}

		// Two's complement int64 overflow guarantees that any overflow
		// is detected and reported.  This is impossible for Bitcoin, but
		// perhaps possible if an alt increases the total money supply.
		totalSatoshi += satoshi
		if totalSatoshi < 0 {
			str := fmt.Sprintf("total value of all transaction "+
				"outputs exceeds max allowed value of %v",
				jaxutil.MaxSatoshi)
			return NewRuleError(ErrBadTxOutValue, str)
		}
		if totalSatoshi > jaxutil.MaxSatoshi {
			str := fmt.Sprintf("total value of all transaction "+
				"outputs is %v which is higher than max "+
				"allowed value of %v", totalSatoshi,
				jaxutil.MaxSatoshi)
			return NewRuleError(ErrBadTxOutValue, str)
		}
	}

	// Check for duplicate transaction inputs.
	existingTxOut := make(map[wire.OutPoint]struct{})
	for _, txIn := range msgTx.TxIn {
		if _, exists := existingTxOut[txIn.PreviousOutPoint]; exists {
			return NewRuleError(ErrDuplicateTxInputs, "transaction "+
				"contains duplicate inputs")
		}
		existingTxOut[txIn.PreviousOutPoint] = struct{}{}
	}

	// Coinbase script length must be between min and max length.
	if IsCoinBase(tx) {
		slen := len(msgTx.TxIn[0].SignatureScript)
		if slen < MinCoinbaseScriptLen || slen > MaxCoinbaseScriptLen {
			str := fmt.Sprintf("coinbase transaction script length "+
				"of %d is out of range (min: %d, max: %d)",
				slen, MinCoinbaseScriptLen, MaxCoinbaseScriptLen)
			return NewRuleError(ErrBadCoinbaseScriptLen, str)
		}
	} else {
		// Previous transaction outputs referenced by the inputs to this
		// transaction must not be null. Null allowed only for ShardsSwapTxs.
		for _, txIn := range msgTx.TxIn {
			if isNullOutpoint(&txIn.PreviousOutPoint) && !msgTx.SwapTx() {
				return NewRuleError(ErrBadTxInput,
					"transaction input refers to previous output that is null")
			}
		}
	}

	return nil
}

// checkProofOfWork ensures the block header bits which indicate the target
// difficulty is in min/max range and that the block Hash is less than the
// target difficulty as claimed.
//
// The flags modify the behavior of this function as follows:
//  - BFNoPoWCheck: The check to ensure the block Hash is less than the target
//    difficulty is not performed.
func checkProofOfWork(header wire.BlockHeader, chainCfg *chaincfg.Params, flags BehaviorFlags) error {
	// The target difficulty must be larger than zero.
	target := pow.CompactToBig(header.Bits())
	if target.Sign() <= 0 {
		str := fmt.Sprintf("block target difficulty of %064x is too low", target)
		return NewRuleError(ErrUnexpectedDifficulty, str)
	}

	// The target difficulty must be less than the maximum allowed.
	if target.Cmp(chainCfg.PowParams.PowLimit) > 0 {
		str := fmt.Sprintf("block target difficulty of %064x is higher than max of %064x", target,
			chainCfg.PowParams.PowLimit)
		return NewRuleError(ErrUnexpectedDifficulty, str)
	}

	// The block Hash must be less than the claimed target unless the flag
	// to avoid proof of work checks is set.
	if flags&BFNoPoWCheck != BFNoPoWCheck {
		// according to merge-mining scheme,
		// we are checking the difficulty of the BTC-container
		hash := header.PoWHash()
		// The block Hash must be less than the claimed target.
		hashNum := pow.HashToBig(&hash)
		if hashNum.Cmp(target) > 0 {
			str := fmt.Sprintf("block Hash of %064x is higher than expected max of %064x", hashNum, target)
			return NewRuleError(ErrHighHash, str)
		}

		if chainCfg.PowParams.HashSorting && !pow.ValidateHashSortingRule(target, chainCfg.PowParams.ChainIDCount, chainCfg.ChainID) {
			str := fmt.Sprintf("block Hash of %064x is not match for chain %d by hash-sorting rules",
				hashNum, chainCfg.ChainID)
			return NewRuleError(ErrHashSortingRuleNotMatch, str)
		}
	}

	return nil
}

// CheckProofOfWork ensures the block header bits which indicate the target
// difficulty is in min/max range and that the block Hash is less than the
// target difficulty as claimed.
func CheckProofOfWork(block *jaxutil.Block, chainCfg *chaincfg.Params) error {
	return checkProofOfWork(block.MsgBlock().Header, chainCfg, BFNone)
}

// CountSigOps returns the number of signature operations for all transaction
// input and output scripts in the provided transaction.  This uses the
// quicker, but imprecise, signature operation counting mechanism from
// txscript.
func CountSigOps(tx *jaxutil.Tx) int {
	msgTx := tx.MsgTx()

	// Accumulate the number of signature operations in all transaction
	// inputs.
	totalSigOps := 0
	for _, txIn := range msgTx.TxIn {
		numSigOps := txscript.GetSigOpCount(txIn.SignatureScript)
		totalSigOps += numSigOps
	}

	// Accumulate the number of signature operations in all transaction
	// outputs.
	for _, txOut := range msgTx.TxOut {
		numSigOps := txscript.GetSigOpCount(txOut.PkScript)
		totalSigOps += numSigOps
	}

	return totalSigOps
}

// CountP2SHSigOps returns the number of signature operations for all input
// transactions which are of the pay-to-script-Hash type.  This uses the
// precise, signature operation counting mechanism from the script engine which
// requires access to the input transaction scripts.
func CountP2SHSigOps(tx *jaxutil.Tx, isCoinBaseTx bool, utxoView *UtxoViewpoint) (int, error) {
	// Coinbase transactions have no interesting inputs.
	if isCoinBaseTx {
		return 0, nil
	}

	// Accumulate the number of signature operations in all transaction
	// inputs.
	msgTx := tx.MsgTx()
	totalSigOps := 0
	missingCount := 0
	thisIsSwapTx := tx.MsgTx().SwapTx()
	for txInIndex, txIn := range msgTx.TxIn {
		// Ensure the referenced input transaction is available.
		utxo := utxoView.LookupEntry(txIn.PreviousOutPoint)
		if utxo == nil && thisIsSwapTx {
			missingCount += 1
			continue
		}

		if utxo == nil || utxo.IsSpent() {
			str := fmt.Sprintf(
				"output %v referenced from transaction %s:%d either does not exist or has already been spent",
				txIn.PreviousOutPoint,
				tx.Hash(), txInIndex)
			return 0, NewRuleError(ErrMissingTxOut, str)
		}

		// We're only interested in pay-to-script-Hash types, so skip
		// this input if it's not one.
		pkScript := utxo.PkScript()
		if !txscript.IsPayToScriptHash(pkScript) {
			continue
		}

		// Count the precise number of signature operations in the
		// referenced public key script.
		sigScript := txIn.SignatureScript
		numSigOps := txscript.GetPreciseSigOpCount(sigScript, pkScript,
			true)

		// We could potentially overflow the accumulator so check for
		// overflow.
		lastSigOps := totalSigOps
		totalSigOps += numSigOps
		if totalSigOps < lastSigOps {
			str := fmt.Sprintf("the public key script from output "+
				"%v contains too many signature operations - "+
				"overflow", txIn.PreviousOutPoint)
			return 0, NewRuleError(ErrTooManySigOps, str)
		}
	}

	if thisIsSwapTx {
		err := ValidateSwapTxStructure(msgTx, missingCount)
		if err != nil {
			return 0, err
		}
	}

	return totalSigOps, nil
}

// checkBlockHeaderSanity performs some preliminary checks on a block header to
// ensure it is sane before continuing with processing.  These checks are
// context free.
//
// The flags do not modify the behavior of this function directly, however they
// are needed to pass along to checkProofOfWork.
func checkBlockHeaderSanity(header wire.BlockHeader, powLimit *chaincfg.Params, timeSource MedianTimeSource, flags BehaviorFlags) error {
	// Ensure the proof of work bits in the block header is in min/max range
	// and the block Hash is less than the target value described by the
	// bits.
	err := checkProofOfWork(header, powLimit, flags)
	if err != nil {
		return err
	}

	// A block timestamp must not have a greater precision than one second.
	// This check is necessary because Go time.Time values support
	// nanosecond precision whereas the consensus rules only apply to
	// seconds and it's much nicer to deal with standard Go time values
	// instead of converting to seconds everywhere.
	if !header.Timestamp().Equal(time.Unix(header.Timestamp().Unix(), 0)) {
		str := fmt.Sprintf("block timestamp of %v has a higher precision than one second", header.Timestamp())
		return NewRuleError(ErrInvalidTime, str)
	}

	// Ensure the block time is not too far in the future.
	maxTimestamp := timeSource.AdjustedTime().Add(time.Second * MaxTimeOffsetSeconds)
	if header.Timestamp().After(maxTimestamp) {
		str := fmt.Sprintf("block timestamp of %v is too far in the future", header.Timestamp())
		return NewRuleError(ErrTimeTooNew, str)
	}

	return nil
}

// CheckBlockSanityWF performs some preliminary checks on a block to ensure it is
// sane before continuing with block processing.  These checks are context free.
//
// The flags do not modify the behavior of this function directly, however they
// are needed to pass along to checkBlockHeaderSanity.
func CheckBlockSanityWF(block *jaxutil.Block, chainParams *chaincfg.Params, timeSource MedianTimeSource, flags BehaviorFlags) error {
	msgBlock := block.MsgBlock()
	header := msgBlock.Header
	err := checkBlockHeaderSanity(header, chainParams, timeSource, flags)
	if err != nil {
		return err
	}

	// A block must have at least one transaction.
	numTx := len(msgBlock.Transactions)
	if numTx == 0 {
		return NewRuleError(ErrNoTransactions, "block does not contain any transactions")
	}

	// A block must not have more transactions than the max block payload or
	// else it is certainly over the weight limit.
	if numTx > MaxBlockBaseSize {
		str := fmt.Sprintf("block contains too many transactions - got %d, max %d",
			numTx, MaxBlockBaseSize)
		return NewRuleError(ErrBlockTooBig, str)
	}

	// A block must not exceed the maximum allowed block payload when
	// serialized.
	serializedSize := msgBlock.SerializeSizeStripped()
	if serializedSize > MaxBlockBaseSize {
		str := fmt.Sprintf("serialized block is too big - got %d, max %d", serializedSize, MaxBlockBaseSize)
		return NewRuleError(ErrBlockTooBig, str)
	}

	// The first transaction in a block must be a coinbase.
	transactions := block.Transactions()
	if !IsCoinBase(transactions[0]) {
		return NewRuleError(ErrFirstTxNotCoinbase, "first transaction in block is not a coinbase")
	}

	// A block must not have more than one coinbase.
	for i, tx := range transactions[1:] {
		if IsCoinBase(tx) {
			str := fmt.Sprintf("block contains second coinbase at index %d", i+1)
			return NewRuleError(ErrMultipleCoinbases, str)
		}
	}

	// Do some preliminary checks on each transaction to ensure they are
	// sane before continuing.
	for _, tx := range transactions {
		err := CheckTransactionSanity(tx)
		if err != nil {
			return err
		}
	}

	// Build merkle tree and ensure the calculated merkle root matches the
	// entry in the block header.  This also has the effect of caching all
	// of the transaction hashes in the block to speed up future Hash
	// checks.  Jaxnetd builds the tree here and checks the merkle root
	// after the following checks, but there is no reason not to check the
	// merkle root matches here.
	merkles := BuildMerkleTreeStore(block.Transactions(), false)
	calculatedMerkleRoot := merkles[len(merkles)-1]
	root := header.MerkleRoot()
	if !root.IsEqual(calculatedMerkleRoot) {
		str := fmt.Sprintf("block merkle root is invalid - block header indicates %v, but calculated value is %v",
			header.MerkleRoot(), calculatedMerkleRoot)
		return NewRuleError(ErrBadMerkleRoot, str)
	}

	// Check for duplicate transactions.  This check will be fairly quick
	// since the transaction hashes are already cached due to building the
	// merkle tree above.
	existingTxHashes := make(map[chainhash.Hash]struct{})
	for _, tx := range transactions {
		hash := tx.Hash()
		if _, exists := existingTxHashes[*hash]; exists {
			str := fmt.Sprintf("block contains duplicate transaction %v", hash)
			return NewRuleError(ErrDuplicateTx, str)
		}
		existingTxHashes[*hash] = struct{}{}
	}

	// The number of signature operations must be less than the maximum
	// allowed per block.
	totalSigOps := 0
	for _, tx := range transactions {
		// We could potentially overflow the accumulator so check for
		// overflow.
		lastSigOps := totalSigOps
		totalSigOps += (CountSigOps(tx) * WitnessScaleFactor)

		if totalSigOps < lastSigOps || totalSigOps > MaxBlockSigOpsCost {
			str := fmt.Sprintf("block contains too many signature operations - got %v, max %v",
				totalSigOps, MaxBlockSigOpsCost)
			return NewRuleError(ErrTooManySigOps, str)
		}
	}

	return nil
}

// CheckBlockSanity performs some preliminary checks on a block to ensure it is
// sane before continuing with block processing.  These checks are context free.
func CheckBlockSanity(block *jaxutil.Block, chainParams *chaincfg.Params, timeSource MedianTimeSource) error {
	return CheckBlockSanityWF(block, chainParams, timeSource, BFNone)
}

// ExtractCoinbaseHeight attempts to extract the height of the block from the
// scriptSig of a coinbase transaction.  Coinbase heights are only present in
// blocks of version 2 or later.  This was added as part of BIP0034.
func ExtractCoinbaseHeight(coinbaseTx *jaxutil.Tx) (int32, error) {
	sigScript := coinbaseTx.MsgTx().TxIn[0].SignatureScript
	if len(sigScript) < 1 {
		str := "the coinbase signature script for blocks of " +
			"version %d or greater must start with the " +
			"length of the serialized block height"
		str = fmt.Sprintf(str, serializedHeightVersion)
		return 0, NewRuleError(ErrMissingCoinbaseHeight, str)
	}

	// Detect the case when the block height is a small integer encoded with
	// as single byte.
	opcode := int(sigScript[0])
	if opcode == txscript.OP_0 {
		return 0, nil
	}
	if opcode >= txscript.OP_1 && opcode <= txscript.OP_16 {
		return int32(opcode - (txscript.OP_1 - 1)), nil
	}

	// Otherwise, the opcode is the length of the following bytes which
	// encode in the block height.
	serializedLen := int(sigScript[0])
	if len(sigScript[1:]) < serializedLen {
		str := "the coinbase signature script for blocks of " +
			"version %d or greater must start with the " +
			"serialized block height"
		str = fmt.Sprintf(str, serializedLen)
		return 0, NewRuleError(ErrMissingCoinbaseHeight, str)
	}

	serializedHeightBytes := make([]byte, 8)
	copy(serializedHeightBytes, sigScript[1:serializedLen+1])
	serializedHeight := binary.LittleEndian.Uint64(serializedHeightBytes)

	return int32(serializedHeight), nil
}

// CheckSerializedHeight checks if the signature script in the passed
// transaction starts with the serialized block height of wantHeight.
func CheckSerializedHeight(coinbaseTx *jaxutil.Tx, wantHeight int32) error {
	serializedHeight, err := ExtractCoinbaseHeight(coinbaseTx)
	if err != nil {
		return err
	}

	if serializedHeight != wantHeight {
		str := fmt.Sprintf("the coinbase signature script serialized "+
			"block height is %d when %d was expected",
			serializedHeight, wantHeight)
		return NewRuleError(ErrBadCoinbaseHeight, str)
	}
	return nil
}

// CheckTransactionInputs performs a series of checks on the inputs to a
// transaction to ensure they are valid.  An example of some of the checks
// include verifying all inputs exist, ensuring the coinbase seasoning
// requirements are met, detecting double spends, validating all values and fees
// are in the legal range and the total output amount doesn't exceed the input
// amount, and verifying the signatures to prove the spender was the owner of
// the bitcoins and therefore allowed to spend them.  As it checks the inputs,
// it also calculates the total fees for the transaction and returns that value.
//
// NOTE: The transaction MUST have already been sanity checked with the
// CheckTransactionSanity function prior to calling this function.
func CheckTransactionInputs(tx *jaxutil.Tx, txHeight int32, utxoView *UtxoViewpoint, chainParams *chaincfg.Params) (int64, error) {
	// Coinbase transactions have no inputs.
	if IsCoinBase(tx) {
		return 0, nil
	}

	var (
		totalSatoshiIn int64
		txHash         = tx.Hash()
		missedInputs   = map[int]struct{}{}
		thisIsSwapTx   = tx.MsgTx().SwapTx()
	)

	for txInIndex, txIn := range tx.MsgTx().TxIn {
		// Ensure the referenced input transaction is available.
		utxo := utxoView.LookupEntry(txIn.PreviousOutPoint)
		if utxo == nil && thisIsSwapTx {
			missedInputs[txInIndex] = struct{}{}
			continue
		}

		if utxo == nil || utxo.IsSpent() {
			str := fmt.Sprintf(
				"output %v referenced from transaction %s:%d either does not exist or has already been spent",
				txIn.PreviousOutPoint,
				tx.Hash(), txInIndex)
			return 0, NewRuleError(ErrMissingTxOut, str)
		}

		// Ensure the transaction is not spending coins which have not
		// yet reached the required coinbase maturity.
		if utxo.IsCoinBase() {
			originHeight := utxo.BlockHeight()
			blocksSincePrev := txHeight - originHeight
			coinbaseMaturity := int32(chainParams.CoinbaseMaturity)
			if blocksSincePrev < coinbaseMaturity {
				str := fmt.Sprintf("tried to spend coinbase "+
					"transaction output %v from height %v "+
					"at height %v before required maturity "+
					"of %v blocks", txIn.PreviousOutPoint,
					originHeight, txHeight,
					coinbaseMaturity)
				return 0, NewRuleError(ErrImmatureSpend, str)
			}
		}

		txIn.Age = txHeight - utxo.BlockHeight()

		if thisIsSwapTx {
			err := ValidateSwapTxStructure(tx.MsgTx(), len(missedInputs))
			if err != nil {
				return 0, err
			}
		}

		// Ensure the transaction amounts are in range.  Each of the
		// output values of the input transactions must not be negative
		// or more than the max allowed per transaction.  All amounts in
		// a transaction are in a unit value known as a satoshi.  One
		// bitcoin is a quantity of satoshi as defined by the
		// SatoshiPerBitcoin constant.
		originTxSatoshi := utxo.Amount()
		if originTxSatoshi < 0 {
			str := fmt.Sprintf("transaction output has negative "+
				"value of %v", jaxutil.Amount(originTxSatoshi))
			return 0, NewRuleError(ErrBadTxOutValue, str)
		}
		if originTxSatoshi > jaxutil.MaxSatoshi {
			str := fmt.Sprintf("transaction output value of %v is "+
				"higher than max allowed value of %v",
				jaxutil.Amount(originTxSatoshi),
				jaxutil.MaxSatoshi)
			return 0, NewRuleError(ErrBadTxOutValue, str)
		}

		// The total of all outputs must not be more than the max
		// allowed per transaction.  Also, we could potentially overflow
		// the accumulator so check for overflow.
		lastSatoshiIn := totalSatoshiIn
		totalSatoshiIn += originTxSatoshi
		if totalSatoshiIn < lastSatoshiIn ||
			totalSatoshiIn > jaxutil.MaxSatoshi {
			str := fmt.Sprintf("total value of all transaction "+
				"inputs is %v which is higher than max "+
				"allowed value of %v", totalSatoshiIn,
				jaxutil.MaxSatoshi)
			return 0, NewRuleError(ErrBadTxOutValue, str)
		}
	}

	// Calculate the total output amount for this transaction.  It is safe
	// to ignore overflow and out of range errors here because those error
	// conditions would have already been caught by checkTransactionSanity.
	var totalSatoshiOut int64
	for outIndex, txOut := range tx.MsgTx().TxOut {
		// for swap txs, the missed inputs are inputs from another chain,
		// outputs with matching indices are not added to the current chain and should be ignored
		if _, ok := missedInputs[outIndex]; ok {
			continue
		}
		totalSatoshiOut += txOut.Value
	}

	// Ensure the transaction does not spend more than its inputs.
	if totalSatoshiIn < totalSatoshiOut && !thisIsSwapTx { // todo(mike):
		str := fmt.Sprintf("total value of all transaction inputs for "+
			"transaction %v is %v which is less than the amount "+
			"spent of %v", txHash, totalSatoshiIn, totalSatoshiOut)
		return 0, NewRuleError(ErrSpendTooHigh, str)
	}

	// NOTE: jaxnetd checks if the transaction fees are < 0 here, but that
	// is an impossible condition because of the check above that ensures
	// the inputs are >= the outputs.
	txFeeInSatoshi := totalSatoshiIn - totalSatoshiOut
	return txFeeInSatoshi, nil
}

// ValidateSwapTxStructure validates formats of the cross shard swap tx.
// wire.TxVerCrossShardSwap transaction is a special tx for atomic swap between chains.
// It can contain only TWO or FOUR inputs and TWO or FOUR outputs.
// TxIn and TxOut are strictly associated with each other by index.
// One pair corresponds to the current chain. The second is for another, unknown chain.
//
// | # | --- []TxIn ----- | --- | --- []TxOut ----- | # |
// | - | ---------------- | --- | ----------------- | - |
// | 0 | TxIn_0 ∈ Shard_X | --> | TxOut_0 ∈ Shard_X | 0 |
// | 1 | TxIn_1 ∈ Shard_X | --> | TxOut_1 ∈ Shard_X | 1 |
// | 2 | TxIn_2 ∈ Shard_Y | --> | TxOut_2 ∈ Shard_Y | 2 |
// | 3 | TxIn_3 ∈ Shard_Y | --> | TxOut_3 ∈ Shard_Y | 3 |
//
// The order is not deterministic.
func ValidateSwapTxStructure(tx *wire.MsgTx, missedUTXO int) error {
	inLen := len(tx.TxIn)
	outLen := len(tx.TxOut)
	if inLen != outLen || (inLen != 2 && inLen != 4) {
		str := fmt.Sprintf("cross shard swap tx has invalid format: [%d]TxIn, [%d]TxOut",
			inLen, outLen)
		return NewRuleError(ErrInvalidShardSwapInOuts, str)
	}

	// this is a special case for callers
	// who have no information about missing UTXOs within their context.
	if missedUTXO == -1 {
		return nil
	}

	if missedUTXO > inLen/2 {
		str := fmt.Sprintf(
			"cross shard swap tx contains too many unknown inputs: [%d]TxIn, [%d]TxOut, [%d] missed",
			inLen, outLen, missedUTXO)
		return NewRuleError(ErrInvalidShardSwapInOuts, str)
	}
	return nil
}
