// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
// + build integration_test

package txutils

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core/txscript"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type T testing.T

func (t *T) Errorf(format string, args ...interface{}) { t.Fatalf(format, args...) }

// func TestMain(m *testing.M) {
//
// }

func TestOperator_SpendUTXO(t *testing.T) {
	cfg := ManagerCfg{
		Net: "testnet",
		RPC: NodeRPC{
			Host: "127.0.0.1:28334",
			User: "somerpc",
			Pass: "somerpc",
		},
		PrivateKey: "",
	}
	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	aliceAddr := "myGdvt6vRNgrFZFtU5FNhW5gxwRqKBcLGv"
	txHash := "8e8de99c0bf81f95b010e53f74bfd2c4d608227938f279954f062185be052cd6"
	kd, err := NewKeyData("3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b", cfg.NetParams())
	assert.NoError(t, err)

	tx, err := op.SpendUTXO(*kd, txHash, 0, aliceAddr, 1_0000_0000)
	assert.NoError(t, err)
	assert.NotNil(t, tx)
}

func TestMakeMultiSigScript(ot *testing.T) {
	t := (*T)(ot)
	var shardID uint32 = 1
	aliceSk := "6443fb332e1cbfe456674aacf2be1327b6f9fc9c782061ee04ca35e17608d651"
	bobSk := "6bb4b4a9d5512c84f14bd38248dafb80c2424ae50a0495be8e4f657d734f1bd4"
	evaAddress := "mwnAejT1i6Fra7npajqEe6G3A22DFbU5aK"
	utxoSearchOffset := 0
	amount := int64(1000000000) - OneCoin

	// -----------------------------------------------------------------------------------------
	// ---/---- PREPARE ----\----
	cfg := ManagerCfg{
		Net:     "testnet",
		ShardID: shardID,
		RPC: NodeRPC{
			Host: "116.202.107.209:18334",
			User: "somerpc",
			Pass: "somerpc",
		},
		PrivateKey: "",
	}

	aliceKP, err := NewKeyData(aliceSk, cfg.NetParams())
	assert.NoError(t, err)

	bobKP, err := NewKeyData(bobSk, cfg.NetParams())
	assert.NoError(t, err)

	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	aliceUTXO, _, err := op.TxMan.ForShard(shardID).
		CollectUTXO(aliceKP.Address.EncodeAddress(), int64(utxoSearchOffset))
	assert.NoError(t, err)
	// -----------------------------------------------------------------------------------------

	// -----------------------------------------------------------------------------------------
	// ---/---- CREATE MULTISIG ADDRESS ----\----
	signers := []string{
		aliceKP.AddressPubKey.String(),
		bobKP.AddressPubKey.String(),
	}

	multiSigScript, err := MakeMultiSigScript(signers, len(signers), cfg.NetParams())
	assert.NoError(t, err)
	t.Log(multiSigScript.Address)
	// -----------------------------------------------------------------------------------------

	// -----------------------------------------------------------------------------------------
	// ---/---- SEND COINS TO MULTISIG ADDRESS  ----\----
	toMultiSigAddrTx, err := op.TxMan.
		ForShard(shardID).
		WithKeys(aliceKP).
		NewTx(multiSigScript.Address, amount, UTXOFromRows(aliceUTXO))
	assert.NoError(t, err)

	// publish created transaction

	txHash, err := op.TxMan.RPC().ForShard(shardID).SendRawTransaction(toMultiSigAddrTx.RawTX, true)
	assert.NoError(t, err)

waitLoop:
	for {
		// wait for the transaction to be added to the block
		out, err := op.TxMan.RPC().ForShard(shardID).GetTxOut(txHash, 0, false)
		assert.NoError(t, err)
		if out != nil && out.Confirmations > 1 {
			fmt.Println("tx mined into block")
			break waitLoop
		}

		time.Sleep(time.Second)
	}
	// -----------------------------------------------------------------------------------------

	// -----------------------------------------------------------------------------------------
	// ---/---- CREATE TX TO SPEND MULTISIG UTXO  ----\----
	multiSigOutTxHash := toMultiSigAddrTx.TxHash
	multiSigOutIndex := uint32(0)
	for i, out := range toMultiSigAddrTx.RawTX.TxOut {
		decoded, _ := DecodeScript(out.PkScript, cfg.NetParams())
		for _, address := range decoded.Addresses {
			if address == multiSigScript.Address {
				multiSigOutIndex = uint32(i)
			}
		}
	}
	amount = amount - OneCoin
	multiSigSpendTx, err := op.NewMultiSigSpendTx(*aliceKP, multiSigOutTxHash,
		multiSigScript.RedeemScript, multiSigOutIndex, evaAddress, amount)
	assert.NoError(t, err)
	// -----------------------------------------------------------------------------------------

	// -----------------------------------------------------------------------------------------
	// ---/---- ADD SECOND SIGNATURE TO SPEND MULTISIG UTXO TX ----\----
	multiSigSpendTx, err = op.AddSignatureToTx(*aliceKP, multiSigSpendTx.SignedTx, multiSigScript.RedeemScript)
	assert.NoError(t, err)

	multiSigSpendTx, err = op.AddSignatureToTx(*bobKP, multiSigSpendTx.SignedTx, multiSigScript.RedeemScript)
	assert.NoError(t, err)
	// -----------------------------------------------------------------------------------------

	fmt.Println(EncodeTx(multiSigSpendTx.RawTX))
	vm, err := txscript.NewEngine(multiSigScript.RawRedeemScript, multiSigSpendTx.RawTX, 0,
		txscript.StandardVerifyFlags, nil, nil, amount)
	assert.NoError(t, err)

	err = vm.Execute()
	// assert.NoError(t, err)
	// -----------------------------------------------------------------------------------------

	// ---/---- SUBMIT MULTI SIG UTXO TX ----\----
	// publish created transaction
	txHash, err = op.TxMan.RPC().ForShard(shardID).SendRawTransaction(multiSigSpendTx.RawTX, true)
	assert.NoError(t, err)

	for {
		// wait for the transaction to be added to the block
		out, err := op.TxMan.RPC().ForShard(shardID).GetTxOut(txHash, 0, false)
		assert.NoError(t, err)
		if out != nil && out.Confirmations > 2 {
			println("tx mined into block")
			break
		}

		time.Sleep(time.Second)
	}
	// -----------------------------------------------------------------------------------------
}

