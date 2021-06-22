// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaindata

import (
	"time"

	"gitlab.com/jaxnet/core/shard.core/types/blocknode"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
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
	Hash        chainhash.Hash // The hash of the block.
	Height      int32          // The height of the block.
	Bits        uint32         // The difficulty bits of the block.
	BlockSize   uint64         // The size of the block.
	BlockWeight uint64         // The weight of the block.
	NumTxns     uint64         // The number of txns in the block.
	TotalTxns   uint64         // The total number of txns in the chain.
	MedianTime  time.Time      // Median time as per CalcPastMedianTime.
}

// NewBestState returns a new best stats instance for the given parameters.
func NewBestState(node blocknode.IBlockNode, blockSize, blockWeight, numTxns,
	totalTxns uint64, medianTime time.Time) *BestState {

	return &BestState{
		Hash:        node.GetHash(),
		Height:      node.Height(),
		Bits:        node.Bits(),
		BlockSize:   blockSize,
		BlockWeight: blockWeight,
		NumTxns:     numTxns,
		TotalTxns:   totalTxns,
		MedianTime:  medianTime,
	}
}
