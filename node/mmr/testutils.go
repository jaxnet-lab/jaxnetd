package mmr

import (
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
	"math/big"
	"time"
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
func (tn *TBlockNode) Header() wire.BlockHeader { panic("bender") }

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
