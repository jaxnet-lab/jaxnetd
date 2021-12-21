// Copyright (c) 2015-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaindata

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const (
	// blockHdrSize is the size of a block header.  This is simply the
	// constant from wire and is only provided here for convenience since
	// wire.MaxBeaconBlockHeaderPayload is quite long.
	// blockHdrSize = shard.MaxBeaconBlockHeaderPayload

	// LatestUtxoSetBucketVersion is the current version of the utxo set
	// bucket that is used to track all unspent outputs.
	LatestUtxoSetBucketVersion = 2

	// LatestSpendJournalBucketVersion is the current version of the spend
	// journal bucket that is used to track all spent transactions for use
	// in reorgs.
	LatestSpendJournalBucketVersion = 1
)

var (
	// BlockIndexBucketName is the name of the db bucket used to house to the
	// block headers and contextual information.
	BlockIndexBucketName = []byte("blockheaderidx")

	// HashIndexBucketName is the name of the db bucket used to house to the
	// [block Hash] -> [block height] index.
	HashIndexBucketName = []byte("hashidx")

	// HeightIndexBucketName is the name of the db bucket used to house to the
	// [block height] -> [block Hash] index.
	HeightIndexBucketName = []byte("heightidx")

	// ChainStateKeyName is the name of the db key used to store the best
	// chain state.
	ChainStateKeyName = []byte("chainstate")

	// MMRRootsToHashBucketName is unordered storage of mmr root and
	// corresponding last block hash for this root.
	// [mmr root hash] -> [block Hash]
	MMRRootsToHashBucketName = []byte("mmr_root_to_hash")

	// HashToMMRRootBucketName is unordered storage of block hash and
	// corresponding  mmr root for this block.
	// [mmr root hash] -> [block Hash]
	// [block Hash] -> [mmr root hash]
	HashToMMRRootBucketName = []byte("hash_to_mmr_root")

	// BlockHashToSerialID is the name of the db bucket used to house the
	// [block Hash] -> [serial id] index.
	BlockHashToSerialID = []byte("hash_to_serialid")

	// SerialIDToPrevBlock is the name of the db bucket used to house the
	// block serial id to hash and previous serial id.
	//  [serial id] -> [block Hash; previous serial id]
	SerialIDToPrevBlock = []byte("serialid_to_prev_block")

	// SpendJournalVersionKeyName is the name of the db key used to store
	// the version of the spend journal currently in the database.
	SpendJournalVersionKeyName = []byte("spendjournalversion")

	// SpendJournalBucketName is the name of the db bucket used to house
	// transactions outputs that are spent in each block.
	SpendJournalBucketName = []byte("spendjournal")

	// UtxoSetVersionKeyName is the name of the db key used to store the
	// version of the utxo set currently in the database.
	UtxoSetVersionKeyName = []byte("utxosetversion")

	// UtxoSetBucketName is the name of the db bucket used to house the
	// unspent transaction output set.
	UtxoSetBucketName = []byte("utxosetv2")

	// EADAddressesBucketNameV2 is the name of the db bucket used to house the
	// net addresses of Exchange Agents.
	EADAddressesBucketNameV2 = []byte("ead_addresses_v2")

	// BestChainSerialIDsBucketName - is the name of the db bucket used to house the
	// list of serialIDs of the blocks in the best chain. Values are written to it on node shutdown.
	// Bucket is used for quick catch-up launch
	BestChainSerialIDsBucketName = []byte("bestchain_serialids")

	ShardCreationsBucketName = []byte("shard_creations")

	// byteOrder is the preferred byte order used for serializing numeric
	// fields for storage in the database.
	byteOrder = binary.LittleEndian
)

// ErrNotInMainChain signifies that a block Hash or height that is not in the
// main chain was requested.
type ErrNotInMainChain string

// Error implements the error interface.
func (e ErrNotInMainChain) Error() string {
	return string(e)
}

// IsNotInMainChainErr returns whether or not the passed error is an
// ErrNotInMainChain error.
func IsNotInMainChainErr(err error) bool {
	_, ok := err.(ErrNotInMainChain)
	return ok
}

// ErrDeserialize signifies that a problem was encountered when deserializing
// data.
type ErrDeserialize string

// Error implements the error interface.
func (e ErrDeserialize) Error() string {
	return string(e)
}

// IsDeserializeErr returns whether or not the passed error is an ErrDeserialize
// error.
func IsDeserializeErr(err error) bool {
	_, ok := err.(ErrDeserialize)
	return ok
}

// dbFetchVersion fetches an individual version with the given key from the
// metadata bucket.  It is primarily used to track versions on entities such as
// buckets.  It returns zero if the provided key does not exist.
func dbFetchVersion(dbTx database.Tx, key []byte) uint32 {
	serialized := dbTx.Metadata().Get(key)
	if serialized == nil {
		return 0
	}

	return byteOrder.Uint32(serialized)
}

// DBPutVersion uses an existing database transaction to update the provided
// key in the metadata bucket to the given version.  It is primarily used to
// track versions on entities such as buckets.
func DBPutVersion(dbTx database.Tx, key []byte, version uint32) error {
	var serialized [4]byte
	byteOrder.PutUint32(serialized[:], version)
	return dbTx.Metadata().Put(key, serialized[:])
}

// DBFetchOrCreateVersion uses an existing database transaction to attempt to
// fetch the provided key from the metadata bucket as a version and in the case
// it doesn't exist, it adds the entry with the provided default version and
// returns that.  This is useful during upgrades to automatically handle loading
// and adding version keys as necessary.
func DBFetchOrCreateVersion(dbTx database.Tx, key []byte, defaultVersion uint32) (uint32, error) {
	version := dbFetchVersion(dbTx, key)
	if version == 0 {
		version = defaultVersion
		err := DBPutVersion(dbTx, key, version)
		if err != nil {
			return 0, err
		}
	}

	return version, nil
}

