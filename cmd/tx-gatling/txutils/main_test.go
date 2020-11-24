// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
// + build integration_test

package txutils

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core/txscript"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type T testing.T

func (t *T) Errorf(format string, args ...interface{}) { t.Fatalf(format, args...) }

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

func TestSendTx(ot *testing.T) {
	t := (*T)(ot)

	cfg := ManagerCfg{
		Net: "fastnet",
		RPC: NodeRPC{
			Host: "116.202.107.209:18333",
			User: "jaxnetrpc",
			Pass: "AUL6VBjoQnhP3bfFzl",
			// Host: "127.0.0.1:18333",
			// User: "somerpc",
			// Pass: "somerpc",
		},
		PrivateKey: "",
	}

	op, err := NewOperator(cfg)
	assert.NoError(t, err)
	rawTx := ""

	data, err := hex.DecodeString(rawTx)
	assert.NoError(t, err)
	signedTx := &txmodels.SwapTransaction{}
	err = signedTx.UnmarshalBinary(data)
	assert.NoError(t, err)
	_, err = op.TxMan.RPC().ForShard(1).SendRawTransaction(signedTx.RawTX, true)
	assert.NoError(t, err)
	fmt.Printf("Submitted to first shard: %s\n", signedTx.TxHash)

	// ---/---- SUBMIT Shards Swap TX to 2nd Shard ----\----
	_, err = op.TxMan.RPC().ForShard(1).SendRawTransaction(signedTx.RawTX, true)
	assert.NoError(t, err)
	fmt.Printf("Submitted to second shard: %s\n", signedTx.TxHash)

}

