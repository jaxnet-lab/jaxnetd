// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package txutils

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
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

func (app *Operator) AddSignatureToTx(signer KeyData, txBody, redeemScript string) (*txmodels.Transaction, error) {
	msgTx, err := DecodeTx(txBody)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode tx")
	}

	msgTx, err = app.TxMan.WithKeys(&signer).AddSignatureToTx(msgTx, redeemScript)
	if err != nil {
		return nil, errors.Wrap(err, "unable add signature")
	}

	return &txmodels.Transaction{
		TxHash:   msgTx.TxHash().String(),
		SignedTx: EncodeTx(msgTx),
		RawTX:    msgTx,
	}, nil
}

func (app *Operator) NewMultiSigSpendTx(signer KeyData,
	txHash, redeemScript string, outIndex uint32, destination string, amount int64) (*txmodels.Transaction, error) {
	utxo, err := app.UTXOByHash(txHash, outIndex, redeemScript)
	if err != nil {
		return nil, err
	}

	tx, err := app.TxMan.WithKeys(&signer).NewTx(destination, amount, SingleUTXO(*utxo))
	if err != nil {
		return nil, errors.Wrap(err, "new tx error")
	}

	return tx, nil
}

// SpendUTXO creates new transaction that spends UTXO with 'outIndex' from transaction with 'txHash'.
// Into new transactions will be added one Input and one or two Outputs, if 'amount' less than UTXO value.
func (app *Operator) SpendUTXO(signer KeyData,
	txHash string, outIndex uint32, destination string, amount int64) (*txmodels.Transaction, error) {
	utxo, err := app.UTXOByHash(txHash, outIndex, "")
	if err != nil {
		return nil, err
	}

	tx, err := app.TxMan.WithKeys(&signer).NewTx(destination, amount, SingleUTXO(*utxo))
	if err != nil {
		return nil, errors.Wrap(err, "new tx error")
	}

	return tx, nil
}

func (app *Operator) UTXOByHash(txHash string, outIndex uint32, redeemScript string) (*txmodels.UTXO, error) {
	hash, err := chainhash.NewHashFromStr(txHash)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode tx hash")
	}
	rawTx, err := app.TxMan.RPC.
		ForShard(app.TxMan.cfg.ShardID).
		GetRawTransaction(hash)
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

	utxo := &txmodels.UTXO{
		TxHash:     msgTx.TxHash().String(),
		OutIndex:   outIndex,
		Value:      outValue,
		Used:       false,
		PKScript:   hex.EncodeToString(outRawScript),
		ScriptType: decodedScript.Type,
	}

	if redeemScript != "" {
		rawScript, err := hex.DecodeString(redeemScript)
		if err != nil {
			return nil, errors.Wrap(err, "unable to decode hex script")
		}

		script, err := app.TxMan.DecodeScript(rawScript)
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse script")
		}

		utxo.PKScript = redeemScript
		utxo.ScriptType = script.Type
	}

	return utxo, nil
}