// -----------------------------------------------------------------------------
// The transaction spend journal consists of an entry for each block connected
// to the main chain which contains the transaction outputs the block spends
// serialized such that the order is the reverse of the order they were spent.
//
// This is required because reorganizing the chain necessarily entails
// disconnecting blocks to get back to the point of the fork which implies
// unspending all of the transaction outputs that each block previously spent.
// Since the utxo set, by definition, only contains unspent transaction outputs,
// the spent transaction outputs must be resurrected from somewhere.  There is
// more than one way this could be done, however this is the most straight
// forward method that does not require having a transaction index and unpruned
// blockchain.
//
// NOTE: This format is NOT self describing.  The additional details such as
// the number of entries (transaction inputs) are expected to come from the
// block itself and the utxo set (for legacy entries).  The rationale in doing
// this is to save space.  This is also the reason the spent outputs are
// serialized in the reverse order they are spent because later transactions are
// allowed to spend outputs from earlier ones in the same block.
//
// The reserved field below used to keep track of the version of the containing
// transaction when the height in the header code was non-zero, however the
// height is always non-zero now, but keeping the extra reserved field allows
// backwards compatibility.
//
// The serialized format is:
//
//   [<header code><reserved><compressed txout>],...
//
//   Field                Type     Size
//   header code          VLQ      variable
//   reserved             byte     1
//   compressed txout
//     compressed amount  VLQ      variable
//     compressed script  []byte   variable
//
// The serialized header code format is:
//   bit 0 - containing transaction is a coinbase
//   bits 1-x - height of the block that contains the spent txout
//
// Example 1:
// From block 170 in main blockchain.
//
//    1300320511db93e1dcdb8a016b49840f8c53bc1eb68a382e97b1482ecad7b148a6909a5c
//    <><><------------------------------------------------------------------>
//     | |                                  |
//     | reserved                  compressed txout
//    header code
//
//  - header code: 0x13 (coinbase, height 9)
//  - reserved: 0x00
//  - compressed txout 0:
//    - 0x32: VLQ-encoded compressed amount for 5000000000 (50 BTC)
//    - 0x05: special script type pay-to-pubkey
//    - 0x11...5c: x-coordinate of the pubkey
//
// Example 2:
// Adapted from block 100025 in main blockchain.
//
//    8b99700091f20f006edbc6c4d31bae9f1ccc38538a114bf42de65e868b99700086c64700b2fb57eadf61e106a100a7445a8c3f67898841ec
//    <----><><----------------------------------------------><----><><---------------------------------------------->
//     |    |                         |                        |    |                         |
//     |    reserved         compressed txout                  |    reserved         compressed txout
//    header code                                          header code
//
//  - Last spent output:
//    - header code: 0x8b9970 (not coinbase, height 100024)
//    - reserved: 0x00
//    - compressed txout:
//      - 0x91f20f: VLQ-encoded compressed amount for 34405000000 (344.05 BTC)
//      - 0x00: special script type pay-to-pubkey-Hash
//      - 0x6e...86: pubkey Hash
//  - Second to last spent output:
//    - header code: 0x8b9970 (not coinbase, height 100024)
//    - reserved: 0x00
//    - compressed txout:
//      - 0x86c647: VLQ-encoded compressed amount for 13761000000 (137.61 BTC)
//      - 0x00: special script type pay-to-pubkey-Hash
//      - 0xb2...ec: pubkey Hash
// -----------------------------------------------------------------------------

// SpentTxOut contains a spent transaction output and potentially additional
// contextual information such as whether or not it was contained in a coinbase
// transaction, the version of the transaction it was contained in, and which
// block height the containing transaction was included in.  As described in
// the comments above, the additional contextual information will only be valid
// when this spent txout is spending the last unspent output of the containing
// transaction.
type SpentTxOut struct {
	// Amount is the amount of the output.
	Amount int64

	// PkScipt is the the public key script for the output.
	PkScript []byte

	// Height is the height of the the block containing the creating tx.
	Height int32

	// Denotes if the creating tx is a coinbase.
	IsCoinBase bool
}

// spentTxOutHeaderCode returns the calculated header code to be used when
// serializing the provided stxo entry.
func spentTxOutHeaderCode(stxo *SpentTxOut) uint64 {
	// As described in the serialization format comments, the header code
	// encodes the height shifted over one bit and the coinbase flag in the
	// lowest bit.
	headerCode := uint64(stxo.Height) << 1
	if stxo.IsCoinBase {
		headerCode |= 0x01
	}

	return headerCode
}

// spentTxOutSerializeSize returns the number of bytes it would take to
// serialize the passed stxo according to the format described above.
func spentTxOutSerializeSize(stxo *SpentTxOut) int {
	size := serializeSizeVLQ(spentTxOutHeaderCode(stxo))
	if stxo.Height > 0 {
		// The legacy v1 spend journal format conditionally tracked the
		// containing transaction version when the height was non-zero,
		// so this is required for backwards compat.
		size += serializeSizeVLQ(0)
	}
	return size + compressedTxOutSize(uint64(stxo.Amount), stxo.PkScript)
}

// putSpentTxOut serializes the passed stxo according to the format described
// above directly into the passed target byte slice.  The target byte slice must
// be at least large enough to handle the number of bytes returned by the
// SpentTxOutSerializeSize function or it will panic.
func putSpentTxOut(target []byte, stxo *SpentTxOut) int {
	headerCode := spentTxOutHeaderCode(stxo)
	offset := putVLQ(target, headerCode)
	if stxo.Height > 0 {
		// The legacy v1 spend journal format conditionally tracked the
		// containing transaction version when the height was non-zero,
		// so this is required for backwards compat.
		offset += putVLQ(target[offset:], 0)
	}
	return offset + putCompressedTxOut(target[offset:], uint64(stxo.Amount),
		stxo.PkScript)
}

