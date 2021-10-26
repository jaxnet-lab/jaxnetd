package mmr

import (
	"math/big"
	"testing"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

func TestTreeContainer_SetNodeToMmrWithReorganization(t *testing.T) {
	mmrContainer := TreeContainer{
		BlocksMMRTree: NewTree(),
		RootToBlock:   map[chainhash.Hash]chainhash.Hash{},
	}

	mmrContainer.SetNodeToMmrWithReorganization(nil)
}

type TBlockNode struct {
	hash          chainhash.Hash
	prevMMRRoot   chainhash.Hash
	actualMMRRoot chainhash.Hash
	height        int32
	difficulty    uint64
	parent        *TBlockNode
}

func (tn *TBlockNode) GetHash() chainhash.Hash       { return tn.hash }
func (tn *TBlockNode) PrevMMRRoot() chainhash.Hash   { return tn.prevMMRRoot }
func (tn *TBlockNode) ActualMMRRoot() chainhash.Hash { return tn.actualMMRRoot }
func (tn *TBlockNode) PrevHash() chainhash.Hash      { return tn.Parent().GetHash() }
func (tn *TBlockNode) Height() int32                 { return tn.height }
func (tn *TBlockNode) Difficulty() uint64            { return tn.difficulty }
func (tn *TBlockNode) Parent() blocknodes.IBlockNode { return tn.parent }

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
func (tn *TBlockNode) SetActualMMRRoot(chainhash.Hash)              { panic("bender") }
func (tn *TBlockNode) Header() wire.BlockHeader                     { panic("bender") }
func (tn *TBlockNode) Ancestor(int32) blocknodes.IBlockNode         { panic("bender") }
func (tn *TBlockNode) CalcPastMedianTime() time.Time                { panic("bender") }
func (tn *TBlockNode) CalcPastMedianTimeForN(int) time.Time         { panic("bender") }
func (tn *TBlockNode) CalcMedianVoteK() uint32                      { panic("bender") }
func (tn *TBlockNode) RelativeAncestor(int32) blocknodes.IBlockNode { panic("bender") }
func (tn *TBlockNode) NewHeader() wire.BlockHeader                  { panic("bender") }
