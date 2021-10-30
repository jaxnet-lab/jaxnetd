/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"fmt"
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
		wantV  Value
	}{
		{
			fields: fields{Weight: new(big.Int).SetUint64(0xABCD_FFFF_4567_0123)},
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
			b := Leaf{
				Hash:   tt.fields.Hash,
				Weight: tt.fields.Weight,
			}
			gotV := b.Value()
			if !reflect.DeepEqual(gotV, tt.wantV) {
				t.Errorf("Value() = %v, want %v", gotV, tt.wantV)
			}

			nB := gotV.Block()
			_ = nB.Value()
			if !reflect.DeepEqual(nB, b) {
				spew.Dump(nB)
				spew.Dump(b)
				t.Errorf("Leaf() = %v, want %v", nB, b)
			}
		})
	}

	for _, blocks := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		fmt.Println("calc for N", blocks)
		N := blocks
		j := 0
		p := 1
		for p < N {
			fmt.Println("p =", p)
			j = p
			p *= 2
			for 2*j < N {
				fmt.Println(" j =", j)
				fmt.Println(" -> new node=", j+p)
				j += 2 * p
			}
		}
		fmt.Println()
	}

}

func TestBuildMerkleTreeStore(t *testing.T) {
	tests := []struct {
		blocks []Leaf
		want   []*Leaf
	}{
		{
			blocks: []Leaf{
				{Weight: new(big.Int).SetUint64(1)},
			},
			want: []*Leaf{
				{Weight: new(big.Int).SetUint64(1)},
			},
		},
		{
			blocks: []Leaf{
				{Weight: new(big.Int).SetUint64(1)}, {Weight: new(big.Int).SetUint64(20)},
			},
			want: []*Leaf{
				{Weight: new(big.Int).SetUint64(1)}, {Weight: new(big.Int).SetUint64(20)},
				// root
				{Weight: new(big.Int).SetUint64(21)},
			},
		},

		{
			blocks: []Leaf{
				{Weight: new(big.Int).SetUint64(1)}, {Weight: new(big.Int).SetUint64(20)}, {Weight: new(big.Int).SetUint64(300)},
			},

			want: []*Leaf{
				// zero layer
				{Weight: new(big.Int).SetUint64(1)}, {Weight: new(big.Int).SetUint64(20)}, {Weight: new(big.Int).SetUint64(300)}, nil, // reserved slot

				// 1st layer
				{Weight: new(big.Int).SetUint64(21)}, {Weight: new(big.Int).SetUint64(300)},
				// root
				{Weight: new(big.Int).SetUint64(321)},
			},
		},

		{
			blocks: []Leaf{
				{Weight: new(big.Int).SetUint64(1)}, {Weight: new(big.Int).SetUint64(20)}, {Weight: new(big.Int).SetUint64(300)}, {Weight: new(big.Int).SetUint64(4000)},
			},
			want: []*Leaf{
				// zero layer
				{Weight: new(big.Int).SetUint64(1)}, {Weight: new(big.Int).SetUint64(20)}, {Weight: new(big.Int).SetUint64(300)}, {Weight: new(big.Int).SetUint64(4000)},

				// 1st layer
				{Weight: new(big.Int).SetUint64(21)}, {Weight: new(big.Int).SetUint64(4300)},
				// root
				{Weight: new(big.Int).SetUint64(4321)},
			},
		},
		// {
		// 	blocks: []Leaf{
		// 		{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000}, {Weight: 50000},
		// 	},
		// 	want: []*Leaf{
		// 		// zero layer
		// 		{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
		// 		{Weight: 50000}, nil, nil, nil, // reserved slots
		//
		// 		// 1st layer
		// 		{Weight: 21}, {Weight: 4300}, {Weight: 50000}, nil,
		//
		// 		// 2nd layer
		// 		{Weight: 4321}, {Weight: 50000},
		//
		// 		// root
		// 		{Weight: 54321},
		// 	},
		// },
		//
		// {
		// 	blocks: []Leaf{
		// 		{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000}, {Weight: 50000}, {Weight: 600000},
		// 	},
		// 	want: []*Leaf{
		// 		// zero layer
		// 		{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
		// 		{Weight: 50000}, {Weight: 600000}, nil, nil, // reserved slot
		//
		// 		// 1st layer
		// 		{Weight: 21}, {Weight: 4300}, {Weight: 650000}, nil, // reserved slot
		//
		// 		// 2nd layer
		// 		{Weight: 4321}, {Weight: 650000},
		//
		// 		// root
		// 		{Weight: 654321},
		// 	},
		// },
		//
		// {
		// 	blocks: []Leaf{
		// 		{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
		// 		{Weight: 50000}, {Weight: 600000}, {Weight: 7000000},
		// 	},
		// 	want: []*Leaf{
		// 		// zero layer
		// 		{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
		// 		{Weight: 50000}, {Weight: 600000}, {Weight: 7000000}, nil, // reserved slot
		//
		// 		// 1st layer
		// 		{Weight: 21}, {Weight: 4300}, {Weight: 650000}, {Weight: 7000000},
		//
		// 		// 2nd layer
		// 		{Weight: 4321}, {Weight: 7650000},
		//
		// 		// root
		// 		{Weight: 7654321},
		// 	},
		// },

		// {
		// 	blocks: []Leaf{
		// 		{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
		// 		{Weight: 50000}, {Weight: 600000}, {Weight: 7000000}, {Weight: 80000000},
		// 	},
		// 	want: []*Leaf{
		// 		// zero layer
		// 		{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
		// 		{Weight: 50000}, {Weight: 600000}, {Weight: 7000000}, {Weight: 80000000},
		//
		// 		// 1st layer
		// 		{Weight: 21}, {Weight: 4300}, {Weight: 650000}, {Weight: 87000000},
		//
		// 		// 2nd layer
		// 		{Weight: 4321}, {Weight: 87650000},
		//
		// 		// root
		// 		{Weight: 87654321},
		// 	},
		// },
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
				if got[i].Weight.String() != tt.want[i].Weight.String() {
					t.Errorf("BuildMerkleTreeStore(): [%v].ChainWeight %v != %v", i, got[i].Weight, tt.want[i].Weight)
				}
			}

		})
	}
}
