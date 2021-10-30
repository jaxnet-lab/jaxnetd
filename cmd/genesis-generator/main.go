/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"

	btcdwire "github.com/btcsuite/btcd/wire"
	"github.com/davecgh/go-spew/spew"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// Initial Distribution:
// ----------------------------------------------------------------
// Seed investors       1407468  1G2nSCLFVqdztGdtzHWZyxwf4kfFyLAm6a
// Seed investors       1407468  1FH3DXshoSx2sZB3twFqanjMWHFQA3UPj
// Seed investors       1407468  1PDgiorHZcogRU4u6Tvi19iTvogxrD7skZ
// Seed investors       1407468  1G4VQvdh1NkkA7mqRaJEvmeSvunEJsE631
// Long term reserves   4128948  1G4VQvdh1NkkA7mqRaJEvmeSvunEJsE631
// Long term reserves   4128948  1JKETv3sqnSznHKc3DzX4y1oTwBkwBXaPK
// Strategic Investors  2000000  1GHBqYN8fHHMeSBfy74aLQUzasi9caUPRS
// F&F                   868484  1ihL7yfN5oZBAy6etZZjs974Ddc1z2GWe
// Advisors              341750  14MYxbip8NiopYJunfV9tx831tHBk6YUCx
// Liquidity pool       2000000  1PYrpwFkW7tCgB8tfqh9JKJEfJBxwKc7xV
// OpEx Wallet          1750000  13D72pswMbKxCiiDcgAz2DsmLb4qmYaZjq
// Team                  850500  15nQrfKrfxnMSBCgYsia3p2YUj1HSvf7QE
// Team                  850500  12t9VZ9gx3b81kd6hTLs8f1N96p9oGoQxb
// OpEx Wallet #2      10000000  1MmUCgFWnGivbtGsTgyEzCB69QFb6JCt4m
// OpEx Wallet #3       1750000  15ShmG5K7dudRvCSRXT55RzRs43YBw133V
// Team                  850500  15NYXZ6CQFhm2mPJmtMy8i6c3Sz4B6mZrU
// Team                  850500  159qkF9aH3ZuLHphuawfvXFtA2HYnnJyND
// ----------------------------------------------------------------
//                     36000000

func main() {
	aux := extractBTCAux()
	fmt.Println("BTC AUX:>")
	spew.Dump(aux)

	hash := aux.BlockHash()

	script, _ := txscript.NewScriptBuilder().
		AddData(hash[:]).
		AddData([]byte("Jax.Network enters the race! ")).
		AddData([]byte{0x73, 0x68}).
		Script()

	fmt.Println("JAX SIG_SCRIPT:>")
	spew.Dump(script)

	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{Index: 0xffffffff},
				SignatureScript:  script,
				Sequence:         0xffffffff,
			},
		},
		LockTime: 0,
	}

	type set struct {
		amount  int64
		address string
	}

	distributionSet := []set{
		// Seed investors
		{1407468, "1G2nSCLFVqdztGdtzHWZyxwf4kfFyLAm6a"},
		// Seed investors
		{1407468, "1FH3DXshoSx2sZB3twFqanjMWHFQA3UPj"},
		// Seed investors
		{1407468, "1PDgiorHZcogRU4u6Tvi19iTvogxrD7skZ"},
		// Seed investors
		{1407468, "1G4VQvdh1NkkA7mqRaJEvmeSvunEJsE631"},
		// Long term reserves
		{4128948, "1G4VQvdh1NkkA7mqRaJEvmeSvunEJsE631"},
		// Long term reserves
		{4128948, "1JKETv3sqnSznHKc3DzX4y1oTwBkwBXaPK"},
		// Strategic Investors
		{2000000, "1GHBqYN8fHHMeSBfy74aLQUzasi9caUPRS"},
		// F&F
		{868484, "1ihL7yfN5oZBAy6etZZjs974Ddc1z2GWe"},
		// Advisors
		{341750, "14MYxbip8NiopYJunfV9tx831tHBk6YUCx"},
		// Liquidity pool
		{2000000, "1PYrpwFkW7tCgB8tfqh9JKJEfJBxwKc7xV"},
		// OpEx Wallet
		{1750000, "13D72pswMbKxCiiDcgAz2DsmLb4qmYaZjq"},
		// Team
		{850500, "15nQrfKrfxnMSBCgYsia3p2YUj1HSvf7QE"},
		// Team
		{850500, "12t9VZ9gx3b81kd6hTLs8f1N96p9oGoQxb"},
		// OpEx Wallet #2
		{10000000, "1MmUCgFWnGivbtGsTgyEzCB69QFb6JCt4m"},
		// OpEx Wallet #3
		{1750000, "15ShmG5K7dudRvCSRXT55RzRs43YBw133V"},
		// Team
		{850500, "15NYXZ6CQFhm2mPJmtMy8i6c3Sz4B6mZrU"},
		// Team
		{850500, "159qkF9aH3ZuLHphuawfvXFtA2HYnnJyND"},
	}

	for _, pair := range distributionSet {
		address, _ := jaxutil.DecodeAddress(pair.address, &chaincfg.MainNetParams)
		script, _ := txscript.PayToAddrScript(address)
		tx.AddTxOut(&wire.TxOut{
			Value:    pair.amount * chaincfg.HaberStornettaPerJAXNETCoin,
			PkScript: script,
		})
	}

	spew.Dump(tx)
	hexTx, _ := tx.SerializeToHex()
	println(hexTx)
}

