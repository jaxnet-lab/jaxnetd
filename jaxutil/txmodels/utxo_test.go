// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txmodels

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
)

func _TestUTXOIndex_AddTxs(t *testing.T) {
	testAddress := "test_address"
	tests := []struct {
		name string
		txs  []*jaxjson.SearchRawTransactionsResult
		rows UTXORows
	}{
		{
			name: "",
			txs: []*jaxjson.SearchRawTransactionsResult{
				{
					Txid: "test_1",
					Vin: []jaxjson.VinPrevOut{
						{Coinbase: "111", Txid: "", Vout: 0},
					},
					Vout: []jaxjson.Vout{
						{Value: 1, N: 0},
						{Value: 1, N: 1},
						{Value: 1, N: 2},
					},
				},
				{
					Txid: "test_2",
					Vin: []jaxjson.VinPrevOut{
						{Coinbase: "", Txid: "test_1", Vout: 1},
						{Coinbase: "", Txid: "test_1", Vout: 2},
					},
					Vout: []jaxjson.Vout{
						{Value: 2, N: 0},
					},
				},
			},
			rows: UTXORows{
				{Address: testAddress, TxHash: "test_1", OutIndex: 0, Value: 100_000_000},
				{Address: testAddress, TxHash: "test_2", OutIndex: 0, Value: 200_000_000},
			},
		},
	}

	for _, tt := range tests {
		index := NewUTXOIndex()
		t.Run(tt.name, func(t *testing.T) {
			// index.AddTxs(tt.txs)
			rows := index.Rows()
			assert.Equal(t, tt.rows, rows)
		})
	}
}
