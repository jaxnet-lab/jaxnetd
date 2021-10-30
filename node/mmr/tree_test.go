package mmr

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

var hash = func(h string) chainhash.Hash {
	return chainhash.HashH([]byte(h))
}

func TestMerkleTree(t *testing.T) {
	blocks := getBlocks()

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
			if root.Weight.String() != tt.expectedRoot.Weight.String() {
				t.Errorf("BuildMerkleTreeStore: chainWeight(): %v != %v", root.Weight, tt.expectedRoot.Weight)
			}

			if root.Hash != tt.expectedRoot.Hash {
				t.Errorf("BuildMerkleTreeStore: Hash: %v != %v", root.Hash, tt.expectedRoot.Hash)
			}

			root, _ = BuildMerkleTreeStoreNG(tt.blocks)
			if root.Weight.String() != tt.expectedRoot.Weight.String() {
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

			if tree.chainWeight.String() != tt.expectedRoot.Weight.String() {
				t.Errorf("chainWeight(): %v != %v", tree.chainWeight, tt.expectedRoot.Weight)
			}

			if tree.CurrentRoot() != tt.expectedRoot.Hash {
				t.Errorf("Hash: %v != %v", tree.CurrentRoot(), tt.expectedRoot.Hash)
			}
		})
	}

}

func TestMerkleTreeMethods(t *testing.T) {
	blocks := getBlocks()

	altBlocks := []Leaf{
		{Hash: hash("leaf_0_weight_0"), Weight: new(big.Int).SetInt64(143_000)},
		{Hash: hash("leaf_1_weight_1"), Weight: new(big.Int).SetInt64(143_001)},
		{Hash: hash("leaf_2_weight_2"), Weight: new(big.Int).SetInt64(143_002)},
		{Hash: hash("alt_leaf_3_weight_3"), Weight: new(big.Int).SetInt64(343_003)},
		{Hash: hash("alt_leaf_4_weight_4"), Weight: new(big.Int).SetInt64(343_004)},
		{Hash: hash("alt_leaf_5_weight_5"), Weight: new(big.Int).SetInt64(343_005)},
		{Hash: hash("alt_leaf_6_weight_6"), Weight: new(big.Int).SetInt64(343_006)},
		{Hash: hash("alt_leaf_7_weight_7"), Weight: new(big.Int).SetInt64(343_007)},
		{Hash: hash("alt_leaf_8_weight_8"), Weight: new(big.Int).SetInt64(343_008)},
	}

	testCases := []struct {
		blockID            int
		height             int32
		expectedHash       chainhash.Hash
		expectedTreeWeight *big.Int
	}{
		{
			blockID:            2,
			height:             2,
			expectedHash:       blocks[1].Hash,
			expectedTreeWeight: new(big.Int).Add(blocks[0].Weight, blocks[1].Weight),
		},
		{
			blockID:            0,
			height:             0,
			expectedHash:       chainhash.ZeroHash,
			expectedTreeWeight: new(big.Int).SetInt64(0),
		},
		{
			blockID:      8,
			height:       8,
			expectedHash: blocks[7].Hash,
			expectedTreeWeight: new(big.Int).Add(
				new(big.Int).Add(
					new(big.Int).Add(blocks[0].Weight, blocks[1].Weight),
					new(big.Int).Add(blocks[2].Weight, blocks[3].Weight),
				),
				new(big.Int).Add(
					new(big.Int).Add(blocks[4].Weight, blocks[5].Weight),
					new(big.Int).Add(blocks[6].Weight, blocks[7].Weight),
				),
			),
		},
	}
	_ = altBlocks

	for _, tt := range testCases {
		tree := getFilledTree(blocks)
		tree.RmBlock(blocks[tt.blockID].Hash, tt.height)
		assert.Equal(t, tt.expectedTreeWeight.String(), tree.chainWeight.String())
		assert.Equal(t, tt.expectedHash, tree.Current().Hash)
		_, ok := tree.hashToHeight[blocks[tt.blockID].Hash]
		assert.Equal(t, false, ok)
		_, ok = tree.mountainTops[blocks[tt.blockID].Hash]
		assert.Equal(t, false, ok)
	}
}

