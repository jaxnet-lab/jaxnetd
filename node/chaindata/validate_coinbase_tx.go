/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaindata

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/jaxutil/bch"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// CoinbaseFlags is added to the coinbase script of a generated block
// and is used to monitor BIP16 support as well as blocks that are
// generated via jaxnetd.
const CoinbaseFlags = "/P2SH/jaxnetd/"

const JaxnetScriptSigMarker = "6a61786e6574" // "jaxnet" in hex
var JaxnetScriptSigMarkerBytes = []byte{0x6a, 0x61, 0x78, 0x6e, 0x65, 0x74}

// StandardCoinbaseScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.  In particular,
// it starts with the block Height that is required by version 2 blocks and adds
// the extra nonce as well as additional coinbase flags.
func StandardCoinbaseScript(nextBlockHeight int32, shardID uint32, extraNonce uint64) ([]byte, error) {
	return txscript.NewScriptBuilder().
		AddInt64(int64(nextBlockHeight)).
		AddInt64(int64(shardID)).
		AddInt64(int64(extraNonce)).
		AddData([]byte(CoinbaseFlags)).
		Script()
}

// BTCCoinbaseScript returns a standard script suitable for use as the
// signature script of the coinbase transaction of a new block.  In particular,
// it starts with the block Height that is required by version 2 blocks and adds
// the extra nonce as well as additional coinbase flags.
func BTCCoinbaseScript(nextBlockHeight int64, extraNonce, beaconHash []byte) ([]byte, error) {
	return txscript.NewScriptBuilder().
		AddInt64(nextBlockHeight).
		AddData(extraNonce).
		AddData(JaxnetScriptSigMarkerBytes).
		AddData(beaconHash).
		AddData(JaxnetScriptSigMarkerBytes).
		AddData([]byte(CoinbaseFlags)).
		Script()
}

// CreateCoinbaseTx returns a coinbase transaction paying an appropriate subsidy
// based on the passed block Height to the provided address.  When the address
// is nil, the coinbase transaction will instead be redeemable by anyone.
//
// See the comment for NewBlockTemplate for more information about why the nil
// address handling is useful.
func CreateCoinbaseTx(value int64, nextHeight int32, shardID uint32, addr jaxutil.Address) (*jaxutil.Tx, error) {
	extraNonce := uint64(0)
	coinbaseScript, err := StandardCoinbaseScript(nextHeight, shardID, extraNonce)
	if err != nil {
		return nil, err
	}

	// Create the script to pay to the provided payment address if one was
	// specified.  Otherwise create a script that allows the coinbase to be
	// redeemable by anyone.
	var pkScript []byte
	if addr != nil {
		var err error
		pkScript, err = txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		scriptBuilder := txscript.NewScriptBuilder()
		pkScript, err = scriptBuilder.AddOp(txscript.OP_TRUE).Script()
		if err != nil {
			return nil, err
		}
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		// Coinbase transactions have no inputs, so previous outpoint is
		// zero hash and max index.
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex),
		SignatureScript:  coinbaseScript,
		Sequence:         wire.MaxTxInSequenceNum,
	})

	tx.AddTxOut(&wire.TxOut{Value: value, PkScript: pkScript})
	return jaxutil.NewTx(tx), nil
}

func CreateBitcoinCoinbaseTx(value, fee int64, nextHeight int32, addr jaxutil.Address, beaconHash []byte, burnReward bool) (*jaxutil.Tx, error) {
	btcCoinbase, err := CreateJaxCoinbaseTx(value, fee, nextHeight, 0, addr, burnReward, false)
	if err != nil {
		return nil, err
	}
	btcCoinbase.MsgTx().TxIn[0].SignatureScript, err = BTCCoinbaseScript(int64(nextHeight), make([]byte, 8), beaconHash)
	return btcCoinbase, err
}

func CreateJaxCoinbaseTx(value, fee int64, height int32,
	shardID uint32, addr jaxutil.Address, burnReward, beacon bool) (*jaxutil.Tx, error) {
	return CreateJaxCoinbaseTxWithBurn(value, fee, height, shardID, addr, burnReward, beacon, types.RawJaxBurnScript)
}

