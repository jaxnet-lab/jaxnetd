/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package storage

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
)

type UTXORepo struct {
	file string

	index *txmodels.UTXOIndex
}

func NewUTXORepo(dataDir string, additionalKeys ...string) UTXORepo {
	tag := "a"
	for _, i2 := range additionalKeys {
		tag = tag + "-" + i2
	}

	return UTXORepo{
		file: path.Join(dataDir,
			fmt.Sprintf("utxo-index.%s.dat", tag)),
		index: txmodels.NewUTXOIndex(),
	}
}

func (collector *UTXORepo) SelectForAmount(amount int64, shardID uint32) (txmodels.UTXORows, error) {
	rows, change := collector.index.CollectForAmount(amount, shardID)
	if change > 0 {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, amount-change)
	}

	return rows, nil
}

func (collector *UTXORepo) Index() *txmodels.UTXOIndex {
	return collector.index
}

func (collector *UTXORepo) SetIndex(index *txmodels.UTXOIndex) {
	collector.index = index
}

func (collector *UTXORepo) ResetUsedFlag() {
	collector.index.ResetUsedFlag()
}

func (collector *UTXORepo) ReadIndex() error {
	collector.index = txmodels.NewUTXOIndex()
	if _, err := os.Stat(collector.file); os.IsNotExist(err) {
		return nil
	}

	data, err := ioutil.ReadFile(collector.file)
	if err != nil {
		return errors.Wrap(err, "unable to read index")
	}

	err = collector.index.UnmarshalBinary(data)
	if err != nil {
		return errors.Wrap(err, "unable to unmarshal index")
	}

	return nil
}

func (collector *UTXORepo) SaveIndex() error {
	data, err := collector.index.MarshalBinary()
	if err != nil {
		return errors.Wrap(err, "unable to marshal index")
	}

	err = ioutil.WriteFile(collector.file, data, 0644)
	if err != nil {
		return errors.Wrap(err, "unable to save index")
	}

	return nil
}
