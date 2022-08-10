package mmr

import (
	"log"
	"math/big"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type TBlockNode struct {
	hash          chainhash.Hash
	prevMMRRoot   chainhash.Hash
	actualMMRRoot chainhash.Hash
	height        int32
	difficulty    *big.Int
	parent        *TBlockNode
}

func (tn *TBlockNode) GetHash() chainhash.Hash     { return tn.hash }
func (tn *TBlockNode) PrevMMRRoot() chainhash.Hash { return tn.prevMMRRoot }
func (tn *TBlockNode) PrevHash() chainhash.Hash    { return tn.Parent().GetHash() }
func (tn *TBlockNode) Height() int32               { return tn.height }
func (tn *TBlockNode) PowWeight() *big.Int         { return tn.difficulty }

func (tn *TBlockNode) Parent() blocknodes.IBlockNode {
	if tn.parent == nil {
		return blocknodes.IBlockNode(nil)
	}
	return tn.parent
}

func (tn *TBlockNode) SetActualMMRRoot(actualMMRRoot chainhash.Hash) {
	tn.actualMMRRoot = actualMMRRoot
}

func (tn *TBlockNode) Header() wire.BlockHeader                     { panic("bender") }
func (tn *TBlockNode) SerialID() int64                              { panic("bender") }
func (tn *TBlockNode) Version() int32                               { panic("bender") }
func (tn *TBlockNode) Bits() uint32                                 { panic("bender") }
func (tn *TBlockNode) K() uint32                                    { panic("bender") }
func (tn *TBlockNode) VoteK() uint32                                { panic("bender") }
func (tn *TBlockNode) Status() blocknodes.BlockStatus               { panic("bender") }
func (tn *TBlockNode) WorkSum() *big.Int                            { panic("bender") }
func (tn *TBlockNode) Timestamp() int64                             { panic("bender") }
func (tn *TBlockNode) ExpansionApproved() bool                      { panic("bender") }
func (tn *TBlockNode) SetStatus(blocknodes.BlockStatus)             { panic("bender") }
func (tn *TBlockNode) Ancestor(int32) blocknodes.IBlockNode         { panic("bender") }
func (tn *TBlockNode) CalcPastMedianTime() time.Time                { panic("bender") }
func (tn *TBlockNode) CalcPastMedianTimeForN(int) time.Time         { panic("bender") }
func (tn *TBlockNode) CalcMedianVoteK() uint32                      { panic("bender") }
func (tn *TBlockNode) RelativeAncestor(int32) blocknodes.IBlockNode { panic("bender") }
func (tn *TBlockNode) NewHeader() wire.BlockHeader                  { panic("bender") }
func (tn *TBlockNode) ActualMMRRoot() chainhash.Hash                { panic("bender") }

// GenerateNLeafs - function for testing which generates n leafs to insert in MMR tree, filled with
// realistic data
func GenerateNLeafs(n int64) []TreeNode {
	var (
		prevLeaf TreeNode
		leaf     TreeNode
		leafs    []TreeNode
	)

	prevLeaf = TreeNode{
		Hash:   chainhash.HashH(nil),
		Weight: big.NewInt(0),
	}
	prevLeaf.v = append(prevLeaf.Hash.CloneBytes(), prevLeaf.Weight.Bytes()...)
	leafs = append(leafs, prevLeaf)

	for i := int64(1); i <= n; i++ {
		leaf = TreeNode{
			Hash:   chainhash.HashH(prevLeaf.Hash.CloneBytes()),
			Weight: big.NewInt(i),
		}
		leaf.v = append(leaf.Hash.CloneBytes(), prevLeaf.Weight.Bytes()...)
		leafs = append(leafs, leaf)

		prevLeaf = leaf
	}

	return leafs
}

func GenerateBlockNodeChain(c *TreeContainer, n int64) {
	blocks := GenerateNLeafs(n)

	blockNodes := make([]*TBlockNode, len(blocks))

	genesisBlockNode := &TBlockNode{
		hash:          blocks[0].Hash,
		prevMMRRoot:   chainhash.ZeroHash,
		actualMMRRoot: chainhash.Hash{},
		height:        0,
		difficulty:    blocks[0].Weight,
		parent:        nil,
	}
	c.SetNodeToMmrWithReorganization(genesisBlockNode)
	blockNodes[0] = genesisBlockNode

	for i := 1; i < len(blocks); i++ {
		prevMMRRoot := c.Current().Hash
		blNode := &TBlockNode{
			hash:          blocks[i].Hash,
			prevMMRRoot:   prevMMRRoot,
			actualMMRRoot: chainhash.Hash{},
			height:        int32(i),
			difficulty:    blocks[i].Weight,
			parent:        blockNodes[i-1],
		}
		ok := c.SetNodeToMmrWithReorganization(blNode)
		if !ok {
			log.Fatalf("cannot create tree")
		}
		blockNodes[i] = blNode
	}
}
