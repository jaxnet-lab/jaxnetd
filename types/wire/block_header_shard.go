// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"io"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/encoder"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

const (
	// MaxShardBlockHeaderPayload is the maximum number of bytes a block ShardHeader can be.
	// PrevBlock and MerkleRoot hashes + Timestamp 4 bytes + Bits 4 bytes +
	// + mergeMiningNumber 4 bytes + MaxBeaconBlockHeaderPayload.
	MaxShardBlockHeaderPayload = MaxBeaconBlockHeaderPayload * 2
)

// ShardHeader defines information about a block and is used in the bitcoin
// block (MsgBlock) and headers (MsgHeaders) messages.
type ShardHeader struct {
	// blocksMMRRoot is an actual root of the MerkleMountainRange tree for current block
	blocksMMRRoot chainhash.Hash

	// Merkle tree reference to hash of all transactions for the block.
	merkleRoot chainhash.Hash

	// Difficulty target for the block.
	bits uint32

	// Merge-mining number is the miner’s claim about how
	// many shards he was mining
	mergeMiningNumber uint32

	bCHeader BeaconHeader

	CoinbaseAux
}

func EmptyShardHeader() *ShardHeader { return &ShardHeader{bCHeader: *EmptyBeaconHeader()} }

// NewShardBlockHeader returns a new BlockHeader using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewShardBlockHeader(blocksMerkleMountainRoot, merkleRootHash chainhash.Hash, bits uint32,
	bcHeader BeaconHeader, aux CoinbaseAux) *ShardHeader {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &ShardHeader{
		blocksMMRRoot: blocksMerkleMountainRoot,
		merkleRoot:    merkleRootHash,
		bits:          bits,
		bCHeader:      bcHeader,
		CoinbaseAux:   aux,
	}
}

// Copy creates a deep copy of a BlockHeader so that the original does not get
// modified when the copy is manipulated.
func (h *ShardHeader) Copy() BlockHeader {
	clone := *h

	// all fields except this are passed by value
	// so we manually copy the following fields to prevent side effects
	bc := h.bCHeader.Copy().BeaconHeader()
	clone.bCHeader = *bc
	return &clone
}

func (h *ShardHeader) BeaconHeader() *BeaconHeader      { return &h.bCHeader }
func (h *ShardHeader) SetBeaconHeader(bh *BeaconHeader) { h.bCHeader = *bh }

func (h *ShardHeader) Bits() uint32        { return h.bits }
func (h *ShardHeader) SetBits(bits uint32) { h.bits = bits }

func (h *ShardHeader) MerkleRoot() chainhash.Hash        { return h.merkleRoot }
func (h *ShardHeader) SetMerkleRoot(hash chainhash.Hash) { h.merkleRoot = hash }

func (h *ShardHeader) BlocksMerkleMountainRoot() chainhash.Hash { return h.blocksMMRRoot }
func (h *ShardHeader) SetBlocksMerkleMountainRoot(root chainhash.Hash) {
	h.blocksMMRRoot = root
}

func (h *ShardHeader) Timestamp() time.Time     { return h.bCHeader.btcAux.Timestamp }
func (h *ShardHeader) SetTimestamp(t time.Time) { h.bCHeader.btcAux.Timestamp = t }

func (h *ShardHeader) Version() BVersion { return h.bCHeader.version }

func (h *ShardHeader) Nonce() uint32     { return h.bCHeader.btcAux.Nonce }
func (h *ShardHeader) SetNonce(n uint32) { h.bCHeader.SetNonce(n) }

func (h *ShardHeader) K() uint32         { return h.bCHeader.k }
func (h *ShardHeader) SetK(value uint32) { h.bCHeader.k = value }

func (h *ShardHeader) VoteK() uint32         { return h.bCHeader.voteK }
func (h *ShardHeader) SetVoteK(value uint32) { h.bCHeader.voteK = value }

func (h *ShardHeader) MaxLength() int { return MaxShardBlockHeaderPayload }

func (h *ShardHeader) MergeMiningNumber() uint32     { return h.mergeMiningNumber }
func (h *ShardHeader) SetMergeMiningNumber(n uint32) { h.mergeMiningNumber = n }

func (h *ShardHeader) MergeMiningRoot() chainhash.Hash         { return h.bCHeader.MergeMiningRoot() }
func (h *ShardHeader) SetMergeMiningRoot(value chainhash.Hash) { h.bCHeader.SetMergeMiningRoot(value) }