func extractBTCAux() (aux wire.BTCBlockAux) {
	hexStr, err := ioutil.ReadFile("./btc_block")
	if err != nil {
		log.Fatal(err.Error())
	}

	rawData, err := hex.DecodeString(string(hexStr))
	if err != nil {
		log.Fatal(err.Error())
	}

	btcBlock := btcdwire.MsgBlock{}
	err = btcBlock.Deserialize(bytes.NewBuffer(rawData))
	if err != nil {
		log.Fatal(err.Error())
	}

	spew.Dump(btcBlock.Header)

	aux = BtcBlockToBlockAux(&btcBlock)

	fmt.Println(aux.BlockHash())
	fmt.Println(btcBlock.BlockHash())
	err = aux.CoinbaseAux.Validate(aux.MerkleRoot)
	if err != nil {
		log.Fatal(err.Error())
	}

	return
}

func BtcBlockToBlockAux(btcBlock *btcdwire.MsgBlock) wire.BTCBlockAux {
	btcCoinbaseTx := btcBlock.Transactions[0].Copy()
	coinbaseTx := BtcTxToJaxTx(btcCoinbaseTx)

	txHashSet := CollectTxHashes(btcBlock.Transactions, false)

	return wire.BTCBlockAux{
		Version:    btcBlock.Header.Version,
		PrevBlock:  chainhash.Hash(btcBlock.Header.PrevBlock),
		MerkleRoot: chainhash.Hash(btcBlock.Header.MerkleRoot),
		Timestamp:  btcBlock.Header.Timestamp,
		Bits:       btcBlock.Header.Bits,
		Nonce:      btcBlock.Header.Nonce,
		CoinbaseAux: wire.CoinbaseAux{
			Tx:            coinbaseTx,
			TxMerkleProof: chainhash.BuildCoinbaseMerkleTreeProof(txHashSet),
		},
	}
}

func CollectTxHashes(transactions []*btcdwire.MsgTx, witness bool) []chainhash.Hash {
	hashes := make([]chainhash.Hash, len(transactions))

	// Create the base transaction hashes and populate the array with them.
	for i, tx := range transactions {
		// If we're computing a witness merkle root, instead of the
		// regular txid, we use the modified wtxid which includes a
		// transaction's witness data within the digest. Additionally,
		// the coinbase's wtxid is all zeroes.
		switch {
		case witness && i == 0:
			var zeroHash chainhash.Hash
			hashes[i] = zeroHash
		case witness:
			wSha := tx.WitnessHash()
			hashes[i] = chainhash.Hash(wSha)
		default:
			hashes[i] = chainhash.Hash(tx.TxHash())
		}
	}

	return hashes
}
func BtcTxToJaxTx(tx *btcdwire.MsgTx) wire.MsgTx {
	// tx := btcBlock.Transactions[0].Copy()
	msgTx := wire.MsgTx{
		Version:  tx.Version,
		TxIn:     make([]*wire.TxIn, len(tx.TxIn)),
		TxOut:    make([]*wire.TxOut, len(tx.TxOut)),
		LockTime: tx.LockTime,
	}

	for i := range msgTx.TxIn {
		msgTx.TxIn[i] = &wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash(tx.TxIn[i].PreviousOutPoint.Hash),
				Index: tx.TxIn[i].PreviousOutPoint.Index,
			},
			SignatureScript: tx.TxIn[i].SignatureScript,
			Witness:         wire.TxWitness(tx.TxIn[i].Witness),
			Sequence:        tx.TxIn[i].Sequence,
		}
	}

	for i := range msgTx.TxOut {
		msgTx.TxOut[i] = &wire.TxOut{
			Value:    tx.TxOut[i].Value,
			PkScript: tx.TxOut[i].PkScript,
		}
	}
	return msgTx
}
