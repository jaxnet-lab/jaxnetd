// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaindata

import (
	"bytes"
	"fmt"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

const (
	// CoinbaseWitnessDataLen is the required length of the only element within
	// the coinbase's witness data if the coinbase transaction contains a
	// witness commitment.
	CoinbaseWitnessDataLen = 32

	// CoinbaseWitnessPkScriptLength is the length of the public key script
	// containing an OP_RETURN, the WitnessMagicBytes, and the witness
	// commitment itself. In order to be a valid candidate for the output
	// containing the witness commitment
	CoinbaseWitnessPkScriptLength = 38
)

// WitnessMagicBytes is the prefix marker within the public key script
// of a coinbase output to indicate that this output holds the witness
// commitment for a block.
var WitnessMagicBytes = []byte{
	txscript.OP_RETURN,
	txscript.OP_DATA_36,
	0xaa,
	0x21,
	0xa9,
	0xed,
}

// BuildMerkleTreeStore creates a merkle tree from a slice of transactions,
// stores it using a linear array, and returns a slice of the backing array.  A
// linear array was chosen as opposed to an actual tree structure since it uses
// about half as much memory.  The following describes a merkle tree and how it
// is stored in a linear array.
//
// A merkle tree is a tree in which every non-leaf node is the Hash of its
// children nodes.  A diagram depicting how this works for bitcoin transactions
// where h(x) is a double sha256 follows:
//
//	         root = h1234 = h(h12 + h34)
//	        /                           \
//	  h12 = h(h1 + h2)            h34 = h(h3 + h4)
//	   /            \              /            \
//	h1 = h(tx1)  h2 = h(tx2)    h3 = h(tx3)  h4 = h(tx4)
//
// The above stored as a linear array is as follows:
//
// 	[h1 h2 h3 h4 h12 h34 root]
//
// As the above shows, the merkle root is always the last element in the array.
//
// The number of inputs is not always a power of two which results in a
// balanced tree structure as above.  In that case, parent nodes with no
// children are also zero and parent nodes with only a single left node
// are calculated by concatenating the left node with itself before hashing.
// Since this function uses nodes that are pointers to the hashes, empty nodes
// will be nil.
//
// The additional bool parameter indicates if we are generating the merkle tree
// using witness transaction id's rather than regular transaction id's. This
// also presents an additional case wherein the wtxid of the coinbase transaction
// is the zeroHash.
func BuildMerkleTreeStore(transactions []*jaxutil.Tx, witness bool) []*chainhash.Hash {
	merkles := make([]chainhash.Hash, len(transactions))

	// Create the base transaction hashes and populate the array with them.
	for i, tx := range transactions {
		// If we're computing a witness merkle root, instead of the
		// regular txid, we use the modified wtxid which includes a
		// transaction's witness data within the digest. Additionally,
		// the coinbase's wtxid is all zeroes.
		switch {
		case witness && i == 0:
			var zeroHash chainhash.Hash
			merkles[i] = zeroHash
		case witness:
			wSha := tx.MsgTx().WitnessHash()
			merkles[i] = wSha
		default:
			merkles[i] = *tx.Hash()
		}
	}
	return chainhash.BuildMerkleTreeStore(merkles)
}

func CollectTxHashes(transactions []*jaxutil.Tx, witness bool) []chainhash.Hash {
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
			wSha := tx.MsgTx().WitnessHash()
			hashes[i] = wSha
		default:
			hashes[i] = *tx.Hash()
		}
	}

	return hashes
}

