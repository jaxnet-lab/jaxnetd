// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package txutils

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core/network/rpcclient"
	"gitlab.com/jaxnet/core/shard.core/network/rpcutli"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

// UTXOProvider is provider that cat give list of txmodels.UTXO for provided amount.
type UTXOProvider interface {
	// SelectForAmount returns a list of txmodels.UTXO which satisfies the passed amount.
	SelectForAmount(amount int64, shardID uint32, addresses ...string) (txmodels.UTXORows, error)
	// GetForAmount returns a single txmodels.UTXO with Value GreaterOrEq passed amount.
	GetForAmount(amount int64, shardID uint32, addresses ...string) (*txmodels.UTXO, error)
}

// UTXOFromCSV is an implementation of UTXOProvider that collects txmodels.UTXORows from CSV file,
// value of UTXOFromCSV is a path to file.
type UTXOFromCSV string

func (path UTXOFromCSV) GetForAmount(amount int64, shardID uint32, addresses ...string) (*txmodels.UTXO, error) {
	rows, err := storage.NewCSVStorage(string(path)).FetchData()
	if err != nil {
		return nil, err
	}
	collected := rows.GetSingle(amount, shardID)
	if collected == nil {
		return nil, fmt.Errorf("not found UTXO for amount (need %d)", amount)
	}
	return collected, nil
}

func (path UTXOFromCSV) SelectForAmount(amount int64, shardID uint32, addresses ...string) (txmodels.UTXORows, error) {
	rows, err := storage.NewCSVStorage(string(path)).FetchData()
	if err != nil {
		return nil, err
	}
	collected, change := rows.CollectForAmount(amount, shardID)
	if change > 0 {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, amount-change)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("not found UTXO for amount (need %d)", amount)
	}
	return collected, nil
}

// UTXOFromRows is a wrapper for txmodels.UTXORows to implement the UTXOProvider.
type UTXOFromRows txmodels.UTXORows

func (rows UTXOFromRows) GetForAmount(amount int64, shardID uint32, addresses ...string) (*txmodels.UTXO, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("not found UTXO for amount (need %d)", amount)
	}

	collected := txmodels.UTXORows(rows).GetSingle(amount, shardID)
	if collected == nil {
		return nil, fmt.Errorf("not found UTXO for amount (need %d)", amount)
	}

	return collected, nil
}
func (rows UTXOFromRows) SelectForAmount(amount int64, shardID uint32, addresses ...string) (txmodels.UTXORows, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("not found UTXO for amount (need %d)", amount)
	}

	collected, change := txmodels.UTXORows(rows).CollectForAmount(amount, shardID)
	if change > 0 {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, amount-change)
	}
	return collected, nil
}

// UTXOFromRows is a wrapper for txmodels.UTXO to implement the UTXOProvider.
type SingleUTXO txmodels.UTXO

func (row SingleUTXO) GetForAmount(amount int64, shardID uint32, addresses ...string) (*txmodels.UTXO, error) {
	if row.Value < amount {
		return nil, fmt.Errorf("not found UTXO for amount (need %d)", amount)
	}

	return (*txmodels.UTXO)(&row), nil
}

func (row SingleUTXO) SelectForAmount(amount int64, shardID uint32, addresses ...string) (txmodels.UTXORows, error) {
	if row.Value < amount {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, row.Value)
	}

	return []txmodels.UTXO{txmodels.UTXO(row)}, nil
}

// EncodeTx serializes and encodes to hex wire.MsgTx.
func EncodeTx(tx *wire.MsgTx) string {
	buf := bytes.NewBuffer([]byte{})
	_ = tx.Serialize(buf)
	return hex.EncodeToString(buf.Bytes())
}

// EncodeTx decodes hex-encoded wire.MsgTx.
func DecodeTx(hexTx string) (*wire.MsgTx, error) {
	raw, err := hex.DecodeString(hexTx)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(raw)

	tx := &wire.MsgTx{}
	err = tx.Deserialize(buf)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func TxToJson(mtx *wire.MsgTx, chainParams *chaincfg.Params) btcjson.TxRawDecodeResult {
	return btcjson.TxRawDecodeResult{
		Txid:     mtx.TxHash().String(),
		Version:  mtx.Version,
		Locktime: mtx.LockTime,
		Vin:      rpcutli.ToolsXt{}.CreateVinList(mtx),
		Vout:     rpcutli.ToolsXt{}.CreateVoutList(mtx, chainParams, nil),
	}
}

func SendTx(txMan *TxMan, senderKP *KeyData, shardID uint32, destination string, amount int64, timeLock uint32) (string, error) {
	senderAddress := senderKP.Address.EncodeAddress()
	senderUTXOIndex := storage.NewUTXORepo("", senderAddress)

	var err error
	// err = senderUTXOIndex.ReadIndex()
	// if err != nil {
	// 	return "", errors.Wrap(err, "unable to open UTXO index")
	// }

	err = senderUTXOIndex.CollectFromRPC(txMan.RPC(), shardID, map[string]bool{senderAddress: true})
	if err != nil {
		return "", errors.Wrap(err, "unable to collect UTXO")
	}

	tx, err := txMan.ForShard(shardID).WithKeys(senderKP).
		NewTx(destination, amount, &senderUTXOIndex)
	if err != nil {
		return "", errors.Wrap(err, "unable to create new tx")
	}
	if tx == nil || tx.RawTX == nil {
		return "", errors.New("tx empty")
	}
	_, err = txMan.RPC().ForShard(shardID).SendRawTransaction(tx.RawTX, true)
	if err != nil {
		return "", errors.Wrap(err, "unable to publish new tx")
	}
	// err = senderUTXOIndex.SaveIndex()
	// if err != nil {
	// 	return "", errors.Wrap(err, "unable to save UTXO index")
	// }
	// fmt.Printf("Sent tx %s at shard %d\n", tx.TxHash, shardID)
	return tx.TxHash, nil
}

func WaitForTx(rpcClient *rpcclient.Client, shardID uint32, txHash string, _ uint32) error {
	hash, _ := chainhash.NewHashFromStr(txHash)
	timer := time.NewTimer(time.Minute)

	for {
		select {
		case <-timer.C:
			return errors.New("tx waiting deadline")
		default:
			// wait for the transaction to be added to the block
			txDetails, err := rpcClient.ForShard(shardID).GetRawTransactionVerbose(hash)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			if txDetails != nil && txDetails.Confirmations > 2 {
				fmt.Printf("...tx %s mined in block @ %d shard\n", txHash, shardID)
				timer.Stop()
				return nil
			}

			time.Sleep(time.Second)
		}

	}
}
