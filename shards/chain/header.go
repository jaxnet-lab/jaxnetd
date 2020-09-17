package chain

import (
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/encoder"
	"io"
	"time"
)

// FilterType is used to represent a filter type.
type FilterType uint8

const (
	// GCSFilterRegular is the regular filter type.
	GCSFilterRegular FilterType = iota
)

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

	Version() int32
	Read(r io.Reader) error
	Write(r io.Writer) error
	BtcEncode(w io.Writer, prev uint32, enc encoder.MessageEncoding) error
	//Size() int
}

type Block interface {
}
