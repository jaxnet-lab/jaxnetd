// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"encoding/binary"
	"io"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/encoder"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

const (
	// MaxBeaconBlockHeaderPayload is the maximum number of bytes a block BeaconHeader can be.
	// Version 4 bytes + Timestamp 4 bytes + Bits 4 bytes + Nonce 4 bytes + Shards 4 bytes +
	// PrevBlock, MerkleRoot, MergeMiningRoot hashes + tree X bytes. //todo: add treeEncoding len
	MaxBeaconBlockHeaderPayload = 1024
)

// BeaconHeader defines information about a block and is used in the bitcoin
// block (MsgBlock) and headers (MsgHeaders) messages.
type BeaconHeader struct {
	// Version of the block.  This is not the same as the protocol version.
	version BVersion

	// blocksMerkleMountainRoot is an actual root of the MMR tree for current block
	blocksMerkleMountainRoot chainhash.Hash

	// Merkle tree reference to hash of all transactions for the block.
	merkleRoot chainhash.Hash

	// Difficulty target for the block.
	bits uint32

	// Root of Merge-mining (orange) tree
	mergeMiningRoot chainhash.Hash

	// Encoding of the Merge-mining tree
	treeEncoding []uint8

	// shards uint32
	shards uint32

	// k is inflation-fix coefficient for current mining epoch.
	k uint32

	// voteK is a proposed inflation-fix coefficient for next mining epoch.
	voteK uint32

	// btcAux is a container with the bitcoin auxiliary header, required for merge mining.
	btcAux BTCBlockAux // todo (mike): do not store all hashes, only merkle path

}

func EmptyBeaconHeader() *BeaconHeader { return &BeaconHeader{} }

// NewBeaconBlockHeader returns a new BlockHeader using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewBeaconBlockHeader(version BVersion, blocksMerkleMountainRoot, merkleRootHash chainhash.Hash,
	mergeMiningRoot chainhash.Hash, timestamp time.Time, bits uint32, nonce uint32) *BeaconHeader {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &BeaconHeader{
		version:                  version,
		blocksMerkleMountainRoot: blocksMerkleMountainRoot,
		merkleRoot:               merkleRootHash,
		mergeMiningRoot:          mergeMiningRoot,
		bits:                     bits,
		btcAux: BTCBlockAux{
			Version:     0,
			PrevBlock:   chainhash.Hash{},
			MerkleRoot:  chainhash.Hash{},
			Timestamp:   timestamp,
			Bits:        bits,
			Nonce:       nonce,
			CoinbaseAux: CoinbaseAux{}.New(),
		},
	}

}

func (h *BeaconHeader) BeaconHeader() *BeaconHeader      { return h }
func (h *BeaconHeader) SetBeaconHeader(bh *BeaconHeader) { *h = *bh }

func (h *BeaconHeader) Bits() uint32        { return h.bits }
func (h *BeaconHeader) SetBits(bits uint32) { h.bits = bits }

func (h *BeaconHeader) Nonce() uint32     { return h.btcAux.Nonce }
func (h *BeaconHeader) SetNonce(n uint32) { h.btcAux.Nonce = n }

func (h *BeaconHeader) BlocksMerkleMountainRoot() chainhash.Hash { return h.blocksMerkleMountainRoot }
func (h *BeaconHeader) SetBlocksMerkleMountainRoot(root chainhash.Hash) {
	h.blocksMerkleMountainRoot = root
}

func (h *BeaconHeader) MerkleRoot() chainhash.Hash        { return h.merkleRoot }
func (h *BeaconHeader) SetMerkleRoot(hash chainhash.Hash) { h.merkleRoot = hash }

func (h *BeaconHeader) Timestamp() time.Time     { return h.btcAux.Timestamp }
func (h *BeaconHeader) SetTimestamp(t time.Time) { h.btcAux.Timestamp = t }

func (h *BeaconHeader) Version() BVersion     { return h.version }
func (h *BeaconHeader) SetVersion(v BVersion) { h.version = v }

func (h *BeaconHeader) MergeMiningRoot() chainhash.Hash         { return h.mergeMiningRoot }
func (h *BeaconHeader) SetMergeMiningRoot(value chainhash.Hash) { h.mergeMiningRoot = value }

func (h *BeaconHeader) Shards() uint32         { return h.shards }
func (h *BeaconHeader) SetShards(value uint32) { h.shards = value }

func (h *BeaconHeader) K() uint32         { return h.k }
func (h *BeaconHeader) SetK(value uint32) { h.k = value }

