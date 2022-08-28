package mmr

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

var hash = func(h string) chainhash.Hash {
	return chainhash.HashH([]byte(h))
}

func TestMerkleTree(t *testing.T) {
	blocks := getNBlocks(26)

	tests := []struct {
		blocks       []TreeNode
		want         []*TreeNode
		expectedRoot TreeNode
		name         string
	}{
		{
			name: "1 leaf",
			blocks: []TreeNode{
				blocks[0],
			},
			// root = block1
			expectedRoot: blocks[0],
		},

		{
			name: "2 leaves",
			blocks: []TreeNode{
				blocks[0],
				blocks[1],
			},
			// root = node12 = block1 + block2
			expectedRoot: *hashLeafs(&blocks[0], &blocks[1]),
		},

		{
			name: "3 leaves",
			blocks: []TreeNode{
				blocks[0],
				blocks[1],
				blocks[2],
			},
			// root = node12 + block3 = H(block1 + block2) + block3
			expectedRoot: *hashLeafs(
				hashLeafs(&blocks[0], &blocks[1]),
				&blocks[2],
			),
		},

		{
			name: "4 leaves",
			blocks: []TreeNode{
				blocks[0],
				blocks[1],
				blocks[2],
				blocks[3],
			},
			// root = node12 + node34 = H(block1 + block2) + H(block3 + block4)
			expectedRoot: *hashLeafs(
				hashLeafs(&blocks[0], &blocks[1]),
				hashLeafs(&blocks[2], &blocks[3]),
			),
		},

		{
			name: "5 leaves",
			blocks: []TreeNode{
				blocks[0],
				blocks[1],
				blocks[2],
				blocks[3],
				blocks[4],
			},
			// root = node12_34 + block5 = H( H(block1 + block2) + H(block3 + block4) ) + block5
			expectedRoot: *hashLeafs(
				hashLeafs(
					hashLeafs(&blocks[0], &blocks[1]),
					hashLeafs(&blocks[2], &blocks[3]),
				),
				&blocks[4],
			),
		},
		{
			name: "6 leaves",
			blocks: []TreeNode{
				blocks[0],
				blocks[1],
				blocks[2],
				blocks[3],
				blocks[4],
				blocks[5],
			},
			// root = node12_34 + node56 = H( H(block1 + block2) + H(block3 + block4) ) + H(block5 + block6)
			expectedRoot: *hashLeafs(
				hashLeafs(
					hashLeafs(&blocks[0], &blocks[1]),
					hashLeafs(&blocks[2], &blocks[3]),
				),
				hashLeafs(&blocks[4], &blocks[5]),
			),
		},
		{
			name: "7 leaves",
			blocks: []TreeNode{
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
			expectedRoot: *hashLeafs(
				hashLeafs(
					hashLeafs(&blocks[0], &blocks[1]),
					hashLeafs(&blocks[2], &blocks[3]),
				),
				hashLeafs(
					hashLeafs(&blocks[4], &blocks[5]),
					&blocks[6],
				),
			),
		},

		{
			name: "8 leaves",
			blocks: []TreeNode{
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
			expectedRoot: *hashLeafs(
				hashLeafs(
					hashLeafs(&blocks[0], &blocks[1]),
					hashLeafs(&blocks[2], &blocks[3]),
				),
				hashLeafs(
					hashLeafs(&blocks[4], &blocks[5]),
					hashLeafs(&blocks[6], &blocks[7]),
				),
			),
		},
		{
			name: "25 leaves",
			blocks: []TreeNode{
				blocks[0], blocks[1], blocks[2], blocks[3],
				blocks[4], blocks[5], blocks[6], blocks[7],

				blocks[8], blocks[9], blocks[10], blocks[11],
				blocks[12], blocks[13], blocks[14], blocks[15],

				blocks[16], blocks[17], blocks[18], blocks[19],
				blocks[20], blocks[21], blocks[22], blocks[23],
				blocks[24],
			},
			// 0: H(0,1) H(2,3) H(4,5) H(6,7) H(8,9) H(10,11) H(12,13) H(14,15) H(16,17) H(18,19) H(20,21) H(22,23) 24
			// 1: H(h_01, h_23)  H(h_45, h_67) H(h_89, h_1011) H(h_1213, h_1415) H(h_1617, h_1819) H(h_2021, h_2223) 24
			// 2: H(h_01_23, h_45_67) H(h_89_1011, h_1213_1415) H(h_1617_1819, h_2021_2223) 24
			// 3: H(h_01_23__45_67, h_89_1011__1213_1415) H(h_1617_1819__2021_2223, 24)
			// 5: H(h_01_23__45_67___89_1011__1213_1415, h_1617_1819__2021_2223___24)
			// 6: root
			// root = H(
			//		H'''(
			//			H''(
			//				H'(H(0,1),H(2,3)'),
			//				H'(H(4,5),H(6,7)')
			//				''),
			//			H''(
			//				H'(H(8,9),H(10,11)'),
			//				H'(H(12,13),H(14,15)')
			//				'')
			//			'''),
			//		H'''(
			//			H''(
			//				H'(H(16,17),H(18,19)'),
			//				H'(H(20,21),H(22,23)')
			//				''),
			//			24 ''')
			//		)
			expectedRoot: *hashBranches(
				hashBranches( // H'''
					hashBranches( // H''
						hashBranches( // H'
							hashBranches(&blocks[0], &blocks[1]), hashBranches(&blocks[2], &blocks[3]),
						),
						hashBranches( // H'
							hashBranches(&blocks[4], &blocks[5]), hashBranches(&blocks[6], &blocks[7]),
						),
					),

					hashBranches( // H''
						hashBranches( // H'
							hashBranches(&blocks[8], &blocks[9]), hashBranches(&blocks[10], &blocks[11]),
						),
						hashBranches( // H'
							hashBranches(&blocks[12], &blocks[13]), hashBranches(&blocks[14], &blocks[15]),
						),
					),
				),
				hashBranches( // H'''
					hashBranches( // H''
						hashBranches( // H'
							hashBranches(&blocks[16], &blocks[17]), hashBranches(&blocks[18], &blocks[19]),
						),
						hashBranches( // H'
							hashBranches(&blocks[20], &blocks[21]), hashBranches(&blocks[22], &blocks[23]),
						),
					),
					&blocks[24],
				),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTree()
			for i, block := range tt.blocks {
				if i == 3 {
					println()
				}
				tree.AppendBlock(block.Hash, block.Weight)
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

	altBlocks := []TreeNode{
		{Hash: hash("leaf_0_weight_0"), Weight: newInt(143_000)},
		{Hash: hash("leaf_1_weight_1"), Weight: newInt(143_001)},
		{Hash: hash("leaf_2_weight_2"), Weight: newInt(143_002)},
		{Hash: hash("alt_leaf_3_weight_3"), Weight: newInt(343_003)},
		{Hash: hash("alt_leaf_4_weight_4"), Weight: newInt(343_004)},
		{Hash: hash("alt_leaf_5_weight_5"), Weight: newInt(343_005)},
		{Hash: hash("alt_leaf_6_weight_6"), Weight: newInt(343_006)},
		{Hash: hash("alt_leaf_7_weight_7"), Weight: newInt(343_007)},
		{Hash: hash("alt_leaf_8_weight_8"), Weight: newInt(343_008)},
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
			expectedTreeWeight: newInt(0),
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
		_, ok = tree.rootToHeight[blocks[tt.blockID].Hash]
		assert.Equal(t, false, ok)
	}
}

func TestMerkleTreeResetRootTo(t *testing.T) {
	blocks := getBlocks()

	altBlocks := []TreeNode{
		{Hash: hash("leaf_0_weight_0"), Weight: newInt(143_000)},
		{Hash: hash("leaf_1_weight_1"), Weight: newInt(143_001)},
		{Hash: hash("leaf_2_weight_2"), Weight: newInt(143_002)},
		{Hash: hash("alt_leaf_3_weight_3"), Weight: newInt(343_003)},
		{Hash: hash("alt_leaf_4_weight_4"), Weight: newInt(343_004)},
		{Hash: hash("alt_leaf_5_weight_5"), Weight: newInt(343_005)},
		{Hash: hash("alt_leaf_6_weight_6"), Weight: newInt(343_006)},
		{Hash: hash("alt_leaf_7_weight_7"), Weight: newInt(343_007)},
		{Hash: hash("alt_leaf_8_weight_8"), Weight: newInt(343_008)},
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
		_, ok = tree.rootToHeight[blocks[tt.blockID].Hash]
		assert.Equal(t, tt.blockSaved, ok)
	}
}

func TestMerkleTreeMMRRoot(t *testing.T) {
	blocks := getBlocks()
	treeWithoutRms := getFilledTree(blocks)
	treeWithRms := getFilledTree(blocks)

	treeWithRms.RmBlock(blocks[5].Hash, 5)
	for i := 5; i <= 8; i++ {
		treeWithRms.AppendBlock(blocks[i].Hash, blocks[i].Weight)
	}

	assert.Equal(t, treeWithRms.rootHash, treeWithoutRms.rootHash)
}

func getFilledTree(blocks []TreeNode) *BlocksMMRTree {
	tree := NewTree()
	for _, block := range blocks {
		tree.AppendBlock(block.Hash, block.Weight)
	}

	return tree
}

func getBlocks() []TreeNode {
	return []TreeNode{
		{Hash: hash("leaf_0_weight_0"), Weight: newInt(143_000)},
		{Hash: hash("leaf_1_weight_1"), Weight: newInt(143_001)},
		{Hash: hash("leaf_2_weight_2"), Weight: newInt(143_002)},
		{Hash: hash("leaf_3_weight_3"), Weight: newInt(143_003)},
		{Hash: hash("leaf_4_weight_4"), Weight: newInt(143_004)},
		{Hash: hash("leaf_5_weight_5"), Weight: newInt(143_005)},
		{Hash: hash("leaf_6_weight_6"), Weight: newInt(143_006)},
		{Hash: hash("leaf_7_weight_7"), Weight: newInt(143_007)},
		{Hash: hash("leaf_8_weight_8"), Weight: newInt(143_008)},
	}
}

func getNBlocks(n int) []TreeNode {
	leafs := make([]TreeNode, n)
	for i := 0; i < n; i++ {
		leafs[i] = TreeNode{
			Hash:   hash("leaf_" + strconv.Itoa(i)),
			Weight: newInt(int64(143_000 + i)),
			Height: int32(i),
			final:  true,
		}
	}

	return leafs
}

func hashBranches(left, right *TreeNode) *TreeNode {
	if left == nil {
		return nil
	}

	if right == nil {
		return &TreeNode{Hash: left.Hash, Weight: left.Weight}
	}

	lv := left.Bytes()
	rv := right.Bytes()

	data := make([]byte, len(lv)+len(rv))

	copy(data[:len(lv)], lv[:])
	copy(data[len(rv):], rv[:])

	return &TreeNode{
		Hash:   chainhash.HashH(data),
		Weight: new(big.Int).Add(left.Weight, right.Weight),
	}
}
