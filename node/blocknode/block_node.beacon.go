package blocknode

import (
	"math/big"
	"sort"
	"time"

	"gitlab.com/jaxnet/core/shard.core/types"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/pow"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

var zeroHash chainhash.Hash

const (
	medianTimeBlocks = 11
)

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

	// parent is the parent block for this node.
	parent IBlockNode

	// hash is the double sha 256 of the block.
	hash chainhash.Hash

	// workSum is the total amount of work in the chain up to and including
	// this node.
	workSum *big.Int

	// height is the position in the block chain.
	height int32

	// Some fields from block headers to aid in best chain selection and
	// reconstructing headers from memory.  These must be treated as
	// immutable and are intentionally ordered to avoid padding on 64-bit
	// platforms.
	version    int32
	bits       uint32
	nonce      uint32
	timestamp  int64
	merkleRoot chainhash.Hash
	mmrRoot    chainhash.Hash

	// status is a bitfield representing the validation state of the block. The
	// status field, unlike the other fields, may be written to and so should
	// only be accessed using the concurrent-safe NodeStatus method on
	// blockIndex once the node has been added to the global index.
	status BlockStatus
}

// initBeaconBlockNode initializes a block node from the given header and parent node,
// calculating the height and workSum from the respective fields on the parent.
// This function is NOT safe for concurrent access.  It must only be called when
// initially creating a node.
func initBeaconBlockNode(node *BeaconBlockNode, blockHeader wire.BlockHeader, parent IBlockNode) {
	*node = BeaconBlockNode{
		hash:       blockHeader.BlockHash(),
		workSum:    pow.CalcWork(blockHeader.Bits()),
		version:    int32(blockHeader.Version()),
		bits:       blockHeader.Bits(),
		nonce:      blockHeader.Nonce(),
		timestamp:  blockHeader.Timestamp().Unix(),
		merkleRoot: blockHeader.MerkleRoot(),
	}
	if parent != nil {
		node.parent = parent
		node.height = parent.Height() + 1
		node.workSum = node.workSum.Add(parent.WorkSum(), node.workSum)
	}
}

// newBlockNode returns a new block node for the given block header and parent
// node, calculating the height and workSum from the respective fields on the
// parent. This function is NOT safe for concurrent access.
func NewBeaconBlockNode(blockHeader wire.BlockHeader, parent IBlockNode) *BeaconBlockNode {
	var node BeaconBlockNode
	initBeaconBlockNode(&node, blockHeader, parent)
	return &node
}

func (node *BeaconBlockNode) NewNode() IBlockNode {
	var res BeaconBlockNode
	return &res
}

func (node *BeaconBlockNode) GetHeight() int32             { return node.height }
func (node *BeaconBlockNode) GetHash() chainhash.Hash      { return node.hash }
func (node *BeaconBlockNode) Version() int32               { return node.version }
func (node *BeaconBlockNode) Height() int32                { return node.height }
func (node *BeaconBlockNode) Bits() uint32                 { return node.bits }
func (node *BeaconBlockNode) SetBits(value uint32)         { node.bits = value }
func (node *BeaconBlockNode) Parent() IBlockNode           { return node.parent }
func (node *BeaconBlockNode) WorkSum() *big.Int            { return node.workSum }
func (node *BeaconBlockNode) Timestamp() int64             { return node.timestamp }
func (node *BeaconBlockNode) Status() BlockStatus          { return node.status }
func (node *BeaconBlockNode) SetStatus(status BlockStatus) { node.status = status }

func (node *BeaconBlockNode) NewHeader() wire.BlockHeader {
	return new(wire.BeaconHeader)
}

// header constructs a block header from the node and returns it.
//
// This function is safe for concurrent access.
func (node *BeaconBlockNode) Header() wire.BlockHeader {
	// No lock is needed because all accessed fields are immutable.
	prevHash := &zeroHash
	if node.parent != nil {
		h := node.parent.GetHash()
		prevHash = &h
	}

	return wire.NewBeaconBlockHeader(wire.BVersion(node.version), *prevHash,
		node.merkleRoot, node.mmrRoot, time.Unix(node.timestamp, 0), node.bits, node.nonce)
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
	// Create a slice of the previous few block timestamps used to calculate
	// the median per the number defined by the constant medianTimeBlocks.
	timestamps := make([]int64, medianTimeBlocks)
	numNodes := 0
	iterNode := IBlockNode(node)
	for i := 0; i < medianTimeBlocks && iterNode != nil; i++ {
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
	// aware that should the medianTimeBlocks constant ever be changed to an
	// even number, this code will be wrong.
	medianTimestamp := timestamps[numNodes/2]
	return time.Unix(medianTimestamp, 0)
}
