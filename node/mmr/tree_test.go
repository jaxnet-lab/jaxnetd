package mmr

import (
	"fmt"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

func TestTree_AddBlock(t1 *testing.T) {
	type arg struct {
		hash   string
		diff   uint64
		height int32
	}

	tree := NewTree()
	tt := []arg{
		{hash: "361d6688ba1645379f0f6dd65d816302aab22caf0c0daa49000fe3d53786a042", diff: 0, height: 0},
		{hash: "8584369a88a2793528847c0b808422a28df5296de03a6a375049e0db886e8caa", diff: 0, height: 1},
		{hash: "bc328828650e3874e6484e0330180e784702d69db4f33ca1840f3e9e52bdf6db", diff: 0, height: 2},
		{hash: "2d81807dd3bb40808f1a58a024d2ef746742a99b1d8dfbec54430e0574cd53cc", diff: 0, height: 3},
		{hash: "31b89dda195fea061625ee2b39a440e24041a0839538cbe59f9d587d3188a5bf", diff: 0, height: 4},
		{hash: "81ba669d2de9183d45e27bdeba340211fb38de0e45b7eae5e357be8f9983a32e", diff: 0, height: 5},
		{hash: "1c567e5653e01329423dee2ca1764a69a37546fb4010d7b4ecd6ca123eedbc07", diff: 0, height: 6},
	}

	for _, a := range tt {
		h, _ := chainhash.NewHashFromStr(a.hash)
		tree.AddBlock(*h, a.diff, a.height)
		fmt.Println("MRR_ROOT_FOR_BLOCK:>", h.String(), a.height, tree.CurrentRoot().String())

	}
}

func TestMerkleTree(t *testing.T) {
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
			tree := NewTree()
			for _, block := range tt.blocks {
				tree.AddBlock(block.Hash, block.Weight, int32(block.Weight))
			}

			if tree.chainWeight != tt.want[len(tt.want)-1].Weight {
				t.Errorf("chainWeight(): %v != %v", tree.chainWeight, tt.want[len(tt.want)-1].Weight)
			}
		})
	}
}
