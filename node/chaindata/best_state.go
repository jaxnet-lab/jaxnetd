// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaindata

import (
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// BestState houses information about the current best block and other info
// related to the state of the main chain as it exists from the point of view of
// the current best block.
//
// The BestSnapshot method can be used to obtain access to this information
// in a concurrent safe manner and the data will not be changed out from under
// the caller when chain state changes occur as the function name implies.
// However, the returned snapshot must be treated as immutable since it is
// shared by all callers.
type BestState struct {
	Hash           chainhash.Hash // The hash of the block.
	Height         int32          // The height of the block.
	Bits           uint32         // The difficulty bits of the block.
	K              uint32         // The K coefficient.
	Shards         uint32         // The last known number of Shards.
	BlockSize      uint64         // The size of the block.
	BlockWeight    uint64         // The weight of the block.
	NumTxns        uint64         // The number of txns in the block.
	TotalTxns      uint64         // The total number of txns in the chain.
	MedianTime     time.Time      // Median time as per CalcPastMedianTime.
	LastSerialID   int64
	CurrentMMRRoot chainhash.Hash // Actual root of the MMR Tree.
	ChainWeight    uint64
}

// NewBestState returns a new best stats instance for the given parameters.
func NewBestState(node blocknodes.IBlockNode, actualMMRRoot chainhash.Hash,
	blockSize, blockWeight, chainWeight, numTxns, totalTxns uint64,
	medianTime time.Time, lastSerialID int64) *BestState {

	return &BestState{
		Hash:           node.GetHash(),
		CurrentMMRRoot: actualMMRRoot,
		Height:         node.Height(),
		Bits:           node.Bits(),
		K:              node.Header().BeaconHeader().K(),
		Shards:         node.Header().BeaconHeader().Shards(),
		BlockSize:      blockSize,
		BlockWeight:    blockWeight,
		NumTxns:        numTxns,
		TotalTxns:      totalTxns,
		MedianTime:     medianTime,
		LastSerialID:   lastSerialID,
		ChainWeight:    chainWeight,
	}
}
