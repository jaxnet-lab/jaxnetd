package storage

import (
	"os"

	"github.com/gocarina/gocsv"
)

type CSVStorage struct {
	path string
	file *os.File
}

func NewCSVStorage(path string, append bool) *CSVStorage {
	mode := os.O_RDWR | os.O_CREATE
	if append {
		// mode |= os.O_APPEND
	} else {
		// mode |= os.O_TRUNC
	}

	file, err := os.OpenFile(path, mode, 0644)
	if os.IsPermission(err) {
		file, err = os.Create(path)
	}

	storage := &CSVStorage{
		path: path,
		file: file,
	}

	if !append {
		_ = storage.WriteHeader()
	}

	return storage
}

func (storage *CSVStorage) Shutdown() {
	_ = storage.file.Close()
}

func (storage *CSVStorage) FetchData() (UTXORows, error) {
	rows := make([]UTXO, 0)
	err := gocsv.UnmarshalFile(storage.file, &rows)
	return rows, err
}

func (storage *CSVStorage) SaveRows(rows []UTXO) error {
	return gocsv.MarshalFile(rows, storage.file)
}

func (storage *CSVStorage) WriteHeader() error {
	writer := gocsv.DefaultCSVWriter(storage.file)
	err := writer.Write([]string{
		"address",
		"height",
		"tx_hash",
		"out_index",
		"value",
	})
	if err != nil {
		return err
	}

	writer.Flush()
	return nil
}
