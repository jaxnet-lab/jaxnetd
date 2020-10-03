package wire

import (
	"io"
	"time"

	"gitlab.com/jaxnet/core/shard.core/node/encoder"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
)

const (
	flagsReserve = 4

	ExpansionApprove = 1 << iota
	ExpansionExec
)

type BlockHeader interface {
	BlockData() []byte
	BlockHash() chainhash.Hash
	PrevBlock() chainhash.Hash
	SetPrevBlock(prevBlock chainhash.Hash)
	Timestamp() time.Time
	SetTimestamp(time.Time)
	MerkleRoot() chainhash.Hash
	SetMerkleRoot(chainhash.Hash)

	Bits() uint32
	SetBits(uint32)
	Nonce() uint32
	SetNonce(uint32)

	Version() BVersion
	Read(r io.Reader) error
	Write(r io.Writer) error
	BtcEncode(w io.Writer, prev uint32, enc encoder.MessageEncoding) error
	// Size() int

	SetMergeMiningRoot(value chainhash.Hash)
	MergeMiningRoot() chainhash.Hash
}

type BVersion int32

func NewBVersion(version int32) BVersion {
	return BVersion(version << flagsReserve)
}

func (bv BVersion) Version() int32 {
	return int32(bv) >> flagsReserve
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

const BlockHeaderLen = 80

type HeaderConstructor interface {
	NewEmptyHeader() BlockHeader
	BlockHeaderOverhead() int
	MaxBlockHeaderPayload() int
	IsBeacon() bool
	ShardID() uint32
}

type BeaconHeaderConstructor struct{}

func (b BeaconHeaderConstructor) NewEmptyHeader() BlockHeader { return &BeaconHeader{} }
func (b BeaconHeaderConstructor) IsBeacon() bool              { return true }
func (b BeaconHeaderConstructor) ShardID() uint32             { return 0 }
func (b BeaconHeaderConstructor) MaxBlockHeaderPayload() int  { return MaxBeaconBlockHeaderPayload }
func (b BeaconHeaderConstructor) BlockHeaderOverhead() int {
	return MaxBeaconBlockHeaderPayload + encoder.MaxVarIntPayload
}

type ShardHeaderConstructor struct{ ID uint32 }

func (b ShardHeaderConstructor) NewEmptyHeader() BlockHeader { return &ShardHeader{} }
func (b ShardHeaderConstructor) IsBeacon() bool              { return false }
func (b ShardHeaderConstructor) ShardID() uint32             { return b.ID }
func (b ShardHeaderConstructor) MaxBlockHeaderPayload() int  { return MaxShardBlockHeaderPayload }
func (b ShardHeaderConstructor) BlockHeaderOverhead() int {
	return MaxShardBlockHeaderPayload + encoder.MaxVarIntPayload
}