func CreateJaxCoinbaseTxWithBurn(value, fee int64, height int32,
	shardID uint32, addr jaxutil.Address, burnReward, beacon bool, jaxBurnScript []byte) (*jaxutil.Tx, error) {
	extraNonce := uint64(0)
	coinbaseScript, err := StandardCoinbaseScript(height, shardID, extraNonce)
	if err != nil {
		return nil, err
	}

	feeAddress, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return nil, err
	}

	pkScript := feeAddress
	if burnReward {
		pkScript = jaxBurnScript
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		// Coinbase transactions have no inputs, so previous outpoint is
		// zero hash and max index.
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex),
		SignatureScript:  coinbaseScript,
		Sequence:         wire.MaxTxInSequenceNum,
	})

	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: jaxBurnScript})
	if beacon {
		switch addr.(type) {
		case *jaxutil.AddressPubKey:
		case *jaxutil.AddressPubKeyHash:
		case *jaxutil.AddressScriptHash:
		default:
			return nil, errors.New("miner address must be *AddressPubKeyHash, *AddressPubKey or *AddressScriptHash")
		}

		const baseReward = chaincfg.BeaconBaseReward * int64(chaincfg.HaberStornettaPerJAXNETCoin)
		lockScript, err := txscript.HTLCScript(addr, chaincfg.BeaconRewardLockPeriod)
		if err != nil {
			return nil, err
		}

		tx.AddTxOut(&wire.TxOut{Value: baseReward, PkScript: pkScript})
		tx.AddTxOut(&wire.TxOut{Value: value - baseReward, PkScript: lockScript})
	} else {
		tx.AddTxOut(&wire.TxOut{Value: value, PkScript: pkScript})
	}

	tx.AddTxOut(&wire.TxOut{Value: fee, PkScript: feeAddress})

	return jaxutil.NewTx(tx), nil
}

func validateCoinbaseAux(merkleRoot chainhash.Hash, aux *wire.CoinbaseAux) error {
	coinbaseHash := aux.Tx.TxHash()
	if !chainhash.ValidateCoinbaseMerkleTreeProof(coinbaseHash, aux.TxMerkleProof, merkleRoot) {
		return errors.New("tx_merkle tree root is not match with blockMerkle root")
	}

	return nil
}

func checkBtcVanityAddress(script []byte) bool {
	_, address, _, _ := txscript.ExtractPkScriptAddrs(script, &chaincfg.MainNetParams)
	for _, addr := range address {
		if strings.HasPrefix(addr.EncodeAddress(), "1JAX") {
			return true
		}
	}
	return false
}

