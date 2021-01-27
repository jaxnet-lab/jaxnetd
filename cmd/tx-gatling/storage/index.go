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
	"gitlab.com/jaxnet/core/shard.core/network/rpcclient"
	"gitlab.com/jaxnet/core/shard.core/txscript"
)

type UTXORepo struct {
	file string

	index *txmodels.UTXOIndex
}

func (collector *UTXORepo) RedeemScript(address string) (script string) {
	// todo: implement this
	return address
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

func (collector *UTXORepo) SelectForAmount(amount int64, shardID uint32, addresses ...string) (txmodels.UTXORows, error) {
	var filter map[string]struct{} = nil
	if len(addresses) > 0 {
		filter = make(map[string]struct{}, len(addresses))

		for _, address := range addresses {
			filter[address] = struct{}{}
		}
	}

	rows, change := collector.index.CollectForAmountFiltered(amount, shardID, filter)
	if change > 0 {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, amount-change)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("not found UTXO for amount (need %d)", amount)
	}
	return rows, nil
}

func (collector *UTXORepo) Balance(shardId uint32, addresses ...string) (int64, error) {
	filter := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		filter[address] = struct{}{}
	}

	var sum int64
	for _, utxo := range collector.index.Rows() {
		if utxo.ScriptType == txscript.EADAddress.String() {
			continue
		}
		if _, ok := filter[utxo.Address]; ok && utxo.ShardID == shardId {
			sum += utxo.Value
		}
	}

	return sum, nil
}

func (collector *UTXORepo) ListUTXOs(skip, take int64, flags map[string]string) (int64, txmodels.UTXORows, error) {
	total := skip + take
	max := int64(len(collector.index.Rows()) - 1)
	if skip > max {
		return 0, nil, errors.New("can't skip, not enough utxo records")
	}
	if total > max {
		total = max
	}

	return total, collector.index.Rows()[skip:total], nil
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

func (collector *UTXORepo) CollectFromRPC(rpcClient *rpcclient.Client, shardID uint32, filter map[string]bool) error {
	maturityThreshold := int64(rpcClient.ChainParams().CoinbaseMaturity) + 2

	result, err := rpcClient.ForShard(shardID).ListTxOut()
	if err != nil {
		return errors.Wrap(err, "unable to get utxo list")
	}

	for _, outResult := range result.List {
	addressLookup:
		for _, skAddress := range outResult.ScriptPubKey.Addresses {
			if outResult.Coinbase && outResult.Confirmations < maturityThreshold {
				continue
			}
			if filter[skAddress] {
				collector.index.AddUTXO(txmodels.UTXO{
					ShardID:    shardID,
					Address:    skAddress,
					Height:     outResult.BlockHeight,
					TxHash:     outResult.TxHash,
					OutIndex:   outResult.Index,
					Value:      outResult.Value,
					Used:       outResult.Used,
					PKScript:   outResult.ScriptPubKey.Hex,
					ScriptType: outResult.ScriptPubKey.Type,
				})
				break addressLookup
			}
		}
	}

	return nil
}