package txmanager

import (
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmanager/models"
)

type UTXOProvider interface {
	SelectForAmount(amount int64) (models.UTXORows, error)
}

type UTXOFromCSV string

func (path UTXOFromCSV) SelectForAmount(amount int64) (models.UTXORows, error) {
	rows, err := storage.NewCSVStorage(string(path)).FetchData()
	if err != nil {
		return nil, err
	}

	return rows.CollectForAmount(amount), nil
}

type UTXOFromRows models.UTXORows

func (rows UTXOFromRows) SelectForAmount(amount int64) (models.UTXORows, error) {
	return models.UTXORows(rows).CollectForAmount(amount), nil
}

type SingleUTXO models.UTXO

func (row SingleUTXO) SelectForAmount(amount int64) (models.UTXORows, error) {
	return []models.UTXO{models.UTXO(row)}, nil
}
