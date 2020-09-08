package chain

import (
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"math/big"
	"time"
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

// blockStatus is a bit field representing the validation state of the block.
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

type IBlockNode interface {
	NewNode() IBlockNode
	GetHash() chainhash.Hash
	Height() int32
	Version() int32
	Bits() uint32
	SetBits(uint32)
	Status() BlockStatus
	SetStatus(status BlockStatus)
	NewHeader() BlockHeader

	Header() BlockHeader
	Parent() IBlockNode
	Ancestor(height int32) IBlockNode
	CalcPastMedianTime() time.Time
	RelativeAncestor(distance int32) IBlockNode
	WorkSum() *big.Int
	Timestamp() int64
}

var DefaultChain IChain
