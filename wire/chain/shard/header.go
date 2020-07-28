package shard

import (
	"bytes"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/wire/encoder"
	"io"
	"time"
)

// MaxBlockHeaderPayload is the maximum number of bytes a block Header can be.
// Version 4 bytes + Timestamp 4 bytes + Bits 4 bytes + Nonce 4 bytes +
// PrevBlock and MerkleRoot hashes.
const MaxBlockHeaderPayload = 16 + (chainhash.HashSize * 2)

//

// BlockHeader defines information about a block and is used in the bitcoin
// block (MsgBlock) and headers (MsgHeaders) messages.
type Header struct {
	// Version of the block.  This is not the same as the protocol version.
	version int32

	// Hash of the previous block Header in the block chain.
	prevBlock chainhash.Hash

	// Merkle tree reference to hash of all transactions for the block.
	merkleRoot chainhash.Hash

	merkleMountainRange chainhash.Hash

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	timestamp time.Time

	// Difficulty target for the block.
	bits uint32

	// Nonce used to generate the block.
	nonce uint32
}

// blockHeaderLen is a constant that represents the number of bytes for a block
// Header.
const blockHeaderLen = 80

func (h *Header) PrevBlock() chainhash.Hash {
	return h.prevBlock
}

func (h *Header) BlockData() []byte {
	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockHeaderPayload))
	_ = WriteBlockHeader(buf, h)
	return buf.Bytes()
}

// BlockHash computes the block identifier hash for the given block Header.
func (h *Header) BlockHash() chainhash.Hash {
	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockHeaderPayload))
	_ = WriteBlockHeader(buf, h)

	return chainhash.DoubleHashH(buf.Bytes())
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding block headers stored to disk, such as in a
// database, as opposed to decoding block headers from the wire.
func (h *Header) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
	return ReadBlockHeader(r, h)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding block headers to be stored to disk, such as in a
// database, as opposed to encoding block headers for the wire.
func (h *Header) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
	return WriteBlockHeader(w, h)
}

// Deserialize decodes a block Header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *Header) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of ReadBlockHeader.
	return ReadBlockHeader(r, h)
}

// Serialize encodes a block Header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *Header) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of WriteBlockHeader.
	return WriteBlockHeader(w, h)
}

// NewBlockHeader returns a new BlockHeader using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewBlockHeader(version int32, prevHash, merkleRootHash chainhash.Hash,
	mmr chainhash.Hash,
	timestamp time.Time,
	bits uint32, nonce uint32) *Header {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &Header{
		version:             version,
		prevBlock:           prevHash,
		merkleRoot:          merkleRootHash,
		merkleMountainRange: mmr,
		timestamp:           timestamp, //time.Unix(time.Now().Unix(), 0),
		bits:                bits,
		nonce:               nonce,
	}
}

// ReadBlockHeader reads a bitcoin block Header from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func ReadBlockHeader(r io.Reader, bh *Header) error {
	return encoder.ReadElements(r, &bh.version, &bh.prevBlock, &bh.merkleRoot,
		(*encoder.Uint32Time)(&bh.timestamp), &bh.bits, &bh.nonce)
}

// WriteBlockHeader writes a bitcoin block Header to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func WriteBlockHeader(w io.Writer, bh *Header) error {
	sec := uint32(bh.timestamp.Unix())
	return encoder.WriteElements(w, bh.version, &bh.prevBlock, &bh.merkleRoot,
		sec, bh.bits, bh.nonce)
}

//// BlockHeader defines information about a block and is used in the bitcoin
//// block (MsgBlock) and headers (MsgHeaders) messages.
//type Header struct {
//	// Version of the block.  This is not the same as the protocol version.
//	version int32
//
//	// Hash of the previous block Header in the block chain.
//	prevBlock chainhash.Hash
//
//	// Merkle tree reference to hash of all transactions for the block.
//	merkleRoot chainhash.Hash
//
//	merkleMountainRange chainhash.Hash
//
//	// Time the block was created.  This is, unfortunately, encoded as a
//	// uint32 on the wire and therefore is limited to 2106.
//	timestamp time.Time
//
//	// Difficulty target for the block.
//	bits uint32
//
//	// Nonce used to generate the block.
//	nonce uint32
//}
//
//func NewHeader(version int32, prev chainhash.Hash, merkeleRoot chainhash.Hash, ts time.Time, bits uint32, nonce uint32) chain.BlockHeader {
//	return &Header{
//		version:    version,
//		prevBlock:  prev,
//		merkleRoot: merkeleRoot,
//		timestamp:  ts,
//		bits:       bits,
//		nonce:      nonce,
//	}
//}
//
//// blockHeaderLen is a constant that represents the number of bytes for a block
//// Header.
//const blockHeaderLen = 80
//
//func (h *Header) Size() int {
//	return blockHeaderLen
//}
//
//func (h *Header) BlockData() []byte {
//	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockHeaderPayload))
//	_ = WriteBlockHeader(buf, h)
//	return buf.Bytes()
//}
//
//// BlockHash computes the block identifier hash for the given block Header.
//func (h *Header) BlockHash() chainhash.Hash {
//	// Encode the Header and double sha256 everything prior to the number of
//	// transactions.  Ignore the error returns since there is no way the
//	// encode could fail except being out of memory which would cause a
//	// run-time panic.
//	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockHeaderPayload))
//	_ = WriteBlockHeader(buf, h)
//
//
//
//	//fmt.Printf("block %+v \n%x\n", *h, buf.Bytes())
//	return chainhash.DoubleHashH(buf.Bytes())
//}
//
//// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
//// This is part of the Message interface implementation.
//// See Deserialize for decoding block headers stored to disk, such as in a
//// database, as opposed to decoding block headers from the wire.
////func (h *Header) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
////	return ReadBlockHeader(r, h)
////}
//
//// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
//// This is part of the Message interface implementation.
//// See Serialize for encoding block headers to be stored to disk, such as in a
//// database, as opposed to encoding block headers for the wire.
//func (h *Header) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
//	return WriteBlockHeader(w, h)
//}
//
//func (h *Header) PrevBlock() *chainhash.Hash {
//	return &h.prevBlock
//}
//
func (h *Header) Timestamp() time.Time {
	return h.timestamp
}

