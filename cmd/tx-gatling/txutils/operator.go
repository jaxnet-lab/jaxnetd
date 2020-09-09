package txutils

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmodels"
)

type Operator struct {
	TxMan *TxMan
}

func NewOperator(config ManagerCfg) (Operator, error) {
	var err error
	op := Operator{}
	op.TxMan, err = NewTxMan(config)
	if err != nil {
		return op, errors.Wrap(err, "unable to init TxMan")
	}
	return op, nil
}

func (app *Operator) NewMultiSigTx(signer KeyData,
	dataFile, firstPubKey, secondPubKey string, amount int64) (*txmodels.Transaction, error) {
	repo := storage.NewCSVStorage(dataFile)
	utxo, err := repo.FetchData()
	if err != nil {
		return nil, cli.NewExitError(errors.Wrap(err, "unable to fetch UTXO"), 1)
	}

	rawFirstRecipient, err := hex.DecodeString(firstPubKey)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode first PubKey")
	}
	firstPK, err := btcutil.NewAddressPubKey(rawFirstRecipient, app.TxMan.NetParams)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode first PubKey")
	}

	rawSecondRecipient, err := hex.DecodeString(secondPubKey)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode second PubKey")
	}
	secondPK, err := btcutil.NewAddressPubKey(rawSecondRecipient, app.TxMan.NetParams)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode second PubKey")
	}

	app.TxMan.SetKey(&signer)
	tx, err := app.TxMan.NewMultiSig2of2Tx(
		firstPK,
		secondPK,
		amount,
		UTXOFromRows(utxo))
	if err != nil {
		return nil, errors.Wrap(err, "unable to create new multi sig tx")
	}

	return &tx, nil
}

func (app *Operator) AddSignatureToTx(signer KeyData, txBody string) (*txmodels.Transaction, error) {
	msgTx, err := DecodeTx(txBody)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode tx")
	}

	app.TxMan.SetKey(&signer)
	msgTx, err = app.TxMan.AddSignatureToTx(msgTx)
	if err != nil {
		return nil, errors.Wrap(err, "unable add signature")
	}

	return &txmodels.Transaction{
		TxHash:   msgTx.TxHash().String(),
		SignedTx: EncodeTx(msgTx),
		RawTX:    msgTx,
	}, nil
}

func (app *Operator) SpendUTXO(signer KeyData,
	txHash string, outIndex uint32, destination string) (*txmodels.Transaction, error) {
	hash, err := chainhash.NewHashFromStr(txHash)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode tx hash")
	}
	rawTx, err := app.TxMan.RPC.GetRawTransaction(hash)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get raw tx")
	}

	msgTx := rawTx.MsgTx()
	outValue := msgTx.TxOut[outIndex].Value
	outRawScript := msgTx.TxOut[outIndex].PkScript

	decodedScript, err := app.TxMan.DecodeScript(outRawScript)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode script")
	}

	utxo := txmodels.UTXO{
		TxHash:     msgTx.TxHash().String(),
		OutIndex:   outIndex,
		Value:      outValue,
		Used:       false,
		PKScript:   hex.EncodeToString(outRawScript),
		ScriptType: decodedScript.Type,
	}

	fee, err := app.TxMan.NetworkFee()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get network fee")
	}

	amountToSend := outValue - fee

	app.TxMan.SetKey(&signer)
	tx, err := app.TxMan.NewTx(destination, amountToSend, SingleUTXO(utxo))
	if err != nil {
		return nil, errors.Wrap(err, "new tx error")
	}

	return &tx, nil
}
