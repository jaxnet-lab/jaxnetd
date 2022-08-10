/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

func TestBlock_Value(t *testing.T) {
	type fields struct {
		Hash   chainhash.Hash
		Weight *big.Int
	}
	tests := []struct {
		name   string
		fields fields
		wantV  nodeValue
	}{
		{
			fields: fields{Weight: newUint(0xABCD_FFFF_4567_0123)},
			wantV: []byte{
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				// 0x23, 0x01, 0x67, 0x45, 0xFF, 0xFF, 0xCD, 0xAB,
				0xAB, 0xCD, 0xFF, 0xFF, 0x45, 0x67, 0x01, 0x23,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := node{
				Hash:   tt.fields.Hash,
				Weight: tt.fields.Weight,
			}
			gotV := b.Bytes()
			if !reflect.DeepEqual(gotV, tt.wantV) {
				t.Errorf("Value() = %v, want %v", gotV, tt.wantV)
			}

			nB := nodeValue(gotV).intoNode()
			_ = nB.Bytes()
			if !reflect.DeepEqual(nB, b) {
				spew.Dump(nB)
				spew.Dump(b)
				t.Errorf("node() = %v, want %v", nB, b)
			}
		})
	}

}