// decodeSpentTxOut decodes the passed serialized stxo entry, possibly followed
// by other data, into the passed stxo struct.  It returns the number of bytes
// read.
// nolint: gomnd
func decodeSpentTxOut(serialized []byte, stxo *SpentTxOut) (int, error) {
	// Ensure there are bytes to decode.
	if len(serialized) == 0 {
		return 0, ErrDeserialize("no serialized bytes")
	}

	// Deserialize the header code.
	code, offset := deserializeVLQ(serialized)
	if offset >= len(serialized) {
		return offset, ErrDeserialize("unexpected end of data after " +
			"header code")
	}

	// Decode the header code.
	//
	// Bit 0 indicates containing transaction is a coinbase.
	// Bits 1-x encode height of containing transaction.
	stxo.IsCoinBase = code&0x01 != 0
	stxo.Height = int32(code >> 1)
	if stxo.Height > 0 {
		// The legacy v1 spend journal format conditionally tracked the
		// containing transaction version when the height was non-zero,
		// so this is required for backwards compat.
		_, bytesRead := deserializeVLQ(serialized[offset:])
		offset += bytesRead
		if offset >= len(serialized) {
			return offset, ErrDeserialize("unexpected end of data " +
				"after reserved")
		}
	}

	// Decode the compressed txout.
	amount, pkScript, bytesRead, err := decodeCompressedTxOut(
		serialized[offset:])
	offset += bytesRead
	if err != nil {
		return offset, ErrDeserialize(fmt.Sprintf("unable to decode "+
			"txout: %v", err))
	}
	stxo.Amount = int64(amount)
	stxo.PkScript = pkScript
	return offset, nil
}

// deserializeSpendJournalEntry decodes the passed serialized byte slice into a
// slice of spent txouts according to the format described in detail above.
//
// Since the serialization format is not self describing, as noted in the
// format comments, this function also requires the transactions that spend the
// txouts.
func deserializeSpendJournalEntry(serialized []byte, txns []*wire.MsgTx) ([]SpentTxOut, error) {
	// Calculate the total number of stxos.
	var numStxos int
	for _, tx := range txns {
		numStxos += len(tx.TxIn)
	}

	// When a block has no spent txouts there is nothing to serialize.
	if len(serialized) == 0 {
		// Ensure the block actually has no stxos.  This should never
		// happen unless there is database corruption or an empty entry
		// erroneously made its way into the database.
		if numStxos != 0 {
			return nil, AssertError(fmt.Sprintf("mismatched spend "+
				"journal serialization - no serialization for "+
				"expected %d stxos", numStxos))
		}

		return nil, nil
	}

	// Loop backwards through all transactions so everything is read in
	// reverse order to match the serialization order.
	stxoIdx := numStxos - 1
	offset := 0
	stxos := make([]SpentTxOut, numStxos)
	for txIdx := len(txns) - 1; txIdx > -1; txIdx-- {
		tx := txns[txIdx]

		// Loop backwards through all of the transaction inputs and read
		// the associated stxo.
		for txInIdx := len(tx.TxIn) - 1; txInIdx > -1; txInIdx-- {
			txIn := tx.TxIn[txInIdx]
			stxo := &stxos[stxoIdx]
			stxoIdx--

			n, err := decodeSpentTxOut(serialized[offset:], stxo)
			offset += n
			if err != nil {
				return nil, ErrDeserialize(fmt.Sprintf("unable "+
					"to decode stxo for %v: %v",
					txIn.PreviousOutPoint, err))
			}
		}
	}

	return stxos, nil
}

// serializeSpendJournalEntry serializes all of the passed spent txouts into a
// single byte slice according to the format described in detail above.
func serializeSpendJournalEntry(stxos []SpentTxOut) []byte {
	if len(stxos) == 0 {
		return nil
	}

	// Calculate the size needed to serialize the entire journal entry.
	var size int
	for i := range stxos {
		size += spentTxOutSerializeSize(&stxos[i])
	}
	serialized := make([]byte, size)

	// Serialize each individual stxo directly into the slice in reverse
	// order one after the other.
	var offset int
	for i := len(stxos) - 1; i > -1; i-- {
		offset += putSpentTxOut(serialized[offset:], &stxos[i])
	}

	return serialized
}

// DBFetchSpendJournalEntry fetches the spend journal entry for the passed block
// and deserializes it into a slice of spent txout entries.
//
// NOTE: Legacy entries will not have the coinbase flag or height set unless it
// was the final output spend in the containing transaction.  It is up to the
// caller to handle this properly by looking the information up in the utxo set.
func DBFetchSpendJournalEntry(dbTx database.Tx, block *jaxutil.Block) ([]SpentTxOut, error) {
	// Exclude the coinbase transaction since it can't spend anything.
	spendBucket := dbTx.Metadata().Bucket(SpendJournalBucketName)
	serialized := spendBucket.Get(block.Hash()[:])
	blockTxns := block.MsgBlock().Transactions[1:]
	stxos, err := deserializeSpendJournalEntry(serialized, blockTxns)
	if err != nil {
		// Ensure any deserialization errors are returned as database
		// corruption errors.
		if IsDeserializeErr(err) {
			return nil, database.Error{
				ErrorCode: database.ErrCorruption,
				Description: fmt.Sprintf("corrupt spend "+
					"information for %v: %v", block.Hash(),
					err),
			}
		}

		return nil, err
	}

	return stxos, nil
}

// DBPutSpendJournalEntry uses an existing database transaction to update the
// spend journal entry for the given block Hash using the provided slice of
// spent txouts.   The spent txouts slice must contain an entry for every txout
// the transactions in the block spend in the order they are spent.
func DBPutSpendJournalEntry(dbTx database.Tx, blockHash *chainhash.Hash, stxos []SpentTxOut) error {
	spendBucket := dbTx.Metadata().Bucket(SpendJournalBucketName)
	serialized := serializeSpendJournalEntry(stxos)
	return spendBucket.Put(blockHash[:], serialized)
}

// DBRemoveSpendJournalEntry uses an existing database transaction to remove the
// spend journal entry for the passed block Hash.
func DBRemoveSpendJournalEntry(dbTx database.Tx, blockHash *chainhash.Hash) error {
	spendBucket := dbTx.Metadata().Bucket(SpendJournalBucketName)
	return spendBucket.Delete(blockHash[:])
}