// ExtractWitnessCommitment attempts to locate, and return the witness
// commitment for a block. The witness commitment is of the form:
// SHA256(witness root || witness nonce). The function additionally returns a
// boolean indicating if the witness root was located within any of the txOut's
// in the passed transaction. The witness commitment is stored as the data push
// for an OP_RETURN with special magic bytes to aide in location.
func ExtractWitnessCommitment(tx *jaxutil.Tx) ([]byte, bool) {
	// The witness commitment *must* be located within one of the coinbase
	// transaction's outputs.
	if !IsCoinBase(tx) {
		return nil, false
	}

	msgTx := tx.MsgTx()
	for i := len(msgTx.TxOut) - 1; i >= 0; i-- {
		// The public key script that contains the witness commitment
		// must shared a prefix with the WitnessMagicBytes, and be at
		// least 38 bytes.
		pkScript := msgTx.TxOut[i].PkScript
		if len(pkScript) >= CoinbaseWitnessPkScriptLength &&
			bytes.HasPrefix(pkScript, WitnessMagicBytes) {

			// The witness commitment itself is a 32-byte Hash
			// directly after the WitnessMagicBytes. The remaining
			// bytes beyond the 38th byte currently have no consensus
			// meaning.
			start := len(WitnessMagicBytes)
			end := CoinbaseWitnessPkScriptLength
			return msgTx.TxOut[i].PkScript[start:end], true
		}
	}

	return nil, false
}

// ValidateWitnessCommitment validates the witness commitment (if any) found
// within the coinbase transaction of the passed block.
func ValidateWitnessCommitment(blk *jaxutil.Block) error {
	// If the block doesn't have any transactions at all, then we won't be
	// able to extract a commitment from the non-existent coinbase
	// transaction. So we exit early here.
	if len(blk.Transactions()) == 0 {
		str := "cannot validate witness commitment of block without " +
			"transactions"
		return NewRuleError(ErrNoTransactions, str)
	}

	coinbaseTx := blk.Transactions()[0]
	if len(coinbaseTx.MsgTx().TxIn) == 0 {
		return NewRuleError(ErrNoTxInputs, "transaction has no inputs")
	}

	witnessCommitment, witnessFound := ExtractWitnessCommitment(coinbaseTx)

	// If we can't find a witness commitment in any of the coinbase's
	// outputs, then the block MUST NOT contain any transactions with
	// witness data.
	if !witnessFound {
		for _, tx := range blk.Transactions() {
			msgTx := tx.MsgTx()
			if msgTx.HasWitness() {
				str := fmt.Sprintf("block contains transaction with witness" +
					" data, yet no witness commitment present")
				return NewRuleError(ErrUnexpectedWitness, str)
			}
		}
		return nil
	}

	// At this point the block contains a witness commitment, so the
	// coinbase transaction MUST have exactly one witness element within
	// its witness data and that element must be exactly
	// CoinbaseWitnessDataLen bytes.
	coinbaseWitness := coinbaseTx.MsgTx().TxIn[0].Witness
	if len(coinbaseWitness) != 1 {
		str := fmt.Sprintf("the coinbase transaction has %d items in "+
			"its witness stack when only one is allowed",
			len(coinbaseWitness))
		return NewRuleError(ErrInvalidWitnessCommitment, str)
	}
	witnessNonce := coinbaseWitness[0]
	if len(witnessNonce) != CoinbaseWitnessDataLen {
		str := fmt.Sprintf("the coinbase transaction witness nonce "+
			"has %d bytes when it must be %d bytes",
			len(witnessNonce), CoinbaseWitnessDataLen)
		return NewRuleError(ErrInvalidWitnessCommitment, str)
	}

	// Finally, with the preliminary checks out of the way, we can check if
	// the extracted witnessCommitment is equal to:
	// SHA256(witnessMerkleRoot || witnessNonce). Where witnessNonce is the
	// coinbase transaction's only witness item.
	witnessMerkleTree := BuildMerkleTreeStore(blk.Transactions(), true)
	witnessMerkleRoot := witnessMerkleTree[len(witnessMerkleTree)-1]

	var witnessPreimage [chainhash.HashSize * 2]byte
	copy(witnessPreimage[:], witnessMerkleRoot[:])
	copy(witnessPreimage[chainhash.HashSize:], witnessNonce)

	computedCommitment := chainhash.DoubleHashB(witnessPreimage[:])
	if !bytes.Equal(computedCommitment, witnessCommitment) {
		str := fmt.Sprintf("witness commitment does not match: "+
			"computed %v, coinbase includes %v", computedCommitment,
			witnessCommitment)
		return NewRuleError(ErrWitnessCommitmentMismatch, str)
	}

	return nil
}
