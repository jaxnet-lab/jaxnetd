/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package blocknodes

import (
	"math/big"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const (

	// StatusDataStored indicates that the block's payload is stored on disk.
	StatusDataStored BlockStatus = 1 << iota

	// StatusValid indicates that the block has been fully validated.
	StatusValid

	// StatusValidateFailed indicates that the block has failed validation.
	StatusValidateFailed

	// StatusInvalidAncestor indicates that one of the block's ancestors has
	// has failed validation, thus the block is also invalid.
	StatusInvalidAncestor

	// StatusNone indicates that the block has no validation state flags set.
	//
	// NOTE: This must be defined last in order to avoid influencing iota.
	StatusNone BlockStatus = 0
)

type IBlockNode interface {
	// GetHash returns hash of the block (including aux data).
	GetHash() chainhash.Hash
	// PrevMMRRoot is the root of the MMR tree before this node was added to the chain.
	PrevMMRRoot() chainhash.Hash
	// ActualMMRRoot is the root of the MMR tree after adding this node to the chain.
	ActualMMRRoot() chainhash.Hash
	// PrevHash returns hash of parent node.
	PrevHash() chainhash.Hash

	Height() int32
	SerialID() int64
	Version() int32
	Bits() uint32
	Difficulty() uint64
	K() uint32
	VoteK() uint32
	Status() BlockStatus
	WorkSum() *big.Int
	Timestamp() int64
	ExpansionApproved() bool

	SetStatus(status BlockStatus)
	SetActualMMRRoot(chainhash.Hash)

	Header() wire.BlockHeader
	Parent() IBlockNode
	Ancestor(height int32) IBlockNode
	CalcPastMedianTime() time.Time
	CalcPastMedianTimeForN(nBlocks int) time.Time
	CalcMedianVoteK() uint32
	RelativeAncestor(distance int32) IBlockNode

	NewHeader() wire.BlockHeader // required only for tests
}

// BlockStatus is a bit field representing the validation state of the block.
type BlockStatus byte

// HaveData returns whether the full block data is stored in the database. This
// will return false for a block node where only the header is downloaded or
// kept.
func (status BlockStatus) HaveData() bool {
	return status&StatusDataStored != 0
}

// KnownValid returns whether the block is known to be valid. This will return
// false for a valid block that has not been fully validated yet.
func (status BlockStatus) KnownValid() bool {
	return status&StatusValid != 0
}

// KnownInvalid returns whether the block is known to be invalid. This may be
// because the block itself failed validation or any of its ancestors is
// invalid. This will return false for invalid blocks that have not been proven
// invalid yet.
func (status BlockStatus) KnownInvalid() bool {
	return status&(StatusValidateFailed|StatusInvalidAncestor) != 0
}

// bigFloatSorter implements sort.Interface to allow a slice of big.Float to
// be sorted.
type bigFloatSorter []*big.Float

// Len returns the number of timestamps in the slice.  It is part of the
// sort.Interface implementation.
func (s bigFloatSorter) Len() int {
	return len(s)
}

// Swap swaps the timestamps at the passed indices.  It is part of the
// sort.Interface implementation.
func (s bigFloatSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns whether the timstamp with index i should sort before the
// timestamp with index j.  It is part of the sort.Interface implementation.
func (s bigFloatSorter) Less(i, j int) bool {
	return s[i].Cmp(s[j]) < 0
}