// -----------------------------------------------------------------------------
// The unspent transaction output (utxo) set consists of an entry for each
// unspent output using a format that is optimized to reduce space using domain
// specific compression algorithms.  This format is a slightly modified version
// of the format used in Bitcoin Core.
//
// Each entry is keyed by an outpoint as specified below.  It is important to
// note that the key encoding uses a VLQ, which employs an MSB encoding so
// iteration of utxos when doing byte-wise comparisons will produce them in
// order.
//
// The serialized key format is:
//   <Hash><output index>
//
//   Field                Type             Size
//   Hash                 chainhash.Hash   chainhash.HashSize
//   output index         VLQ              variable
//
// The serialized value format is:
//
//   <header code><compressed txout>
//
//   Field                Type     Size
//   header code          VLQ      variable
//   compressed txout
//     compressed amount  VLQ      variable
//     compressed script  []byte   variable
//
// The serialized header code format is:
//   bit 0 - containing transaction is a coinbase
//   bits 1-x - height of the block that contains the unspent txout
//
// Example 1:
// From tx in main blockchain:
// Blk 1, 0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098:0
//
//    03320496b538e853519c726a2c91e61ec11600ae1390813a627c66fb8be7947be63c52
//    <><------------------------------------------------------------------>
//     |                                          |
//   header code                         compressed txout
//
//  - header code: 0x03 (coinbase, height 1)
//  - compressed txout:
//    - 0x32: VLQ-encoded compressed amount for 5000000000 (50 BTC)
//    - 0x04: special script type pay-to-pubkey
//    - 0x96...52: x-coordinate of the pubkey
//
// Example 2:
// From tx in main blockchain:
// Blk 113931, 4a16969aa4764dd7507fc1de7f0baa4850a246de90c45e59a3207f9a26b5036f:2
//
//    8cf316800900b8025be1b3efc63b0ad48e7f9f10e87544528d58
//    <----><------------------------------------------>
//      |                             |
//   header code             compressed txout
//
//  - header code: 0x8cf316 (not coinbase, height 113931)
//  - compressed txout:
//    - 0x8009: VLQ-encoded compressed amount for 15000000 (0.15 BTC)
//    - 0x00: special script type pay-to-pubkey-Hash
//    - 0xb8...58: pubkey Hash
//
// Example 3:
// From tx in main blockchain:
// Blk 338156, 1b02d1c8cfef60a189017b9a420c682cf4a0028175f2f563209e4ff61c8c3620:22
//
//    a8a2588ba5b9e763011dd46a006572d820e448e12d2bbb38640bc718e6
//    <----><-------------------------------------------------->
//      |                             |
//   header code             compressed txout
//
//  - header code: 0xa8a258 (not coinbase, height 338156)
//  - compressed txout:
//    - 0x8ba5b9e763: VLQ-encoded compressed amount for 366875659 (3.66875659 BTC)
//    - 0x01: special script type pay-to-script-Hash
//    - 0x1d...e6: script Hash
// -----------------------------------------------------------------------------

// maxUint32VLQSerializeSize is the maximum number of bytes a max uint32 takes
// to serialize as a VLQ.
var maxUint32VLQSerializeSize = serializeSizeVLQ(1<<32 - 1)

// outpointKeyPool defines a concurrent safe free list of byte slices used to
// provide temporary buffers for outpoint database keys.
var outpointKeyPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, chainhash.HashSize+maxUint32VLQSerializeSize)
		return &b // Pointer to slice to avoid boxing alloc.
	},
}

// outpointKey returns a key suitable for use as a database key in the utxo set
// while making use of a free list.  A new buffer is allocated if there are not
// already any available on the free list.  The returned byte slice should be
// returned to the free list by using the recycleOutpointKey function when the
// caller is done with it _unless_ the slice will need to live for longer than
// the caller can calculate such as when used to write to the database.
func outpointKey(outpoint wire.OutPoint) *[]byte {
	// A VLQ employs an MSB encoding, so they are useful not only to reduce
	// the amount of storage space, but also so iteration of utxos when
	// doing byte-wise comparisons will produce them in order.
	key, ok := outpointKeyPool.Get().(*[]byte)
	if !ok {
		return nil
	}
	idx := uint64(outpoint.Index)
	*key = (*key)[:chainhash.HashSize+serializeSizeVLQ(idx)]
	copy(*key, outpoint.Hash[:])
	putVLQ((*key)[chainhash.HashSize:], idx)
	return key
}

func keyToOutpoint(key []byte) (outpoint wire.OutPoint) {
	copy(outpoint.Hash[:], key[:chainhash.HashSize])
	idx, _ := deserializeVLQ(key[chainhash.HashSize:])
	outpoint.Index = uint32(idx)
	return
}

// recycleOutpointKey puts the provided byte slice, which should have been
// obtained via the outpointKey function, back on the free list.
func recycleOutpointKey(key *[]byte) {
	outpointKeyPool.Put(key)
}

// utxoEntryHeaderCode returns the calculated header code to be used when
// serializing the provided utxo entry.
func utxoEntryHeaderCode(entry *UtxoEntry) (uint64, error) {
	if entry.IsSpent() {
		return 0, AssertError("attempt to serialize spent utxo header")
	}

	// As described in the serialization format comments, the header code
	// encodes the height shifted over one bit and the coinbase flag in the
	// lowest bit.
	headerCode := uint64(entry.BlockHeight()) << 1
	if entry.IsCoinBase() {
		headerCode |= 0x01
	}

	return headerCode, nil
}

// serializeUtxoEntry returns the entry serialized to a format that is suitable
// for long-term storage.  The format is described in detail above.
func serializeUtxoEntry(entry *UtxoEntry) ([]byte, error) {
	// Spent outputs have no serialization.
	if entry.IsSpent() {
		return nil, nil
	}

	// Encode the header code.
	headerCode, err := utxoEntryHeaderCode(entry)
	if err != nil {
		return nil, err
	}

	// Calculate the size needed to serialize the entry.
	size := serializeSizeVLQ(headerCode) +
		compressedTxOutSize(uint64(entry.Amount()), entry.PkScript())

	// Serialize the header code followed by the compressed unspent
	// transaction output.
	serialized := make([]byte, size)
	offset := putVLQ(serialized, headerCode)
	putCompressedTxOut(serialized[offset:], uint64(entry.Amount()),
		entry.PkScript())

	return serialized, nil
}

