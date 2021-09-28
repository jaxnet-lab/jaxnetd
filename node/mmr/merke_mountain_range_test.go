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
			fields: fields{Weight: 0xABCD_FFFF_4567_0123},
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

func TestBuildMerkleTreeStore(t *testing.T) {
	tests := []struct {
		blocks []Block
		want   []*Block
	}{
		{
			blocks: []Block{
				{Weight: 1},
			},
			want: []*Block{
				{Weight: 1},
			},
		},
		{
			blocks: []Block{
				{Weight: 1}, {Weight: 20},
			},
			want: []*Block{
				{Weight: 1}, {Weight: 20},
				// root
				{Weight: 21},
			},
		},

		{
			blocks: []Block{
				{Weight: 1}, {Weight: 20}, {Weight: 300},
			},

			want: []*Block{
				// zero layer
				{Weight: 1}, {Weight: 20}, {Weight: 300}, nil, // reserved slot

				// 1st layer
				{Weight: 21}, {Weight: 300},
				// root
				{Weight: 321},
			},
		},

		{
			blocks: []Block{
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
			},
			want: []*Block{
				// zero layer
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},

				// 1st layer
				{Weight: 21}, {Weight: 4300},
				// root
				{Weight: 4321},
			},
		},
		{
			blocks: []Block{
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000}, {Weight: 50000},
			},
			want: []*Block{
				// zero layer
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
				{Weight: 50000}, nil, nil, nil, // reserved slots

				// 1st layer
				{Weight: 21}, {Weight: 4300}, {Weight: 50000}, nil,

				// 2nd layer
				{Weight: 4321}, {Weight: 50000},

				// root
				{Weight: 54321},
			},
		},

		{
			blocks: []Block{
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000}, {Weight: 50000}, {Weight: 600000},
			},
			want: []*Block{
				// zero layer
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
				{Weight: 50000}, {Weight: 600000}, nil, nil, // reserved slot

				// 1st layer
				{Weight: 21}, {Weight: 4300}, {Weight: 650000}, nil, // reserved slot

				// 2nd layer
				{Weight: 4321}, {Weight: 650000},

				// root
				{Weight: 654321},
			},
		},

		{
			blocks: []Block{
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
				{Weight: 50000}, {Weight: 600000}, {Weight: 7000000},
			},
			want: []*Block{
				// zero layer
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
				{Weight: 50000}, {Weight: 600000}, {Weight: 7000000}, nil, // reserved slot

				// 1st layer
				{Weight: 21}, {Weight: 4300}, {Weight: 650000}, {Weight: 7000000},

				// 2nd layer
				{Weight: 4321}, {Weight: 7650000},

				// root
				{Weight: 7654321},
			},
		},

		{
			blocks: []Block{
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
				{Weight: 50000}, {Weight: 600000}, {Weight: 7000000}, {Weight: 80000000},
			},
			want: []*Block{
				// zero layer
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
				{Weight: 50000}, {Weight: 600000}, {Weight: 7000000}, {Weight: 80000000},

				// 1st layer
				{Weight: 21}, {Weight: 4300}, {Weight: 650000}, {Weight: 87000000},

				// 2nd layer
				{Weight: 4321}, {Weight: 87650000},

				// root
				{Weight: 87654321},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := BuildMerkleTreeStore(tt.blocks)
			if len(got) != len(tt.want) {
				t.Errorf("BuildMerkleTreeStore(): len(%v) != len(%v)", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] == nil && tt.want[i] == nil {
					continue
				}
				if got[i].Weight != tt.want[i].Weight {
					t.Errorf("BuildMerkleTreeStore(): [%v].Weight %v != %v", i, got[i].Weight, tt.want[i].Weight)
				}
			}

		})
	}
}
