package mining

import (
	"bytes"
	"errors"
	"fmt"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

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

func CreateJaxCoinbaseTx(value, fee int64, nextHeight int32,
	shardID uint32, addr jaxutil.Address, burnReward, beacon bool) (*jaxutil.Tx, error) {
	extraNonce := uint64(0)
	coinbaseScript, err := StandardCoinbaseScript(nextHeight, shardID, extraNonce)
	if err != nil {
		return nil, err
	}

	feeAddress, err := txscript.PayToAddrScript(addr)
	if err != nil {
		feeAddress, _ = txscript.NewScriptBuilder().AddOp(txscript.OP_TRUE).Script()
	}

	jaxBurnAddr, _ := jaxutil.DecodeAddress(types.JaxBurnAddr, &chaincfg.MainNetParams)
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
		const baseReward = chaincfg.BeaconBaseReward * int64(jaxutil.SatoshiPerJAXNETCoin)
		lockScript, err := txscript.HTLCScript(addr, chaincfg.BeaconRewardLockPeriod)
		if err != nil {
			return nil, err
		}

		tx.AddTxOut(&wire.TxOut{Value: baseReward, PkScript: pkScript})
		if burnReward {
			tx.AddTxOut(&wire.TxOut{Value: value - baseReward, PkScript: jaxBurn})
		} else {
			tx.AddTxOut(&wire.TxOut{Value: value - baseReward, PkScript: lockScript})
		}

	} else {
		tx.AddTxOut(&wire.TxOut{Value: value, PkScript: pkScript})
	}
	tx.AddTxOut(&wire.TxOut{Value: fee, PkScript: feeAddress})

	return jaxutil.NewTx(tx), nil
}

func ValidateBTCCoinbase(aux *wire.BTCBlockAux) (rewardBurned bool, err error) {
	if len(aux.Tx.TxOut) != 3 {
		return false, nil
	}

	addr, _ := jaxutil.DecodeAddress(types.JaxBurnAddr, &chaincfg.MainNetParams)
	jaxBurn := addr.ScriptAddress()

	const errMsg = "invalid format of btc aux coinbase tx: "
	btcCoinbaseTx := aux.Tx
	btcJaxNetLinkOut := bytes.Equal(btcCoinbaseTx.TxOut[0].PkScript, jaxBurn) &&
		btcCoinbaseTx.TxOut[0].Value == 0
	if !btcJaxNetLinkOut {
		err = errors.New(errMsg + "first out must be zero and have JaxNetLink")
		return
	}

	if btcCoinbaseTx.TxOut[1].Value > 6_2500_0000 {
		err = errors.New(errMsg + "reward greater than 6.25 BTC")
		return
	}

	if btcCoinbaseTx.TxOut[1].Value < 6_2500_0000 {
		if btcCoinbaseTx.TxOut[1].Value > 5000_0000 {
			err = errors.New(errMsg + "fee greater than 0.5 BTC")
			return
		}
	}

	rewardBurned = bytes.Equal(btcCoinbaseTx.TxOut[1].PkScript, jaxBurn)

	return
}

func ValidateBeaconCoinbase(aux *wire.BeaconHeader, coinbase *wire.MsgTx, expectedReward int64) (rewardBurned bool, err error) {
	btcBurnReward, err := ValidateBTCCoinbase(aux.BTCAux())
	if err != nil {
		return false, err
	}
	const errMsg = "invalid format of beacon coinbase tx: "
	if len(coinbase.TxOut) != 4 {
		err = errors.New(errMsg + "must have 4 outs")
		return false, err
	}

	addr, _ := jaxutil.DecodeAddress(types.JaxBurnAddr, &chaincfg.MainNetParams)
	jaxBurn := addr.ScriptAddress()

	jaxNetLinkOut := bytes.Equal(coinbase.TxOut[0].PkScript, jaxBurn) &&
		coinbase.TxOut[0].Value == 0
	if !jaxNetLinkOut {
		err = errors.New(errMsg + "first out must be zero and have JaxNetLink")
		return false, err
	}

	jaxBurnReward := bytes.Equal(coinbase.TxOut[1].PkScript, jaxBurn)
	if jaxBurnReward && !bytes.Equal(coinbase.TxOut[2].PkScript, jaxBurn) {
		err = errors.New(errMsg + "all reward must be burned")
		return false, err
	}

	if btcBurnReward && !jaxBurnReward {
		err = errors.New(errMsg + "BTC burned, JaxNet reward prohibited")
		return false, err
	}
	if !btcBurnReward && jaxBurnReward {
		err = errors.New(errMsg + "BTC not burned, JaxNet burn prohibited")
		return false, err
	}

	if expectedReward == -1 {
		// skip reward validation if we don't know required reward
		return
	}

	const baseReward = chaincfg.BeaconBaseReward * int64(jaxutil.SatoshiPerJAXNETCoin)

	properReward := coinbase.TxOut[1].Value == baseReward &&
		coinbase.TxOut[2].Value == expectedReward-baseReward

	if !properReward {
		err = fmt.Errorf(errMsg+"invalid value of second out - has(%d) expected(%d)",
			coinbase.TxOut[1].Value, baseReward)
		return false, err
	}

	return jaxBurnReward, nil
}

func ValidateShardCoinbase(shardHeader *wire.ShardHeader, shardCoinbaseTx *wire.MsgTx, expectedReward int64) error {
	addr, _ := jaxutil.DecodeAddress(types.JaxBurnAddr, &chaincfg.MainNetParams)
	jaxBurn := addr.ScriptAddress()

	const errMsg = "invalid format of shard coinbase tx: "

	if len(shardCoinbaseTx.TxOut) != 3 {
		return errors.New(errMsg + "less than 3 out")
	}

	shardJaxNetLinkOut := bytes.Equal(shardCoinbaseTx.TxOut[0].PkScript, jaxBurn) &&
		shardCoinbaseTx.TxOut[0].Value == 0
	if !shardJaxNetLinkOut {
		return errors.New(errMsg + "first out must be zero and have JaxNetLink")
	}

	shardReward := shardCoinbaseTx.TxOut[1].Value

	if shardReward != expectedReward {
		return errors.New(errMsg + "value of second output not eq to expected reward")
	}

	shardJaxBurnReward := bytes.Equal(shardCoinbaseTx.TxOut[1].PkScript, jaxBurn)
	if shardJaxBurnReward {
		return nil
	}

	beaconCoinbase := shardHeader.CoinbaseAux.Tx
	beaconBurned, err := ValidateBeaconCoinbase(shardHeader.BeaconHeader(), &beaconCoinbase, -1)
	if err != nil {
		return err
	}

	if !beaconBurned && !shardJaxBurnReward {
		return errors.New(errMsg + "BTC & JaxNet not burned, Jax reward prohibited")
	}
	if beaconBurned && shardJaxBurnReward {
		return errors.New(errMsg + "BTC & JaxNet burned, Jax burn prohibited")
	}

	return nil
}