// DeserializeUtxoEntry decodes a utxo entry from the passed serialized byte
// slice into a new UtxoEntry using a format that is suitable for long-term
// storage.  The format is described in detail above.
// nolint: gomnd
func DeserializeUtxoEntry(serialized []byte) (*UtxoEntry, error) {
	// Deserialize the header code.
	code, offset := deserializeVLQ(serialized)
	if offset >= len(serialized) {
		return nil, ErrDeserialize("unexpected end of data after header")
	}

	// Decode the header code.
	//
	// Bit 0 indicates whether the containing transaction is a coinbase.
	// Bits 1-x encode height of containing transaction.
	isCoinBase := code&0x01 != 0
	blockHeight := int32(code >> 1)

	// Decode the compressed unspent transaction output.
	amount, pkScript, _, err := decodeCompressedTxOut(serialized[offset:])
	if err != nil {
		return nil, ErrDeserialize(fmt.Sprintf("unable to decode "+
			"utxo: %v", err))
	}

	entry := &UtxoEntry{
		amount:      int64(amount),
		pkScript:    pkScript,
		blockHeight: blockHeight,
		packedFlags: 0,
	}
	if isCoinBase {
		entry.packedFlags |= tfCoinBase
	}

	return entry, nil
}

// dbFetchUtxoEntryByHash attempts to find and fetch a utxo for the given Hash.
// It uses a cursor and seek to try and do this as efficiently as possible.
//
// When there are no entries for the provided Hash, nil will be returned for the
// both the entry and the error.
func dbFetchUtxoEntryByHash(dbTx database.Tx, hash *chainhash.Hash) (*UtxoEntry, error) {
	// Attempt to find an entry by seeking for the Hash along with a zero
	// index.  Due to the fact the keys are serialized as <Hash><index>,
	// where the index uses an MSB encoding, if there are any entries for
	// the Hash at all, one will be found.
	cursor := dbTx.Metadata().Bucket(UtxoSetBucketName).Cursor()
	key := outpointKey(wire.OutPoint{Hash: *hash, Index: 0})
	ok := cursor.Seek(*key)
	recycleOutpointKey(key)
	if !ok {
		return nil, nil
	}

	// An entry was found, but it could just be an entry with the next
	// highest Hash after the requested one, so make sure the hashes
	// actually match.
	cursorKey := cursor.Key()
	if len(cursorKey) < chainhash.HashSize {
		return nil, nil
	}
	if !bytes.Equal(hash[:], cursorKey[:chainhash.HashSize]) {
		return nil, nil
	}

	return DeserializeUtxoEntry(cursor.Value())
}

// DBFetchUtxoEntry uses an existing database transaction to fetch the specified
// transaction output from the utxo set.
//
// When there is no entry for the provided output, nil will be returned for both
// the entry and the error.
func DBFetchUtxoEntry(dbTx database.Tx, outpoint wire.OutPoint) (*UtxoEntry, error) {
	// Fetch the unspent transaction output information for the passed
	// transaction output.  Return now when there is no entry.
	key := outpointKey(outpoint)
	utxoBucket := dbTx.Metadata().Bucket(UtxoSetBucketName)
	serializedUtxo := utxoBucket.Get(*key)
	recycleOutpointKey(key)
	if serializedUtxo == nil {
		return nil, nil
	}

	// A non-nil zero-length entry means there is an entry in the database
	// for a spent transaction output which should never be the case.
	if len(serializedUtxo) == 0 {
		return nil, AssertError(fmt.Sprintf("database contains entry "+
			"for spent tx output %v", outpoint))
	}

	// Deserialize the utxo entry and return it.
	entry, err := DeserializeUtxoEntry(serializedUtxo)
	if err != nil {
		// Ensure any deserialization errors are returned as database
		// corruption errors.
		if IsDeserializeErr(err) {
			return nil, database.Error{
				ErrorCode: database.ErrCorruption,
				Description: fmt.Sprintf("corrupt utxo entry "+
					"for %v: %v", outpoint, err),
			}
		}

		return nil, err
	}

	return entry, nil
}

func DBFetchUtxoEntries(dbTx database.Tx, limit int) (map[wire.OutPoint]*UtxoEntry, error) {
	view := make(map[wire.OutPoint]*UtxoEntry, limit)
	utxoBucket := dbTx.Metadata().Bucket(UtxoSetBucketName)
	count := 0

	err := utxoBucket.ForEach(func(rawKey, serializedUtxo []byte) error {
		if count == limit {
			return nil
		}

		if rawKey == nil || serializedUtxo == nil {
			return nil
		}

		// A non-nil zero-length entry means there is an entry in the database
		// for a spent transaction output which should never be the case.
		outpoint := keyToOutpoint(rawKey)
		if len(serializedUtxo) == 0 {
			return AssertError(fmt.Sprintf("database contains entry "+
				"for spent tx output %v", outpoint))
		}

		// Deserialize the utxo entry and return it.
		entry, err := DeserializeUtxoEntry(serializedUtxo)
		if err != nil {
			// Ensure any deserialization errors are returned as database
			// corruption errors.
			if IsDeserializeErr(err) {
				return database.Error{
					ErrorCode:   database.ErrCorruption,
					Description: fmt.Sprintf("corrupt utxo entry for %v: %v", outpoint, err),
				}
			}
			return err
		}

		view[outpoint] = entry
		count++
		return nil
	})

	return view, err
}

// DBPutUtxoView uses an existing database transaction to update the utxo set
// in the database based on the provided utxo view contents and state.  In
// particular, only the entries that have been marked as modified are written
// to the database.
func DBPutUtxoView(dbTx database.Tx, view *UtxoViewpoint) error {
	utxoBucket := dbTx.Metadata().Bucket(UtxoSetBucketName)
	for outpoint, entry := range view.entries {
		// No need to update the database if the entry was not modified.
		if entry == nil || !entry.isModified() {
			continue
		}

		// Remove the utxo entry if it is spent.
		if entry.IsSpent() {
			key := outpointKey(outpoint)
			err := utxoBucket.Delete(*key)
			recycleOutpointKey(key)
			if err != nil {
				return err
			}

			continue
		}

		// Serialize and store the utxo entry.
		serialized, err := serializeUtxoEntry(entry)
		if err != nil {
			return err
		}
		key := outpointKey(outpoint)
		err = utxoBucket.Put(*key, serialized)
		// NOTE: The key is intentionally not recycled here since the
		// database interface contract prohibits modifications.  It will
		// be garbage collected normally when the database is done with
		// it.
		if err != nil {
			return err
		}
	}

	return nil
}