func (h *BeaconHeader) VoteK() uint32         { return h.voteK }
func (h *BeaconHeader) SetVoteK(value uint32) { h.voteK = value }

func (h *BeaconHeader) BTCAux() *BTCBlockAux         { return &h.btcAux }
func (h *BeaconHeader) SetBTCAux(header BTCBlockAux) { h.btcAux = header }

// UpdateCoinbaseScript sets new coinbase script, rebuilds BTCBlockAux.TxMerkle
// and recalculates the BTCBlockAux.MerkleRoot with the updated extra nonce.
func (h *BeaconHeader) UpdateCoinbaseScript(coinbaseScript []byte) {
	h.btcAux.UpdateCoinbaseScript(coinbaseScript)
}

func (h *BeaconHeader) MergedMiningTree() []byte            { return h.treeEncoding }
func (h *BeaconHeader) SetMergedMiningTree(treeData []byte) { h.treeEncoding = treeData }

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

// BlockHash computes the block identifier hash for the given block BeaconHeader.
func (h *BeaconHeader) BlockHash() chainhash.Hash {
	buf := bytes.NewBuffer(make([]byte, 0, MaxBeaconBlockHeaderPayload))
	_ = writeBeaconBlockHeader(buf, h)

	return chainhash.DoubleHashH(buf.Bytes())
}

// BeaconExclusiveHash computes the hash of the BeaconHeader without BtcAux.
// This hash needs to be set into Bitcoin coinbase tx, as proof of merge mining.
func (h *BeaconHeader) BeaconExclusiveHash() chainhash.Hash {
	buf := bytes.NewBuffer(make([]byte, 0, MaxBeaconBlockHeaderPayload))
	encoder.WriteElements(buf,
		h.version,
		&h.blocksMerkleMountainRoot,
		&h.merkleRoot,
		&h.mergeMiningRoot,
		h.bits,
		&h.shards,
		&h.k,
		&h.voteK,
		&h.treeEncoding,
	)

	return chainhash.DoubleHashH(buf.Bytes())
}

// PoWHash computes the hash for block that will be used to check ProofOfWork.
func (h *BeaconHeader) PoWHash() chainhash.Hash {
	return h.btcAux.BlockHash()
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding block headers stored to disk, such as in a
// database, as opposed to decoding block headers from the wire.
func (h *BeaconHeader) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
	return readBeaconBlockHeader(r, h)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding block headers to be stored to disk, such as in a
// database, as opposed to encoding block headers for the wire.
func (h *BeaconHeader) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
	return writeBeaconBlockHeader(w, h)
}

// Deserialize decodes a block BeaconHeader from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BeaconHeader) Read(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of readBeaconBlockHeader.
	return readBeaconBlockHeader(r, h)
}

// Serialize encodes a block BeaconHeader from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BeaconHeader) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of WriteBlockHeader.
	return writeBeaconBlockHeader(w, h)
}

// Write encodes a block BeaconHeader from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BeaconHeader) Write(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of WriteBlockHeader.
	return writeBeaconBlockHeader(w, h)
}

// Copy creates a deep copy of a BlockHeader so that the original does not get
// modified when the copy is manipulated.
func (h *BeaconHeader) Copy() BlockHeader {
	clone := *h

	// all fields except this are passed by value
	// so we manually copy the following fields to prevent side effects
	clone.treeEncoding = make([]byte, len(h.treeEncoding))
	copy(clone.treeEncoding, h.treeEncoding)

	clone.btcAux = *h.btcAux.Copy()
	return &clone
}

// readBeaconBlockHeader reads a bitcoin block BeaconHeader from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func readBeaconBlockHeader(r io.Reader, bh *BeaconHeader) error {
	err := encoder.ReadElements(r, &bh.version,
		&bh.blocksMerkleMountainRoot,
		&bh.merkleRoot,
		&bh.mergeMiningRoot,
		&bh.bits,
		&bh.shards,
		&bh.k,
		&bh.voteK,
		&bh.treeEncoding)
	if err != nil {
		return err
	}

	return readBTCBlockHeader(r, &bh.btcAux)
}

// writeBeaconBlockHeader writes a bitcoin block BeaconHeader to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func writeBeaconBlockHeader(w io.Writer, bh *BeaconHeader) error {
	err := encoder.WriteElements(w,
		bh.version,
		&bh.blocksMerkleMountainRoot,
		&bh.merkleRoot,
		&bh.mergeMiningRoot,
		bh.bits,
		&bh.shards,
		&bh.k,
		&bh.voteK,
		&bh.treeEncoding)
	if err != nil {
		return err
	}
	return writeBTCBlockHeader(w, &bh.btcAux)
}
