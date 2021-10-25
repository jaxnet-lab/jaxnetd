package mmr

import (
	"encoding/binary"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

func TestTree_AddBlock(t1 *testing.T) {
	type arg struct {
		hash   string
		diff   uint64
		height uint64
	}

	tt := []arg{
		{hash: "361d6688ba1645379f0f6dd65d816302aab22caf0c0daa49000fe3d53786a042", diff: 0, height: 0},
		{hash: "8584369a88a2793528847c0b808422a28df5296de03a6a375049e0db886e8caa", diff: 0, height: 1},
		{hash: "bc328828650e3874e6484e0330180e784702d69db4f33ca1840f3e9e52bdf6db", diff: 0, height: 2},
		{hash: "2d81807dd3bb40808f1a58a024d2ef746742a99b1d8dfbec54430e0574cd53cc", diff: 0, height: 3},
		{hash: "31b89dda195fea061625ee2b39a440e24041a0839538cbe59f9d587d3188a5bf", diff: 0, height: 4},
		{hash: "81ba669d2de9183d45e27bdeba340211fb38de0e45b7eae5e357be8f9983a32e", diff: 0, height: 5},
		{hash: "1c567e5653e01329423dee2ca1764a69a37546fb4010d7b4ecd6ca123eedbc07", diff: 0, height: 6},
	}
	nodes := []Leaf{}
	tree := NewTree()
	prevRoot := chainhash.ZeroHash
	for _, a := range tt {
		h, _ := chainhash.NewHashFromStr(a.hash)
		tree.AddBlock(*h, a.height)
		root := tree.CurrentRoot()
		if prevRoot.IsEqual(&root) {
			t1.Errorf("the root is not recalculated")
		}

		nodes = append(nodes, Leaf{
			Hash:   *h,
			Weight: a.height,
		})

		// tree := BuildMerkleTreeStore(nodes)
		// fmt.Println(prevRoot, a.hash, tree[len(tree)-1].Hash)
		// prevRoot = tree[len(tree)-1].Hash
		// prevRoot = prevRoot
	}

}

func TestMerkleTree(t *testing.T) {
	s2h := func(h string) chainhash.Hash {
		return chainhash.HashH([]byte(h))
	}

	concatHashes := func(h1, h2 chainhash.Hash, w1, w2 uint64) chainhash.Hash {
		var buf [80]byte
		copy(buf[:32], h1[:32])
		binary.LittleEndian.PutUint64(buf[32:], w1)
		copy(buf[40:], h2[:32])
		binary.LittleEndian.PutUint64(buf[72:], w2)
		//		return fmt.Sprintf("%s%b", h1.String(), w1) + fmt.Sprintf("%s%b", h2.String(), w2)
		return chainhash.HashH(buf[:])
	}

	hash_w_1 := s2h("leaf_weight_1")
	hash_w_20 := s2h("leaf_weight_20")
	hash_w_300 := s2h("leaf_weight_300")
	hash_w_4000 := s2h("leaf_weight_4000")
	hash_w_50000 := s2h("leaf_weight_50000")
	hash_w_21 := concatHashes(s2h("leaf_weight_1"), s2h("leaf_weight_20"), 1, 20)
	hash_w_321 := concatHashes(hash_w_300, hash_w_21, 300, 21)

	tests := []struct {
		blocks []Leaf
		want   []*Leaf
	}{
		{
			blocks: []Leaf{
				{Weight: 1, Hash: hash_w_1},
			},
			want: []*Leaf{
				{Weight: 1, Hash: hash_w_1},
			},
		},
		{
			blocks: []Leaf{
				{Weight: 1, Hash: hash_w_1}, {Weight: 20, Hash: hash_w_20},
			},
			want: []*Leaf{
				{Weight: 1, Hash: hash_w_1}, {Weight: 20, Hash: hash_w_20},
				// root
				{Weight: 21, Hash: hash_w_21},
			},
		},

		{
			blocks: []Leaf{
				{Weight: 1, Hash: hash_w_1}, {Weight: 20, Hash: hash_w_20}, {Weight: 300, Hash: hash_w_300},
			},

			want: []*Leaf{
				// zero layer
				{Weight: 1, Hash: hash_w_1}, {Weight: 20, Hash: hash_w_20}, {Weight: 300, Hash: hash_w_300}, nil, // reserved slot

				// 1st layer
				{Weight: 21, Hash: hash_w_21}, {Weight: 300, Hash: hash_w_300},
				// root
				{Weight: 321, Hash: hash_w_321},
			},
		},

		{
			blocks: []Leaf{
				{Weight: 1, Hash: hash_w_1}, {Weight: 20, Hash: hash_w_20}, {Weight: 300, Hash: hash_w_300}, {Weight: 4000, Hash: hash_w_4000},
			},
			want: []*Leaf{
				// zero layer
				{Weight: 1, Hash: hash_w_1}, {Weight: 20, Hash: hash_w_20}, {Weight: 300, Hash: hash_w_300}, {Weight: 4000, Hash: hash_w_4000},

				// 1st layer
				{Weight: 21, Hash: hash_w_21}, {Weight: 4300, Hash: concatHashes(hash_w_300, hash_w_4000, 300, 4000)},
				// root
				{Weight: 4321, Hash: concatHashes(hash_w_21, concatHashes(hash_w_300, hash_w_4000, 300, 4000),
					21, 4300)},
			},
		},
		{
			blocks: []Leaf{
				{Weight: 1, Hash: hash_w_1}, {Weight: 20, Hash: hash_w_20}, {Weight: 300, Hash: hash_w_300}, {Weight: 4000, Hash: hash_w_4000}, {Weight: 50000, Hash: hash_w_50000},
			},
			want: []*Leaf{
				// zero layer
				{Weight: 1, Hash: hash_w_1}, {Weight: 20, Hash: hash_w_20}, {Weight: 300, Hash: hash_w_300}, {Weight: 4000, Hash: hash_w_4000},
				{Weight: 50000, Hash: hash_w_50000}, nil, nil, nil, // reserved slots

				// 1st layer
				{Weight: 21, Hash: hash_w_21}, {Weight: 4300, Hash: concatHashes(hash_w_300, hash_w_4000, 300, 4000)}, {Weight: 50000, Hash: hash_w_50000}, nil,

				// 2nd layer
				{Weight: 4321, Hash: concatHashes(hash_w_21, concatHashes(hash_w_300, hash_w_4000, 300, 4000), 21, 4000)}, {Weight: 50000, Hash: hash_w_50000},

				// root
				{Weight: 54321, Hash: concatHashes(concatHashes(hash_w_21, concatHashes(hash_w_300, hash_w_4000, 300, 4000), 21, 4000), hash_w_50000, 4321, 50000)},
			},
		},

		{
			blocks: []Leaf{
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000}, {Weight: 50000}, {Weight: 600000},
			},
			want: []*Leaf{
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
			blocks: []Leaf{
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
				{Weight: 50000}, {Weight: 600000}, {Weight: 7000000},
			},
			want: []*Leaf{
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
			blocks: []Leaf{
				{Weight: 1}, {Weight: 20}, {Weight: 300}, {Weight: 4000},
				{Weight: 50000}, {Weight: 600000}, {Weight: 7000000}, {Weight: 80000000},
			},
			want: []*Leaf{
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
			prevRoot := chainhash.ZeroHash
			for _, block := range tt.blocks {
				tree.AddBlock(block.Hash, block.Weight)

				root := tree.CurrentRoot()
				if prevRoot.IsEqual(&root) {
					t.Errorf("the root is not recalculated")
				}
			}

			if tree.chainWeight != tt.want[len(tt.want)-1].Weight {
				t.Errorf("chainWeight(): %v != %v", tree.chainWeight, tt.want[len(tt.want)-1].Weight)
			}

			if tree.CurrentRoot() != tt.want[len(tt.want)-1].Hash {
				t.Errorf("Hash: %v != %v", tree.CurrentRoot(), tt.want[len(tt.want)-1].Hash)
			}
		})
	}
}
