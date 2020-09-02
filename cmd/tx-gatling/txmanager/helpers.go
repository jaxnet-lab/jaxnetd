package txmanager

import (
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmanager/models"
)

type UTXOCollector func(amount int64) (models.UTXORows, error)

func UTXOFromCSV(path string) UTXOCollector {
	return func(amount int64) (models.UTXORows, error) {
		rows, err := storage.NewCSVStorage(path).FetchData()
		if err != nil {
			return nil, err
		}

		return rows.CollectForAmount(amount), nil
	}
}

func CollectorFromRows(rows models.UTXORows) UTXOCollector {
	return func(amount int64) (models.UTXORows, error) {
		return rows.CollectForAmount(amount), nil
	}
}

func SingleUTXO(row models.UTXO) UTXOCollector {
	return func(amount int64) (models.UTXORows, error) {
		return []models.UTXO{row}, nil
	}
}