// DBPutEADAddresses ...
func DBPutEADAddresses(dbTx database.Tx, updateSet map[string]*wire.EADAddresses) error {
	bucket := dbTx.Metadata().Bucket(EADAddressesBucketNameV2)
	for owner, entry := range updateSet {
		if entry == nil {
			if err := bucket.Delete([]byte(owner)); err != nil {
				return err
			}
			continue
		}

		// Serialize and store the ead address entry.
		w := bytes.NewBuffer(nil)
		err := entry.BtcEncode(w, wire.ProtocolVersion, wire.BaseEncoding)
		if err != nil {
			return err
		}

		err = bucket.Put([]byte(owner), w.Bytes())
		if err != nil {
			return err
		}
	}

	return nil
}

// DBFetchAllEADAddresses ...
func DBFetchAllEADAddresses(dbTx database.Tx) (map[string]*wire.EADAddresses, error) {
	view := map[string]*wire.EADAddresses{}
	utxoBucket := dbTx.Metadata().Bucket(EADAddressesBucketNameV2)
	if utxoBucket == nil {
		return view, nil
	}

	err := utxoBucket.ForEach(func(rawKey, serializedData []byte) error {
		if rawKey == nil || serializedData == nil {
			return nil
		}

		eadAddress := new(wire.EADAddresses)
		r := bytes.NewBuffer(serializedData)
		err := eadAddress.BtcDecode(r, wire.ProtocolVersion, wire.BaseEncoding)
		if err != nil {
			return database.Error{
				ErrorCode:   database.ErrCorruption,
				Description: fmt.Sprintf("corrupt ead addresses entry for %v: %v", string(rawKey), err),
			}
		}

		view[string(rawKey)] = eadAddress
		return nil
	})

	return view, err
}

// DBFetchEADAddresses ...
func DBFetchEADAddresses(dbTx database.Tx, ownerPK string) (*wire.EADAddresses, error) {
	bucket := dbTx.Metadata().Bucket(EADAddressesBucketNameV2)
	serializedData := bucket.Get([]byte(ownerPK))

	if serializedData == nil {
		return nil, nil
	}

	eadAddress := new(wire.EADAddresses)
	r := bytes.NewBuffer(serializedData)
	err := eadAddress.BtcDecode(r, wire.ProtocolVersion, wire.BaseEncoding)
	if err != nil {
		return nil, database.Error{
			ErrorCode:   database.ErrCorruption,
			Description: fmt.Sprintf("corrupt ead addresses entry for %v: %v", ownerPK, err),
		}
	}

	return eadAddress, err
}

// -----------------------------------------------------------------------------
// The block index consists of two buckets with an entry for every block in the
// main chain.  One bucket is for the Hash to height mapping and the other is
// for the height to Hash mapping.
//
// The serialized format for values in the Hash to height bucket is:
//   <height>
//
//   Field      Type     Size
//   height     uint32   4 bytes
//
// The serialized format for values in the height to Hash bucket is:
//   <Hash>
//
//   Field      Type             Size
//   Hash       chainhash.Hash   chainhash.HashSize
// -----------------------------------------------------------------------------

// DBPutBlockIndex uses an existing database transaction to update or add the
// block index entries for the Hash to height and height to Hash mappings for
// the provided values.
func DBPutBlockIndex(dbTx database.Tx, hash *chainhash.Hash, height int32) error {
	// Serialize the height for use in the index entries.
	var serializedHeight [4]byte
	byteOrder.PutUint32(serializedHeight[:], uint32(height))

	// Add the block Hash to height mapping to the index.
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(HashIndexBucketName)
	if err := hashIndex.Put(hash[:], serializedHeight[:]); err != nil {
		return err
	}

	// Add the block height to Hash mapping to the index.
	heightIndex := meta.Bucket(HeightIndexBucketName)
	return heightIndex.Put(serializedHeight[:], hash[:])
}

// DBRemoveBlockIndex uses an existing database transaction remove block index
// entries from the Hash to height and height to Hash mappings for the provided
// values.
func DBRemoveBlockIndex(dbTx database.Tx, hash *chainhash.Hash, height int32) error {
	// Remove the block Hash to height mapping.
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(HashIndexBucketName)
	if err := hashIndex.Delete(hash[:]); err != nil {
		return err
	}

	// Remove the block height to Hash mapping.
	var serializedHeight [4]byte
	byteOrder.PutUint32(serializedHeight[:], uint32(height))
	heightIndex := meta.Bucket(HeightIndexBucketName)
	return heightIndex.Delete(serializedHeight[:])
}

// DBFetchHeightByHash uses an existing database transaction to retrieve the
// height for the provided Hash from the index.
func DBFetchHeightByHash(dbTx database.Tx, hash *chainhash.Hash) (int32, error) {
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(HashIndexBucketName)
	serializedHeight := hashIndex.Get(hash[:])
	if serializedHeight == nil {
		str := fmt.Sprintf("block %s is not in the main chain", hash)
		return 0, ErrNotInMainChain(str)
	}

	return int32(byteOrder.Uint32(serializedHeight)), nil
}

// -----------------------------------------------------------------------------
// The best chain state consists of the best block hash and height, the total
// number of transactions up to and including those in the best block, and the
// accumulated work sum up to and including the best block.
//
// The serialized format is:
//
//   <block hash><block height><total txns><work sum length><work sum>
//
//   Field             Type             Size
//   block hash        chainhash.Hash   chainhash.HashSize
//   mmr root hash     chainhash.Hash   chainhash.HashSize
//   block height      uint32           4 bytes
//   last serial id    uint32           8 bytes
//   total txns        uint64           8 bytes
//   work sum length   uint32           4 bytes
//   work sum          big.Int          work sum length
// -----------------------------------------------------------------------------

// BestChainState represents the data to be stored the database for the current
// best chain state.
type BestChainState struct {
	Hash         chainhash.Hash
	MMRRoot      chainhash.Hash
	height       uint32
	LastSerialID int64
	TotalTxns    uint64
	workSum      *big.Int
}

