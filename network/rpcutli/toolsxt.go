// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package rpcutli

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain"
	"gitlab.com/jaxnet/core/shard.core/node/chaindata"
	"gitlab.com/jaxnet/core/shard.core/txscript"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/pow"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

// MaxProtocolVersion is the max protocol version the server supports.
const MaxProtocolVersion = 70002

type ToolsXt struct{}

// MessageToHex serializes a message to the wire protocol encoding using the
// latest protocol version and returns a hex-encoded string of the result.
func (xt ToolsXt) MessageToHex(msg wire.Message) (string, error) {
	var buf bytes.Buffer
	if err := msg.BtcEncode(&buf, MaxProtocolVersion, wire.WitnessEncoding); err != nil {
		context := errors.Wrapf(err, "Failed to encode msg of type %T", msg)
		return "", btcjson.NewRPCError(btcjson.ErrRPCInternal.Code, context.Error())
	}
	return hex.EncodeToString(buf.Bytes()), nil
}

// WitnessToHex formats the passed witness stack as a slice of hex-encoded
// strings to be used in a JSON response.
func (xt ToolsXt) WitnessToHex(witness wire.TxWitness) []string {
	// Ensure nil is returned when there are no entries versus an empty
	// slice so it can properly be omitted as necessary.
	if len(witness) == 0 {
		return nil
	}

	result := make([]string, 0, len(witness))
	for _, wit := range witness {
		result = append(result, hex.EncodeToString(wit))
	}

	return result
}

// CreateVinList returns a slice of JSON objects for the inputs of the passed
// transaction.
func (xt ToolsXt) CreateVinList(mtx *wire.MsgTx, age int32) []btcjson.Vin {
	// Coinbase transactions only have a single txin by definition.
	vinList := make([]btcjson.Vin, len(mtx.TxIn))
	if chaindata.IsCoinBaseTx(mtx) {
		txIn := mtx.TxIn[0]
		vinList[0].Coinbase = hex.EncodeToString(txIn.SignatureScript)
		vinList[0].Sequence = txIn.Sequence
		vinList[0].Witness = xt.WitnessToHex(txIn.Witness)
		vinList[0].Age = txIn.Age
		return vinList
	}

	for i, txIn := range mtx.TxIn {
		// The disassembled string will contain [error] inline
		// if the script doesn't fully parse, so ignore the
		// error here.
		disbuf, _ := txscript.DisasmString(txIn.SignatureScript)

		vinEntry := &vinList[i]
		vinEntry.Txid = txIn.PreviousOutPoint.Hash.String()
		vinEntry.Vout = txIn.PreviousOutPoint.Index
		vinEntry.Sequence = txIn.Sequence
		vinEntry.ScriptSig = &btcjson.ScriptSig{
			Asm: disbuf,
			Hex: hex.EncodeToString(txIn.SignatureScript),
		}
		vinEntry.Age = txIn.Age
		if txIn.Age == 0 {
			vinEntry.Age = age
		}

		if mtx.HasWitness() {
			vinEntry.Witness = xt.WitnessToHex(txIn.Witness)
		}
	}

	return vinList
}

// CreateVoutList returns a slice of JSON objects for the outputs of the passed
// transaction.
func (xt ToolsXt) CreateVoutList(mtx *wire.MsgTx, chainParams *chaincfg.Params, filterAddrMap map[string]struct{}) []btcjson.Vout {
	voutList := make([]btcjson.Vout, 0, len(mtx.TxOut))
	for i, v := range mtx.TxOut {
		// The disassembled string will contain [error] inline if the
		// script doesn't fully parse, so ignore the error here.
		disbuf, _ := txscript.DisasmString(v.PkScript)

		// Ignore the error here since an error means the script
		// couldn't parse and there is no additional information about
		// it anyways.
		scriptClass, addrs, reqSigs, _ := txscript.ExtractPkScriptAddrs(
			v.PkScript, chainParams)

		// Encode the addresses while checking if the address passes the
		// filter when needed.
		passesFilter := len(filterAddrMap) == 0
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

		if !passesFilter {
			continue
		}

		var vout btcjson.Vout
		vout.N = uint32(i)
		vout.Value = btcutil.Amount(v.Value).ToBTC()
		vout.ScriptPubKey.Addresses = encodedAddrs
		vout.ScriptPubKey.Asm = disbuf
		vout.ScriptPubKey.Hex = hex.EncodeToString(v.PkScript)
		vout.ScriptPubKey.Type = scriptClass.String()
		vout.ScriptPubKey.ReqSigs = int32(reqSigs)

		voutList = append(voutList, vout)
	}

	return voutList
}

