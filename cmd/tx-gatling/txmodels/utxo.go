// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package txmodels

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"sync"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/txscript"
)

type IndexKey struct {
	Shard uint32
	Hash  string
}

// UTXOIndex is a storage for UTXO data.
type UTXOIndex struct {
	sync.RWMutex

	// map[ tx_id => block_height ]
	blocks map[IndexKey]int64
	// map[ tx_id => map[ out_n => UTXO index ] ]
	txs map[IndexKey]map[uint32]uint

	lastUsed  map[uint32]int
	lastBlock map[uint32]int64

	utxo   []UTXO
	lastID uint
}

func NewUTXOIndex() *UTXOIndex {
	return &UTXOIndex{
		blocks:    map[IndexKey]int64{},
		txs:       map[IndexKey]map[uint32]uint{},
		lastUsed:  map[uint32]int{},
		lastBlock: map[uint32]int64{},
	}
}

type gobUTXOIndex struct {
	Blocks           map[IndexKey]int64
	Txs              map[IndexKey]map[uint32]uint
	LastUsedByShard  map[uint32]int
	LastBlockByShard map[uint32]int64

	Utxo   []UTXO
	LastID uint
}

func (index *UTXOIndex) UnmarshalBinary(data []byte) error {
	index.Lock()
	defer index.Unlock()

	val := gobUTXOIndex{}
	err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(&val)
	if err != nil {
		return err
	}

	index.blocks = val.Blocks
	index.txs = val.Txs
	index.utxo = val.Utxo
	index.lastID = val.LastID
	index.lastBlock = val.LastBlockByShard
	index.lastUsed = val.LastUsedByShard
	return nil
}

func (index *UTXOIndex) MarshalBinary() (data []byte, err error) {
	buf := bytes.NewBuffer(nil)
	err = gob.NewEncoder(buf).Encode(gobUTXOIndex{
		Blocks:           index.blocks,
		Txs:              index.txs,
		Utxo:             index.utxo,
		LastID:           index.lastID,
		LastBlockByShard: index.lastBlock,
		LastUsedByShard:  index.lastUsed,
	})
	return buf.Bytes(), err
}

func (index *UTXOIndex) LastBlock(shardID uint32) int64 {
	return index.lastBlock[shardID]
}

func (index *UTXOIndex) ResetUsedFlag() {
	index.Lock()
	defer index.Unlock()
	for i := range index.utxo {
		u := index.utxo[i]
		u.Used = false
		index.utxo[i] = u
	}
	index.lastUsed = map[uint32]int{}
}

func (index *UTXOIndex) RmUTXO(txHash string, utxoIndexID, shardID uint32) {
	index.Lock()
	defer index.Unlock()
	key := IndexKey{Shard: shardID, Hash: txHash}
	txInd, ok := index.txs[key]
	if !ok {
		return
	}

	delete(txInd, utxoIndexID)

	if len(txInd) == 0 {
		delete(index.blocks, key)
		delete(index.txs, key)
	} else {
		index.txs[key] = txInd
	}
}

func (index *UTXOIndex) MarkUsed(txHash string, utxoIndexID, shardID uint32) {
	index.Lock()
	defer index.Unlock()

	key := IndexKey{Shard: shardID, Hash: txHash}
	txUTXOs, ok := index.txs[key]
	if !ok {
		return
	}
	utxoID, ok := txUTXOs[utxoIndexID]
	if !ok {
		return
	}

	index.utxo[utxoID].Used = true
}

func (index *UTXOIndex) AddUTXO(utxo UTXO) {
	index.Lock()
	defer index.Unlock()

	key := IndexKey{Shard: utxo.ShardID, Hash: utxo.TxHash}
	txInd, ok := index.txs[key]
	if !ok {
		txInd = map[uint32]uint{}
	}

	index.utxo = append(index.utxo, utxo)
	txInd[utxo.OutIndex] = index.lastID
	index.lastID++

	index.txs[key] = txInd
	index.blocks[key] = utxo.Height

	if index.lastBlock[utxo.ShardID] < utxo.Height {
		index.lastBlock[utxo.ShardID] = utxo.Height
	}
}

