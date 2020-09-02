package models

import (
	"encoding/hex"

	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
	"gitlab.com/jaxnet/core/shard.core.git/wire"
)

type DraftTx struct {
	Amount         int64
	NetworkFee     int64
	UTXO           UTXORows
	ReceiverScript []byte
}

// SetPayToAddress creates regular pay-to-address script.
// 	address is hex-encoded btcutil.Address
func (tx *DraftTx) SetPayToAddress(address string, params *chaincfg.Params) error {
	destinationAddress, err := btcutil.DecodeAddress(address, params)
	if err != nil {
		return err
	}

	tx.ReceiverScript, err = txscript.PayToAddrScript(destinationAddress)
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

	tx.ReceiverScript, err = txscript.PayToAddrScript(scriptAddr)
	if err != nil {
		return err
	}
	return nil
}

func (tx DraftTx) Destination() string {
	return hex.EncodeToString(tx.ReceiverScript)
}

type Transaction struct {
	TxHash      string      `json:"tx_hash" csv:"tx_hash"`
	Source      string      `json:"source" csv:"source"`
	Destination string      `json:"destination" csv:"destination"`
	Amount      int64       `json:"amount" csv:"amount"`
	UnsignedTx  string      `json:"unsigned_tx" csv:"unsigned_tx"`
	SignedTx    string      `json:"signed_tx" csv:"signed_tx"`
	RawTX       *wire.MsgTx `json:"-" csv:"-"`
}
