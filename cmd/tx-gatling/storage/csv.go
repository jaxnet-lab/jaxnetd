package storage

import (
	"os"

	"github.com/gocarina/gocsv"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
)

type CSVStorage struct {
	path string
	file *os.File
}

func NewCSVStorage(path string) *CSVStorage {
	return &CSVStorage{path: path}
}

func (storage *CSVStorage) open(readOnly, truncate bool) error {
	mode := os.O_RDWR | os.O_CREATE
	if truncate {
		mode |= os.O_TRUNC
	}

	if readOnly {
		mode = os.O_RDONLY
	}

	file, err := os.OpenFile(storage.path, mode, 0644)
	if os.IsPermission(err) {
		file, err = os.Create(storage.path)
	}

	storage.file = file
	return err
}

func (storage *CSVStorage) Close() {
	if storage.file != nil {
		_ = storage.file.Close()
	}
}

func (storage *CSVStorage) FetchData() (txmodels.UTXORows, error) {
	if err := storage.open(true, false); err != nil {
		return nil, err
	}
	defer storage.Close()

	rows := make([]txmodels.UTXO, 0)
	err := gocsv.UnmarshalFile(storage.file, &rows)
	return rows, err
}

func (storage *CSVStorage) SaveRows(rows []txmodels.UTXO) error {
	if err := storage.open(false, true); err != nil {
		return err
	}
	defer storage.Close()

	return gocsv.MarshalFile(rows, storage.file)
}
