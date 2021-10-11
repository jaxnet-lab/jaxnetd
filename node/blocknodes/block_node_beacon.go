/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package blocknodes

import (
	"math/big"
	"sort"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// beaconMedianTimeBlocks is the number of previous blocks which should be
// used to calculate the median time used to validate block timestamps.
const beaconMedianTimeBlocks = 11

// BeaconBlockNode represents a block within the block chain and is primarily used to
// aid in selecting the best chain to be the main chain.  The main chain is
// stored into the block database.
type BeaconBlockNode struct {
	// NOTE: Additions, deletions, or modifications to the order of the
	// definitions in this struct should not be changed without considering
	// how it affects alignment on 64-bit platforms.  The current order is
	// specifically crafted to result in minimal padding.  There will be
	// hundreds of thousands of these in memory, so a few extra bytes of
	// padding adds up.

	parent     IBlockNode     // parent is the parent block for this node.
	hash       chainhash.Hash // hash is the double sha 256 of the block.
	workSum    *big.Int       // workSum is the total amount of work in the chain up to and including this node.
	height     int32          // height is the position in the block chain.
	serialID   int64          // serialID is the absolute unique id of current block.
	timestamp  int64
	difficulty uint64
	header     wire.BlockHeader
	// status is a bitfield representing the validation state of the block. The
	// status field, unlike the other fields, may be written to and so should
	// only be accessed using the concurrent-safe NodeStatus method on
	// blockIndex once the node has been added to the global index.
	status BlockStatus
}

// NewBeaconBlockNode returns a new block node for the given block header and parent
// node, calculating the height and workSum from the respective fields on the
// parent. This function is NOT safe for concurrent access.
func NewBeaconBlockNode(blockHeader wire.BlockHeader, parent IBlockNode, _ uint32) *BeaconBlockNode {
	node := &BeaconBlockNode{
		hash:      blockHeader.BlockHash(),
		workSum:   pow.CalcWork(blockHeader.Bits()),
		timestamp: blockHeader.Timestamp().Unix(),
		header:    blockHeader,
	}
	if blockHeader.Bits() != 0 {
		node.difficulty = pow.CalcWork(blockHeader.Bits()).Uint64()
	}

	if parent != nil && parent != (*BeaconBlockNode)(nil) {
		node.parent = parent
		node.height = parent.Height() + 1
		node.serialID = parent.SerialID() + 1 // todo: check correctness
		node.workSum = node.workSum.Add(parent.WorkSum(), node.workSum)
	}
	return node
}

func (node *BeaconBlockNode) GetHash() chainhash.Hash { return node.hash }
func (node *BeaconBlockNode) BlocksMMRRoot() chainhash.Hash {
	return node.header.BlocksMerkleMountainRoot()
}
func (node *BeaconBlockNode) Version() int32               { return node.header.Version().Version() }
func (node *BeaconBlockNode) Height() int32                { return node.height }
func (node *BeaconBlockNode) SerialID() int64              { return node.serialID }
func (node *BeaconBlockNode) Bits() uint32                 { return node.header.Bits() }
func (node *BeaconBlockNode) Difficulty() uint64           { return node.difficulty }
func (node *BeaconBlockNode) K() uint32                    { return node.header.K() }
func (node *BeaconBlockNode) VoteK() uint32                { return node.header.VoteK() }
func (node *BeaconBlockNode) Parent() IBlockNode           { return node.parent }
func (node *BeaconBlockNode) WorkSum() *big.Int            { return node.workSum }
func (node *BeaconBlockNode) Timestamp() int64             { return node.timestamp }
func (node *BeaconBlockNode) Status() BlockStatus          { return node.status }
func (node *BeaconBlockNode) SetStatus(status BlockStatus) { node.status = status }
func (node *BeaconBlockNode) NewHeader() wire.BlockHeader  { return new(wire.BeaconHeader) }

// Header constructs a block header from the node and returns it.
//
// This function is safe for concurrent access.
func (node *BeaconBlockNode) Header() wire.BlockHeader {
	return node.header
}

// Ancestor returns the ancestor block node at the provided height by following
// the chain backwards from this node.  The returned block will be nil when a
// height is requested that is after the height of the passed node or is less
// than zero.
//
// This function is safe for concurrent access.
func (node *BeaconBlockNode) Ancestor(height int32) IBlockNode {
	if height < 0 || height > node.height {
		return nil
	}

	n := IBlockNode(node)
	for ; n != nil && n.Height() != height; n = n.Parent() {
		// Intentionally left blank
	}

	return n
}

// RelativeAncestor returns the ancestor block node a relative 'distance' blocks
// before this node.  This is equivalent to calling Ancestor with the node's
// height minus provided distance.
//
// This function is safe for concurrent access.
func (node *BeaconBlockNode) RelativeAncestor(distance int32) IBlockNode {
	return node.Ancestor(node.height - distance)
}

// CalcPastMedianTime calculates the median time of the previous few blocks
// prior to, and including, the block node.
//
// This function is safe for concurrent access.
func (node *BeaconBlockNode) CalcPastMedianTime() time.Time {
	return node.CalcPastMedianTimeForN(beaconMedianTimeBlocks)
}

// CalcPastMedianTimeForN calculates the median time of the previous N blocks
// prior to, and including, the block node.
//
// This function is safe for concurrent access.
func (node *BeaconBlockNode) CalcPastMedianTimeForN(nBlocks int) time.Time {
	// Create a slice of the previous few block timestamps used to calculate
	// the median per the number defined by the constant beaconMedianTimeBlocks.
	timestamps := make([]int64, nBlocks)
	numNodes := 0
	iterNode := IBlockNode(node)
	for i := 0; i < nBlocks && iterNode != nil; i++ {
		timestamps[i] = iterNode.Timestamp()
		numNodes++

		iterNode = iterNode.Parent()
	}

	// Prune the slice to the actual number of available timestamps which
	// will be fewer than desired near the beginning of the block chain
	// and sort them.
	timestamps = timestamps[:numNodes]
	sort.Sort(types.TimeSorter(timestamps))

	// NOTE: The consensus rules incorrectly calculate the median for even
	// numbers of blocks.  A true median averages the middle two elements
	// for a set with an even number of elements in it.   Since the constant
	// for the previous number of blocks to be used is odd, this is only an
	// issue for a few blocks near the beginning of the chain.  I suspect
	// this is an optimization even though the result is slightly wrong for
	// a few of the first blocks since after the first few blocks, there
	// will always be an odd number of blocks in the set per the constant.
	//
	// This code follows suit to ensure the same rules are used, however, be
	// aware that should the beaconMedianTimeBlocks constant ever be changed to an
	// even number, this code will be wrong.
	medianTimestamp := timestamps[numNodes/2]
	return time.Unix(medianTimestamp, 0)
}

func (node *BeaconBlockNode) CalcMedianVoteK() uint32 {
	// Create a slice of the previous few block voteKs used to calculate
	// the median per the number defined by the constant beaconMedianTimeBlocks.
	nBlocks := pow.KBeaconEpochLen

	voteKs := make([]*big.Float, nBlocks)
	numNodes := 0
	iterNode := IBlockNode(node)
	for i := 0; i < nBlocks && iterNode != nil; i++ {
		voteKs[i] = pow.UnpackK(iterNode.VoteK())
		numNodes++

		iterNode = iterNode.Parent()
	}

	// Prune the slice to the actual number of available voteKs which
	// will be fewer than desired near the beginning of the block chain
	// and sort them.
	voteKs = voteKs[:numNodes]
	sort.Sort(bigFloatSorter(voteKs))

	// NOTE: The consensus rules incorrectly calculate the median for even
	// numbers of blocks.  A true median averages the middle two elements
	// for a set with an even number of elements in it.   Since the constant
	// for the previous number of blocks to be used is odd, this is only an
	// issue for a few blocks near the beginning of the chain.  I suspect
	// this is an optimization even though the result is slightly wrong for
	// a few of the first blocks since after the first few blocks, there
	// will always be an odd number of blocks in the set per the constant.
	//
	// This code follows suit to ensure the same rules are used, however, be
	// aware that should the beaconMedianTimeBlocks constant ever be changed to an
	// even number, this code will be wrong.
	medianVoteK := voteKs[numNodes/2]
	return pow.PackK(medianVoteK)
}

func (node *BeaconBlockNode) ExpansionApproved() bool {
	nBlocks := 1024

	numNodes := 0
	iterNode := IBlockNode(node)
	expansionApprove := 0
	for i := 0; i < nBlocks && iterNode != nil; i++ {
		version := iterNode.Header().Version()
		if version.ExpansionApproved() {
			expansionApprove += 1
		}
		numNodes++

		iterNode = iterNode.Parent()
	}

	return expansionApprove >= 768
}