func TestMerkleTreeResetRootTo(t *testing.T) {
	blocks := getBlocks()

	altBlocks := []Leaf{
		{Hash: hash("leaf_0_weight_0"), Weight: new(big.Int).SetInt64(143_000)},
		{Hash: hash("leaf_1_weight_1"), Weight: new(big.Int).SetInt64(143_001)},
		{Hash: hash("leaf_2_weight_2"), Weight: new(big.Int).SetInt64(143_002)},
		{Hash: hash("alt_leaf_3_weight_3"), Weight: new(big.Int).SetInt64(343_003)},
		{Hash: hash("alt_leaf_4_weight_4"), Weight: new(big.Int).SetInt64(343_004)},
		{Hash: hash("alt_leaf_5_weight_5"), Weight: new(big.Int).SetInt64(343_005)},
		{Hash: hash("alt_leaf_6_weight_6"), Weight: new(big.Int).SetInt64(343_006)},
		{Hash: hash("alt_leaf_7_weight_7"), Weight: new(big.Int).SetInt64(343_007)},
		{Hash: hash("alt_leaf_8_weight_8"), Weight: new(big.Int).SetInt64(343_008)},
	}

	testCases := []struct {
		blockID            int
		height             int32
		expectedHash       chainhash.Hash
		expectedTreeWeight *big.Int
		blockSaved         bool
	}{
		{
			blockID:            2,
			height:             2,
			expectedHash:       blocks[2].Hash,
			expectedTreeWeight: new(big.Int).Add(new(big.Int).Add(blocks[0].Weight, blocks[1].Weight), blocks[2].Weight),
			blockSaved:         false,
		},
		{
			blockID:            0,
			height:             0,
			expectedHash:       blocks[0].Hash,
			expectedTreeWeight: blocks[0].Weight,
			blockSaved:         true,
		},
		{
			blockID:      8,
			height:       8,
			expectedHash: blocks[8].Hash,
			expectedTreeWeight: new(big.Int).Add(
				new(big.Int).Add(
					new(big.Int).Add(
						new(big.Int).Add(blocks[0].Weight, blocks[1].Weight),
						new(big.Int).Add(blocks[2].Weight, blocks[3].Weight),
					),
					new(big.Int).Add(
						new(big.Int).Add(blocks[4].Weight, blocks[5].Weight),
						new(big.Int).Add(blocks[6].Weight, blocks[7].Weight),
					),
				),
				blocks[8].Weight,
			),
			blockSaved: false,
		},
	}
	_ = altBlocks

	for _, tt := range testCases {
		tree := getFilledTree(blocks)
		tree.ResetRootTo(blocks[tt.blockID].Hash, tt.height)
		assert.Equal(t, tt.expectedTreeWeight.String(), tree.chainWeight.String())
		assert.Equal(t, tt.expectedHash, tree.Current().Hash)
		_, ok := tree.hashToHeight[blocks[tt.blockID].Hash]
		assert.Equal(t, tt.blockSaved, ok)
		_, ok = tree.mountainTops[blocks[tt.blockID].Hash]
		assert.Equal(t, tt.blockSaved, ok)
	}
}

func TestMerkleTreeMMRRoot(t *testing.T) {
	blocks := getBlocks()
	treeWithoutRms := getFilledTree(blocks)
	treeWithRms := getFilledTree(blocks)

	treeWithRms.RmBlock(blocks[5].Hash, 5)
	for i := 5; i <= 8; i++ {
		treeWithRms.AddBlock(blocks[i].Hash, blocks[i].Weight)
	}

	assert.Equal(t, treeWithRms.rootHash, treeWithoutRms.rootHash)
}

func getFilledTree(blocks []Leaf) *BlocksMMRTree {
	tree := NewTree()
	for _, block := range blocks {
		tree.AddBlock(block.Hash, block.Weight)
	}

	return tree
}

func getBlocks() []Leaf {
	return []Leaf{
		{Hash: hash("leaf_0_weight_0"), Weight: new(big.Int).SetInt64(143_000)},
		{Hash: hash("leaf_1_weight_1"), Weight: new(big.Int).SetInt64(143_001)},
		{Hash: hash("leaf_2_weight_2"), Weight: new(big.Int).SetInt64(143_002)},
		{Hash: hash("leaf_3_weight_3"), Weight: new(big.Int).SetInt64(143_003)},
		{Hash: hash("leaf_4_weight_4"), Weight: new(big.Int).SetInt64(143_004)},
		{Hash: hash("leaf_5_weight_5"), Weight: new(big.Int).SetInt64(143_005)},
		{Hash: hash("leaf_6_weight_6"), Weight: new(big.Int).SetInt64(143_006)},
		{Hash: hash("leaf_7_weight_7"), Weight: new(big.Int).SetInt64(143_007)},
		{Hash: hash("leaf_8_weight_8"), Weight: new(big.Int).SetInt64(143_008)},
	}
}
