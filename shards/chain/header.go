package chain

import (
	"io"
	"time"

	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/encoder"
)

// FilterType is used to represent a filter type.
type FilterType uint8

const (
	// GCSFilterRegular is the regular filter type.
	GCSFilterRegular FilterType = iota
)
const (
	ExpansionApprove = 1 << iota
	ExpansionExec
	ShardBlock
)

type BVersion int32

func NewBVersion(version int32) BVersion {
	var v = BVersion(version << 4)

	return v
}

func (bv BVersion) ExpansionApproved() bool {
	return bv&ExpansionApprove == ExpansionApprove
}

func (bv BVersion) SetExpansionApproved() BVersion {
	return bv ^ ExpansionApprove
}

func (bv BVersion) ExpansionMade() bool {
	return bv&ExpansionExec == ExpansionExec
}

func (bv BVersion) SetExpansionMade() BVersion {
	return bv ^ ExpansionExec
}

func (bv BVersion) BeaconChainBlock() bool {
	return bv&ShardBlock != ShardBlock
}

func (bv BVersion) ShardChainBlock() bool {
	return bv&ShardBlock == ShardBlock
}

func (bv BVersion) SetShard() BVersion {
	return bv ^ ShardBlock
}

const BlockHeaderLen = 80

type BlockHeader interface {
	BlockData() []byte
	BlockHash() chainhash.Hash
	PrevBlock() chainhash.Hash
	SetPrevBlock(prevBlock chainhash.Hash)
	Timestamp() time.Time
	SetTimestamp(time.Time)
	MerkleRoot() chainhash.Hash
	SetMerkleRoot(chainhash.Hash)
	SetMergeMiningRoot(value chainhash.Hash)
	MergeMiningRoot() chainhash.Hash
	Bits() uint32
	SetBits(uint32)
	Nonce() uint32
	SetNonce(uint32)

	Version() BVersion
	Read(r io.Reader) error
	Write(r io.Writer) error
	BtcEncode(w io.Writer, prev uint32, enc encoder.MessageEncoding) error
	//Size() int
}

type Block interface {
}