// nolint: gomnd
func ValidateBTCCoinbase(aux *wire.BTCBlockAux) (bool, error) {
	if err := validateCoinbaseAux(aux.MerkleRoot, &aux.CoinbaseAux); err != nil {
		return false, errors.Wrap(err, "invalid btc coinbase aux")
	}
	const errMsg = "invalid format of btc aux coinbase tx: "

	var (
		err                    error
		btcCoinbaseTx          = aux.CoinbaseAux.Tx
		strictLenForTypeB      = len(btcCoinbaseTx.TxOut) == 3 || len(btcCoinbaseTx.TxOut) == 4
		jaxnetMarkerOutPresent = btcCoinbaseTx.TxOut[0].Value == 0 &&
			jaxutil.IsJaxnetBurnRawAddress(btcCoinbaseTx.TxOut[0].PkScript)
		vanityAddressPresent = jaxnetMarkerOutPresent
	)

	if !jaxnetMarkerOutPresent {
		for _, out := range btcCoinbaseTx.TxOut {
			vanityAddressPresent = checkBtcVanityAddress(out.PkScript) || bch.JaxVanityPrefix(out.PkScript)
			if vanityAddressPresent {
				break
			}
		}
	}

	if !strictLenForTypeB {
		// invalid TYPE_A scheme
		if len(btcCoinbaseTx.TxOut) > 7 {
			return false, errors.New("must have less than 7 outputs")
		}

		// invalid TYPE_A scheme
		if !vanityAddressPresent {
			return false, errors.New("at least one out must start with 1JAX... or bitcoincash:qqjax... ")
		}

		// valid TYPE_A scheme
		return false, nil
	}

	var (
		hugeReward      = btcCoinbaseTx.TxOut[1].Value > 6_2500_0000
		normalReward    = btcCoinbaseTx.TxOut[1].Value == 6_2500_0000
		smallReward     = btcCoinbaseTx.TxOut[1].Value < 6_2500_0000
		normalFee       = btcCoinbaseTx.TxOut[2].Value <= 5000_0000
		hugeFee         = btcCoinbaseTx.TxOut[2].Value > 5000_0000
		rewardBurned    = jaxutil.IsJaxnetBurnRawAddress(btcCoinbaseTx.TxOut[1].PkScript)
		validWitnessOut = len(btcCoinbaseTx.TxOut) == 3
	)

	if len(btcCoinbaseTx.TxOut) == 4 {
		nullData := txscript.GetScriptClass(btcCoinbaseTx.TxOut[3].PkScript) == txscript.NullDataTy
		validWitnessOut = nullData && btcCoinbaseTx.TxOut[3].Value == 0
	}

	//	this is valid TYPE_B and TYPE_C scheme.
	if jaxnetMarkerOutPresent && (normalReward || (smallReward && normalFee)) && validWitnessOut {
		return rewardBurned, nil
	}

	if jaxnetMarkerOutPresent && hugeReward {
		err = errors.New(errMsg + "reward greater than 6.25 BTC")
		return false, err
	}

	if jaxnetMarkerOutPresent && smallReward && hugeFee {
		err = errors.New(errMsg + "fee greater than 0.5 BTC")
		return false, err
	}

	if jaxnetMarkerOutPresent && !validWitnessOut {
		err = errors.New(errMsg + "4th out can be only NULL_DATA with zero amount")
		return false, err
	}

	// additional valid TYPE_A scheme
	if !jaxnetMarkerOutPresent && vanityAddressPresent {
		return false, nil
	}

	if !jaxnetMarkerOutPresent {
		err = errors.New(errMsg + "first out must be zero and have JaxNetLink")
		return false, err
	}

	return false, errors.New(errMsg + "unexpected scheme")
}

func validateProofOfInclusion(beaconExclusiveHash chainhash.Hash, signatureScript []byte) bool {
	var markerWithLen = make([]byte, 0, 47)
	markerWithLen = append(markerWithLen,
		0x06,                               // len of marker
		0x6a, 0x61, 0x78, 0x6e, 0x65, 0x74, // JaxnetScriptSigMarker
		0x20, // len of hash - 32 bytes
	)
	markerWithLen = append(markerWithLen, beaconExclusiveHash[:]...)
	markerWithLen = append(markerWithLen,
		0x06,                               // len of marker
		0x6a, 0x61, 0x78, 0x6e, 0x65, 0x74, // marker
	)

	return bytes.Contains(signatureScript, markerWithLen)
}

