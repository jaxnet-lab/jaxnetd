package wire

import (
	"bytes"
	"io"
	"time"

	"gitlab.com/jaxnet/core/shard.core/node/encoder"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
)

const (
	// MaxShardBlockHeaderPayload is the maximum number of bytes a block ShardHeader can be.
	// Version 4 bytes + Timestamp 4 bytes + Bits 4 bytes + Nonce 4 bytes +
	// PrevBlock and MerkleRoot hashes.
	MaxShardBlockHeaderPayload = 16 + (chainhash.HashSize * 2)

	// ShardBlockHeaderLen is a constant that represents the number of bytes for a block
	// ShardHeader.
	ShardBlockHeaderLen = 80
)

// BlockHeader defines information about a block and is used in the bitcoin
// block (MsgBlock) and headers (MsgHeaders) messages.
type ShardHeader struct {
	// Hash of the previous block ShardHeader in the block chain.
	prevBlock chainhash.Hash

	// Merkle tree reference to hash of all transactions for the block.
	merkleRoot chainhash.Hash

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	timestamp time.Time

	// Difficulty target for the block.
	bits uint32

	// Merge-mining number is the miner’s claim about how
	// many shards he was mining
	MergeMiningNumber uint32

	BCHeader BeaconHeader
}

func EmptyShardHeader() *ShardHeader { return &ShardHeader{BCHeader: *EmptyBeaconHeader()} }

// NewShardBlockHeader returns a new BlockHeader using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewShardBlockHeader(prevHash, merkleRootHash chainhash.Hash, timestamp time.Time, bits uint32,
	bcHeader BeaconHeader) *ShardHeader {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &ShardHeader{
		prevBlock:  prevHash,
		merkleRoot: merkleRootHash,
		timestamp:  timestamp,
		bits:       bits,
		BCHeader:   bcHeader,
	}
}

func (h *ShardHeader) BeaconHeader() *BeaconHeader      { return &h.BCHeader }
func (h *ShardHeader) SetBeaconHeader(bh *BeaconHeader) { h.BCHeader = *bh }

func (h *ShardHeader) Bits() uint32        { return h.bits }
func (h *ShardHeader) SetBits(bits uint32) { h.bits = bits }

func (h *ShardHeader) MerkleRoot() chainhash.Hash        { return h.merkleRoot }
func (h *ShardHeader) SetMerkleRoot(hash chainhash.Hash) { h.merkleRoot = hash }

func (h *ShardHeader) PrevBlock() chainhash.Hash             { return h.prevBlock }
func (h *ShardHeader) SetPrevBlock(prevBlock chainhash.Hash) { h.prevBlock = prevBlock }

func (h *ShardHeader) Timestamp() time.Time     { return h.timestamp }
func (h *ShardHeader) SetTimestamp(t time.Time) { h.timestamp = t }

func (h *ShardHeader) Version() BVersion { return h.BCHeader.version }
func (h *ShardHeader) Nonce() uint32     { return h.BCHeader.nonce }
func (h *ShardHeader) SetNonce(n uint32) { h.BCHeader.SetNonce(n) }

func (h *ShardHeader) MergeMiningRoot() chainhash.Hash         { return h.BCHeader.MergeMiningRoot() }
func (h *ShardHeader) SetMergeMiningRoot(value chainhash.Hash) { h.BCHeader.SetMergeMiningRoot(value) }

func (h *ShardHeader) BlockData() []byte {
	buf := bytes.NewBuffer(make([]byte, 0, MaxShardBlockHeaderPayload))
	_ = WriteBeaconBlockHeader(buf, h)
	return buf.Bytes()
}

// ShardBlockHash computes the block identifier hash for the given block ShardHeader.
func (h *ShardHeader) ShardBlockHash() chainhash.Hash {
	buf := bytes.NewBuffer(make([]byte, 0, MaxShardBlockHeaderPayload))
	_ = WriteShardBlockHeader(buf, h)

	return chainhash.DoubleHashH(buf.Bytes())
}

// BlockHash computes the block identifier hash for the BeaconChain Container for the given block.
func (h *ShardHeader) BlockHash() chainhash.Hash {
	return h.BCHeader.BlockHash()
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding block headers stored to disk, such as in a
// database, as opposed to decoding block headers from the wire.
func (h *ShardHeader) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
	return ReadShardBlockHeader(r, h)
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
	// a result, make use of ReadBeaconBlockHeader.
	return ReadShardBlockHeader(r, h)
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

// ReadBeaconBlockHeader reads a bitcoin block ShardHeader from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func ReadShardBlockHeader(r io.Reader, bh *ShardHeader) error {
	return encoder.ReadElements(r,
		&bh.prevBlock,
		&bh.merkleRoot,
		(*encoder.Uint32Time)(&bh.timestamp),
		&bh.MergeMiningNumber,
	)
}

// WriteShardBlockHeader writes a bitcoin block ShardHeader to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func WriteShardBlockHeader(w io.Writer, h BlockHeader) error {
	bh := h.(*ShardHeader)
	sec := uint32(bh.timestamp.Unix())
	return encoder.WriteElements(w,
		&bh.prevBlock,
		&bh.merkleRoot,
		sec,
		bh.MergeMiningNumber,
	)
}
