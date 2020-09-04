package txmanager

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmanager/models"
	"gitlab.com/jaxnet/core/shard.core.git/wire"
)

// UTXOProvider is provider that cat give list of models.UTXO for provided amount.
type UTXOProvider interface {
	SelectForAmount(amount int64) (models.UTXORows, error)
}

// UTXOFromCSV is an implementation of UTXOProvider that collects models.UTXORows from CSV file,
// value of UTXOFromCSV is a path to file.
type UTXOFromCSV string

func (path UTXOFromCSV) SelectForAmount(amount int64) (models.UTXORows, error) {
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

// UTXOFromRows is a wrapper for models.UTXORows to implement the UTXOProvider.
type UTXOFromRows models.UTXORows

func (rows UTXOFromRows) SelectForAmount(amount int64) (models.UTXORows, error) {
	collected := models.UTXORows(rows).CollectForAmount(amount)
	sum := collected.GetSum()
	if sum < amount {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, sum)
	}
	return collected, nil
}

// UTXOFromRows is a wrapper for models.UTXO to implement the UTXOProvider.
type SingleUTXO models.UTXO

func (row SingleUTXO) SelectForAmount(amount int64) (models.UTXORows, error) {
	if row.Value < amount {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, row.Value)
	}

	return []models.UTXO{models.UTXO(row)}, nil
}

// EncodeTx serializes and encodes to hex wire.MsgTx.
func EncodeTx(tx *wire.MsgTx) string {
	buf := bytes.NewBuffer([]byte{})
	_ = tx.Serialize(buf)
	println("serialized tx")
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
