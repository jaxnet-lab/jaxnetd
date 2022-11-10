/*
 * Copyright (c) 2022 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package txutils

import (
	"encoding/hex"
	"math"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/jaxutil/txmodels"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

func TestRegularTx(t *testing.T) {
	net := chaincfg.NetName("mainnet")
	kp, _ := GenerateKey(net.Params(), true)
	secretKey := hex.EncodeToString(kp.PrivateKey.Serialize())

	type td struct {
		compressed bool
		witness    bool
	}
	utxoData := []td{
		{compressed: true, witness: false},
		{compressed: false, witness: false},
		{compressed: true, witness: true},
		{compressed: false, witness: true},
	}

	for i, data := range utxoData {
		var (
			net                = chaincfg.NetName("mainnet")
			inputAmount int64  = 950385
			fee         int64  = 385
			chainID     uint32 = math.MaxUint32
		)

		keys, err := NewKeyData(secretKey, net.Params(), data.compressed)
		if err != nil {
			t.Error("test_case", i, err)
			t.FailNow()
			return
		}

		var pkScript []byte
		var address string
		var scriptType string

		if data.witness {
			pkScript, err = txscript.PayToAddrScript(keys.WitnessAddress)
			if err != nil {
				t.Error("test_case", i, err)
				t.FailNow()
				return
			}
			address = keys.WitnessAddress.EncodeAddress()
			scriptType = txscript.WitnessV0PubKeyHashTy.String()
		} else {
			pkScript, err = txscript.PayToAddrScript(keys.AddressHash)
			if err != nil {
				t.Error("test_case", i, err)
				t.FailNow()
				return
			}
			address = keys.AddressHash.EncodeAddress()
			scriptType = txscript.PubKeyHashTy.String()
		}

		utxo := txmodels.UTXO{
			ShardID:    chainID,
			Address:    address,
			Height:     0,
			TxHash:     "583c8df06c9a2edbe8598b3b768baa0ab279140588caa126feee0c00fb5e8f18",
			OutIndex:   3,
			Value:      inputAmount,
			Used:       false,
			PKScript:   hex.EncodeToString(pkScript),
			ScriptType: scriptType,
		}

		msgTx, err := NewTxBuilder(net).
			SetShardID(chainID).
			SetChangeDestination(address).
			SetFee(chainID, fee).
			SetUTXOProvider(UTXOFromRows([]txmodels.UTXO{utxo})).
			SetDestination(address, 500000).
			SetDestination(address, 950385-500000-fee).
			IntoTx(func(shardID uint32) (int64, int64, error) { return fee, 0, nil }, keys)
		if err != nil {
			t.Error("test_case", i, err)
			t.FailNow()
			return
		}

		vm, err := txscript.NewEngine(pkScript, msgTx, 0,
			txscript.StandardVerifyFlags, nil, nil, inputAmount)
		if err != nil {
			t.Error("test_case", i, "unable to init txScript engine ", err)
			t.FailNow()
			return
		}

		if err = vm.Execute(); err != nil {
			t.Error("test_case", i, "tx script exec failed ", err)
			t.FailNow()
			return
		}
	}
}

func TestHTLCTx(t *testing.T) {
	net := chaincfg.NetName("mainnet")
	kp, _ := GenerateKey(net.Params(), true)
	secretKey := hex.EncodeToString(kp.PrivateKey.Serialize())

	type td struct {
		compressed bool
		witness    bool
	}
	utxoData := []td{
		{compressed: true, witness: false},
		{compressed: false, witness: false},
		{compressed: true, witness: true},
		{compressed: false, witness: true},
	}

	lockTime := int32(2000)
	for i, data := range utxoData {
		var (
			inputAmount int64  = 950385
			fee         int64  = 385
			chainID     uint32 = 1
		)

		keys, err := NewKeyData(secretKey, net.Params(), data.compressed)
		if err != nil {
			t.Error("test_case", i, err)
			t.FailNow()
			return
		}

		var pkScript []byte
		if data.witness {
			pkScript, err = txscript.HTLCScript(keys.WitnessAddress, lockTime)
			if err != nil {
				t.Error("test_case", i, err)
				t.FailNow()
				return
			}
		} else {
			pkScript, err = txscript.HTLCScript(keys.AddressHash, lockTime)
			if err != nil {
				t.Error("test_case", i, err)
				t.FailNow()
				return
			}
		}

		addr, _ := jaxutil.NewHTLCAddress(pkScript, &chaincfg.MainNetParams)

		utxo := txmodels.UTXO{
			ShardID:    chainID,
			Address:    addr.EncodeAddress(),
			Height:     0,
			TxHash:     "583c8df06c9a2edbe8598b3b768baa0ab279140588caa126feee0c00fb5e8f18",
			OutIndex:   3,
			Value:      inputAmount,
			Used:       false,
			PKScript:   hex.EncodeToString(pkScript),
			ScriptType: txscript.PubKeyHashTy.String(),
		}

		msgTx, err := NewTxBuilder(net).
			SetShardID(chainID).
			SetChangeDestination(addr.EncodeAddress()).
			SetFee(chainID, fee).
			SetSenders(addr.EncodeAddress()).
			SetUTXOProvider(UTXOFromRows([]txmodels.UTXO{utxo})).
			SetDestination(addr.EncodeAddress(), 500000).
			SetDestination(addr.EncodeAddress(), 950385-500000-fee).
			IntoTx(func(shardID uint32) (int64, int64, error) { return fee, 0, nil }, keys)
		if err != nil {
			t.Error("test_case", i, err)
			t.FailNow()
			return
		}

		vm, err := txscript.NewEngine(pkScript, msgTx, 0,
			txscript.StandardVerifyFlags, nil, nil, inputAmount)
		if err != nil {
			t.Error("test_case", i, "unable to init txScript engine ", err)
			t.FailNow()
			return
		}

		if err = vm.Execute(); err == nil {
			t.Error("test_case", i, "tx script exec failed ", err)
			t.FailNow()
			return
		}

		msgTx.TxIn[0].Age = lockTime + 1
		vm, err = txscript.NewEngine(pkScript, msgTx, 0,
			txscript.StandardVerifyFlags, nil, nil, inputAmount)
		if err != nil {
			t.Error("test_case", i, "unable to init txScript engine ", err)
			t.FailNow()
			return
		}

		if err = vm.Execute(); err != nil {
			t.Error("test_case", i, "tx script exec failed ", err)
			t.FailNow()
			return
		}
	}
}
