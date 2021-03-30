// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
// + build integration_test

package txutils

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core/network/rpcclient"
	"gitlab.com/jaxnet/core/shard.core/txscript"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
	"golang.org/x/sync/errgroup"
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
	// 49ca5764af92b2f744ac5d8b10fdb349b5d7f8c8351cfa1bd27163fffcb0571c
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

	minerKP, err := NewKeyData(minerSK, cfg.NetParams())
	assert.NoError(t, err)

	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	fmt.Printf("Collectiong UTXO from first shard...\n")
	minerRepo := storage.NewUTXORepo("")
	err = minerRepo.CollectFromRPC(op.TxMan.RPC(), shardID1, map[string]bool{minerAddr: true})
	assert.NoError(t, err)

	rows, _ := minerRepo.SelectForAmount(5000000000, shardID1)
	assert.Equal(t, 1, len(rows))
	shard1UTXO := rows[0]

	err = minerRepo.CollectFromRPC(op.TxMan.RPC(), shardID2, map[string]bool{minerAddr: true})
	assert.NoError(t, err)

	rows, _ = minerRepo.SelectForAmount(5000000000, shardID2)
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
			// Host: "116.202.107.209:22333",
			User: "jaxnetrpc",
			Pass: "AUL6VBjoQnhP3bfFzl",
			Host: "127.0.0.1:18333",
			// User: "somerpc",
			// Pass: "somerpc",
			// Host: "116.203.250.136:18333",
			// User: "jaxnetrpc",
			// Pass: "ec0bb2575b06bfdf",
		},
		PrivateKey: "",
	}

	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	minerSK := "3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b"

	aliceSk := "6443fb332e1cbfe456674aacf2be1327b6f9fc9c782061ee04ca35e17608d651"
	aliceKP, err := NewKeyData(aliceSk, cfg.NetParams())
	assert.NoError(t, err)

	bobSk := "6bb4b4a9d5512c84f14bd38248dafb80c2424ae50a0495be8e4f657d734f1bd4"
	bobKP, err := NewKeyData(bobSk, cfg.NetParams())
	assert.NoError(t, err)

	fmt.Println("Send deposit to Alice and Bob...")
	{
		minerKP, err := NewKeyData(minerSK, cfg.NetParams())
		assert.NoError(t, err)
		txHashAtShard1, err := SendTx(op.TxMan, minerKP, shardID1, aliceKP.Address.EncodeAddress(), OneCoin, 0)
		assert.NoError(t, err)

		txHashAtShard2, err := SendTx(op.TxMan, minerKP, shardID2, aliceKP.Address.EncodeAddress(), OneCoin, 0)
		assert.NoError(t, err)

		eGroup := errgroup.Group{}
		eGroup.Go(func() error { return WaitForTx(op.TxMan.RPC(), shardID1, txHashAtShard1, 0) })
		eGroup.Go(func() error { return WaitForTx(op.TxMan.RPC(), shardID2, txHashAtShard2, 0) })
		err = eGroup.Wait()
		assert.NoError(t, err)
	}

	signers := []string{
		aliceKP.AddressPubKey.String(),
		bobKP.AddressPubKey.String(),
	}

	multiSigScript, err := MakeMultiSigScript(signers, len(signers), cfg.NetParams())
	assert.NoError(t, err)

	fmt.Println("Lock funds on multi-sig address: ", multiSigScript.Address)
	{
		txHashAtShard1, err := SendTx(op.TxMan, aliceKP, shardID1, multiSigScript.Address, OneCoin, 0)
		assert.NoError(t, err)

		txHashAtShard2, err := SendTx(op.TxMan, aliceKP, shardID2, multiSigScript.Address, OneCoin, 0)
		assert.NoError(t, err)

		eGroup := errgroup.Group{}
		eGroup.Go(func() error { return WaitForTx(op.TxMan.RPC(), shardID1, txHashAtShard1, 0) })
		eGroup.Go(func() error { return WaitForTx(op.TxMan.RPC(), shardID2, txHashAtShard2, 0) })
		err = eGroup.Wait()
		_, height1, err := op.TxMan.RPC().ForShard(shardID1).GetBestBlock()
		assert.NoError(t, err)
		_, height2, err := op.TxMan.RPC().ForShard(shardID1).GetBestBlock()
		assert.NoError(t, err)
		fmt.Printf("Current height @ shard(%d) = %d; @ shard(%d) = %d\n", shardID1, height1, shardID2, height2)
		// assert.NoError(t, err)
	}

	multisigUTXOIndex := storage.NewUTXORepo("", "multisig")

	var getMultisigOut = func(shardID uint32) txmodels.UTXO {
		fmt.Printf("Collectiong UTXO from %d shard...\n", shardID1)
		err = multisigUTXOIndex.CollectFromRPC(op.TxMan.RPC(), shardID, map[string]bool{multiSigScript.Address: true})
		assert.NoError(t, err)

		rows, _ := multisigUTXOIndex.SelectForAmount(OneCoin, shardID)
		assert.Equal(t, 1, len(rows))

		utxo := rows[0]
		utxo, err = SetRedeemScript(utxo, multiSigScript.RedeemScript, cfg.NetParams())
		assert.NoError(t, err)

		// err = multisigUTXOIndex.SaveIndex()
		// assert.NoError(t, err)

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

	fmt.Println(swapTX.RawTX.TxHash())
	fmt.Println(EncodeTx(swapTX.RawTX))
	// ---/---- ADD SECOND SIGNATURE TO SPEND MULTISIG UTXO TX ----\----
	{
		var swapTxWithMultisig *wire.MsgTx
		swapTxWithMultisig, err = op.TxMan.WithKeys(bobKP).AddSignatureToSwapTx(swapTX.RawTX,
			[]uint32{shardID1, shardID2},
			multiSigScript.RedeemScript)
		assert.NoError(t, err)
		swapTX.RawTX = swapTxWithMultisig
		swapTX.SignedTx = EncodeTx(swapTxWithMultisig)

		fmt.Println(swapTxWithMultisig.TxHash())
		fmt.Println(EncodeTx(swapTxWithMultisig))
	}
	// -----------------------------------------------------------------------------------------

	// ---/---- SUBMIT Shards Swap TX ----\----
	fmt.Println(" SUBMIT Shards Swap TX")
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

func TestTimeLockTx(ot *testing.T) {
	t := (*T)(ot)

	cfg := ManagerCfg{
		Net: "fastnet",
		RPC: NodeRPC{
			Host: "116.203.250.136:18333",
			User: "jaxnetrpc",
			Pass: "ec0bb2575b06bfdf",
			// Host: "116.202.107.209:18333",
			// User: "jaxnetrpc",
			// Pass: "AUL6VBjoQnhP3bfFzl",
			// Host: "127.0.0.1:18333",
			// User: "somerpc",
			// Pass: "somerpc",
		},
		PrivateKey: "",
	}
	shardID := uint32(1)
	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	minerSK := "3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b"
	minerKP, err := NewKeyData(minerSK, cfg.NetParams())
	assert.NoError(t, err)

	aliceSk := "6443fb332e1cbfe456674aacf2be1327b6f9fc9c782061ee04ca35e17608d651"
	aliceKP, err := NewKeyData(aliceSk, cfg.NetParams())
	assert.NoError(t, err)
	bobSk := "6bb4b4a9d5512c84f14bd38248dafb80c2424ae50a0495be8e4f657d734f1bd4"
	bobKP, err := NewKeyData(bobSk, cfg.NetParams())
	assert.NoError(t, err)

	{
		txHashAtShard1, err := SendTx(op.TxMan, minerKP, shardID, aliceKP.Address.EncodeAddress(), OneCoin, 0)
		assert.NoError(t, err)

		eGroup := errgroup.Group{}
		eGroup.Go(func() error { return WaitForTx(op.TxMan.RPC(), shardID, txHashAtShard1, 0) })
		err = eGroup.Wait()
		assert.NoError(t, err)
	}

	{
		txHashAtShard1, err := SendTx(op.TxMan, aliceKP, shardID, bobKP.Address.EncodeAddress(), OneCoin, 0)
		assert.NoError(t, err)

		eGroup := errgroup.Group{}
		eGroup.Go(func() error { return WaitForTx(op.TxMan.RPC(), shardID, txHashAtShard1, 0) })
		err = eGroup.Wait()
		assert.NoError(t, err)
	}

	{
		txHashAtShard1, err := SendTx(op.TxMan, bobKP, shardID, aliceKP.Address.EncodeAddress(), OneCoin, 0)
		assert.NoError(t, err)

		eGroup := errgroup.Group{}
		eGroup.Go(func() error { return WaitForTx(op.TxMan.RPC(), shardID, txHashAtShard1, 0) })
		err = eGroup.Wait()
		assert.NoError(t, err)
	}

}

func TestEADRegistration(ot *testing.T) {
	t := (*T)(ot)

	cfg := ManagerCfg{
		Net: "fastnet",
		RPC: NodeRPC{
			// Host: "116.203.250.136:18333",
			// User: "jaxnetrpc",
			// Pass: "ec0bb2575b06bfdf",
			// Host: "116.202.107.209:18333",
			// User: "jaxnetrpc",
			// Pass: "AUL6VBjoQnhP3bfFzl",
			Host: "127.0.0.1:18333",
			User: "somerpc",
			Pass: "somerpc",
		},
		PrivateKey: "",
	}
	shardID := uint32(0)
	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	minerSK := "3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b"
	minerKP, err := NewKeyData(minerSK, cfg.NetParams())
	assert.NoError(t, err)
	//
	{
		var scripts [][]byte
		for i := 10; i < 22; i++ {
			ipV4 := net.IPv4(77, 244, 36, 32)
			expTime := int64(1608157135)
			port := int64(43801)

			scriptAddress, err := txscript.EADAddressScript(txscript.EADScriptData{
				ShardID:        uint32(i / 2),
				IP:             ipV4,
				Port:           port,
				ExpirationDate: expTime,
				Owner:          minerKP.AddressPubKey,
				OpCode:         txscript.EADAddressDelete,
			})
			assert.NoError(t, err)
			scripts = append(scripts, scriptAddress)
		}

		senderAddress := minerKP.Address.EncodeAddress()
		senderUTXOIndex := storage.NewUTXORepo("", senderAddress)
		err = senderUTXOIndex.CollectFromRPC(op.TxMan.RPC(), shardID, map[string]bool{senderAddress: true})
		assert.NoError(t, err)

		tx, err := op.TxMan.WithKeys(minerKP).
			ForShard(0).
			NewEADRegistrationTx(5, &senderUTXOIndex, scripts...)
		assert.NoError(t, err)

		_, err = op.TxMan.RPC().ForBeacon().SendRawTransaction(tx.RawTX, true)
		assert.NoError(t, err)

		fmt.Printf("Sent tx %s at shard %d\n", tx.TxHash, shardID)

		eGroup := errgroup.Group{}
		eGroup.Go(func() error { return WaitForTx(op.TxMan.RPC(), shardID, tx.TxHash, 0) })
		err = eGroup.Wait()
		assert.NoError(t, err)

		addresses, err := op.TxMan.RPC().ListEADAddresses(nil, nil)
		assert.NoError(t, err)

		fmt.Printf("%+v\n", addresses)
	}

}

func TestEAD(ot *testing.T) {
	t := (*T)(ot)

	cfg := ManagerCfg{
		Net: "fastnet",
		RPC: NodeRPC{
			// Host: "116.203.250.136:18333",
			// User: "jaxnetrpc",
			// Pass: "ec0bb2575b06bfdf",
			Host: "116.202.107.209:22333",
			User: "jaxnetrpc",
			Pass: "AUL6VBjoQnhP3bfFzl",
			// Host: "127.0.0.1:18333",
			// User: "somerpc",
			// Pass: "somerpc",
		},
		PrivateKey: "",
	}
	shardID := uint32(0)
	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	minerSK := "3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b"
	minerKP, err := NewKeyData(minerSK, cfg.NetParams())
	assert.NoError(t, err)
	//
	{
		var scriptsToCreate [][]byte
		var scriptsToDelete [][]byte
		for i := 1; i < 12; i++ {
			ipV4 := net.IPv4(77, 244, 36, 88)
			expTime := int64(1608157135)
			port := int64(43801)
			data := txscript.EADScriptData{
				ShardID:        uint32(i),
				IP:             ipV4,
				Port:           port,
				ExpirationDate: expTime,
				Owner:          minerKP.AddressPubKey,
			}

			scriptAddress, err := txscript.EADAddressScript(data)
			assert.NoError(t, err)
			scriptsToCreate = append(scriptsToCreate, scriptAddress)

			data.OpCode = txscript.EADAddressDelete
			scriptAddress, err = txscript.EADAddressScript(data)
			assert.NoError(t, err)
			scriptsToDelete = append(scriptsToDelete, scriptAddress)
		}

		senderAddress := minerKP.Address.EncodeAddress()
		senderUTXOIndex := storage.NewUTXORepo("", senderAddress)
		err = senderUTXOIndex.CollectFromRPC(op.TxMan.RPC(), shardID, map[string]bool{senderAddress: true})
		assert.NoError(t, err)

		sendClosure := func(scripts ...[]byte) {

			tx, err := op.TxMan.WithKeys(minerKP).
				ForShard(0).
				NewEADRegistrationTx(5, &senderUTXOIndex, scripts...)
			assert.NoError(t, err)

			_, err = op.TxMan.RPC().ForBeacon().SendRawTransaction(tx.RawTX, true)
			assert.NoError(t, err)

			fmt.Printf("Sent tx %s at shard %d\n", tx.TxHash, shardID)

			eGroup := errgroup.Group{}
			eGroup.Go(func() error { return WaitForTx(op.TxMan.RPC(), shardID, tx.TxHash, 0) })
			err = eGroup.Wait()
			assert.NoError(t, err)

			addresses, err := op.TxMan.RPC().ListEADAddresses(nil, nil)
			assert.NoError(t, err)
			prettyPrint(addresses)
		}

		sendClosure(scriptsToCreate...)
		sendClosure(scriptsToDelete...)
	}

}

func TestEADSpend(ot *testing.T) {
	t := (*T)(ot)

	cfg := ManagerCfg{
		Net: "fastnet",
		RPC: NodeRPC{
			// Host: "116.203.250.136:18333",
			// User: "jaxnetrpc",
			// Pass: "ec0bb2575b06bfdf",
			// Host: "116.202.107.209:18333",
			// User: "jaxnetrpc",
			// Pass: "AUL6VBjoQnhP3bfFzl",
			Host: "127.0.0.1:18333",
			User: "somerpc",
			Pass: "somerpc",
		},
		PrivateKey: "",
	}
	// shardID := uint32(0)
	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	minerSK := "3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b"
	minerKP, err := NewKeyData(minerSK, cfg.NetParams())
	assert.NoError(t, err)

	rawHash := "8cfd9e761fac9e81d308a11d29b66f86e1e46c1e6411291fd8a56a2943b085c6"
	hash, _ := chainhash.NewHashFromStr("8cfd9e761fac9e81d308a11d29b66f86e1e46c1e6411291fd8a56a2943b085c6")
	txOut, err := op.TxMan.RPC().GetTxOut(hash, 0, false)
	assert.NoError(t, err)

	utxo := SingleUTXO{
		TxHash:     rawHash,
		OutIndex:   0,
		Value:      int64(txOut.Value * btcutil.SatoshiPerBitcoin),
		Used:       false,
		PKScript:   txOut.ScriptPubKey.Hex,
		ScriptType: txOut.ScriptPubKey.Type,
	}
	// DecodeScript()
	rawScript, _ := hex.DecodeString(txOut.ScriptPubKey.Hex)

	tx, err := op.TxMan.WithKeys(minerKP).NewTx(minerKP.Address.EncodeAddress(), 0, utxo)
	assert.NoError(t, err)

	// _, err = op.TxMan.RPC().SendRawTransaction(tx.RawTX, false)
	// assert.NoError(t, err)
	// fmt.Printf("Sent tx %s at shard %d\n", tx.TxHash, shardID)

	vm, err := txscript.NewEngine(rawScript, tx.RawTX, 0,
		txscript.StandardVerifyFlags, nil, nil, 0)
	assert.NoError(t, err)

	err = vm.Execute()
	assert.NoError(t, err)
}

func TestTxValidation(ot *testing.T) {
	t := (*T)(ot)
	client, err := rpcclient.New(&rpcclient.ConnConfig{
		Params:       "fastnet",
		Host:         "127.0.0.1:18333",
		User:         "somerpc",
		Pass:         "somerpc",
		DisableTLS:   true,
		HTTPPostMode: true,
	}, nil)
	assert.NoError(t, err)

	encodedTx := "0300010002511428b26d2068c69abc4da2dce341b5707ebca6ac3dc4f5f2679835857a591800000000fd1b0100483045022100c1142c35f1fbf84baf1f829fb6152398bbc3425c41587363ffa180cc008c52630220798fd2a55bb64d29c7de75055a5b3943fe1bdb504d773a80b391d3b99aed175d01473044022036c07552b0a1680433931674ecf7f3f3436aa02d7020bea396ce7dacd42d6c2d022040b04a9e9f2e316aa6aa9dbfbb7b9dbafd160fd9ed975b69ca9e020e187f2197014c87524104e7a2fb1f4c1ff3b6da4f756c14e50c375dffb1d9bda1d90ae96e25847341d3ee36ba3fa698be0c07f67622febe229ea8e40e92d1b0b180672be47cd8d45b42994104486596c443d5b00f198474f47a5ce5498968e546114687b9d067e94d2d7d074b3654af29677cec36ce468c84d9612609786b955d13c2265bc28eda1a2d093e6152ae5000000002b870389525192e1a47eefdd85e9d13f51df6e9fc50777a6bad56e7cc4b2ea700000000fd1b0100473044022040e3adfc83b447f5fef9b5409271ea90e9235f0e8e8d00dbf9443c3193a6850a0220537245b66b194f0f20d3fdc4b9c12b077d9eba8ed3613507435ae3ae2223924501483045022100fe045ee09ff754cfa2fef8c4a34c0499c4cdc8a77b8b8f093824be22daaa0f6c022008a0b05f5a13d0608edd96bb85bb753e3d66a424a95087b06c11137ee6b137f3014c87524104e7a2fb1f4c1ff3b6da4f756c14e50c375dffb1d9bda1d90ae96e25847341d3ee36ba3fa698be0c07f67622febe229ea8e40e92d1b0b180672be47cd8d45b42994104486596c443d5b00f198474f47a5ce5498968e546114687b9d067e94d2d7d074b3654af29677cec36ce468c84d9612609786b955d13c2265bc28eda1a2d093e6152ae5000000002dcd1f505000000001976a914b2629111cf79c2f1cd025a7aebc403fc9bb5d48b88acdcd1f505000000001976a914cbcd1259bcd2c1147387e3536becf53d0cdd988188ac00000000"
	multiSigAddress := "2Mx9kFdzEY9hyR63LutcEfGdB7ypA5ijW1B"
	net := chaincfg.NetName("fastnet")
	tx, err := DecodeTx(encodedTx)
	assert.NoError(t, err)
	// assert.True(t, tx.LockTime > 0)
	assert.True(t, tx.SwapTx())
	assert.True(t, len(tx.TxIn) == 2)
	assert.True(t, len(tx.TxOut) == 2)

	for _, in := range tx.TxIn {
		scriptData, err := DecodeScript(in.SignatureScript, net.Params())
		assert.NoError(t, err)
		prettyPrint(scriptData)
		assert.Equal(ot, multiSigAddress, scriptData.P2sh)
		// timelock value
		assert.Equal(ot, 80, in.Sequence)

		fmt.Println(in.PreviousOutPoint.Hash.String(), in.PreviousOutPoint.Index)
		txOut, err := client.ForShard(1).GetTxOut(&in.PreviousOutPoint.Hash, in.PreviousOutPoint.Index, true)
		if err != nil || txOut == nil {
			txOut, err = client.ForShard(2).GetTxOut(&in.PreviousOutPoint.Hash, in.PreviousOutPoint.Index, true)
		}
		assert.NoError(t, err)
		assert.NotNil(t, txOut)
		assert.True(t, txOut.Value > 0)
		prettyPrint(txOut)

		assert.True(t, len(txOut.ScriptPubKey.Addresses) > 0)
		for _, address := range txOut.ScriptPubKey.Addresses {
			// validate that all addresses here
			fmt.Println(address)
		}
	}

	for _, out := range tx.TxOut {
		assert.True(t, out.Value > 0)
		scriptData, err := DecodeScript(out.PkScript, net.Params())
		assert.NoError(t, err)
		prettyPrint(scriptData)
		for _, address := range scriptData.Addresses {
			// validate that all addresses here
			fmt.Println(address)
		}
	}
}

func prettyPrint(val interface{}) {
	data, _ := json.MarshalIndent(val, "", "  ")
	fmt.Println(string(data))
}

func TestCheckIsSignedByPubKey(t *testing.T) {
	netName := chaincfg.NetName("fastnet")
	shardID := uint32(0)

	alice, err := GenerateKey(netName.Params())
	assert.NoError(t, err)

	bob, err := GenerateKey(netName.Params())
	assert.NoError(t, err)

	multisig, err := MakeMultiSigScript([]string{alice.AddressPubKey.String(), bob.AddressPubKey.String()}, 2, netName.Params())
	assert.NoError(t, err)

	script, err := txmodels.GetPayToAddressScript(multisig.Address, netName.Params())
	assert.NoError(t, err)

	tx, err := NewTxBuilder("fastnet").
		SetSenders(multisig.Address).
		AddRedeemScripts(multisig.RedeemScript).
		SetDestinationWithUTXO(alice.Address.EncodeAddress(), 10, txmodels.UTXORows{{
			ShardID:  shardID,
			Address:  multisig.Address,
			TxHash:   "8e8de99c0bf81f95b010e53f74bfd2c4d608227938f279954f062185be052cd6",
			Value:    10,
			PKScript: hex.EncodeToString(script),
		}}).
		IntoTx(func(shardID uint32) (int64, error) { return 0, nil }, alice)

	assert.NoError(t, err)

	var hasSignature bool
	hasSignature, err = txscript.CheckIsSignedByPubKey(tx, 0, multisig.RawRedeemScript, alice.AddressPubKey.PubKey())
	assert.NoError(t, err)
	assert.True(t, hasSignature)

	hasSignature, err = txscript.CheckIsSignedByPubKey(tx, 0, multisig.RawRedeemScript, bob.AddressPubKey.PubKey())
	assert.NoError(t, err)
	assert.False(t, hasSignature)

	script, err = txmodels.GetPayToAddressScript(alice.Address.EncodeAddress(), netName.Params())
	assert.NoError(t, err)

	out := tx.TxOut[0]
	tx, err = NewTxBuilder("fastnet").
		SetSenders(alice.Address.EncodeAddress()).
		SetDestinationWithUTXO(bob.Address.EncodeAddress(), 10, txmodels.UTXORows{{
			ShardID:  shardID,
			Address:  alice.Address.EncodeAddress(),
			TxHash:   tx.TxHash().String(),
			Value:    out.Value,
			PKScript: hex.EncodeToString(script),
		}}).
		IntoTx(func(shardID uint32) (int64, error) { return 0, nil }, alice)
	assert.NoError(t, err)

	hasSignature, err = txscript.CheckIsSignedByPubKey(tx, 0, script, alice.AddressPubKey.PubKey())
	assert.NoError(t, err)
	assert.True(t, hasSignature)

	hasSignature, err = txscript.CheckIsSignedByPubKey(tx, 0, script, bob.AddressPubKey.PubKey())
	assert.NoError(t, err)
	assert.False(t, hasSignature)
}