func TestMakeSwapTx(ot *testing.T) {
	t := (*T)(ot)
	var shardID1 uint32 = 1
	var shardID2 uint32 = 2

	minerSK := "3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b"
	// -----------------------------------------------------------------------------------------

	// ---/---- PREPARE ----\----
	cfg := ManagerCfg{
		Net: "fastnet",
		RPC: NodeRPC{
			// Host: "116.202.107.209:18333",
			Host: "127.0.0.1:18333",
			User: "somerpc",
			Pass: "somerpc",
		},
		PrivateKey: "",
	}

	minerKP, err := NewKeyData(minerSK, cfg.NetParams())
	assert.NoError(t, err)

	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	shard1UTXO := txmodels.UTXO{
		ShardID:    shardID1,
		Address:    "mxQsksaTJb11i7vSxAUL6VBjoQnhP3bfFz",
		Value:      5000000000,
		Height:     1,
		TxHash:     "5bbf6a3a876add0beddeea39f6d79c63a1fdaeaa147e591543689ebdbc739ec5",
		OutIndex:   0,
		Used:       false,
		PKScript:   "76a914b953dad0e79288eea918085c9b72c3ca5482349388ac",
		ScriptType: "pubkeyhash",
	}
	assert.NoError(t, err)

	shard2UTXO := txmodels.UTXO{
		ShardID:    shardID2,
		Address:    "mxQsksaTJb11i7vSxAUL6VBjoQnhP3bfFz",
		Value:      5000000000,
		Height:     1,
		TxHash:     "541019173655286cc0249c2a6fc8a7ae02e1f14dc59ae77fe0a26b1a245b8a90",
		OutIndex:   0,
		Used:       false,
		PKScript:   "76a914b953dad0e79288eea918085c9b72c3ca5482349388ac",
		ScriptType: "pubkeyhash",
	}
	assert.NoError(t, err)

	// -----------------------------------------------------------------------------------------
	destinationAtShard1 := "mwnAejT1i6Fra7npajqEe6G3A22DFbU5aK"
	destinationAtShard2 := "mz6Z8dFaxEAMmnR5ha4CVnKykeY6S3dUwi"
	spendingMap := map[string]txmodels.UTXO{
		destinationAtShard1: shard1UTXO,
		destinationAtShard2: shard2UTXO,
	}
	swapTX, err := op.TxMan.WithKeys(minerKP).NewSwapTx(spendingMap, false)
	assert.NoError(t, err)

	// ---/---- SUBMIT Shards Swap TX to 1st Shard ----\----
	// publish created transaction
	txHash, err := op.TxMan.RPC().ForShard(shard1UTXO.ShardID).SendRawTransaction(swapTX.RawTX, true)
	assert.NoError(t, err)
	fmt.Printf("Sent Tx\nHash: %s\nBody: %s\n", swapTX.TxHash, swapTX.SignedTx)

	// for {
	// 	// wait for the transaction to be added to the block
	// 	out, err := op.TxMan.RPC().ForShard(shard2UTXO.ShardID).GetTxOut(txHash, 0, false)
	// 	assert.NoError(t, err)
	// 	if out != nil && out.Confirmations > 2 {
	// 		println("tx mined into block")
	// 		break
	// 	}
	//
	// 	time.Sleep(time.Second)
	// }

	// ---/---- SUBMIT Shards Swap TX to 2nd Shard ----\----
	txHash, err = op.TxMan.RPC().ForShard(shard2UTXO.ShardID).SendRawTransaction(swapTX.RawTX, true)
	assert.NoError(t, err)
	return
	for {
		// wait for the transaction to be added to the block
		out, err := op.TxMan.RPC().ForShard(shard2UTXO.ShardID).GetTxOut(txHash, 0, false)
		assert.NoError(t, err)
		if out != nil && out.Confirmations > 2 {
			println("tx mined into block")
			break
		}

		time.Sleep(time.Second)
	}
	// -----------------------------------------------------------------------------------------
}

