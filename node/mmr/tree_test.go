package mmr

import (
	"testing"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

func TestMerkleTree(t *testing.T) {
	hash := func(h string) chainhash.Hash {
		return chainhash.HashH([]byte(h))
	}

	blocks := []Leaf{
		{Hash: hash("leaf_0_weight_0"), Weight: 143_000},
		{Hash: hash("leaf_1_weight_1"), Weight: 143_001},
		{Hash: hash("leaf_2_weight_2"), Weight: 143_002},
		{Hash: hash("leaf_3_weight_3"), Weight: 143_003},
		{Hash: hash("leaf_4_weight_4"), Weight: 143_004},
		{Hash: hash("leaf_5_weight_5"), Weight: 143_005},
		{Hash: hash("leaf_6_weight_6"), Weight: 143_006},
		{Hash: hash("leaf_7_weight_7"), Weight: 143_007},
		{Hash: hash("leaf_8_weight_8"), Weight: 143_008},
	}

	tests := []struct {
		blocks       []Leaf
		want         []*Leaf
		expectedRoot Leaf
		name         string
	}{
		{
			name: "1 leaf",
			blocks: []Leaf{
				blocks[0],
			},
			// root = block1
			expectedRoot: blocks[0],
		},

		{
			name: "2 leaves",
			blocks: []Leaf{
				blocks[0],
				blocks[1],
			},
			// root = node12 = block1 + block2
			expectedRoot: *HashMerkleBranches(&blocks[0], &blocks[1]),
		},

		{
			name: "3 leaves",
			blocks: []Leaf{
				blocks[0],
				blocks[1],
				blocks[2],
			},
			// root = node12 + block3 = H(block1 + block2) + block3
			expectedRoot: *HashMerkleBranches(
				HashMerkleBranches(&blocks[0], &blocks[1]),
				&blocks[2],
			),
		},

		{
			name: "4 leaves",
			blocks: []Leaf{
				blocks[0],
				blocks[1],
				blocks[2],
				blocks[3],
			},
			// root = node12 + node34 = H(block1 + block2) + H(block3 + block4)
			expectedRoot: *HashMerkleBranches(
				HashMerkleBranches(&blocks[0], &blocks[1]),
				HashMerkleBranches(&blocks[2], &blocks[3]),
			),
		},

		{
			name: "5 leaves",
			blocks: []Leaf{
				blocks[0],
				blocks[1],
				blocks[2],
				blocks[3],
				blocks[4],
			},
			// root = node12_34 + block5 = H( H(block1 + block2) + H(block3 + block4) ) + block5
			expectedRoot: *HashMerkleBranches(
				HashMerkleBranches(
					HashMerkleBranches(&blocks[0], &blocks[1]),
					HashMerkleBranches(&blocks[2], &blocks[3]),
				),
				&blocks[4],
			),
		},
		{
			name: "6 leaves",
			blocks: []Leaf{
				blocks[0],
				blocks[1],
				blocks[2],
				blocks[3],
				blocks[4],
				blocks[5],
			},
			// root = node12_34 + node56 = H( H(block1 + block2) + H(block3 + block4) ) + H(block5 + block6)
			expectedRoot: *HashMerkleBranches(
				HashMerkleBranches(
					HashMerkleBranches(&blocks[0], &blocks[1]),
					HashMerkleBranches(&blocks[2], &blocks[3]),
				),
				HashMerkleBranches(&blocks[4], &blocks[5]),
			),
		},
		{
			name: "7 leaves",
			blocks: []Leaf{
				blocks[0],
				blocks[1],
				blocks[2],
				blocks[3],
				blocks[4],
				blocks[5],
				blocks[6],
			},
			// root = node12_34 + node56_7 =
			//      H( H(block1 + block2) + H(block3 + block4) ) + H( H(block5 + block6) + block7) )
			expectedRoot: *HashMerkleBranches(
				HashMerkleBranches(
					HashMerkleBranches(&blocks[0], &blocks[1]),
					HashMerkleBranches(&blocks[2], &blocks[3]),
				),
				HashMerkleBranches(
					HashMerkleBranches(&blocks[4], &blocks[5]),
					&blocks[6],
				),
			),
		},

		{
			name: "8 leaves",
			blocks: []Leaf{
				blocks[0],
				blocks[1],
				blocks[2],
				blocks[3],
				blocks[4],
				blocks[5],
				blocks[6],
				blocks[7],
			},
			// root = node12_34 + node56_78 =
			//      H( H(block1 + block2) + H(block3 + block4) ) + H( H(block5 + block6) + H(block7 + block8) )
			expectedRoot: *HashMerkleBranches(
				HashMerkleBranches(
					HashMerkleBranches(&blocks[0], &blocks[1]),
					HashMerkleBranches(&blocks[2], &blocks[3]),
				),
				HashMerkleBranches(
					HashMerkleBranches(&blocks[4], &blocks[5]),
					HashMerkleBranches(&blocks[6], &blocks[7]),
				),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			store := BuildMerkleTreeStore(tt.blocks)
			root := store[len(store)-1]
			if root.Weight != tt.expectedRoot.Weight {
				t.Errorf("BuildMerkleTreeStore: chainWeight(): %v != %v", root.Weight, tt.expectedRoot.Weight)
			}

			if root.Hash != tt.expectedRoot.Hash {
				t.Errorf("BuildMerkleTreeStore: Hash: %v != %v", root.Hash, tt.expectedRoot.Hash)
			}

			root, _ = BuildMerkleTreeStoreNG(tt.blocks)
			if root.Weight != tt.expectedRoot.Weight {
				t.Errorf("BuildMerkleTreeStoreNG: chainWeight(): %v != %v", root.Weight, tt.expectedRoot.Weight)
			}

			if root.Hash != tt.expectedRoot.Hash {
				t.Errorf("BuildMerkleTreeStoreNG: Hash: %v != %v", root.Hash, tt.expectedRoot.Hash)
			}

			// todo: check merkle to hash && hash to merkle

			tree := NewTree()
			for i, block := range tt.blocks {
				if i == 3 {
					println()
				}
				tree.AddBlock(block.Hash, block.Weight)
			}

			if tree.chainWeight != tt.expectedRoot.Weight {
				t.Errorf("chainWeight(): %v != %v", tree.chainWeight, tt.expectedRoot.Weight)
			}

			if tree.CurrentRoot() != tt.expectedRoot.Hash {
				t.Errorf("Hash: %v != %v", tree.CurrentRoot(), tt.expectedRoot.Hash)
			}
		})
	}

}

func TestMerkleTreeMethods(t *testing.T) {
	hash := func(h string) chainhash.Hash {
		return chainhash.HashH([]byte(h))
	}

	blocks := []Leaf{
		{Hash: hash("leaf_0_weight_0"), Weight: 143_000},
		{Hash: hash("leaf_1_weight_1"), Weight: 143_001},
		{Hash: hash("leaf_2_weight_2"), Weight: 143_002},
		{Hash: hash("leaf_3_weight_3"), Weight: 143_003},
		{Hash: hash("leaf_4_weight_4"), Weight: 143_004},
		{Hash: hash("leaf_5_weight_5"), Weight: 143_005},
		{Hash: hash("leaf_6_weight_6"), Weight: 143_006},
		{Hash: hash("leaf_7_weight_7"), Weight: 143_007},
		{Hash: hash("leaf_8_weight_8"), Weight: 143_008},
	}

	altBlocks := []Leaf{
		{Hash: hash("leaf_0_weight_0"), Weight: 143_000},
		{Hash: hash("leaf_1_weight_1"), Weight: 143_001},
		{Hash: hash("leaf_2_weight_2"), Weight: 143_002},
		{Hash: hash("alt_leaf_3_weight_3"), Weight: 343_003},
		{Hash: hash("alt_leaf_4_weight_4"), Weight: 343_004},
		{Hash: hash("alt_leaf_5_weight_5"), Weight: 343_005},
		{Hash: hash("alt_leaf_6_weight_6"), Weight: 343_006},
		{Hash: hash("alt_leaf_7_weight_7"), Weight: 343_007},
		{Hash: hash("alt_leaf_8_weight_8"), Weight: 343_008},
	}
	_ = altBlocks

	tree := NewTree()
	for _, block := range blocks {
		tree.AddBlock(block.Hash, block.Weight)
	}

	tree.ResetRootTo(blocks[3].Hash, 4)

}
