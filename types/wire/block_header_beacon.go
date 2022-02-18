// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

const (
	// MaxBeaconBlockHeaderPayload is the maximum number of bytes a block BeaconHeader can be.
	// Version 4 bytes + Timestamp 4 bytes + Bits 4 bytes + Nonce 4 bytes + Shards 4 bytes +
	// PrevBlock, MerkleRoot, MergeMiningRoot hashes + tree X bytes. //todo: add treeEncoding len
	MaxBeaconBlockHeaderPayload = 1024
)

var (
	beaconMagicByte uint8 = 0x6d
	shardMagic      uint8 = 0x73
)

// BeaconHeader defines information about a block and is used in the bitcoin
// block (MsgBlock) and headers (MsgHeaders) messages.
type BeaconHeader struct {
	// Version of the block.  This is not the same as the protocol version.
	version BVersion

	// prevMMRRoot is an actual root of the MerkleMountainRange tree for current block
	prevMMRRoot chainhash.Hash

	// height the order of this block in chain
	height int32

	// Hash of the previous block BeaconHeader in the block chain.
	prevBlock chainhash.Hash

	// Merkle tree reference to hash of all transactions for the block.
	merkleRoot chainhash.Hash

	// Difficulty target for the block.
	bits uint32

	// The total chainWeight of all blocks in the chain
	chainWeight *big.Int

	// shards uint32
	shards uint32

	// k is inflation-fix coefficient for current mining epoch.
	k uint32

	// voteK is a proposed inflation-fix coefficient for next mining epoch.
	voteK uint32

	// Root of Merge-mining (orange) tree
	mergeMiningRoot chainhash.Hash

	// Merge-mining number is the miner’s claim about how
	// many shards he was mining
	mergeMiningNumber uint32

	// Encoding of the Merge-mining tree
	treeEncoding []byte

	// Merge-mining proof
	mergeMiningProof []chainhash.Hash

	treeCodingLengthBits uint32

	// btcAux is a container with the bitcoin auxiliary header, required for merge mining.
	btcAux BTCBlockAux
}

func EmptyBeaconHeader() *BeaconHeader { return &BeaconHeader{} }