// serializeBestChainState returns the serialization of the passed block best
// chain state.  This is data to be stored in the chain state bucket.
func serializeBestChainState(state BestChainState) []byte {
	// Calculate the full size needed to serialize the chain state.
	workSumBytes := state.workSum.Bytes()
	workSumBytesLen := uint32(len(workSumBytes))
	serializedLen := chainhash.HashSize + chainhash.HashSize + 4 + 8 + 8 + 4 + workSumBytesLen

	// Serialize the chain state.
	serializedData := make([]byte, serializedLen)

	copy(serializedData[0:chainhash.HashSize], state.Hash[:])
	offset := uint32(chainhash.HashSize)

	copy(serializedData[offset:offset+chainhash.HashSize], state.MMRRoot[:])
	offset += uint32(chainhash.HashSize)

	byteOrder.PutUint32(serializedData[offset:], state.height)
	offset += 4

	byteOrder.PutUint64(serializedData[offset:], uint64(state.LastSerialID))
	offset += 8

	byteOrder.PutUint64(serializedData[offset:], state.TotalTxns)
	offset += 8

	byteOrder.PutUint32(serializedData[offset:], workSumBytesLen)
	offset += 4
	copy(serializedData[offset:], workSumBytes)
	return serializedData
}

// DeserializeBestChainState deserializes the passed serialized best chain
// state.  This is data stored in the chain state bucket and is updated after
// every block is connected or disconnected form the main chain.
// block.
func DeserializeBestChainState(serializedData []byte) (BestChainState, error) {
	// Ensure the serialized data has enough bytes to properly deserialize
	// the Hash, height, total transactions, and work sum length.
	if len(serializedData) < chainhash.HashSize+16 {
		return BestChainState{}, database.Error{
			ErrorCode:   database.ErrCorruption,
			Description: "corrupt best chain state",
		}
	}

	state := BestChainState{}
	copy(state.Hash[:], serializedData[0:chainhash.HashSize])
	offset := uint32(chainhash.HashSize)
	copy(state.MMRRoot[:], serializedData[offset:offset+chainhash.HashSize])
	offset += uint32(chainhash.HashSize)
	state.height = byteOrder.Uint32(serializedData[offset : offset+4])
	offset += 4
	state.LastSerialID = int64(byteOrder.Uint64(serializedData[offset : offset+8]))
	offset += 8
	state.TotalTxns = byteOrder.Uint64(serializedData[offset : offset+8])
	offset += 8
	workSumBytesLen := byteOrder.Uint32(serializedData[offset : offset+4])
	offset += 4

	// Ensure the serialized data has enough bytes to deserialize the work
	// sum.
	if uint32(len(serializedData[offset:])) < workSumBytesLen {
		return BestChainState{}, database.Error{
			ErrorCode:   database.ErrCorruption,
			Description: "corrupt best chain state",
		}
	}
	workSumBytes := serializedData[offset : offset+workSumBytesLen]
	state.workSum = new(big.Int).SetBytes(workSumBytes)

	return state, nil
}

// DBPutBestState uses an existing database transaction to update the best chain
// state with the given parameters.
func DBPutBestState(dbTx database.Tx, snapshot *BestState, workSum *big.Int) error {
	// Serialize the current best chain state.
	serializedData := serializeBestChainState(BestChainState{
		Hash:         snapshot.Hash,
		MMRRoot:      snapshot.CurrentMMRRoot,
		height:       uint32(snapshot.Height),
		LastSerialID: snapshot.LastSerialID,
		TotalTxns:    snapshot.TotalTxns,
		workSum:      workSum,
	})

	// Store the current best chain state into the database.
	return dbTx.Metadata().Put(ChainStateKeyName, serializedData)
}

// DeserializeBlockRow parses a value in the block index bucket into a block
// header and block status bitfield.
func DeserializeBlockRow(blockRow []byte) (wire.BlockHeader, blocknodes.BlockStatus, int64, error) {
	buffer := bytes.NewReader(blockRow)

	header, err := wire.DecodeHeader(buffer)
	if err != nil {
		return nil, blocknodes.StatusNone, 0, err
	}

	statusByte, err := buffer.ReadByte()
	if err != nil {
		return nil, blocknodes.StatusNone, 0, err
	}

	dest := make([]byte, 8)
	_, err = buffer.Read(dest)
	if err != nil {
		return nil, blocknodes.StatusNone, 0, err
	}

	blockSerialID := byteOrder.Uint64(dest)

	return header, blocknodes.BlockStatus(statusByte), int64(blockSerialID), nil
}

// DBFetchBlockByNode uses an existing database transaction to retrieve the
// raw block for the provided node, deserialize it, and return a jaxutil.Block
// with the height set.
func DBFetchBlockByNode(dbTx database.Tx, node blocknodes.IBlockNode) (*jaxutil.Block, error) {
	// Load the raw block bytes from the database.
	h := node.GetHash()
	blockBytes, err := dbTx.FetchBlock(&h)
	if err != nil {
		return nil, err
	}

	// Create the encapsulated block and set the height appropriately.
	block, err := jaxutil.NewBlockFromBytes(blockBytes)
	if err != nil {
		return nil, err
	}

	return block, nil
}

// DBStoreBlockNode stores the block header and validation status to the block
// index bucket. This overwrites the current entry if there exists one.
func DBStoreBlockNode(dbTx database.Tx, node blocknodes.IBlockNode) error {
	// Serialize block data to be stored.

	w := bytes.NewBuffer(make([]byte, 0, dbTx.Chain().MaxBlockHeaderPayload()+1))
	header := node.Header()
	err := header.Write(w)
	if err != nil {
		return err
	}
	err = w.WriteByte(byte(node.Status()))
	if err != nil {
		return err
	}
	sidData := make([]byte, 8)
	byteOrder.PutUint64(sidData, uint64(node.SerialID()))
	_, err = w.Write(sidData)
	if err != nil {
		return err
	}
	value := w.Bytes()

	// Write block header data to block index bucket.
	blockHash := node.GetHash()
	key := blockIndexKey(&blockHash, uint32(node.Height()))

	blockIndexBucket := dbTx.Metadata().Bucket(BlockIndexBucketName)
	return blockIndexBucket.Put(key, value)
}