// CreateTxRawResult converts the passed transaction and associated parameters
// to a raw transaction JSON object.
func (xt *ToolsXt) CreateTxRawResult(chainParams *chaincfg.Params, mtx *wire.MsgTx,
	txHash string, blkHeader wire.BlockHeader, blkHash string,
	blkHeight int32, chainHeight int32) (*btcjson.TxRawResult, error) {

	mtxHex, err := xt.MessageToHex(mtx)
	if err != nil {
		return nil, err
	}

	txReply := &btcjson.TxRawResult{
		Hex:        mtxHex,
		Txid:       txHash,
		ChainName:  chainParams.Name,
		Hash:       mtx.WitnessHash().String(),
		Size:       int32(mtx.SerializeSize()),
		Vsize:      int32(GetTxVirtualSize(btcutil.NewTx(mtx))),
		Weight:     int32(chaindata.GetTransactionWeight(btcutil.NewTx(mtx))),
		Vin:        xt.CreateVinList(mtx, 1+chainHeight-blkHeight),
		Vout:       xt.CreateVoutList(mtx, chainParams, nil),
		InAmount:   0,
		OutAmount:  0,
		Fee:        0,
		Version:    mtx.Version,
		LockTime:   mtx.LockTime,
		CoinbaseTx: chaindata.IsCoinBaseTx(mtx),
		OrphanTx:   blkHeight == -1,
	}

	for _, vout := range mtx.TxOut {
		txReply.OutAmount += vout.Value
	}

	if blkHeader != nil {
		// This is not a typo, they are identical in bitcoind as well.
		txReply.Time = blkHeader.Timestamp().Unix()
		txReply.Blocktime = blkHeader.Timestamp().Unix()
		txReply.BlockHash = blkHash
		txReply.Confirmations = uint64(1 + chainHeight - blkHeight)
	}

	return txReply, nil
}

// GetDifficultyRatio returns the proof-of-work difficulty as a multiple of the
// minimum difficulty using the passed bits field from the header of a block.
func (xt ToolsXt) GetDifficultyRatio(bits uint32, params *chaincfg.Params) (float64, error) {
	// The minimum difficulty is the max possible proof-of-work limit bits
	// converted back to a number.  Note this is not the same as the proof of
	// work limit directly because the block difficulty is encoded in a block
	// with the compact form which loses precision.
	max := pow.CompactToBig(params.PowLimitBits)
	target := pow.CompactToBig(bits)

	difficulty := new(big.Rat).SetFrac(max, target)
	outString := difficulty.FloatString(8)
	diff, err := strconv.ParseFloat(outString, 64)
	if err != nil {
		return 0, err
	}
	return diff, nil
}

// SoftForkStatus converts a ThresholdState state into a human readable string
// corresponding to the particular state.
func (xt ToolsXt) SoftForkStatus(state blockchain.ThresholdState) (string, error) {
	switch state {
	case blockchain.ThresholdDefined:
		return "defined", nil
	case blockchain.ThresholdStarted:
		return "started", nil
	case blockchain.ThresholdLockedIn:
		return "lockedin", nil
	case blockchain.ThresholdActive:
		return "active", nil
	case blockchain.ThresholdFailed:
		return "failed", nil
	default:
		return "", fmt.Errorf("unknown deployment state: %v", state)
	}
}

// EncodeTemplateID encodes the passed details into an ID that can be used to
// uniquely identify a block template.
func (xt ToolsXt) EncodeTemplateID(prevHash *chainhash.Hash, lastGenerated time.Time) string {
	return fmt.Sprintf("%s-%d", prevHash.String(), lastGenerated.Unix())
}

// DecodeTemplateID decodes an ID that is used to uniquely identify a block
// template.  This is mainly used as a mechanism to track when to update clients
// that are using long polling for block templates.  The ID consists of the
// previous block hash for the associated template and the time the associated
// template was generated.
func (xt ToolsXt) DecodeTemplateID(templateID string) (*chainhash.Hash, int64, error) {
	fields := strings.Split(templateID, "-")
	if len(fields) != 2 {
		return nil, 0, errors.New("invalid longpollid format")
	}

	prevHash, err := chainhash.NewHashFromStr(fields[0])
	if err != nil {
		return nil, 0, errors.New("invalid longpollid format")
	}
	lastGenerated, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return nil, 0, errors.New("invalid longpollid format")
	}

	return prevHash, lastGenerated, nil
}