func (index *UTXOIndex) RowsCopy() UTXORows {
	index.RLock()
	defer index.RUnlock()

	rows := make(UTXORows, len(index.utxo))
	copy(rows, index.utxo)
	return rows
}

// CollectForAmount aggregates UTXOs to meet the requested amount. All selected UTXOs will be marked as USED.
func (index *UTXOIndex) CollectForAmount(amount int64, shardID uint32) (UTXORows, int64) {
	return index.CollectForAmountFiltered(amount, shardID, nil)
}

func (index *UTXOIndex) CollectForAmountFiltered(amount int64, shardID uint32,
	filter map[string]struct{}) (UTXORows, int64) {
	index.RLock()
	defer index.RUnlock()

	var res UTXORows
	change := amount

	lastUsed := index.lastUsed[shardID]
	for i := lastUsed; i < len(index.utxo); i++ {
		if index.utxo[i].ScriptType == txscript.EADAddress.String() {
			continue
		}
		if index.utxo[i].Used || index.utxo[i].ShardID != shardID {
			continue
		}

		if filter != nil {
			if _, ok := filter[index.utxo[i].Address]; !ok {
				continue
			}
		}

		change -= index.utxo[i].Value
		index.utxo[i].Used = true
		lastUsed = i
		res = append(res, index.utxo[i])

		if change <= 0 {
			break
		}
	}
	index.lastUsed[shardID] = lastUsed

	return res, 0
}

func (index *UTXOIndex) Rows() UTXORows {
	return index.utxo
}

type UTXO struct {
	ShardID    uint32 `json:"shard_id" csv:"shard_id"`
	Address    string `json:"address" csv:"address"`
	Height     int64  `json:"height" csv:"height"`
	TxHash     string `json:"tx_hash" csv:"tx_hash"`
	OutIndex   uint32 `json:"out_index" csv:"out_index"`
	Value      int64  `json:"value" csv:"value"`
	Used       bool   `json:"used" csv:"used"`
	PKScript   string `json:"pk_script" csv:"pk_script"`
	ScriptType string `json:"script_type" csv:"script_type"`
}

func (utxo UTXO) ToShort() ShortUTXO {
	return ShortUTXO{
		Value:    utxo.Value,
		PKScript: utxo.PKScript,
	}
}

type ShortUTXO struct {
	Value        int64  `json:"value" csv:"value"`
	PKScript     string `json:"pk_script" csv:"pk_script"`
	RedeemScript string `json:"redeem_script" csv:"redeem_script" `
}

func (utxo *ShortUTXO) GetScript(btcutil.Address) ([]byte, error) {
	return hex.DecodeString(utxo.RedeemScript)
}

type UTXORows []UTXO

func (rows UTXORows) Len() int { return len(rows) }
func (rows UTXORows) Less(i, j int) bool {
	return rows[i].Value < rows[j].Value
}
func (rows UTXORows) Swap(i, j int) { rows[i], rows[j] = rows[j], rows[i] }

func (rows UTXORows) GetSum() int64 {
	var sum int64
	for _, txOut := range rows {
		if txOut.ScriptType == txscript.EADAddress.String() {
			continue
		}
		sum += txOut.Value
	}
	return sum
}

func (rows UTXORows) CollectForAmount(amount int64, shardID uint32) (UTXORows, int64) {
	var res UTXORows
	change := amount

	for i, utxo := range rows {
		if utxo.ScriptType == txscript.EADAddress.String() {
			continue
		}
		if utxo.Used || utxo.ShardID != shardID {
			continue
		}

		change -= utxo.Value
		rows[i].Used = true
		res = append(res, rows[i])
		if change <= 0 {
			break
		}
	}

	return res, 0
}