//
func (h *Header) MerkleRoot() chainhash.Hash {
	return h.merkleRoot
}

func (h *Header) SetMerkleRoot(hash chainhash.Hash) {
	h.merkleRoot = hash
}

func (h *Header) MerkleMountainRange() chainhash.Hash {
	return h.merkleMountainRange
}

//
func (h *Header) SetTimestamp(t time.Time) {
	h.timestamp = t
}

func (h *Header) Nonce() uint32 {
	return h.nonce
}

func (h *Header) SetNonce(n uint32) {
	h.nonce = n
}

func (h *Header) Bits() uint32 {
	return h.bits
}

func (h *Header) SetBits(bits uint32) {
	h.bits = bits
}

func (h *Header) Version() int32 {
	return h.version
}

//
//// Deserialize decodes a block Header from r into the receiver using a format
//// that is suitable for long-term storage such as a database while respecting
//// the Version field.
//func (h *Header) Deserialize(r io.Reader) error {
//	// At the current time, there is no difference between the wire encoding
//	// at protocol version 0 and the stable long-term storage format.  As
//	// a result, make use of ReadBlockHeader.
//	return ReadBlockHeader(r, h)
//}
//
//// Serialize encodes a block Header from r into the receiver using a format
//// that is suitable for long-term storage such as a database while respecting
//// the Version field.
//func (h *Header) Serialize(w io.Writer) error {
//	// At the current time, there is no difference between the wire encoding
//	// at protocol version 0 and the stable long-term storage format.  As
//	// a result, make use of WriteBlockHeader.
//	return WriteBlockHeader(w, h)
//}
//
//func (h *Header) Read(r io.Reader) error {
//	return ReadBlockHeader(r, h)
//}
//
//func (h *Header) Write(w io.Writer) error {
//	return WriteBlockHeader(w, h)
//}
//
//// NewBlockHeader returns a new BlockHeader using the provided version, previous
//// block hash, merkle root hash, difficulty bits, and nonce used to generate the
//// block with defaults for the remaining fields.
//func NewBlockHeader(version int32, prevHash, merkleRootHash *chainhash.Hash,
//	mmr *chainhash.Hash,
//	bits uint32, nonce uint32) *Header {
//
//	// Limit the timestamp to one second precision since the protocol
//	// doesn't support better.
//	return &Header{
//		version:             version,
//		prevBlock:           *prevHash,
//		merkleRoot:          *merkleRootHash,
//		merkleMountainRange: *mmr,
//		timestamp:           time.Unix(time.Now().Unix(), 0),
//		bits:                bits,
//		nonce:               nonce,
//	}
//}
//
//// ReadBlockHeader reads a bitcoin block Header from r.  See Deserialize for
//// decoding block headers stored to disk, such as in a database, as opposed to
//// decoding from the wire.
//func ReadBlockHeader(r io.Reader, bh *Header) error {
//	//TODO: use encoder from chain
//	return encoder.ReadElements(r, &bh.version, &bh.prevBlock, &bh.merkleRoot,
//		(*encoder.Uint32Time)(&bh.timestamp), &bh.bits, &bh.nonce)
//}
//
//// WriteBlockHeader writes a bitcoin block Header to w.  See Serialize for
//// encoding block headers to be stored to disk, such as in a database, as
//// opposed to encoding for the wire.
//func WriteBlockHeader(w io.Writer, bh *Header) error {
//	sec := uint32(bh.timestamp.Unix())
//	return encoder.WriteElements(w, bh.version, &bh.prevBlock, &bh.merkleRoot,
//		sec, bh.bits, bh.nonce)
//}
