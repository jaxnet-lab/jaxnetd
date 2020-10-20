// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package txmodels

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/txscript"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type DraftTx struct {
	Amount         int64
	NetworkFee     int64
	UTXO           UTXORows
	ReceiverScript []byte
	Address        string
}

// SetPayToAddress creates regular pay-to-address script.
// 	destAddress is hex-encoded btcutil.Address
func (tx *DraftTx) SetPayToAddress(destAddress string, params *chaincfg.Params) error {
	decodedDestAddr, err := btcutil.DecodeAddress(destAddress, params)
	if err != nil {
		return err
	}

	tx.Address = destAddress
	tx.ReceiverScript, err = txscript.PayToAddrScript(decodedDestAddr)
	if err != nil {
		return err
	}
	return nil
}

// SetMultiSig2of2 creates multiSig script, what can be spent only whist 2 of 2 signatures.
// 	firstPubKey and secondPubKey is hex-encoded btcutil.AddressPubKey, NOT an address.
func (tx *DraftTx) SetMultiSig2of2(firstPubKey, secondPubKey *btcutil.AddressPubKey, params *chaincfg.Params) error {
	pkScript, err := txscript.MultiSigScript([]*btcutil.AddressPubKey{firstPubKey, secondPubKey}, 2)
	if err != nil {
		return err
	}

	scriptAddr, err := btcutil.NewAddressScriptHash(pkScript, params)
	if err != nil {
		return err
	}

	tx.Address = scriptAddr.EncodeAddress()
	tx.ReceiverScript = pkScript
	return nil
}

func (tx DraftTx) Destination() string {
	return hex.EncodeToString(tx.ReceiverScript)
}

type Transaction struct {
	TxHash      string `json:"tx_hash" csv:"tx_hash"`
	Source      string `json:"source" csv:"source"`
	Destination string `json:"destination" csv:"destination"`
	Amount      int64  `json:"amount" csv:"amount"`
	SignedTx    string `json:"signed_tx" csv:"signed_tx"`

	RawTX *wire.MsgTx `json:"-" csv:"-"`
}

func (t *Transaction) UnmarshalBinary(data []byte) error {
	var dest = new(gobTx)
	err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(dest)
	if err != nil {
		return err
	}

	t.TxHash = hex.EncodeToString(dest.TxHash)
	t.SignedTx = hex.EncodeToString(dest.SignedTx)

	t.Amount = dest.Amount
	t.Destination = string(dest.Destination)
	t.Source = string(dest.Source)

	t.RawTX = &wire.MsgTx{}
	err = t.RawTX.Deserialize(bytes.NewBuffer(dest.SignedTx))
	if err != nil {
		return err
	}

	return nil
}

func (t Transaction) MarshalBinary() ([]byte, error) {
	var err error
	var dest = new(gobTx)

	dest.Amount = t.Amount
	dest.Source = []byte(t.Source)
	dest.Destination = []byte(t.Destination)

	dest.TxHash, err = hex.DecodeString(t.TxHash)
	if err != nil {
		return nil, err
	}

	dest.SignedTx, err = hex.DecodeString(t.SignedTx)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	err = gob.NewEncoder(buf).Encode(dest)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type gobTx struct {
	TxHash      []byte
	Source      []byte
	Destination []byte
	Amount      int64
	SignedTx    []byte
}
