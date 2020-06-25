package main

import (
	"bytes"
	"encoding/hex"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
	"gitlab.com/jaxnet/core/shard.core.git/wire"
	"log"
)

const (
	txFee       = 10000
	fromAddress = "mijhw2WHeqgimoTqoKMWSCRVs8XFXxk9qx"
	toAddress   = "muph2LMYEHiTUuCC9FusNJ8aWyjySJ9srB"
	fromWIF     = "5Jg5fEQHNF385G1vQunCzBAC9rKakKAqgfVuFet6DN6J32qsmnL"
)

type utxo struct {
	Address     string
	TxID        string
	OutputIndex uint32
	Script      string
	Satoshis    int64
	Height      int64
}

func main() {
	//params := &chaincfg.MainNetParams
	//params.Name = "jaxnet"
	//params.Net = wire.BitcoinNet(0x0B110907)
	//params.PubKeyHashAddrID = byte(0x6F)
	//params.PrivateKeyID = byte(0x80)

	toAddress, err := btcutil.DecodeAddress(toAddress, &chaincfg.JaxNetParams)
	if err != nil {
		log.Fatalf("invalid address: %v", err)
	}

	unspentTx := utxo{
		Address:     "1CzWYLMRUt2dUnvirRtS68vQjgVjYDxsZh",
		TxID:        "5de1f708644f269f4fd87c648cc5d67cac64ebcf5b743f5d5063a141d6a01f14",
		OutputIndex: 0,
		Script:      "76a9142351cbad27a2607960ba370dccd1400c481230fa88ac",
		Satoshis:    8125000,
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.

	hash, err := chainhash.NewHashFromStr(unspentTx.TxID)
	if err != nil {
		log.Fatalf("could not get hash from transaction ID: %v", err)
	}

	outPoint := wire.NewOutPoint(hash, unspentTx.OutputIndex)

	txIn := wire.NewTxIn(outPoint, nil, nil)
	tx.AddTxIn(txIn)

	script, err := hex.DecodeString(unspentTx.Script)
	if err != nil {
		log.Fatalf("could not decode the script: %v", err)
	}

	oldTxOut := wire.NewTxOut(unspentTx.Satoshis, script)
	//oldTxOut := txOut

	// Pay the minimum network fee so that nodes will broadcast the tx.
	outCoin := oldTxOut.Value - txFee

	script, err = txscript.PayToAddrScript(toAddress)
	if err != nil {
		log.Fatalf("could not get pay to address script: %v", err)
	}

	txOut := wire.NewTxOut(outCoin, script)
	tx.AddTxOut(txOut)

	wif, err := btcutil.DecodeWIF(fromWIF)
	if err != nil {
		log.Fatalf("could not decode wif: %v", err)
	}

	sig, err := txscript.SignatureScript(
		tx,                  // The tx to be signed.
		0,                   // The index of the txin the signature is for.
		txOut.PkScript,      // The other half of the script from the PubKeyHash.
		txscript.SigHashAll, // The signature flags that indicate what the sig covers.
		wif.PrivKey,         // The key to generate the signature with.
		true,                // The compress sig flag. This saves space on the blockchain.
	)
	if err != nil {
		log.Fatalf("could not generate signature: %v", err)
	}

	tx.TxIn[0].SignatureScript = sig

	log.Printf("signed raw transaction: %s", txToHex(tx))
}

func txToHex(tx *wire.MsgTx) string {
	buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
	tx.Serialize(buf)
	return hex.EncodeToString(buf.Bytes())
}
