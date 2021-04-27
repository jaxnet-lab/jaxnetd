// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package wire

import (
	"bytes"
	"encoding/binary"
	"io"
	"time"

	"gitlab.com/jaxnet/core/shard.core/node/encoder"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
)

const (
	// MaxBeaconBlockHeaderPayload is the maximum number of bytes a block BeaconHeader can be.
	// Version 4 bytes + Timestamp 4 bytes + Bits 4 bytes + Nonce 4 bytes + Shards 4 bytes +
	// PrevBlock, MerkleRoot, MergeMiningRoot hashes + tree X bytes. //todo: add treeEncoding len
	MaxBeaconBlockHeaderPayload = (4 * 5) + (chainhash.HashSize * 3)

	// beaconBlockHeaderLen is a constant that represents the number of bytes for a block
	// BeaconHeader.
	beaconBlockHeaderLen = 80
)

// BeaconHeader defines information about a block and is used in the bitcoin
// block (MsgBlock) and headers (MsgHeaders) messages.
type BeaconHeader struct {
	// Version of the block.  This is not the same as the protocol version.
	version BVersion

	// Hash of the previous block BeaconHeader in the block chain.
	prevBlock chainhash.Hash

	// Merkle tree reference to hash of all transactions for the block.
	merkleRoot chainhash.Hash

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	timestamp time.Time

	// Difficulty target for the block.
	bits uint32

	// Nonce used to generate the block.
	nonce uint32

	// Root of Merge-mining tree
	mergeMiningRoot chainhash.Hash

	// Encoding of the Merge-mining tree
	treeEncoding []uint8

	// shards uint32
	shards uint32
}

func EmptyBeaconHeader() *BeaconHeader { return &BeaconHeader{} }

// NewBeaconBlockHeader returns a new BlockHeader using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewBeaconBlockHeader(version BVersion, prevHash, merkleRootHash chainhash.Hash,
	mergeMiningRoot chainhash.Hash, timestamp time.Time, bits uint32, nonce uint32) *BeaconHeader {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &BeaconHeader{
		version:         version,
		prevBlock:       prevHash,
		merkleRoot:      merkleRootHash,
		mergeMiningRoot: mergeMiningRoot,
		timestamp:       timestamp,
		bits:            bits,
		nonce:           nonce,
	}
}

func (h *BeaconHeader) BeaconHeader() *BeaconHeader      { return h }
func (h *BeaconHeader) SetBeaconHeader(bh *BeaconHeader) { *h = *bh }

func (h *BeaconHeader) Bits() uint32  { return h.bits }
func (h *BeaconHeader) Nonce() uint32 { return h.nonce }

func (h *BeaconHeader) MerkleRoot() chainhash.Hash { return h.merkleRoot }
func (h *BeaconHeader) PrevBlock() chainhash.Hash  { return h.prevBlock }
func (h *BeaconHeader) Timestamp() time.Time       { return h.timestamp }
func (h *BeaconHeader) Version() BVersion          { return h.version }
func (h *BeaconHeader) SetVersion(v BVersion)      { h.version = v }

func (h *BeaconHeader) SetBits(bits uint32)                   { h.bits = bits }
func (h *BeaconHeader) SetMerkleRoot(hash chainhash.Hash)     { h.merkleRoot = hash }
func (h *BeaconHeader) SetNonce(n uint32)                     { h.nonce = n }
func (h *BeaconHeader) SetPrevBlock(prevBlock chainhash.Hash) { h.prevBlock = prevBlock }
func (h *BeaconHeader) SetTimestamp(t time.Time)              { h.timestamp = t }

func (h *BeaconHeader) MergeMiningRoot() chainhash.Hash         { return h.mergeMiningRoot }
func (h *BeaconHeader) SetMergeMiningRoot(value chainhash.Hash) { h.mergeMiningRoot = value }

func (h *BeaconHeader) Shards() uint32         { return h.shards }
func (h *BeaconHeader) SetShards(value uint32) { h.shards = value }

func (h *BeaconHeader) MergedMiningTree() []byte {
	return h.treeEncoding
}

func (h *BeaconHeader) SetMergedMiningTree(treeData []byte) {
	h.treeEncoding = treeData
}