// ChainErrToGBTErrString converts an error returned from btcchain to a string
// which matches the reasons and format described in BIP0022 for rejection
// reasons.
func (xt ToolsXt) ChainErrToGBTErrString(err error) string {
	// When the passed error is not a RuleError, just return a generic
	// rejected string with the error text.
	ruleErr, ok := err.(chaindata.RuleError)
	if !ok {
		return "rejected: " + err.Error()
	}

	switch ruleErr.ErrorCode {
	case chaindata.ErrDuplicateBlock:
		return "duplicate"
	case chaindata.ErrBlockTooBig:
		return "bad-blk-length"
	case chaindata.ErrBlockWeightTooHigh:
		return "bad-blk-weight"
	case chaindata.ErrBlockVersionTooOld:
		return "bad-version"
	case chaindata.ErrInvalidTime:
		return "bad-time"
	case chaindata.ErrTimeTooOld:
		return "time-too-old"
	case chaindata.ErrTimeTooNew:
		return "time-too-new"
	case chaindata.ErrDifficultyTooLow:
		return "bad-diffbits"
	case chaindata.ErrUnexpectedDifficulty:
		return "bad-diffbits"
	case chaindata.ErrHighHash:
		return "high-hash"
	case chaindata.ErrBadMerkleRoot:
		return "bad-txnmrklroot"
	case chaindata.ErrBadCheckpoint:
		return "bad-checkpoint"
	case chaindata.ErrForkTooOld:
		return "fork-too-old"
	case chaindata.ErrCheckpointTimeTooOld:
		return "checkpoint-time-too-old"
	case chaindata.ErrNoTransactions:
		return "bad-txns-none"
	case chaindata.ErrNoTxInputs:
		return "bad-txns-noinputs"
	case chaindata.ErrNoTxOutputs:
		return "bad-txns-nooutputs"
	case chaindata.ErrTxTooBig:
		return "bad-txns-size"
	case chaindata.ErrBadTxOutValue:
		return "bad-txns-outputvalue"
	case chaindata.ErrDuplicateTxInputs:
		return "bad-txns-dupinputs"
	case chaindata.ErrBadTxInput:
		return "bad-txns-badinput"
	case chaindata.ErrMissingTxOut:
		return "bad-txns-missinginput"
	case chaindata.ErrUnfinalizedTx:
		return "bad-txns-unfinalizedtx"
	case chaindata.ErrDuplicateTx:
		return "bad-txns-duplicate"
	case chaindata.ErrOverwriteTx:
		return "bad-txns-overwrite"
	case chaindata.ErrImmatureSpend:
		return "bad-txns-maturity"
	case chaindata.ErrSpendTooHigh:
		return "bad-txns-highspend"
	case chaindata.ErrBadFees:
		return "bad-txns-fees"
	case chaindata.ErrTooManySigOps:
		return "high-sigops"
	case chaindata.ErrFirstTxNotCoinbase:
		return "bad-txns-nocoinbase"
	case chaindata.ErrMultipleCoinbases:
		return "bad-txns-multicoinbase"
	case chaindata.ErrBadCoinbaseScriptLen:
		return "bad-cb-length"
	case chaindata.ErrBadCoinbaseValue:
		return "bad-cb-value"
	case chaindata.ErrMissingCoinbaseHeight:
		return "bad-cb-height"
	case chaindata.ErrBadCoinbaseHeight:
		return "bad-cb-height"
	case chaindata.ErrScriptMalformed:
		return "bad-script-malformed"
	case chaindata.ErrScriptValidation:
		return "bad-script-validate"
	case chaindata.ErrUnexpectedWitness:
		return "unexpected-witness"
	case chaindata.ErrInvalidWitnessCommitment:
		return "bad-witness-nonce-size"
	case chaindata.ErrWitnessCommitmentMismatch:
		return "bad-witness-merkle-match"
	case chaindata.ErrPreviousBlockUnknown:
		return "prev-blk-not-found"
	case chaindata.ErrInvalidAncestorBlock:
		return "bad-prevblk"
	case chaindata.ErrPrevBlockNotBest:
		return "inconclusive-not-best-prvblk"
	}

	return "rejected: " + err.Error()
}

// GetTxVirtualSize computes the virtual size of a given transaction. A
// transaction's virtual size is based off its weight, creating a discount for
// any witness data it contains, proportional to the current
// blockchain.WitnessScaleFactor value.
func GetTxVirtualSize(tx *btcutil.Tx) int64 {
	// vSize := (weight(tx) + 3) / 4
	//       := (((baseSize * 3) + totalSize) + 3) / 4
	// We add 3 here as a way to compute the ceiling of the prior arithmetic
	// to 4. The division by 4 creates a discount for wit witness data.
	return (chaindata.GetTransactionWeight(tx) + (chaindata.WitnessScaleFactor - 1)) /
		chaindata.WitnessScaleFactor
}