func TestMakeMultiSigSwapTx(ot *testing.T) {
	t := (*T)(ot)
	var shardID1 uint32 = 1
	var shardID2 uint32 = 2

	aliceSk := "6443fb332e1cbfe456674aacf2be1327b6f9fc9c782061ee04ca35e17608d651"
	bobSk := "6bb4b4a9d5512c84f14bd38248dafb80c2424ae50a0495be8e4f657d734f1bd4"
	// -----------------------------------------------------------------------------------------

	// ---/---- PREPARE ----\----
	cfg := ManagerCfg{
		Net: "fastnet",
		RPC: NodeRPC{
			// Host: "116.202.107.209:18333",
			Host: "127.0.0.1:18333",
			User: "somerpc",
			Pass: "somerpc",
		},
		PrivateKey: "",
	}

	aliceKP, err := NewKeyData(aliceSk, cfg.NetParams())
	assert.NoError(t, err)

	bobKP, err := NewKeyData(bobSk, cfg.NetParams())
	assert.NoError(t, err)

	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	signers := []string{
		aliceKP.AddressPubKey.String(),
		bobKP.AddressPubKey.String(),
	}

	multiSigScript, err := MakeMultiSigScript(signers, len(signers), cfg.NetParams())
	assert.NoError(t, err)

	shard1UTXO := txmodels.UTXO{
		ShardID: shardID1,
		Address: multiSigScript.Address,
		Value:   OneCoin,
		// todo: all fields must be filled in with the corresponding values
		Height:     0,
		TxHash:     "",
		OutIndex:   0,
		Used:       false,
		PKScript:   "",
		ScriptType: "",
	}
	shard1UTXO, err = SetRedeemScript(shard1UTXO, multiSigScript.RedeemScript, cfg.NetParams())
	assert.NoError(t, err)

	shard2UTXO := txmodels.UTXO{
		ShardID: shardID2,
		Address: multiSigScript.Address,
		Value:   OneCoin,
		// todo: all fields must be filled in with the corresponding values
		Height:     0,
		TxHash:     "",
		OutIndex:   0,
		Used:       false,
		PKScript:   "",
		ScriptType: "",
	}
	shard2UTXO, err = SetRedeemScript(shard2UTXO, multiSigScript.RedeemScript, cfg.NetParams())
	assert.NoError(t, err)

	// -----------------------------------------------------------------------------------------
	// todo: must be a valid jax.net addresses
	destinationAtShard1 := ""
	destinationAtShard2 := ""
	spendingMap := map[string]txmodels.UTXO{
		destinationAtShard1: shard1UTXO,
		destinationAtShard2: shard2UTXO,
	}
	swapTX, err := op.TxMan.WithKeys(aliceKP).NewSwapTx(spendingMap, false)
	assert.NoError(t, err)

	var swapTxWithMultisig *wire.MsgTx

	swapTxWithMultisig, err = op.TxMan.WithKeys(aliceKP).AddSignatureToSwapTx(swapTX.RawTX,
		[]uint32{shardID1, shardID2},
		multiSigScript.RedeemScript)

	assert.NoError(t, err)
	swapTX.RawTX = swapTxWithMultisig
	swapTX.SignedTx = EncodeTx(swapTxWithMultisig)

	// ---/---- ADD SECOND SIGNATURE TO SPEND MULTISIG UTXO TX ----\----
	swapTxWithMultisig, err = op.TxMan.WithKeys(bobKP).AddSignatureToSwapTx(swapTX.RawTX,
		[]uint32{shardID1, shardID2},
		multiSigScript.RedeemScript)

	assert.NoError(t, err)
	swapTX.RawTX = swapTxWithMultisig
	swapTX.SignedTx = EncodeTx(swapTxWithMultisig)
	// -----------------------------------------------------------------------------------------

	// ---/---- SUBMIT Shards Swap TX to 1st Shard ----\----
	// publish created transaction
	txHash, err := op.TxMan.RPC().ForShard(shard1UTXO.ShardID).SendRawTransaction(swapTxWithMultisig, true)
	assert.NoError(t, err)

	for {
		// wait for the transaction to be added to the block
		out, err := op.TxMan.RPC().ForShard(shard2UTXO.ShardID).GetTxOut(txHash, 0, false)
		assert.NoError(t, err)
		if out != nil && out.Confirmations > 2 {
			println("tx mined into block")
			break
		}

		time.Sleep(time.Second)
	}

	// ---/---- SUBMIT Shards Swap TX to 2nd Shard ----\----
	txHash, err = op.TxMan.RPC().ForShard(shard2UTXO.ShardID).SendRawTransaction(swapTxWithMultisig, true)
	assert.NoError(t, err)

	for {
		// wait for the transaction to be added to the block
		out, err := op.TxMan.RPC().ForShard(shard2UTXO.ShardID).GetTxOut(txHash, 0, false)
		assert.NoError(t, err)
		if out != nil && out.Confirmations > 2 {
			println("tx mined into block")
			break
		}

		time.Sleep(time.Second)
	}
	// -----------------------------------------------------------------------------------------

}
