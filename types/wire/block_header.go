// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

const (
	flagsReserve = 4

	ExpansionApprove = 1 << iota
	ExpansionMade
)

type HeaderBox struct {
	ActualMMRRoot chainhash.Hash
	Header        BlockHeader
}

type BlockHeader interface {
	BeaconHeader() *BeaconHeader
	SetBeaconHeader(bh *BeaconHeader, beaconAux CoinbaseAux)

	Version() BVersion
	Height() int32
	ChainWeight() uint64

	PrevBlocksMMRRoot() chainhash.Hash
	SetPrevBlocksMMRRoot(prevBlock chainhash.Hash)

	Timestamp() time.Time
	SetTimestamp(time.Time)

	MerkleRoot() chainhash.Hash
	SetMerkleRoot(chainhash.Hash)

	Bits() uint32
	SetBits(uint32)

	Nonce() uint32
	SetNonce(uint32)

	K() uint32
	SetK(value uint32)

	VoteK() uint32
	SetVoteK(value uint32)

	// BlockHash computes hash of header data including hash of the aux data.
	// BlockHash must be used as unique block identifier eq to BLOCK_ID.
	BlockHash() chainhash.Hash
	// ExclusiveHash computes hash of header data without any extra aux data.
	// ExclusiveHash needed to build inclusion proofs for merge mining
	ExclusiveHash() chainhash.Hash
	// PoWHash computes the hash for block that will be used to check ProofOfWork.
	PoWHash() chainhash.Hash

	// UpdateCoinbaseScript sets new coinbase script, rebuilds BTCBlockAux.TxMerkleProof
	// and recalculates the BTCBlockAux.MerkleRoot with the updated extra nonce.
	UpdateCoinbaseScript(coinbaseScript []byte)

	MergeMiningNumber() uint32
	SetMergeMiningNumber(n uint32)

	SetShardMerkleProof(path []chainhash.Hash)
	ShardMerkleProof() []chainhash.Hash

	MergeMiningRoot() chainhash.Hash
	SetMergeMiningRoot(value chainhash.Hash)

	Read(r io.Reader) error
	Write(r io.Writer) error
	BtcEncode(w io.Writer, prev uint32, enc MessageEncoding) error

	// Copy creates a deep copy of a BlockHeader so that the original does not get
	// modified when the copy is manipulated.
	Copy() BlockHeader
	MaxLength() int
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
	return bv | ExpansionApprove
}

func (bv BVersion) UnsetExpansionApproved() BVersion {
	if bv&ExpansionApprove == ExpansionApprove {
		return bv ^ ExpansionApprove
	}
	return bv
}

func (bv BVersion) ExpansionMade() bool {
	return bv&ExpansionMade == ExpansionMade
}

func (bv BVersion) SetExpansionMade() BVersion {
	return bv | ExpansionMade
}

func (bv BVersion) UnsetExpansionMade() BVersion {
	if bv&ExpansionMade == ExpansionMade {
		return bv ^ ExpansionMade
	}
	return bv
}

type HeaderConstructor interface {
	EmptyHeader() BlockHeader
	BlockHeaderOverhead() int
	MaxBlockHeaderPayload() int
	IsBeacon() bool
	ShardID() uint32
}

type BeaconHeaderConstructor struct{}

func (b BeaconHeaderConstructor) EmptyHeader() BlockHeader   { return &BeaconHeader{} }
func (b BeaconHeaderConstructor) IsBeacon() bool             { return true }
func (b BeaconHeaderConstructor) ShardID() uint32            { return 0 }
func (b BeaconHeaderConstructor) MaxBlockHeaderPayload() int { return MaxBeaconBlockHeaderPayload }
func (b BeaconHeaderConstructor) BlockHeaderOverhead() int {
	return MaxBeaconBlockHeaderPayload + MaxVarIntPayload
}

type ShardHeaderConstructor struct{ ID uint32 }

func (b ShardHeaderConstructor) EmptyHeader() BlockHeader   { return &ShardHeader{} }
func (b ShardHeaderConstructor) IsBeacon() bool             { return false }
func (b ShardHeaderConstructor) ShardID() uint32            { return b.ID }
func (b ShardHeaderConstructor) MaxBlockHeaderPayload() int { return MaxShardBlockHeaderPayload }
func (b ShardHeaderConstructor) BlockHeaderOverhead() int {
	return MaxShardBlockHeaderPayload + MaxVarIntPayload
}

func DecodeHeader(r io.Reader) (BlockHeader, error) {
	var magicN [1]byte
	err := ReadElement(r, &magicN)
	if err != nil {
		return nil, err
	}

	switch magicN[0] {
	case beaconMagicByte:
		h := &BeaconHeader{}
		err = readBeaconBlockHeader(r, h, true)
		return h, err
	case shardMagic:
		h := &ShardHeader{}
		err = readShardBlockHeader(r, h, true)
		return h, err
	default:
		return nil, fmt.Errorf("invalid magic byte: 0x%0x, expected beacon(0x%0x) or shard(0x%0x)",
			magicN, beaconMagicByte, shardMagic)
	}
}

func DecodeBlock(r io.Reader) (*MsgBlock, error) {
	block := &MsgBlock{}
	err := block.Deserialize(r)
	return block, err
}
