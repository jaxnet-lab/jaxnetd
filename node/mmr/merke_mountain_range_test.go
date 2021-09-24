/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"reflect"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

func TestBlock_Value(t *testing.T) {
	type fields struct {
		Hash   chainhash.Hash
		Weight uint64
	}
	tests := []struct {
		name   string
		fields fields
		wantV  Value
	}{
		{
			fields: fields{Hash: chainhash.Hash{}, Weight: 0xABCD_FFFF_4567_0123},
			wantV: [40]byte{
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0x23, 0x01, 0x67, 0x45, 0xFF, 0xFF, 0xCD, 0xAB,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := Block{
				Hash:   tt.fields.Hash,
				Weight: tt.fields.Weight,
			}
			gotV := b.Value()
			if !reflect.DeepEqual(gotV, tt.wantV) {
				t.Errorf("Value() = %v, want %v", gotV, tt.wantV)
			}

			nB := gotV.Block()
			if !reflect.DeepEqual(nB, b) {
				t.Errorf("Block() = %v, want %v", nB, b)
			}
		})
	}
}
