/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
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
// Seed investors       1407468  165nZP8b3dPqnyoPps14Vfy16VjAQpoSM9
// Seed investors       1407468  153d3mTjmY4Dq8AWoSeZH1oci4vS93Zd9f
// Seed investors       1407468  1EEkyTwn7nogqL7mPMrhqCK4DBs2N5KxgS
// Seed investors       1407468  19usb6HanE9nrQRVe7iamWH6hpBvWDiHyV
// Long term reserves   4128947  1EHFvipK4yTpY8R6huus5SESrZa7u4MEzC
// Long term reserves   4128947  1HarDhor5U659ReyfB2aoxj4fyPWXfBy5j
// Strategic Investors  2000000  13Srcs6QiUtnWL5MNKZXK55tLFukizgd2Q
// F&F                   868484  1AKZEe5KvSV8SuWwdsjbkUM1MyUV2qd4Ga
// Advisors              341750  1LJgnPz6TiDvvFPncwCcHXzPy4QpejmiwP
// Liquidity pool       2000000  1HLHHrmPQydjqQWJYvC97F1mmu5w4vhNJT
// OpEx Wallet          1750000  1DRv2NhFoWbg5steMp1XyxATAtBs382teh
// Team                  850500  1PAKJQ2HY36hJK4ugnCoSi1rqR66pU3VYN
// Team                  850500  1QEZYTtk3yBaG1yYebLYXWmqb2CZ1EQAaM
// OpEx Wallet #2      10000000  1MQa8D1ytfVTEcWmfj9gCoe5oAnHLW1yQM
// OpEx Wallet #3       1750000  17szxFDGkCSLKrkgBomDrhZfSbAfR37Smx
// Team                  850500  1L1ngxiSEKxtNNfTLjPmcNjX77itNs8rNC
// Team                  850500  1Syxe88RJZ4U7V6iwoBNqWzgAXmCsKHPk
// ----------------------------------------------------------------
//                     36000000

const genesisSequence = 0xffffffff

func main() {
	aux := extractBTCAux()
	fmt.Println("BTC AUX:>")
	spew.Dump(aux)
	auxData := bytes.NewBuffer(nil)

	err := aux.Serialize(auxData)
	if err != nil {
		fmt.Println(err)
		return
	}

	hash := aux.BlockHash()
	fmt.Println("HASH", hash)
	fmt.Println(hex.EncodeToString(auxData.Bytes()))

	script := []byte(inclusion)
	fmt.Println("JAX SIG_SCRIPT:>", err)
	spew.Dump(script)

	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{Index: genesisSequence},
				SignatureScript:  script,
				Sequence:         genesisSequence,
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
		{1407468, "165nZP8b3dPqnyoPps14Vfy16VjAQpoSM9"},
		// Seed investors
		{1407468, "153d3mTjmY4Dq8AWoSeZH1oci4vS93Zd9f"},
		// Seed investors
		{1407468, "1EEkyTwn7nogqL7mPMrhqCK4DBs2N5KxgS"},
		// Seed investors
		{1407468, "19usb6HanE9nrQRVe7iamWH6hpBvWDiHyV"},
		// Long term reserves
		{4128947, "1EHFvipK4yTpY8R6huus5SESrZa7u4MEzC"},
		// Long term reserves
		{4128947, "1HarDhor5U659ReyfB2aoxj4fyPWXfBy5j"},
		// Strategic Investors
		{2000000, "13Srcs6QiUtnWL5MNKZXK55tLFukizgd2Q"},
		// F&F
		{868484, "1AKZEe5KvSV8SuWwdsjbkUM1MyUV2qd4Ga"},
		// Advisors
		{341750, "1LJgnPz6TiDvvFPncwCcHXzPy4QpejmiwP"},
		// Liquidity pool
		{2000000, "1HLHHrmPQydjqQWJYvC97F1mmu5w4vhNJT"},
		// OpEx Wallet
		{1750000, "1DRv2NhFoWbg5steMp1XyxATAtBs382teh"},
		// Team
		{850500, "1PAKJQ2HY36hJK4ugnCoSi1rqR66pU3VYN"},
		// Team
		{850500, "1QEZYTtk3yBaG1yYebLYXWmqb2CZ1EQAaM"},
		// OpEx Wallet #2
		{10000000, "1MQa8D1ytfVTEcWmfj9gCoe5oAnHLW1yQM"},
		// OpEx Wallet #3
		{1750000, "17szxFDGkCSLKrkgBomDrhZfSbAfR37Smx"},
		// Team
		{850500, "1L1ngxiSEKxtNNfTLjPmcNjX77itNs8rNC"},
		// Team
		{850500, "1Syxe88RJZ4U7V6iwoBNqWzgAXmCsKHPk"},
	}

	var total int64
	var totalHaberStornetta int64
	fmt.Println()
	for _, pair := range distributionSet {
		address, err := jaxutil.DecodeAddress(pair.address, &chaincfg.MainNetParams)
		if err != nil {
			log.Fatal(err)
		}

		script, err := txscript.PayToAddrScript(address)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("-> %d\t%s\t%s\n", pair.amount, pair.address, hex.EncodeToString(script))

		tx.AddTxOut(&wire.TxOut{
			Value:    pair.amount * chaincfg.HaberStornettaPerJAXNETCoin,
			PkScript: script,
		})
		total += pair.amount
		totalHaberStornetta += pair.amount * chaincfg.HaberStornettaPerJAXNETCoin
	}

	fmt.Println("TOTAL AMOUNT:", total, totalHaberStornetta)
	fmt.Println()
	// spew.Dump(tx)
	hexTx, _ := tx.SerializeToHex()
	println(hexTx)
}

func extractBTCAux() wire.BTCBlockAux {
	var aux wire.BTCBlockAux

	hexStr, err := ioutil.ReadFile("./btc_block.json")
	if err != nil {
		log.Fatal(err.Error())
	}
	dest := struct {
		Result string `json:"result"`
	}{}
	err = json.Unmarshal(hexStr, &dest)
	if err != nil {
		log.Fatal(err.Error())
	}

	rawData, err := hex.DecodeString(dest.Result)
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

	return aux
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

const inclusion = `Sapere Aude JaxNet
We hope that by introducing a truly decentralized energy-standard monetary system, the JaxNet protocol will help to facilitate the transformation of humanity into a type-2 civilization and pave the way for an improved society that champions scientific advancement, justice, anti-corruption, universal basic income, a transaction-tax based government, and others. 
It's our solemn lifetime responsibility to defend our protocol, including our network tokens (JAXNET - JXN, JAX), and work together to bring about the change we wish to see.
On October 31, 2021, the 13th anniversary of the Bitcoin paper, we entered the race to the top...

Special credits to:
Iurii Shyshatskyi
Taras Emelyanenko
Lucas Leger
Mike Sheb
Dima Chizhevsky
Dr. Abdelhakim Senhaji Hafid
Satoshi Nakamoto
Dr. W. Scott Stornetta
Dr. Stuart Haber
Dr. Ralph C. Merkle

- aJaxPrime???`
