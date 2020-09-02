package models

import (
	"encoding/hex"
	"errors"
	"sort"

	"gitlab.com/jaxnet/core/shard.core.git/btcjson"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
)

// UTXOIndex is a storage for UTXO data.
type UTXOIndex struct {
	// map[ tx_id => block_height ]
	blocks map[string]int64
	// map[ tx_id => map[ out_n => UTXO index ] ]
	txs map[string]map[uint32]uint

	utxo   []UTXO
	lastID uint
}

func NewUTXOIndex() *UTXOIndex {
	return &UTXOIndex{
		blocks: map[string]int64{},
		txs:    map[string]map[uint32]uint{},
	}
}

func (index *UTXOIndex) RmUTXO(txHash string, utxoIndexID uint32) {
	txInd, ok := index.txs[txHash]
	if !ok {
		return
	}

	delete(txInd, utxoIndexID)

	if len(txInd) == 0 {
		delete(index.blocks, txHash)
		delete(index.txs, txHash)
	} else {
		index.txs[txHash] = txInd
	}
}

func (index *UTXOIndex) MarkUsed(txHash string, utxoIndexID uint32) {
	txUTXOs, ok := index.txs[txHash]
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
	txInd, ok := index.txs[utxo.TxHash]
	if !ok {
		txInd = map[uint32]uint{}
	}

	index.lastID++
	index.utxo = append(index.utxo, utxo)
	txInd[utxo.OutIndex] = index.lastID

	index.txs[utxo.TxHash] = txInd
	index.blocks[utxo.TxHash] = utxo.Height

}

// DEPRECATED
func (index *UTXOIndex) AddTxs(txs []*btcjson.SearchRawTransactionsResult) {
	for _, tx := range txs {
		subIndex := map[uint32]uint{}
		for _, out := range tx.Vout {
			// fixme
			subIndex[out.N] = uint(int64(out.Value * 100_000_000))
		}

		index.txs[tx.Txid] = subIndex

		for _, vin := range tx.Vin {
			subIndex, ok := index.txs[vin.Txid]
			if !ok {
				continue
			}

			if _, ok := subIndex[vin.Vout]; ok {
				delete(subIndex, vin.Vout)
			}

			if len(subIndex) == 0 {
				delete(index.txs, vin.Txid)
			}
		}

	}

}

func (index *UTXOIndex) Rows() UTXORows {
	return index.utxo
}

type UTXO struct {
	Address    string `json:"address" csv:"address"`
	Height     int64  `json:"height" csv:"height"`
	TxHash     string `json:"tx_hash" csv:"tx_hash"`
	OutIndex   uint32 `json:"out_index" csv:"out_index"`
	Value      int64  `json:"value" csv:"value"`
	Used       bool   `json:"used" csv:"used"`
	PKScript   string `json:"pk_script" csv:"pk_script"`
	ScriptType string `json:"script_type" csv:"script_type"`
}

func (utxo *UTXO) GetScript(address btcutil.Address) ([]byte, error) {
	if utxo.Address != address.EncodeAddress() {
		return nil, errors.New("nope")
	}

	return hex.DecodeString(utxo.PKScript)
}

type UTXORows []UTXO

func (rows UTXORows) Len() int { return len(rows) }
func (rows UTXORows) Less(i, j int) bool {
	return rows[i].Height < rows[i].Height || rows[i].Value < rows[j].Value || rows[i].Used
}
func (rows UTXORows) Swap(i, j int) { rows[i], rows[j] = rows[j], rows[i] }

func (rows UTXORows) GetSum() int64 {
	var sum int64
	for _, txOut := range rows {
		sum += txOut.Value
	}
	return sum
}

func (rows UTXORows) CollectForAmount(amount int64) UTXORows {
	sort.Sort(sort.Reverse(rows))

	var res UTXORows
	change := amount

	for i, utxo := range rows {
		if utxo.Used {
			continue
		}

		change -= utxo.Value
		if change > 0 {
			rows[i].Used = true
			res = append(res, rows[i])
			continue
		}

		if change <= 0 {
			rows[i].Used = true
			res = append(res, rows[i])
			break
		}
	}

	return res
}
