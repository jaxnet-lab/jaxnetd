// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"errors"
	"io"
	"math"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// MaxBlockHeaderPayload is the maximum number of bytes a block header can be.
// Version 4 bytes + Timestamp 4 bytes + Bits 4 bytes + Nonce 4 bytes +
// PrevBlock and MerkleRoot hashes.
const MaxBlockHeaderPayload = 16 + (chainhash.HashSize * 2)

// BTCBlockAux defines information about a block and is used in the bitcoin
// block (MsgBlock) and headers (MsgHeaders) messages.
type BTCBlockAux struct {
	// Version of the block.  This is not the same as the protocol version.
	Version int32

	// Hash of the previous block header in the block chain.
	PrevBlock chainhash.Hash

	// Merkle tree reference to hash of all transactions for the block.
	MerkleRoot chainhash.Hash

	// Time the block was created.  This is, unfortunately, encoded as a
	// uint32 on the wire and therefore is limited to 2106.
	Timestamp time.Time

	// Difficulty target for the block.
	Bits uint32

	// Nonce used to generate the block.
	Nonce uint32

	CoinbaseAux CoinbaseAux
}

// BlockHash computes the block identifier hash for the given block header.
func (h *BTCBlockAux) BlockHash() chainhash.Hash {
	// Encode the header and double sha256 everything prior to the number of
	// transactions.  Ignore the error returns since there is no way the
	// encode could fail except being out of memory which would cause a
	// run-time panic.
	buf := bytes.NewBuffer(make([]byte, 0, MaxBlockHeaderPayload))

	// CoinbaseAux must be omitted to keep this hash equal to Bitcoin Leaf hash with the same header.
	sec := uint32(h.Timestamp.Unix())
	_ = WriteElements(buf, h.Version, &h.PrevBlock, &h.MerkleRoot, sec, h.Bits, h.Nonce)
	return chainhash.DoubleHashH(buf.Bytes())
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding block headers stored to disk, such as in a
// database, as opposed to decoding block headers from the wire.
func (h *BTCBlockAux) BtcDecode(r io.Reader, _ uint32, _ MessageEncoding) error {
	return readBTCBlockHeader(r, h)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding block headers to be stored to disk, such as in a
// database, as opposed to encoding block headers for the wire.
func (h *BTCBlockAux) BtcEncode(w io.Writer, _ uint32, _ MessageEncoding) error {
	return writeBTCBlockHeader(w, h)
}

// Deserialize decodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BTCBlockAux) Deserialize(r io.Reader) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of readBlockHeader.
	return readBTCBlockHeader(r, h)
}

// Serialize encodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *BTCBlockAux) Serialize(w io.Writer) error {
	// At the current time, there is no difference between the wire encoding
	// at protocol version 0 and the stable long-term storage format.  As
	// a result, make use of writeBlockHeader.
	return writeBTCBlockHeader(w, h)
}

// UpdateCoinbaseScript sets new coinbase script, rebuilds BTCBlockAux.TxMerkleProof
// and recalculates the BTCBlockAux.MerkleRoot with the updated extra nonce.
func (h *BTCBlockAux) UpdateCoinbaseScript(coinbaseScript []byte) {
	h.CoinbaseAux.Tx.TxIn[0].SignatureScript = coinbaseScript
	h.MerkleRoot = h.CoinbaseAux.UpdatedMerkleRoot()
}

// Copy creates a deep copy of a BlockHeader so that the original does not get
// modified when the copy is manipulated.
func (h *BTCBlockAux) Copy() *BTCBlockAux {
	clone := *h

	// all fields except this are passed by value
	// so we manually copy the following fields to prevent side effects
	clone.CoinbaseAux.Tx = *h.CoinbaseAux.Tx.Copy()
	clone.CoinbaseAux.TxMerkleProof = make([]chainhash.Hash, len(h.CoinbaseAux.TxMerkleProof))
	copy(clone.CoinbaseAux.TxMerkleProof, h.CoinbaseAux.TxMerkleProof)

	return &clone
}

// NewBTCBlockHeader returns a new BTCBlockAux using the provided version, previous
// block hash, merkle root hash, difficulty bits, and nonce used to generate the
// block with defaults for the remaining fields.
func NewBTCBlockHeader(version int32, prevHash, merkleRootHash *chainhash.Hash,
	bits uint32, nonce uint32,
) *BTCBlockAux {
	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	return &BTCBlockAux{
		Version:    version,
		PrevBlock:  *prevHash,
		MerkleRoot: *merkleRootHash,
		Timestamp:  time.Unix(time.Now().Unix(), 0),
		Bits:       bits,
		Nonce:      nonce,
	}
}