// DBStoreBlock stores the provided block in the database if it is not already
// there. The full block data is written to ffldb.
func DBStoreBlock(dbTx database.Tx, block *jaxutil.Block) error {
	hasBlock, err := dbTx.HasBlock(block.Hash())
	if err != nil {
		return err
	}
	if hasBlock {
		return nil
	}
	return dbTx.StoreBlock(block)
}

// blockIndexKey generates the binary key for an entry in the block index
// bucket. The key is composed of the block height encoded as a big-endian
// 32-bit unsigned int followed by the 32 byte block Hash.
func blockIndexKey(blockHash *chainhash.Hash, blockHeight uint32) []byte {
	indexKey := make([]byte, chainhash.HashSize+4)
	binary.BigEndian.PutUint32(indexKey[0:4], blockHeight)
	copy(indexKey[4:chainhash.HashSize+4], blockHash[:])
	return indexKey
}

func DBPutMMRRoot(dbTx database.Tx, mmrRoot, blockHash chainhash.Hash) error {
	blockIndexBucket := dbTx.Metadata().Bucket(MMRRootsToHashBucketName)
	err := blockIndexBucket.Put(mmrRoot[:], blockHash[:])
	if err != nil {
		return err
	}

	bucket := dbTx.Metadata().Bucket(HashToMMRRootBucketName)
	return bucket.Put(blockHash[:], mmrRoot[:])
}

func DBGetBlocksMMRRoots(dbTx database.Tx) (map[chainhash.Hash]chainhash.Hash, error) {
	bucket := dbTx.Metadata().Bucket(HashToMMRRootBucketName)
	res := make(map[chainhash.Hash]chainhash.Hash, 4096)
	err := bucket.ForEach(func(key, val []byte) error {
		var keyHash chainhash.Hash
		if len(key) != chainhash.HashSize {
			return fmt.Errorf("invalid hash length of %v, want %v", len(key), chainhash.HashSize)
		}
		copy(keyHash[:], key)

		var value chainhash.Hash
		if len(val) != chainhash.HashSize {
			return fmt.Errorf("invalid hash length of %v, want %v", len(val), chainhash.HashSize)
		}
		copy(value[:], val)

		res[keyHash] = value
		return nil
	})

	return res, err
}

func DBGetMMRRootForBlock(dbTx database.Tx, blockHash *chainhash.Hash) (chainhash.Hash, error) {
	bucket := dbTx.Metadata().Bucket(HashToMMRRootBucketName)
	val := bucket.Get(blockHash[:])
	if len(val) != chainhash.HashSize {
		return chainhash.ZeroHash, errors.New("actual root not found for block " + blockHash.String())
	}
	root := chainhash.Hash{}
	copy(root[:], val)

	return root, nil
}

type ShardInfo struct {
	// ID of the shard chain.
	ID uint32
	// ExpansionHeight chain height of beacon block
	// when expansion has happened for this shard.
	ExpansionHeight int32
	// ExpansionHash hash of beacon block
	// when expansion has happened for this shard.
	ExpansionHash chainhash.Hash
	// ExpansionHeight serial id of beacon block
	// when expansion has happened for this shard.
	SerialID int64
}

func DBStoreShardGenesisInfo(dbTx database.Tx, shardID uint32, blockHeight int32, blockHash *chainhash.Hash, blockSerialID int64) error {
	bucket := dbTx.Metadata().Bucket(ShardCreationsBucketName)
	val := make([]byte, chainhash.HashSize+4+8)
	copy(val[:chainhash.HashSize], blockHash[:])
	binary.LittleEndian.PutUint32(val[chainhash.HashSize:chainhash.HashSize+4], uint32(blockHeight))
	binary.LittleEndian.PutUint64(val[chainhash.HashSize+4:chainhash.HashSize+4+8], uint64(blockSerialID))

	key := make([]byte, 4)
	binary.LittleEndian.PutUint32(key, shardID)

	valueAtKey := bucket.Get(key)
	if valueAtKey != nil {
		return fmt.Errorf("shard with specified ID %d already exists", shardID)
	}

	return bucket.Put(key, val)
}

func DBStoreLastShardInfo(dbTx database.Tx, shardID uint32) error {
	bucket := dbTx.Metadata().Bucket(ShardCreationsBucketName)
	val := make([]byte, 4)
	binary.LittleEndian.PutUint32(val, shardID)

	return bucket.Put([]byte("last_shard_id"), val)
}

func DBGetShardGenesisInfo(dbTx database.Tx) (map[uint32]ShardInfo, uint32) {
	bucket := dbTx.Metadata().Bucket(ShardCreationsBucketName)
	lastShardIDByte := bucket.Get([]byte("last_shard_id"))
	if len(lastShardIDByte) == 0 {
		return nil, 0
	}
	lastShardID := binary.LittleEndian.Uint32(lastShardIDByte)

	shards := make(map[uint32]ShardInfo, lastShardID)

	for i := uint32(1); i <= lastShardID; i++ {
		key := make([]byte, 4)
		binary.LittleEndian.PutUint32(key, i)
		data := bucket.Get(key)

		hash, blockHeight, serialID := decodeShardGenesisInfo(data)
		shards[i] = ShardInfo{
			ID:              i,
			ExpansionHeight: blockHeight,
			ExpansionHash:   hash,
			SerialID:        serialID,
		}
	}

	return shards, lastShardID
}

func decodeShardGenesisInfo(data []byte) (chainhash.Hash, int32, int64) {
	var hash [32]byte
	copy(hash[:], data[:chainhash.HashSize])

	blockHeightBytes := make([]byte, 4)
	copy(blockHeightBytes, data[chainhash.HashSize:chainhash.HashSize+4])
	blockHeight := binary.LittleEndian.Uint32(blockHeightBytes)

	serialIDBytes := make([]byte, 8)
	copy(blockHeightBytes, data[chainhash.HashSize+4:chainhash.HashSize+4+8])
	serialID := binary.LittleEndian.Uint64(serialIDBytes)

	return hash, int32(blockHeight), int64(serialID)
}