func TestMakeSwapTx(ot *testing.T) {
	t := (*T)(ot)
	var shardID1 uint32 = 18
	var shardID2 uint32 = 17

	minerSK := "3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b"
	minerAddr := "mxQsksaTJb11i7vSxAUL6VBjoQnhP3bfFz"
	minerUTXOIndex := storage.NewUTXORepo("", "miner")
	err := minerUTXOIndex.ReadIndex()
	assert.NoError(t, err)
	// -----------------------------------------------------------------------------------------

	// ---/---- PREPARE ----\----
	cfg := ManagerCfg{
		Net: "fastnet",
		RPC: NodeRPC{
			// Host: "116.203.250.136:18333",
			Host: "116.202.107.209:18333",
			User: "jaxnetrpc",
			Pass: "AUL6VBjoQnhP3bfFzl",
			// Host: "127.0.0.1:18333",
			// User: "somerpc",
			// Pass: "somerpc",
		},
		PrivateKey: "",
	}

	minerKP, err := NewKeyData(minerSK, cfg.NetParams())
	assert.NoError(t, err)

	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	fmt.Printf("Collectiong UTXO from first shard...\n")

	index := minerUTXOIndex.Index()
	index, _, err = op.TxMan.CollectUTXOIndex(shardID1,
		index.LastBlock(shardID1),
		map[string]bool{minerAddr: true}, index)
	assert.NoError(t, err)
	minerUTXOIndex.SetIndex(index)

	fmt.Printf("Collectiong UTXO from second shard...\n")
	index, _, err = op.TxMan.CollectUTXOIndex(shardID2,
		index.LastBlock(shardID2),
		map[string]bool{minerAddr: true}, index)
	assert.NoError(t, err)
	minerUTXOIndex.SetIndex(index)

	err = minerUTXOIndex.SaveIndex()
	assert.NoError(t, err)

	rows, _ := minerUTXOIndex.SelectForAmount(5000000000, shardID1)
	assert.Equal(t, 1, len(rows))
	shard1UTXO := rows[0]

	rows, _ = minerUTXOIndex.SelectForAmount(5000000000, shardID2)
	assert.Equal(t, 1, len(rows))
	shard2UTXO := rows[0]

	// -----------------------------------------------------------------------------------------
	destinationAtShard1 := "mwnAejT1i6Fra7npajqEe6G3A22DFbU5aK"
	destinationAtShard2 := "mz6Z8dFaxEAMmnR5ha4CVnKykeY6S3dUwi"
	spendingMap := map[string]txmodels.UTXO{
		destinationAtShard1: shard1UTXO,
		destinationAtShard2: shard2UTXO,
	}
	swapTX, err := op.TxMan.WithKeys(minerKP).NewSwapTx(spendingMap, false)
	assert.NoError(t, err)
	fmt.Printf("Send Tx\nHash: %s\nBody: %s\n", swapTX.TxHash, swapTX.SignedTx)

	// ---/---- SUBMIT Shards Swap TX to 1st Shard ----\----
	// publish created transaction
	txHash, err := op.TxMan.RPC().ForShard(shard1UTXO.ShardID).SendRawTransaction(swapTX.RawTX, true)
	assert.NoError(t, err)
	fmt.Printf("Submitted to first shard: %s\n", swapTX.TxHash)

	// ---/---- SUBMIT Shards Swap TX to 2nd Shard ----\----
	txHash, err = op.TxMan.RPC().ForShard(shard2UTXO.ShardID).SendRawTransaction(swapTX.RawTX, true)
	assert.NoError(t, err)
	fmt.Printf("Submitted to second shard: %s\n", swapTX.TxHash)

	err = minerUTXOIndex.SaveIndex()
	assert.NoError(t, err)

	var firstDone, secondDone bool
	for {
		// wait for the transaction to be added to the block
		firstOut, err := op.TxMan.RPC().ForShard(shard1UTXO.ShardID).GetTxOut(txHash, 0, false)
		assert.NoError(t, err)
		secondOut, err := op.TxMan.RPC().ForShard(shard1UTXO.ShardID).GetTxOut(txHash, 1, false)
		assert.NoError(t, err)

		if (firstOut != nil && firstOut.Confirmations > 2) || (secondOut != nil && secondOut.Confirmations > 2) {
			fmt.Println("tx mined into block @ first shard")
			firstDone = true
		}

		// wait for the transaction to be added to the block
		firstOut, err = op.TxMan.RPC().ForShard(shard2UTXO.ShardID).GetTxOut(txHash, 0, false)
		assert.NoError(t, err)
		secondOut, err = op.TxMan.RPC().ForShard(shard2UTXO.ShardID).GetTxOut(txHash, 1, false)
		assert.NoError(t, err)

		if (firstOut != nil && firstOut.Confirmations > 2) || (secondOut != nil && secondOut.Confirmations > 2) {
			fmt.Println("tx mined into block @ second shard")
			secondDone = true
		}

		if firstDone && secondDone {
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

	// -----------------------------------------------------------------------------------------

	// ---/---- PREPARE ----\----
	cfg := ManagerCfg{
		Net: "fastnet",
		RPC: NodeRPC{
			// Host: "116.203.250.136:18333",
			// Host: "116.202.107.209:18333",
			// User: "jaxnetrpc",
			// Pass: "AUL6VBjoQnhP3bfFzl",
			Host: "127.0.0.1:18333",
			User: "somerpc",
			Pass: "somerpc",
		},
		PrivateKey: "",
	}

	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	var sendTx = func(senderKP *KeyData, shardID uint32, destination string, amount int64, timeLock uint32) {
		senderAddress := senderKP.Address.EncodeAddress()
		senderUTXOIndex := storage.NewUTXORepo("", senderAddress)

		index := senderUTXOIndex.Index()

		index, _, err := op.TxMan.CollectUTXOIndex(shardID,
			index.LastBlock(shardID), map[string]bool{senderAddress: true}, index)

		assert.NoError(t, err)
		senderUTXOIndex.SetIndex(index)

		lop := op.TxMan.ForShard(shardID)
		if timeLock > 0 {
			lop = lop.AddTimeLockAllowance(timeLock)
		}

		tx, err := lop.NewTx(destination, amount, &senderUTXOIndex)
		assert.NoError(t, err)

		_, err = op.TxMan.WithKeys(senderKP).
			RPC().ForShard(shardID).SendRawTransaction(tx.RawTX, true)

		assert.NoError(t, err)
		err = senderUTXOIndex.SaveIndex()
		assert.NoError(t, err)
	}

	minerSK := "3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b"

	aliceSk := "6443fb332e1cbfe456674aacf2be1327b6f9fc9c782061ee04ca35e17608d651"
	aliceKP, err := NewKeyData(aliceSk, cfg.NetParams())
	assert.NoError(t, err)

	bobSk := "6bb4b4a9d5512c84f14bd38248dafb80c2424ae50a0495be8e4f657d734f1bd4"
	bobKP, err := NewKeyData(bobSk, cfg.NetParams())
	assert.NoError(t, err)

	{
		minerKP, err := NewKeyData(minerSK, cfg.NetParams())
		assert.NoError(t, err)
		sendTx(minerKP, shardID1, aliceKP.Address.EncodeAddress(), OneCoin, 0)
		sendTx(minerKP, shardID2, aliceKP.Address.EncodeAddress(), OneCoin, 0)
	}

	signers := []string{
		aliceKP.AddressPubKey.String(),
		bobKP.AddressPubKey.String(),
	}

	multiSigScript, err := MakeMultiSigScript(signers, len(signers), cfg.NetParams())
	assert.NoError(t, err)

	{
		sendTx(aliceKP, shardID1, multiSigScript.Address, OneCoin, 100)
		sendTx(aliceKP, shardID2, multiSigScript.Address, OneCoin, 100)
	}

	fmt.Printf("Collectiong UTXO from first shard...\n")
	multisigUTXOIndex := storage.NewUTXORepo("", "multisig")

	var getMultisigOut = func(shardID uint32) txmodels.UTXO {
		index := multisigUTXOIndex.Index()
		fmt.Printf("Collectiong UTXO from %d shard...\n", shardID1)

		index, _, err = op.TxMan.CollectUTXOIndex(shardID,
			index.LastBlock(shardID),
			map[string]bool{multiSigScript.Address: true}, index)
		assert.NoError(t, err)
		multisigUTXOIndex.SetIndex(index)
		rows, _ := multisigUTXOIndex.SelectForAmount(OneCoin, shardID)
		assert.Equal(t, 1, len(rows))

		utxo := rows[0]
		utxo, err = SetRedeemScript(utxo, multiSigScript.RedeemScript, cfg.NetParams())
		assert.NoError(t, err)

		err = multisigUTXOIndex.SaveIndex()
		assert.NoError(t, err)

		return utxo
	}

	shard1UTXO := getMultisigOut(shardID1)
	shard2UTXO := getMultisigOut(shardID2)

	// -----------------------------------------------------------------------------------------
	destinationAtShard1 := "mwnAejT1i6Fra7npajqEe6G3A22DFbU5aK"
	destinationAtShard2 := "mz6Z8dFaxEAMmnR5ha4CVnKykeY6S3dUwi"
	spendingMap := map[string]txmodels.UTXO{
		destinationAtShard1: shard1UTXO,
		destinationAtShard2: shard2UTXO,
	}
	swapTX, err := op.TxMan.WithKeys(aliceKP).NewSwapTx(spendingMap, false, multiSigScript.RedeemScript)
	assert.NoError(t, err)

	// ---/---- ADD SECOND SIGNATURE TO SPEND MULTISIG UTXO TX ----\----
	{
		var swapTxWithMultisig *wire.MsgTx
		swapTxWithMultisig, err = op.TxMan.WithKeys(bobKP).AddSignatureToSwapTx(swapTX.RawTX,
			[]uint32{shardID1, shardID2},
			multiSigScript.RedeemScript)

		assert.NoError(t, err)
		swapTX.RawTX = swapTxWithMultisig
		swapTX.SignedTx = EncodeTx(swapTxWithMultisig)
	}
	// -----------------------------------------------------------------------------------------

	// ---/---- SUBMIT Shards Swap TX to 1st Shard ----\----
	{
		// publish created transaction
		txHash, err := op.TxMan.RPC().ForShard(shard1UTXO.ShardID).SendRawTransaction(swapTX.RawTX, true)
		assert.NoError(t, err)
		fmt.Printf("Submitted to first shard: %s\n", swapTX.TxHash)

		// ---/---- SUBMIT Shards Swap TX to 2nd Shard ----\----
		txHash, err = op.TxMan.RPC().ForShard(shard2UTXO.ShardID).SendRawTransaction(swapTX.RawTX, true)
		assert.NoError(t, err)
		fmt.Printf("Submitted to second shard: %s\n", swapTX.TxHash)

		var firstDone, secondDone bool
		for {
			// wait for the transaction to be added to the block
			firstOut, err := op.TxMan.RPC().ForShard(shard1UTXO.ShardID).GetTxOut(txHash, 0, false)
			assert.NoError(t, err)
			secondOut, err := op.TxMan.RPC().ForShard(shard1UTXO.ShardID).GetTxOut(txHash, 1, false)
			assert.NoError(t, err)

			if (firstOut != nil && firstOut.Confirmations > 2) || (secondOut != nil && secondOut.Confirmations > 2) {
				fmt.Println("tx mined into block @ first shard")
				firstDone = true
			}

			// wait for the transaction to be added to the block
			firstOut, err = op.TxMan.RPC().ForShard(shard2UTXO.ShardID).GetTxOut(txHash, 0, false)
			assert.NoError(t, err)
			secondOut, err = op.TxMan.RPC().ForShard(shard2UTXO.ShardID).GetTxOut(txHash, 1, false)
			assert.NoError(t, err)

			if (firstOut != nil && firstOut.Confirmations > 2) || (secondOut != nil && secondOut.Confirmations > 2) {
				fmt.Println("tx mined into block @ second shard")
				secondDone = true
			}

			if firstDone && secondDone {
				break
			}

			time.Sleep(time.Second)
		}
	}

}