// ShardExclusiveBlockHash computes the block identifier hash for the given block ShardHeader.
func (h *ShardHeader) ShardExclusiveBlockHash() chainhash.Hash {
	buf := bytes.NewBuffer(make([]byte, 0, MaxShardBlockHeaderPayload))
	_ = writeShardBlockHeaderNoBC(buf, h)

	return chainhash.DoubleHashH(buf.Bytes())
}

// BlockHash computes the block identifier hash for the BeaconChain Container for the given block.
func (h *ShardHeader) BlockHash() chainhash.Hash {
	w := bytes.NewBuffer(make([]byte, 0, MaxBeaconBlockHeaderPayload))

	sec := uint32(h.bCHeader.btcAux.Timestamp.Unix())
	_ = encoder.WriteElements(w,
		&h.blocksMMRRoot,
		&h.merkleRoot,
		&h.bits,
		h.mergeMiningNumber,
		h.bCHeader.version,
		&h.bCHeader.blocksMMRRoot,
		&h.bCHeader.merkleRoot,
		&h.bCHeader.mergeMiningRoot,
		h.bCHeader.bits,
		&h.bCHeader.shards,
		&h.bCHeader.k,
		&h.bCHeader.voteK,
		&h.bCHeader.treeEncoding,
		h.bCHeader.btcAux.Version,
		&h.bCHeader.btcAux.PrevBlock,
		&h.bCHeader.btcAux.MerkleRoot,
		sec,
		h.bCHeader.btcAux.Bits,
		h.bCHeader.btcAux.Nonce,
	)
	return chainhash.DoubleHashH(w.Bytes())
}

// PoWHash computes the hash for block that will be used to check ProofOfWork.
func (h *ShardHeader) PoWHash() chainhash.Hash {
	return h.bCHeader.btcAux.BlockHash()
}

// UpdateCoinbaseScript sets new coinbase script, rebuilds BTCBlockAux.TxMerkle
// and recalculates the BTCBlockAux.MerkleRoot with the updated extra nonce.
func (h *ShardHeader) UpdateCoinbaseScript(coinbaseScript []byte) {
	h.bCHeader.UpdateCoinbaseScript(coinbaseScript)
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding block headers stored to disk, such as in a
// database, as opposed to decoding block headers from the wire.
func (h *ShardHeader) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
	return readShardBlockHeader(r, h)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding block headers to be stored to disk, such as in a
// database, as opposed to encoding block headers for the wire.
func (h *ShardHeader) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
	return WriteShardBlockHeader(w, h)
}

// Deserialize decodes a block ShardHeader from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *ShardHeader) Read(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of readBeaconBlockHeader.
	return readShardBlockHeader(r, h)
}

// Serialize encodes a block ShardHeader from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *ShardHeader) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of WriteBlockHeader.
	return WriteShardBlockHeader(w, h)
}

// Write encodes a block ShardHeader from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *ShardHeader) Write(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of WriteBlockHeader.
	return WriteShardBlockHeader(w, h)
}

// readBeaconBlockHeader reads a bitcoin block ShardHeader from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func readShardBlockHeader(r io.Reader, bh *ShardHeader) error {
	err := encoder.ReadElements(r,
		&bh.blocksMMRRoot,
		&bh.merkleRoot,
		&bh.bits,
		&bh.mergeMiningNumber,
	)
	if err != nil {
		return err
	}
	if err = bh.bCHeader.Read(r); err != nil {
		return err
	}

	return bh.CoinbaseAux.Deserialize(r)
}

// WriteShardBlockHeader writes a bitcoin block ShardHeader to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func WriteShardBlockHeader(w io.Writer, bh *ShardHeader) error {
	err := encoder.WriteElements(w,
		&bh.blocksMMRRoot,
		&bh.merkleRoot,
		&bh.bits,
		bh.mergeMiningNumber,
	)
	if err != nil {
		return err
	}

	err = bh.bCHeader.Write(w)
	if err != nil {
		return err
	}

	return bh.CoinbaseAux.Serialize(w)
}

// WriteShardBlockHeader writes a bitcoin block ShardHeader to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func writeShardBlockHeaderNoBC(w io.Writer, bh *ShardHeader) error {
	return encoder.WriteElements(w,
		&bh.blocksMMRRoot,
		&bh.merkleRoot,
		&bh.bits,
		bh.mergeMiningNumber,
	)
}
