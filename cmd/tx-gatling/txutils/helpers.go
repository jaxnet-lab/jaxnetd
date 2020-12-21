// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package txutils

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core/network/rpcutli"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

// UTXOProvider is provider that cat give list of txmodels.UTXO for provided amount.
type UTXOProvider interface {
	SelectForAmount(amount int64, shardID uint32, addresses ...string) (txmodels.UTXORows, error)
}

// UTXOFromCSV is an implementation of UTXOProvider that collects txmodels.UTXORows from CSV file,
// value of UTXOFromCSV is a path to file.
type UTXOFromCSV string

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

func (rows UTXOFromRows) SelectForAmount(amount int64, shardID uint32, addresses ...string) (txmodels.UTXORows, error) {
	collected, change := txmodels.UTXORows(rows).CollectForAmount(amount, shardID)
	if change > 0 {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, amount-change)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("not found UTXO for amount (need %d)", amount)
	}
	return collected, nil
}

// UTXOFromRows is a wrapper for txmodels.UTXO to implement the UTXOProvider.
type SingleUTXO txmodels.UTXO

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
