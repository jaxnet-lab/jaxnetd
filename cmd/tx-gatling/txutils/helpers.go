package txutils

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core.git/wire"
)

// UTXOProvider is provider that cat give list of txmodels.UTXO for provided amount.
type UTXOProvider interface {
	SelectForAmount(amount int64) (txmodels.UTXORows, error)
}

// UTXOFromCSV is an implementation of UTXOProvider that collects txmodels.UTXORows from CSV file,
// value of UTXOFromCSV is a path to file.
type UTXOFromCSV string

func (path UTXOFromCSV) SelectForAmount(amount int64) (txmodels.UTXORows, error) {
	rows, err := storage.NewCSVStorage(string(path)).FetchData()
	if err != nil {
		return nil, err
	}
	collected := rows.CollectForAmount(amount)
	sum := collected.GetSum()
	if sum < amount {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, sum)
	}
	return collected, nil
}

// UTXOFromRows is a wrapper for txmodels.UTXORows to implement the UTXOProvider.
type UTXOFromRows txmodels.UTXORows

func (rows UTXOFromRows) SelectForAmount(amount int64) (txmodels.UTXORows, error) {
	collected := txmodels.UTXORows(rows).CollectForAmount(amount)
	sum := collected.GetSum()
	if sum < amount {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, sum)
	}
	return collected, nil
}

// UTXOFromRows is a wrapper for txmodels.UTXO to implement the UTXOProvider.
type SingleUTXO txmodels.UTXO

func (row SingleUTXO) SelectForAmount(amount int64) (txmodels.UTXORows, error) {
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