// nolint: gomnd
func ValidateBeaconCoinbase(aux *wire.BeaconHeader, coinbase *wire.MsgTx, expectedReward int64) (bool, error) {
	btcBurnReward, err := ValidateBTCCoinbase(aux.BTCAux())
	if err != nil {
		return false, errors.Wrap(err, "invalid btc aux")
	}

	exclusiveHash := aux.BeaconExclusiveHash()
	hashPresent := validateProofOfInclusion(exclusiveHash,
		aux.BTCAux().CoinbaseAux.Tx.TxIn[0].SignatureScript)
	if !hashPresent {
		return false, fmt.Errorf("bitcoin coinbase must include beacon exclusive hash(%s)", exclusiveHash)
	}

	const errMsg = "invalid format of beacon coinbase tx: "

	if len(coinbase.TxOut) < 4 || len(coinbase.TxOut) > 5 {
		err = errors.New(errMsg + "must have 4 or 5 outs")
		return false, err
	}

	if len(coinbase.TxOut) == 5 {
		nullData := txscript.GetScriptClass(coinbase.TxOut[4].PkScript) == txscript.NullDataTy
		if coinbase.TxOut[4].Value != 0 || !nullData {
			err = errors.New(errMsg + "5th out can be only NULL_DATA with zero amount")
			return false, err
		}
	}

	jaxNetLinkOut := jaxutil.IsJaxnetBurnRawAddress(coinbase.TxOut[0].PkScript) &&
		coinbase.TxOut[0].Value == 0
	if !jaxNetLinkOut {
		err = errors.New(errMsg + "first out must be zero and have JaxNetLink")
		return false, err
	}

	lockScript, err := txscript.ParsePkScript(coinbase.TxOut[2].PkScript)
	if err != nil || lockScript.Class() != txscript.HTLCScriptTy {
		err = errors.New(errMsg + "third output must be lock-script")
		return false, err
	}

	lockTime, err := txscript.ExtractHTLCLockTime(coinbase.TxOut[2].PkScript)
	if err != nil || lockTime != chaincfg.BeaconRewardLockPeriod {
		return false, fmt.Errorf("%s third output must be lock-script for %d blocks",
			errMsg, chaincfg.BeaconRewardLockPeriod)
	}

	jxnBurnReward := jaxutil.IsJaxnetBurnRawAddress(coinbase.TxOut[1].PkScript)

	if btcBurnReward && !jxnBurnReward {
		err = errors.New(errMsg + "BTC burned, JaxNet reward prohibited")
		return false, err
	}
	if !btcBurnReward && jxnBurnReward {
		err = errors.New(errMsg + "BTC not burned, JaxNet burn prohibited")
		return false, err
	}
	const baseReward = chaincfg.BeaconBaseReward * int64(chaincfg.HaberStornettaPerJAXNETCoin)

	if expectedReward == -1 {
		if coinbase.TxOut[1].Value != baseReward {
			err = fmt.Errorf(errMsg+"invalid value of second out - has(%d) expected(%d)",
				coinbase.TxOut[1].Value, baseReward)
			return false, err
		}

		return jxnBurnReward, nil
	}

	properReward := coinbase.TxOut[1].Value == baseReward &&
		coinbase.TxOut[2].Value == expectedReward-baseReward

	if !properReward {
		err = fmt.Errorf(errMsg+"invalid value of second out - has(%d, %d) expected(%d)",
			coinbase.TxOut[1].Value, coinbase.TxOut[2].Value, expectedReward)
		return false, err
	}

	return jxnBurnReward, nil
}

func ValidateShardCoinbase(shardHeader *wire.ShardHeader, shardCoinbaseTx *wire.MsgTx, expectedReward int64) error {
	if err := validateCoinbaseAux(shardHeader.BeaconHeader().MerkleRoot(), shardHeader.BeaconCoinbaseAux()); err != nil {
		return errors.Wrap(err, "invalid beacon coinbase aux")
	}

	const errMsg = "invalid format of shard coinbase tx: "

	if len(shardCoinbaseTx.TxOut) < 3 || len(shardCoinbaseTx.TxOut) > 4 {
		return errors.New(errMsg + "must have 3 or 4 outs")
	}

	shardJaxNetLinkOut := jaxutil.IsJaxnetBurnRawAddress(shardCoinbaseTx.TxOut[0].PkScript) &&
		shardCoinbaseTx.TxOut[0].Value == 0
	if !shardJaxNetLinkOut {
		return errors.New(errMsg + "first out must be zero and have JaxNetLink")
	}

	if len(shardCoinbaseTx.TxOut) == 4 {
		nullData := txscript.GetScriptClass(shardCoinbaseTx.TxOut[3].PkScript) == txscript.NullDataTy
		if shardCoinbaseTx.TxOut[3].Value != 0 || !nullData {
			err := errors.New(errMsg + "4th out can be only NULL_DATA with zero amount")
			return err
		}
	}

	shardReward := shardCoinbaseTx.TxOut[1].Value

	if shardReward != expectedReward {
		return fmt.Errorf("%svalue(%d) of second output not eq to expected reward(%d); mmn(%d) k(%d) ", errMsg,
			shardReward, expectedReward, shardHeader.BeaconHeader().MergeMiningNumber(), shardHeader.K())
	}

	beaconCoinbase := shardHeader.BeaconCoinbaseAux().Tx

	beaconBurned, err := ValidateBeaconCoinbase(shardHeader.BeaconHeader(), &beaconCoinbase, -1)
	if err != nil {
		return errors.Wrap(err, "invalid beacon aux")
	}

	shardJaxBurnReward := jaxutil.IsJaxnetBurnRawAddress(shardCoinbaseTx.TxOut[1].PkScript)
	if shardJaxBurnReward {
		return nil
	}

	if !beaconBurned && !shardJaxBurnReward {
		return errors.New(errMsg + "BTC & JaxNet not burned, Jax reward prohibited")
	}

	return nil
}
