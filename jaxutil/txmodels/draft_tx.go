// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txmodels

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type DraftTx struct {
	Amount         int64
	NetworkFee     int64
	UTXO           UTXORows
	ReceiverScript []byte
	DestAddress    string
}

// SetPayToAddress creates regular pay-to-address script.
// 	destAddress is hex-encoded jaxutil.Address
func (tx *DraftTx) SetPayToAddress(destAddress string, params *chaincfg.Params) error {
	var err error
	tx.DestAddress = destAddress
	tx.ReceiverScript, err = GetPayToAddressScript(destAddress, params)
	return err
}

// GetPayToAddressScript creates regular pay-to-address script.
// 	destAddress is hex-encoded jaxutil.Address
func GetPayToAddressScript(destAddress string, params *chaincfg.Params) ([]byte, error) {
	decodedDestAddr, err := jaxutil.DecodeAddress(destAddress, params)
	if err != nil {
		return nil, err
	}

	return txscript.PayToAddrScript(decodedDestAddr)
}

// SetMultiSig2of2 creates multiSig script, what can be spent only whist 2 of 2 signatures.
// 	firstPubKey and secondPubKey is hex-encoded jaxutil.AddressPubKey, NOT an address.
func (tx *DraftTx) SetMultiSig2of2(firstPubKey, secondPubKey *jaxutil.AddressPubKey, params *chaincfg.Params) error {
	pkScript, err := txscript.MultiSigScript([]*jaxutil.AddressPubKey{firstPubKey, secondPubKey}, 2, true)
	if err != nil {
		return err
	}

	scriptAddr, err := jaxutil.NewAddressScriptHash(pkScript, params)
	if err != nil {
		return err
	}

	tx.DestAddress = scriptAddr.EncodeAddress()
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
	ShardID     uint32 `json:"shard_id" csv:"shard_id"`
	SignedTx    string `json:"signed_tx" csv:"signed_tx"`

	RawTX *wire.MsgTx `json:"-" csv:"-"`
}

func (t *Transaction) UnmarshalBinary(data []byte) error {
	dest := new(gobTx)
	err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(dest)
	if err != nil {
		return err
	}

	t.TxHash = hex.EncodeToString(dest.TxHash)
	t.SignedTx = hex.EncodeToString(dest.SignedTx)

	t.Amount = dest.Amount
	t.ShardID = dest.ShardID
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
	dest := new(gobTx)

	dest.Amount = t.Amount
	dest.ShardID = t.ShardID
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

type SwapTransaction struct {
	TxHash       string         `json:"tx_hash" csv:"tx_hash"`
	Source       string         `json:"source" csv:"source"`
	Destinations map[string]int `json:"destinations" csv:"destinations"`
	SignedTx     string         `json:"signed_tx" csv:"signed_tx"`

	RawTX *wire.MsgTx `json:"-" csv:"-"`
}

func (t *SwapTransaction) UnmarshalBinary(data []byte) error {
	dest := new(gobSwapTx)
	err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(dest)
	if err != nil {
		return err
	}

	t.TxHash = hex.EncodeToString(dest.TxHash)
	t.SignedTx = hex.EncodeToString(dest.SignedTx)

	t.Destinations = dest.Destinations
	t.Source = string(dest.Source)

	t.RawTX = &wire.MsgTx{}
	err = t.RawTX.Deserialize(bytes.NewBuffer(dest.SignedTx))
	if err != nil {
		return err
	}

	return nil
}

func (t SwapTransaction) MarshalBinary() ([]byte, error) {
	var err error
	dest := new(gobSwapTx)

	dest.Source = []byte(t.Source)
	dest.Destinations = t.Destinations

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
	ShardID     uint32
	SignedTx    []byte
}

type gobSwapTx struct {
	TxHash       []byte
	Source       []byte
	Destinations map[string]int
	SignedTx     []byte
}