// NewBeaconBlockHeader returns a new BlockHeader using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewBeaconBlockHeader(version BVersion, height int32, blocksMerkleMountainRoot, prevBlock, merkleRootHash chainhash.Hash,
	mergeMiningRoot chainhash.Hash, timestamp time.Time, bits uint32, chainWeight *big.Int, nonce uint32) *BeaconHeader {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &BeaconHeader{
		height:          height,
		version:         version,
		prevMMRRoot:     blocksMerkleMountainRoot,
		prevBlock:       prevBlock,
		merkleRoot:      merkleRootHash,
		mergeMiningRoot: mergeMiningRoot,
		bits:            bits,
		chainWeight:     chainWeight,
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

func (h *BeaconHeader) BeaconHeader() *BeaconHeader                     { return h }
func (h *BeaconHeader) SetBeaconHeader(bh *BeaconHeader, _ CoinbaseAux) { *h = *bh }

func (h *BeaconHeader) Height() int32         { return h.height }
func (h *BeaconHeader) ChainWeight() *big.Int { return h.chainWeight }

func (h *BeaconHeader) Bits() uint32        { return h.bits }
func (h *BeaconHeader) SetBits(bits uint32) { h.bits = bits }

func (h *BeaconHeader) Nonce() uint32     { return h.btcAux.Nonce }
func (h *BeaconHeader) SetNonce(n uint32) { h.btcAux.Nonce = n }

func (h *BeaconHeader) PrevBlocksMMRRoot() chainhash.Hash { return h.prevMMRRoot }
func (h *BeaconHeader) SetPrevBlocksMMRRoot(root chainhash.Hash) {
	h.prevMMRRoot = root
}

func (h *BeaconHeader) MerkleRoot() chainhash.Hash        { return h.merkleRoot }
func (h *BeaconHeader) SetMerkleRoot(hash chainhash.Hash) { h.merkleRoot = hash }

func (h *BeaconHeader) PrevBlockHash() chainhash.Hash { return h.prevBlock }

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

// UpdateCoinbaseScript sets new coinbase script, rebuilds BTCBlockAux.TxMerkleProof
// and recalculates the BTCBlockAux.MerkleRoot with the updated extra nonce.
func (h *BeaconHeader) UpdateCoinbaseScript(coinbaseScript []byte) {
	h.btcAux.UpdateCoinbaseScript(coinbaseScript)
}

func (h *BeaconHeader) MergeMiningNumber() uint32     { return h.mergeMiningNumber }
func (h *BeaconHeader) SetMergeMiningNumber(n uint32) { h.mergeMiningNumber = n }

func (h *BeaconHeader) MergedMiningTree() []byte     { return h.treeEncoding }
func (h *BeaconHeader) MergedMiningTreeSize() uint32 { return h.treeCodingLengthBits }

func (h *BeaconHeader) ShardMerkleProof() []chainhash.Hash   { return nil }
func (h *BeaconHeader) SetShardMerkleProof([]chainhash.Hash) {}

func (h *BeaconHeader) MergedMiningTreeCodingProof() ([]chainhash.Hash, []byte, uint32) {
	return h.mergeMiningProof, h.treeEncoding, h.treeCodingLengthBits
}

func (h *BeaconHeader) SetMergedMiningTreeCodingProof(hashes []chainhash.Hash, coding []byte, codingLengthBits uint32) {
	h.treeEncoding = coding
	h.treeCodingLengthBits = codingLengthBits
	h.mergeMiningProof = hashes
}

func (h *BeaconHeader) MaxLength() int { return MaxBeaconBlockHeaderPayload }

// ExclusiveHash computes the hash of the BeaconHeader without BtcAux.
// This hash needs to be set into Bitcoin coinbase tx, as proof of merge mining.
func (h *BeaconHeader) ExclusiveHash() chainhash.Hash {
	return h.BeaconExclusiveHash()
}

// PoWHash computes the hash for block that will be used to check ProofOfWork.
func (h *BeaconHeader) PoWHash() chainhash.Hash {
	return h.btcAux.BlockHash()
}

// BlockHash computes the block identifier hash for the given block BeaconHeader.
func (h *BeaconHeader) BlockHash() chainhash.Hash {
	w := bytes.NewBuffer(make([]byte, 0, MaxBeaconBlockHeaderPayload))

	_ = WriteElements(w,
		h.version,
		h.height,
		&h.prevMMRRoot,
		&h.merkleRoot,
		&h.mergeMiningRoot,
		h.bits,
		&h.shards,
		&h.k,
		&h.voteK,
		&h.mergeMiningNumber,
		&h.treeEncoding,
		&h.treeCodingLengthBits,
	)

	btcHash := h.btcAux.BlockHash()
	_ = WriteElements(w, &btcHash)

	return chainhash.DoubleHashH(w.Bytes())
}

// BeaconExclusiveHash computes the hash of the BeaconHeader without BtcAux.
// This hash needs to be set into Bitcoin coinbase tx, as proof of merge mining.
func (h *BeaconHeader) BeaconExclusiveHash() chainhash.Hash {
	buf := bytes.NewBuffer(make([]byte, 0, MaxBeaconBlockHeaderPayload))
	_ = WriteElements(buf,
		h.version,
		h.height,
		&h.prevMMRRoot,
		&h.prevBlock,
		&h.merkleRoot,
		&h.mergeMiningRoot,
		h.bits,
		&h.shards,
		&h.k,
		&h.voteK,
		&h.mergeMiningNumber,
		&h.treeEncoding,
		&h.treeCodingLengthBits,
	)

	return chainhash.DoubleHashH(buf.Bytes())
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding block headers stored to disk, such as in a
// database, as opposed to decoding block headers from the wire.
func (h *BeaconHeader) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	return readBeaconBlockHeader(r, h, false)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding block headers to be stored to disk, such as in a
// database, as opposed to encoding block headers for the wire.
func (h *BeaconHeader) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	return writeBeaconBlockHeader(w, h)
}

// Deserialize decodes a block BeaconHeader from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BeaconHeader) Read(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of readBeaconBlockHeader.
	return readBeaconBlockHeader(r, h, false)
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
func readBeaconBlockHeader(r io.Reader, bh *BeaconHeader, skipMagicCheck bool) error {
	if !skipMagicCheck {
		var magicN [1]byte

		err := ReadElement(r, &magicN)
		if err != nil {
			return err
		}

		if magicN[0] != beaconMagicByte {
			return fmt.Errorf("invalid magic byte: 0x%0x, expected beacon(0x%0x)", magicN, beaconMagicByte)
		}
	}
	err := ReadElements(r,
		&bh.version,
		&bh.height,
		&bh.prevMMRRoot,
		&bh.prevBlock,
		&bh.merkleRoot,
		&bh.mergeMiningRoot,
		&bh.bits,
		&bh.shards,
		&bh.k,
		&bh.voteK,
		&bh.mergeMiningNumber,
		&bh.treeEncoding,
		&bh.treeCodingLengthBits)
	if err != nil {
		return err
	}

	bh.chainWeight, err = ReadBigInt(r)
	if err != nil {
		return err
	}

	bh.mergeMiningProof, err = ReadHashArray(r)
	if err != nil {
		return err
	}

	return readBTCBlockHeader(r, &bh.btcAux)
}

// writeBeaconBlockHeader writes a bitcoin block BeaconHeader to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func writeBeaconBlockHeader(w io.Writer, bh *BeaconHeader) error {
	err := WriteElements(w,
		[1]byte{beaconMagicByte},
		bh.version,
		bh.height,
		&bh.prevMMRRoot,
		&bh.prevBlock,
		&bh.merkleRoot,
		&bh.mergeMiningRoot,
		bh.bits,
		bh.shards,
		bh.k,
		bh.voteK,
		&bh.mergeMiningNumber,
		&bh.treeEncoding,
		bh.treeCodingLengthBits,
	)
	if err != nil {
		return err
	}
	if err := WriteBigInt(w, bh.chainWeight); err != nil {
		return err
	}

	if err := WriteHashArray(w, bh.mergeMiningProof); err != nil {
		return err
	}

	return writeBTCBlockHeader(w, &bh.btcAux)
}

func ReadHashArray(r io.Reader) ([]chainhash.Hash, error) {
	count, err := ReadVarInt(r, ProtocolVersion)
	if err != nil {
		return nil, err
	}

	data := make([]chainhash.Hash, count)
	for i := range data {
		err = ReadElement(r, &data[i])
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func WriteHashArray(w io.Writer, data []chainhash.Hash) error {
	count := uint64(len(data))
	if err := WriteVarInt(w, count); err != nil {
		return err
	}

	for i := range data {
		if err := WriteElement(w, &data[i]); err != nil {
			return err
		}
	}
	return nil
}

func ReadBigInt(r io.Reader) (*big.Int, error) {
	count, err := ReadVarInt(r, ProtocolVersion)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, count)
	err = ReadElement(r, &buf)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(buf), nil
}

func WriteBigInt(w io.Writer, val *big.Int) error {
	data := val.Bytes()
	count := uint64(len(data))
	if err := WriteVarInt(w, count); err != nil {
		return err
	}

	return WriteElement(w, &data)
}
