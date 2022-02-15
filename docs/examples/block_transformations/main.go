/*
 * Copyright (c) 2022 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package main

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	btcdblockchain "github.com/btcsuite/btcd/blockchain"
	btcdwire "github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/davecgh/go-spew/spew"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

//go:embed blockdata
var btcBlockData string // set block data first

func main() {
	var (
		// btc testnet 2140915
		prevHashStr   = "000000000000004d44ac8dacfad95e61283f1ea0864f0d8088c1318d9c672787"
		merkleRootStr = "0eed2dccdadf9b0f472e476e03cbccd4b973b7362cc49832eae159829cf11822"
		// btc testnet 2140916
		hashStr          = "0000000000000051c48b99694a65a54ee5bcc36881d241bd1913c2d68869c3ee"
		version   int32  = 0x20000000
		bits      uint32 = 0x1a00ac63
		nonce     uint32 = 0x235ed3a0
		timestamp int64  = 1644811503 // 2022-02-14 06:05:03 GMT +2
		txs              = []string{
			"ee19cc5e240456d27087eaaf1770b637852f777efc54847e773be171a44fbbbf", // coinbase
			"4a192844a39917bb6110362ae2ac4c823745c8356a05bce8b6c267a2f087be4e",
			"5b3a3089a05d17bda104f117727fa22a109c6e25b5773d7e0794f00ee3902df1",
			"8c26d14a2f0765741c9de103995e5e00c67064340333e48d8be16f3b141bb5ad",
			"862577320ef43a2bcc508302d48d8d067e7cf982afef2307a9c84c86e31f8c7e",
			"f35fe68be3d656ae8a9b5daebe0ab1b02c325a6fa52fb3896c6b40301b6cf3be",
			"f75039a997a8a199d0405aaf0b9c22f0389a2e726135711a3561618c785e64ab",
			"a7554369554635727499e752138a0305654633dfe3d710a0f4b2f8b5c839ee3b",
			"40b3f34b1e1862a1b1cd3937ad380b73d5b4aeac48a9ad9269a3b685e9d0968c",
			"d9ede95935ef2419ca3cc2ddb463a19f6380a8b160414dc094bfe1db19c0e330",
			"5d7d15e1229dc149db53865b1ce31cfbdf25f3175de39e0de0c2dd2757c3ebb8",
			"1c42ce44e4599780fff68a24d5c2b8df48ceb17dc85d7b287e2594481b653b6e",
			"685da1656856b8d29e9098550b72105b9220f9c503d783e04d3cb8cf9e88c681",
			"873ce4637483e9260dff8fc8969553e9ee7f7fd3cf2db13ccb14dc68daf9210b",
			"02fedef91a7b46bb5abe15c58fa875b748cdd01f2ca98d1a640f7ab3698d284b",
			"fb7b35971ec8a302511ca9872b114e80b60e61e123d56eb8d0d8eceac15ae6be",
			"e46d949057cb66b8fef286c6c082b3dbc47ac79a3f93c4312abb6cdb49022fc7",
			"129f3f7523c3198fc600917cc7b0acfb8fd790ed9dea7e7f32dcf6f39218653c",
		}
		// https://tbtc.bitaps.com/raw/transaction/ee19cc5e240456d27087eaaf1770b637852f777efc54847e773be171a44fbbbf
		coinbaseDataStrFromExp = "010000000" +
			"001010000000000000000000000000000000000000000000000000000000000000000ffffffff" +
			"5f03f4aa2008d00001014089a13d6a61786e6574e32f28773bb2869de8b8ed9d7c3a26b0289b9" +
			"63f16ba92521d00f25eccafead06a61786e6574fabe6d6dd4ee2c59448b9d4dc5039b5ea35943e" +
			"f518ea31fdd61000001e05437c80000000000ffffffff0500000000000000001976a9148dd3810" +
			"42ba3bec7ca76d83e890dece6b8cad40588aca25d4b000000000017a914011beb6fb8499e075a57" +
			"027fb0a58384f2d3f784870000000000000000266a24aa21a9edd960d12fcd5bae3184513d03828" +
			"259cd6b3dd50bf7153bf56d7498560d22edcb0000000000000000066a24b9e11b6d000000000000" +
			"00002b6a2952534b424c4f434b3ae0d268cd53b3ec94dd89e98e04acddeb99728a7d27c2f73ecb1" +
			"b4d00002794760120000000000000000000000000000000000000000000000000000000000000000000000000"
	)
	prevHash := newHashFromStr(prevHashStr)
	merkleRoot := newHashFromStr(merkleRootStr)

	btcHeader := wire.NewBTCBlockHeader(
		version,
		prevHash,
		merkleRoot,
		bits,
		nonce,
	)
	btcHeader.Timestamp = time.Unix(timestamp, 0)
	if btcHeader.BlockHash().String() != hashStr {
		interruptOnError(fmt.Errorf("block hash doesn't match: expected=%s, got=%s",
			hashStr, btcHeader.BlockHash()))
	}

	txHashes := make([]chainhash.Hash, len(txs))
	for i, tx := range txs {
		txHashes[i] = *newHashFromStr(tx)
	}

	calculatedMerkleRoot := chainhash.MerkleTreeRoot(txHashes)
	if !calculatedMerkleRoot.IsEqual(merkleRoot) {
		interruptOnError(fmt.Errorf("merkle root doesn't match: expected=%s, got=%s",
			merkleRoot, calculatedMerkleRoot))
	}
	coinbaseTx := new(wire.MsgTx)
	coinbaseData, _ := hex.DecodeString(coinbaseDataStrFromExp)
	err := coinbaseTx.Deserialize(bytes.NewBuffer(coinbaseData))
	interruptOnError(err)

	coinbaseHash := coinbaseTx.TxHash()
	if !coinbaseHash.IsEqual(&txHashes[0]) {
		spew.Dump(coinbaseTx)
		interruptOnError(fmt.Errorf("coinbase tx hash doesn't match: expected=%s, got=%s",
			txHashes[0], coinbaseHash))
	}
	in := coinbaseTx.TxIn[0]
	fmt.Println("original_signature_script_hex:", hex.EncodeToString(in.SignatureScript))
	chunks, _ := txscript.DisasmString(in.SignatureScript)
	fmt.Println("original_signature_script:", chunks)

	btcdBlock := new(btcdwire.MsgBlock)
	btcBlockData := strings.TrimSuffix(btcBlockData, "\n")
	err = btcdBlock.Deserialize(bytes.NewBuffer(hexToBytes(btcBlockData)))
	interruptOnError(err)

	btcdMerkleStore := btcdblockchain.BuildMerkleTreeStore(btcutil.NewBlock(btcdBlock).Transactions(), true)
	merkleStoreWitness := make([]*chainhash.Hash, len(btcdMerkleStore))
	for i := range btcdMerkleStore {
		merkleStoreWitness[i] = (*chainhash.Hash)(btcdMerkleStore[i])
	}

	var (
		nextHeight           int32 = 2148321
		fee                  int64 = 8_3000   // satoshi
		bitcoinTestnetReward int64 = 488_2812 // satoshi
		beaconExclusiveHash        = chainhash.HashH([]byte("only_for_documentation_and_tests"))
		mainnetAddress, _          = jaxutil.DecodeAddress("1JAXmGDsiE2CyK31dYZsMamM18pPebRDAk", &chaincfg.MainNetParams)
		testnetAddress, _          = jaxutil.DecodeAddress("mgWXizcMzrzSfqv6iPoPNSdFuVoWpJGgq4", &chaincfg.TestNetParams)
	)

	fmt.Println("mainnet:", hex.EncodeToString(mainnetAddress.ScriptAddress()))
	fmt.Println("testnet:", hex.EncodeToString(testnetAddress.ScriptAddress()))

	newCoinbaseTx, err := chaindata.CreateBitcoinCoinbaseTx(
		bitcoinTestnetReward,
		fee,
		nextHeight,
		mainnetAddress,
		beaconExclusiveHash.CloneBytes(),
		false,
	)
	interruptOnError(err)

	fmt.Println("FULL FEATURED BTC COINBASE burn=FALSE:")
	AddWitnessCommitment(newCoinbaseTx, merkleStoreWitness)
	printTxForDocs(newCoinbaseTx.MsgTx())

	newCoinbaseTx, err = chaindata.CreateBitcoinCoinbaseTx(
		bitcoinTestnetReward,
		fee,
		nextHeight,
		mainnetAddress,
		beaconExclusiveHash.CloneBytes(),
		true,
	)
	interruptOnError(err)
}

func printTxForDocs(tx *wire.MsgTx) {
	hexTx, _ := tx.SerializeToHex()
	fmt.Println("tx_hash:", tx.TxHash())
	fmt.Println("tx_hex:", hexTx)
	fmt.Printf("version: %0x\n", tx.Version)
	fmt.Printf("locktime: %0x\n", tx.LockTime)

	fmt.Println("tx_in: [")
	for i, in := range tx.TxIn {
		fmt.Printf("  %d: {\n", i)
		fmt.Println("    prev_out_hash:", in.PreviousOutPoint.Hash)
		fmt.Printf("    prev_out_id: %0x\n", in.PreviousOutPoint.Index)
		fmt.Println("    signature_script_hex:", hex.EncodeToString(in.SignatureScript))
		chunks, _ := txscript.DisasmString(in.SignatureScript)
		fmt.Println("    signature_script:", chunks)
		fmt.Printf("    sequence: %0x\n", in.Sequence)

		w := "["
		for n := range in.Witness {
			w += hex.EncodeToString(in.Witness[n])
			w += " "
		}
		w += "]"

		fmt.Println("    tx_witness_hex:", w)
		fmt.Println("  }")
	}
	fmt.Println("]")

	fmt.Println("tx_out: [")
	for i, out := range tx.TxOut {
		fmt.Printf("  %d: {\n", i)
		fmt.Printf("    value: %0x\n", out.Value)
		fmt.Println("    pk_script_hex:", hex.EncodeToString(out.PkScript))
		chunks, _ := txscript.DisasmString(out.PkScript)
		fmt.Println("    pk_script:", chunks)

		scriptClass, address, _, _ := txscript.ExtractPkScriptAddrs(out.PkScript, &chaincfg.TestNet3Params)

		addresses := "["
		for _, addr := range address {
			addresses += addr.EncodeAddress()
			addresses += " "
		}
		addresses += "]"
		fmt.Println("    pk_script_address:", addresses)
		fmt.Println("    script_class:", scriptClass.String())
		fmt.Println("  }")
	}
	fmt.Println("]")
}

// AddWitnessCommitment adds the witness commitment as an OP_RETURN outpout
// within the coinbase tx.  The raw commitment is returned.
// nolint: gocritic
func AddWitnessCommitment(coinbaseTx *jaxutil.Tx, witnessMerkleTree []*chainhash.Hash) {
	// The witness of the coinbase transaction MUST be exactly 32-bytes
	// of all zeroes.
	var witnessNonce [chaindata.CoinbaseWitnessDataLen]byte
	coinbaseTx.MsgTx().TxIn[0].Witness = wire.TxWitness{witnessNonce[:]}

	// Next, obtain the merkle root of a tree which consists of the
	// wtxid of all transactions in the block. The coinbase
	// transaction will have a special wtxid of all zeroes.
	witnessMerkleRoot := witnessMerkleTree[len(witnessMerkleTree)-1]

	// The preimage to the witness commitment is:
	// witnessRoot || coinbaseWitness
	var witnessPreimage [64]byte
	copy(witnessPreimage[:32], witnessMerkleRoot[:])
	copy(witnessPreimage[32:], witnessNonce[:])

	// The witness commitment itself is the double-sha256 of the
	// witness preimage generated above. With the commitment
	// generated, the witness script for the output is: OP_RETURN
	// OP_DATA_36 {0xaa21a9ed || witnessCommitment}. The leading
	// prefix is referred to as the "witness magic bytes".
	witnessCommitment := chainhash.DoubleHashB(witnessPreimage[:])
	witnessScript := append(chaindata.WitnessMagicBytes, witnessCommitment...)

	// Finally, create the OP_RETURN carrying witness commitment
	// output as an additional output within the coinbase.
	commitmentOutput := &wire.TxOut{
		Value:    0,
		PkScript: witnessScript,
	}

	coinbaseTx.MsgTx().AddTxOut(commitmentOutput)
}

func newHashFromStr(hexStr string) *chainhash.Hash {
	hash, _ := chainhash.NewHashFromStr(hexStr)
	return hash
}

func hexToBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic("invalid hex in source file: " + s)
	}
	return b
}

func interruptOnError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
