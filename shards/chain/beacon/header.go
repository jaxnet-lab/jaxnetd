package beacon

import (
	"bytes"
	"io"
	"time"

	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/encoder"
)

// blockHeaderLen is a constant that represents the number of bytes for a block
// header.
const blockHeaderLen = 80

// BlockHeader defines information about a block and is used in the bitcoin
// block (MsgBlock) and headers (MsgHeaders) messages.
type header struct {
	// Version of the block.  This is not the same as the protocol version.
	version chain.BVersion

	// Hash of the previous block header in the block chain.
	prevBlock chainhash.Hash

	// Merkle tree reference to hash of all transactions for the block.
	merkleRoot chainhash.Hash

	mergeMiningRoot chainhash.Hash

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	timestamp time.Time

	// Difficulty target for the block.
	bits uint32

	// Nonce used to generate the block.
	nonce uint32

	treeEncoding []uint8

	shards uint32
}

// NewBlockHeader returns a new BlockHeader using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewBlockHeader(version chain.BVersion, prevHash, merkleRootHash chainhash.Hash,
	mergeMiningRoot chainhash.Hash, timestamp time.Time, bits uint32, nonce uint32) *header {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &header{
		version:         version,
		prevBlock:       prevHash,
		merkleRoot:      merkleRootHash,
		mergeMiningRoot: mergeMiningRoot,
		timestamp:       timestamp,
		bits:            bits,
		nonce:           nonce,
	}
}

func (h *header) Bits() uint32                    { return h.bits }
func (h *header) Nonce() uint32                   { return h.nonce }
func (h *header) MergeMiningRoot() chainhash.Hash { return h.mergeMiningRoot }
func (h *header) MerkleRoot() chainhash.Hash      { return h.merkleRoot }
func (h *header) PrevBlock() chainhash.Hash       { return h.prevBlock }
func (h *header) Timestamp() time.Time            { return h.timestamp }
func (h *header) Version() chain.BVersion         { return h.version }

func (h *header) SetBits(bits uint32)                     { h.bits = bits }
func (h *header) SetMergeMiningRoot(value chainhash.Hash) { h.mergeMiningRoot = value }
func (h *header) SetMerkleRoot(hash chainhash.Hash)       { h.merkleRoot = hash }
func (h *header) SetNonce(n uint32)                       { h.nonce = n }
func (h *header) SetPrevBlock(prevBlock chainhash.Hash)   { h.prevBlock = prevBlock }
func (h *header) SetTimestamp(t time.Time)                { h.timestamp = t }

func (h *header) BlockData() []byte {
	buf := bytes.NewBuffer(make([]byte, 0, maxBlockHeaderPayload))
	_ = writeBlockHeader(buf, h)
	return buf.Bytes()
}

// BlockHash computes the block identifier hash for the given block header.
func (h *header) BlockHash() chainhash.Hash {
	buf := bytes.NewBuffer(make([]byte, 0, maxBlockHeaderPayload))
	_ = writeBlockHeader(buf, h)

	return chainhash.DoubleHashH(buf.Bytes())
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding block headers stored to disk, such as in a
// database, as opposed to decoding block headers from the wire.
func (h *header) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
	return readBlockHeader(r, h)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding block headers to be stored to disk, such as in a
// database, as opposed to encoding block headers for the wire.
func (h *header) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
	return writeBlockHeader(w, h)
}

// Deserialize decodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *header) Read(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of readBlockHeader.
	return readBlockHeader(r, h)
}

// Serialize encodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *header) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of WriteBlockHeader.
	return writeBlockHeader(w, h)
}

// Write encodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *header) Write(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of WriteBlockHeader.
	return writeBlockHeader(w, h)
}

// readBlockHeader reads a bitcoin block header from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func readBlockHeader(r io.Reader, bh *header) error {
	err := encoder.ReadElements(r, &bh.version, &bh.prevBlock, &bh.merkleRoot, &bh.mergeMiningRoot,
		(*encoder.Uint32Time)(&bh.timestamp), &bh.bits, &bh.nonce, &bh.treeEncoding, &bh.shards)
	return err
}

// WriteBlockHeader writes a bitcoin block header to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func writeBlockHeader(w io.Writer, h chain.BlockHeader) error {
	bh := h.(*header)
	sec := uint32(bh.timestamp.Unix())
	return encoder.WriteElements(w, bh.version, &bh.prevBlock, &bh.merkleRoot, &bh.mergeMiningRoot,
		sec, bh.bits, bh.nonce, &bh.treeEncoding, &bh.shards)
}
