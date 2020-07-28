package chain

import (
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"time"
)

const MaxBlockHeaderPayload = 16 + (chainhash.HashSize * 2)

// FilterType is used to represent a filter type.
type FilterType uint8

const (
	// GCSFilterRegular is the regular filter type.
	GCSFilterRegular FilterType = iota
)

const BlockHeaderLen = 80

type BlockHeader interface {
	//BlockData() []byte
	BlockHash() chainhash.Hash
	//PrevBlock() chainhash.Hash
	Timestamp() time.Time
	SetTimestamp(time.Time)
	//SetNonce(uint32)
	//MerkleRoot() chainhash.Hash
	//SetMerkleRoot(chainhash.Hash)
	//MerkleMountainRange() chainhash.Hash
	//Bits() uint32
	//SetBits(uint32)
	//Nonce() uint32
	//
	//Version() int32
	//Read(r io.Reader) error
	//Write(r io.Writer) error
	//Size() int
}

type Block interface {
}
