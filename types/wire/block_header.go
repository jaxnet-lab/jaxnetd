// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"io"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/encoder"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

const (
	flagsReserve = 4

	ExpansionApprove = 1 << iota
	ExpansionExec
)

type BlockHeader interface {
	BeaconHeader() *BeaconHeader
	SetBeaconHeader(bh *BeaconHeader)

	Version() BVersion

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

	K() uint32
	SetK(value uint32)

	VoteK() uint32
	SetVoteK(value uint32)

	BlockHash() chainhash.Hash

	// PoWHash computes the hash for block that will be used to check ProofOfWork.
	PoWHash() chainhash.Hash

	// UpdateCoinbaseScript sets new coinbase script, rebuilds BTCBlockAux.TxMerkle
	// and recalculates the BTCBlockAux.MerkleRoot with the updated extra nonce.
	UpdateCoinbaseScript(coinbaseScript []byte)

	Read(r io.Reader) error
	Write(r io.Writer) error
	BtcEncode(w io.Writer, prev uint32, enc encoder.MessageEncoding) error

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
	return bv&ExpansionExec == ExpansionExec
}

func (bv BVersion) SetExpansionMade() BVersion {
	return bv | ExpansionExec
}

func (bv BVersion) UnsetExpansionMade() BVersion {
	if bv&ExpansionExec == ExpansionExec {
		return bv ^ ExpansionExec
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
	return MaxBeaconBlockHeaderPayload + encoder.MaxVarIntPayload
}

type ShardHeaderConstructor struct{ ID uint32 }

func (b ShardHeaderConstructor) EmptyHeader() BlockHeader {
	return &ShardHeader{bCHeader: BeaconHeader{}}
}
func (b ShardHeaderConstructor) IsBeacon() bool             { return false }
func (b ShardHeaderConstructor) ShardID() uint32            { return b.ID }
func (b ShardHeaderConstructor) MaxBlockHeaderPayload() int { return MaxShardBlockHeaderPayload }
func (b ShardHeaderConstructor) BlockHeaderOverhead() int {
	return MaxShardBlockHeaderPayload + encoder.MaxVarIntPayload
}