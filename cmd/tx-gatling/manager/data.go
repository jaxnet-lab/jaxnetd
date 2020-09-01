package manager

import (
	"encoding/hex"

	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
)

type DraftTx struct {
	Amount         int64
	NetworkFee     int64
	UTXO           storage.UTXORows
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
func (tx *DraftTx) SetMultiSig2of2(firstPubKey, secondPubKey string, params *chaincfg.Params) error {
	firstPubKeyRaw, err := hex.DecodeString(firstPubKey)
	if err != nil {
		return err
	}

	fistAddress, err := btcutil.NewAddressPubKey(firstPubKeyRaw, params)
	if err != nil {
		return err
	}

	secondPubKeyRaw, err := hex.DecodeString(secondPubKey)
	if err != nil {
		return err
	}
	secondAddress, err := btcutil.NewAddressPubKey(secondPubKeyRaw, params)
	if err != nil {
		return err
	}

	pkScript, err := txscript.MultiSigScript([]*btcutil.AddressPubKey{fistAddress, secondAddress}, 2)
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

type sign2of2TxData struct {
	Amount         int64
	NetworkFee     int64
	FistSigner     string
	SecondSigner   string
	UTXO           storage.UTXORows
	ReceiverScript []byte
}

type Transaction struct {
	TxId        string `json:"tx_id" csv:"tx_id"`
	Source      string `json:"source" csv:"source"`
	Destination string `json:"destination" csv:"destination"`
	Amount      int64  `json:"amount" csv:"amount"`
	UnsignedTx  string `json:"unsigned_tx" csv:"unsigned_tx"`
	SignedTx    string `json:"signed_tx" csv:"signed_tx"`
}