func (h *BeaconHeader) MergedMiningTreeCodingProof() (hashes, coding []byte, codingLengthBits uint32) {
	buf := h.treeEncoding[:]
	hashesSize, size := binary.Uvarint(buf)
	buf = buf[size:]

	hashes, buf = buf[:hashesSize], buf[hashesSize:]

	codingSize, size := binary.Uvarint(buf)
	buf = buf[size:]

	coding, buf = buf[:codingSize], buf[codingSize:]

	bitsSize, _ := binary.Uvarint(buf)
	codingLengthBits = uint32(bitsSize)
	return
}

func (h *BeaconHeader) SetMergedMiningTreeCodingProof(hashes, coding []byte, codingLengthBits uint32) {
	buf := make([]byte, 12+len(hashes)+len(coding))

	shift := binary.PutUvarint(buf, uint64(len(hashes)))
	copy(buf[shift:], hashes)
	shift += len(hashes)

	shift += binary.PutUvarint(buf[shift:], uint64(len(coding)))

	copy(buf[shift:], coding)
	shift += len(coding)

	shift += binary.PutUvarint(buf[shift:], uint64(codingLengthBits))
	h.treeEncoding = buf[:shift]
}

func (h *BeaconHeader) MaxLength() int { return MaxBeaconBlockHeaderPayload }

func (h *BeaconHeader) BlockData() []byte {
	buf := bytes.NewBuffer(make([]byte, 0, MaxBeaconBlockHeaderPayload))
	_ = WriteBeaconBlockHeader(buf, h)
	return buf.Bytes()
}

// BlockHash computes the block identifier hash for the given block BeaconHeader.
func (h *BeaconHeader) BlockHash() chainhash.Hash {
	buf := bytes.NewBuffer(make([]byte, 0, MaxBeaconBlockHeaderPayload))
	_ = WriteBeaconBlockHeader(buf, h)

	return chainhash.DoubleHashH(buf.Bytes())
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding block headers stored to disk, such as in a
// database, as opposed to decoding block headers from the wire.
func (h *BeaconHeader) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
	return ReadBeaconBlockHeader(r, h)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding block headers to be stored to disk, such as in a
// database, as opposed to encoding block headers for the wire.
func (h *BeaconHeader) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
	return WriteBeaconBlockHeader(w, h)
}

// Deserialize decodes a block BeaconHeader from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BeaconHeader) Read(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of ReadBeaconBlockHeader.
	return ReadBeaconBlockHeader(r, h)
}

// Serialize encodes a block BeaconHeader from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BeaconHeader) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of WriteBlockHeader.
	return WriteBeaconBlockHeader(w, h)
}

// Write encodes a block BeaconHeader from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BeaconHeader) Write(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of WriteBlockHeader.
	return WriteBeaconBlockHeader(w, h)
}

// Copy creates a deep copy of a BlockHeader so that the original does not get
// modified when the copy is manipulated.
func (h *BeaconHeader) Copy() BlockHeader {
	clone := *h

	// all fields except this are passed by value
	// so we manually copy the following fields to prevent side effects
	clone.treeEncoding = make([]byte, len(h.treeEncoding))
	copy(clone.treeEncoding, h.treeEncoding)
	return &clone
}

// ReadBeaconBlockHeader reads a bitcoin block BeaconHeader from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func ReadBeaconBlockHeader(r io.Reader, bh *BeaconHeader) error {
	err := encoder.ReadElements(r, &bh.version, &bh.prevBlock, &bh.merkleRoot, &bh.mergeMiningRoot,
		(*encoder.Uint32Time)(&bh.timestamp), &bh.bits, &bh.nonce, &bh.shards, &bh.treeEncoding)
	return err
}

// WriteBeaconBlockHeader writes a bitcoin block BeaconHeader to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func WriteBeaconBlockHeader(w io.Writer, bh *BeaconHeader) error {
	sec := uint32(bh.timestamp.Unix())
	return encoder.WriteElements(w, bh.version, &bh.prevBlock, &bh.merkleRoot, &bh.mergeMiningRoot,
		sec, bh.bits, bh.nonce, &bh.shards, &bh.treeEncoding)
}
