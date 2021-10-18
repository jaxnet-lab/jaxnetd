/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaindata

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
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
// it starts with the block height that is required by version 2 blocks and adds
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
// it starts with the block height that is required by version 2 blocks and adds
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
// based on the passed block height to the provided address.  When the address
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
	extraNonce := uint64(0)
	coinbaseScript, err := StandardCoinbaseScript(height, shardID, extraNonce)
	if err != nil {
		return nil, err
	}

	feeAddress, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return nil, err
	}

	jaxBurnAddr, err := jaxutil.DecodeAddress(types.JaxBurnAddr, &chaincfg.MainNetParams)
	if err != nil {
		return nil, err
	}
	jaxBurn := jaxBurnAddr.ScriptAddress()

	var pkScript = feeAddress
	if burnReward {
		pkScript = jaxBurn
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		// Coinbase transactions have no inputs, so previous outpoint is
		// zero hash and max index.
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{}, wire.MaxPrevOutIndex),
		SignatureScript:  coinbaseScript,
		Sequence:         wire.MaxTxInSequenceNum,
	})

	tx.AddTxOut(&wire.TxOut{Value: 0, PkScript: jaxBurn})
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

func ValidateBTCCoinbase(aux *wire.BTCBlockAux) (rewardBurned bool, err error) {
	if err := validateCoinbaseAux(aux.MerkleRoot, &aux.CoinbaseAux); err != nil {
		return false, errors.Wrap(err, "invalid btc coinbase aux")
	}
	btcCoinbaseTx := aux.CoinbaseAux.Tx
	if len(btcCoinbaseTx.TxOut) != 3 {
		if !jaxutil.BtcJaxVanityPrefix(btcCoinbaseTx.TxOut[0].PkScript) &&
			!jaxutil.BchJaxPrefix(btcCoinbaseTx.TxOut[0].PkScript) {
			return false, errors.New("first out must start with 1JAX... or bitcoincash:qqjax... ")
		}

		return false, nil
	}

	const errMsg = "invalid format of btc aux coinbase tx: "

	btcJaxNetLinkOut := jaxutil.IsJaxnetBurnRawAddress(btcCoinbaseTx.TxOut[0].PkScript) &&
		btcCoinbaseTx.TxOut[0].Value == 0

	if !btcJaxNetLinkOut {
		err = errors.New(errMsg + "first out must be zero and have JaxNetLink")
		return false, err
	}

	if btcCoinbaseTx.TxOut[1].Value > 6_2500_0000 {
		err = errors.New(errMsg + "reward greater than 6.25 BTC")
		return false, err
	}

	if btcCoinbaseTx.TxOut[1].Value < 6_2500_0000 {
		if btcCoinbaseTx.TxOut[2].Value > 5000_0000 {
			err = errors.New(errMsg + "fee greater than 0.5 BTC")
			return false, err
		}
	}

	rewardBurned = jaxutil.IsJaxnetBurnRawAddress(btcCoinbaseTx.TxOut[1].PkScript)

	return rewardBurned, nil
}

func ValidateBeaconCoinbase(aux *wire.BeaconHeader, coinbase *wire.MsgTx, expectedReward int64) (bool, error) {
	btcBurnReward, err := ValidateBTCCoinbase(aux.BTCAux())
	if err != nil {
		return false, errors.Wrap(err, "invalid btc aux")
	}

	beaconExclusiveHash := aux.BeaconExclusiveHash()
	exclusiveHash := hex.EncodeToString(beaconExclusiveHash.CloneBytes())
	asmString, _ := txscript.DisasmString(aux.BTCAux().CoinbaseAux.Tx.TxIn[0].SignatureScript)

	hashPresent := false
	chunks := strings.Split(asmString, " ")
	for i, chunk := range chunks {
		if chunk != JaxnetScriptSigMarker {
			continue
		}

		if len(chunks) < i+2 {
			break
		}

		hashPresent = (chunks[i+1] == exclusiveHash) && (chunks[i+2] == JaxnetScriptSigMarker)
		break
	}

	if !hashPresent {
		return false, errors.New("bitcoin coinbase must include beacon exclusive hash")
	}

	const errMsg = "invalid format of beacon coinbase tx: "

	if len(coinbase.TxOut) != 4 {
		err = errors.New(errMsg + "must have 4 outs")
		return false, err
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
	} else {
		lockTime, err := txscript.ExtractHTLCLockTime(coinbase.TxOut[2].PkScript)
		if err != nil || lockTime != chaincfg.BeaconRewardLockPeriod {
			return false, fmt.Errorf("%s third output must be lock-script for %d blocks",
				errMsg, chaincfg.BeaconRewardLockPeriod)
		}
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

	if len(shardCoinbaseTx.TxOut) < 3 {
		return errors.New(errMsg + "less than 3 out")
	}

	shardJaxNetLinkOut := jaxutil.IsJaxnetBurnRawAddress(shardCoinbaseTx.TxOut[0].PkScript) &&
		shardCoinbaseTx.TxOut[0].Value == 0
	if !shardJaxNetLinkOut {
		return errors.New(errMsg + "first out must be zero and have JaxNetLink")
	}

	shardReward := shardCoinbaseTx.TxOut[1].Value

	if shardReward != expectedReward {
		return errors.New(errMsg + "value of second output not eq to expected reward")
	}

	shardJaxBurnReward := jaxutil.IsJaxnetBurnRawAddress(shardCoinbaseTx.TxOut[1].PkScript)
	if shardJaxBurnReward {
		return nil
	}

	beaconCoinbase := shardHeader.BeaconCoinbaseAux().Tx
	beaconBurned, err := ValidateBeaconCoinbase(shardHeader.BeaconHeader(), &beaconCoinbase, -1)
	if err != nil {
		return errors.Wrap(err, "invalid beacon aux")
	}

	if !beaconBurned && !shardJaxBurnReward {
		return errors.New(errMsg + "BTC & JaxNet not burned, Jax reward prohibited")
	}

	return nil
}
