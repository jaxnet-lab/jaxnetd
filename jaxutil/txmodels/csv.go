/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package txmodels

import (
	"os"

	"github.com/gocarina/gocsv"
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

	file, err := os.OpenFile(storage.path, mode, 0o644)
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

func (storage *CSVStorage) FetchData() (UTXORows, error) {
	if err := storage.open(true, false); err != nil {
		return nil, err
	}
	defer storage.Close()

	rows := make([]UTXO, 0)
	err := gocsv.UnmarshalFile(storage.file, &rows)
	return rows, err
}

func (storage *CSVStorage) SaveRows(rows []UTXO) error {
	if err := storage.open(false, true); err != nil {
		return err
	}
	defer storage.Close()

	return gocsv.MarshalFile(rows, storage.file)
}