// readBlockHeader reads a bitcoin block header from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func readBTCBlockHeader(r io.Reader, bh *BTCBlockAux) error {
	err := ReadElements(r, &bh.Version, &bh.PrevBlock, &bh.MerkleRoot,
		(*Uint32Time)(&bh.Timestamp), &bh.Bits, &bh.Nonce)
	if err != nil {
		return err
	}

	return bh.CoinbaseAux.Deserialize(r)
}

// writeBlockHeader writes a bitcoin block header to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func writeBTCBlockHeader(w io.Writer, bh *BTCBlockAux) error {
	sec := uint32(bh.Timestamp.Unix())
	err := WriteElements(w, bh.Version, &bh.PrevBlock, &bh.MerkleRoot,
		sec, bh.Bits, bh.Nonce)
	if err != nil {
		return err
	}
	return bh.CoinbaseAux.Serialize(w)
}

type CoinbaseAux struct {
	// Tx is the first tx from block with reward.
	Tx MsgTx

	// Merkle tree leaves  of all transactions for the block.
	TxMerkleProof []chainhash.Hash
}

func (CoinbaseAux) New() CoinbaseAux {
	tx := MsgTx{
		Version: 0,
		TxIn: []*TxIn{
			{PreviousOutPoint: OutPoint{Hash: chainhash.ZeroHash, Index: math.MaxUint32}},
		},
	}
	return CoinbaseAux{
		Tx:            tx,
		TxMerkleProof: []chainhash.Hash{tx.TxHash()},
	}
}

func (CoinbaseAux) FromBlock(block *MsgBlock, witness bool) CoinbaseAux {
	tx := block.Transactions[0].Copy()
	aux := CoinbaseAux{
		Tx: *tx,
	}
	txHashes := CollectTxHashes(block.Transactions, witness)
	aux.TxMerkleProof = chainhash.BuildCoinbaseMerkleTreeProof(txHashes)
	return aux
}

func CollectTxHashes(transactions []*MsgTx, witness bool) []chainhash.Hash {
	hashes := make([]chainhash.Hash, len(transactions))

	// Create the base transaction hashes and populate the array with them.
	for i, tx := range transactions {
		// If we're computing a witness merkle root, instead of the
		// regular txid, we use the modified wtxid which includes a
		// transaction's witness data within the digest. Additionally,
		// the coinbase's wtxid is all zeroes.
		switch {
		case witness && i == 0:
			var zeroHash chainhash.Hash
			hashes[i] = zeroHash
		case witness:
			wSha := tx.WitnessHash()
			hashes[i] = wSha
		default:
			hashes[i] = tx.TxHash()
		}
	}

	return hashes
}

func (h *CoinbaseAux) UpdatedMerkleRoot() chainhash.Hash {
	if len(h.TxMerkleProof) == 0 {
		return h.Tx.TxHash()
	}

	coinbaseHash := h.Tx.TxHash()
	merkleTreeRoot := chainhash.CoinbaseMerkleTreeProofRoot(coinbaseHash, h.TxMerkleProof)
	return merkleTreeRoot
}

// Copy creates a deep copy of a CoinbaseAux so that the original does not get
// modified when the copy is manipulated.
func (h *CoinbaseAux) Copy() *CoinbaseAux {
	clone := &CoinbaseAux{}

	// all fields except this are passed by value
	// so we manually copy the following fields to prevent side effects
	clone.Tx = *h.Tx.Copy()
	clone.TxMerkleProof = make([]chainhash.Hash, len(h.TxMerkleProof))
	copy(clone.TxMerkleProof, h.TxMerkleProof)

	return clone
}

func (h *CoinbaseAux) Validate(merkleRoot chainhash.Hash) error {
	coinbaseHash := h.Tx.TxHash()
	if !chainhash.ValidateCoinbaseMerkleTreeProof(coinbaseHash, h.TxMerkleProof, merkleRoot) {
		return errors.New("tx_merkle tree root is not match with blockMerkle root")
	}

	return nil
}

// Deserialize decodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *CoinbaseAux) Deserialize(r io.Reader) error {
	err := h.Tx.BtcDecode(r, ProtocolVersion, BaseEncoding)
	if err != nil {
		return err
	}

	h.TxMerkleProof, err = ReadHashArray(r)
	return err
}

// Serialize encodes a block header from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field.
func (h *CoinbaseAux) Serialize(w io.Writer) error {
	if err := h.Tx.BtcEncode(w, ProtocolVersion, BaseEncoding); err != nil {
		return err
	}
	return WriteHashArray(w, h.TxMerkleProof)
}
