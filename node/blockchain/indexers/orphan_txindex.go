/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package indexers

import (
	"fmt"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

const (
	// orphanTxIndexName is the human-readable name for the index.
	orphanTxIndexName = "orphan transaction index"
)

var (
	// txIndexKey is the key of the transaction index and the db bucket used
	// to house it.
	orphanTxIndexKey = []byte("orphantxbyhashidx")
	// idByHashOrphanTxIndexBucketName is the name of the db bucket used to house
	// the block id -> block hash index.
	idByHashOrphanTxIndexBucketName = []byte("idbyhashorphantxidx")

	// hashByIDOrphanTxIndexBucketName is the name of the db bucket used to house
	// the block hash -> block id index.
	hashByIDOrphanTxIndexBucketName = []byte("hashbyidorphantxidx")
)

// OrphanTxIndex implements an orphan transaction by hash index. That is to say, it supports
// querying all orphan transactions by their hash.
type OrphanTxIndex struct {
	db         database.DB
	curBlockID uint32
}

// NewOrphanTxIndex returns a new instance of an indexer that is used to create a
// mapping of the hashes of all transactions in the blockchain to the respective
// block, location within the block, and size of the transaction.
//
// It implements the Indexer interface which plugs into the IndexManager that in
// turn is used by the blockchain package.  This allows the index to be
// seamlessly maintained along with the chain.
func NewOrphanTxIndex(db database.DB) *OrphanTxIndex {
	return &OrphanTxIndex{db: db}
}

// Ensure the OrphanTxIndex type implements the Indexer interface.
var _ Indexer = (*OrphanTxIndex)(nil)

// Key returns the database key to use for the index as a byte slice.
//
// This is part of the Indexer interface.
func (idx *OrphanTxIndex) Key() []byte {
	return orphanTxIndexKey
}

func (idx *OrphanTxIndex) Name() string {
	return orphanTxIndexName
}

// Create is invoked when the indexer manager determines the index needs
// to be created for the first time.  It creates the buckets for the hash-based
// transaction index and the internal block ID indexes.
//
// This is part of the Indexer interface.
func (idx *OrphanTxIndex) Create(dbTx database.Tx) error {
	meta := dbTx.Metadata()
	if _, err := meta.CreateBucketIfNotExists(idByHashOrphanTxIndexBucketName); err != nil {
		return err
	}
	if _, err := meta.CreateBucketIfNotExists(hashByIDOrphanTxIndexBucketName); err != nil {
		return err
	}
	_, err := meta.CreateBucketIfNotExists(orphanTxIndexKey)
	return err
}

// Init initializes the hash-based transaction index.  In particular, it finds
// the highest used block ID and stores it for later use when connecting or
// disconnecting blocks.
//
// This is part of the Indexer interface.orphanTxIndexName
func (idx *OrphanTxIndex) Init() error {
	// Find the latest known block id field for the internal block id
	// index and initialize it.  This is done because it's a lot more
	// efficient to do a single search at initialize time than it is to
	// write another value to the database on every update.
	err := idx.db.View(func(dbTx database.Tx) error {
		// Scan forward in large gaps to find a block id that doesn't
		// exist yet to serve as an upper bound for the binary search
		// below.
		var highestKnown, nextUnknown uint32
		testBlockID := uint32(1)
		increment := uint32(100000)
		for {
			_, err := dbFetchOrphanTxBlockHashByID(dbTx, testBlockID)
			if err != nil {
				nextUnknown = testBlockID
				break
			}

			highestKnown = testBlockID
			testBlockID += increment
		}
		log.Trace().Msgf("Forward scan (highest known %d, next unknown %d)", highestKnown, nextUnknown)

		// No used block IDs due to new database.
		if nextUnknown == 1 {
			return nil
		}

		// Use a binary search to find the final highest used block id.
		// This will take at most ceil(log_2(increment)) attempts.
		for {
			testBlockID = (highestKnown + nextUnknown) / 2
			_, err := dbFetchOrphanTxBlockHashByID(dbTx, testBlockID)
			if err != nil {
				nextUnknown = testBlockID
			} else {
				highestKnown = testBlockID
			}
			log.Trace().Msgf("Binary scan (highest known %d, next unknown %d)", highestKnown, nextUnknown)
			if highestKnown+1 == nextUnknown {
				break
			}
		}

		idx.curBlockID = highestKnown
		return nil
	})
	if err != nil {
		return err
	}

	log.Debug().Msgf("Current internal block ID: %d", idx.curBlockID)
	return nil
}

func (idx *OrphanTxIndex) ConnectBlock(database.Tx, *jaxutil.Block, chainhash.Hash, []chaindata.SpentTxOut) error {
	return nil
}

func (idx *OrphanTxIndex) DisconnectBlock(dbTx database.Tx, block *jaxutil.Block, _ []chaindata.SpentTxOut) error {
	// Increment the internal block ID to use for the block being connected
	// and add all of the transactions in the block to the index.
	newBlockID := idx.curBlockID + 1
	if err := dbAddOrphanTxIndexEntries(dbTx, block, newBlockID); err != nil {
		return err
	}

	// Add the new block ID index entry for the block being connected and
	// update the current internal block ID accordingly.
	err := dbPutBlockIDOrphanTxIndexEntry(dbTx, block.Hash(), newBlockID)
	if err != nil {
		return err
	}
	idx.curBlockID = newBlockID
	return nil

}

func (idx *OrphanTxIndex) TxBlockRegion(hash *chainhash.Hash) (*database.BlockRegion, error) {
	var region *database.BlockRegion
	err := idx.db.View(func(dbTx database.Tx) error {
		var err error
		region, err = dbFetchOrphanTxIndexEntry(dbTx, hash)
		return err
	})
	return region, err
}

// dbAddOrphanTxIndexEntries uses an existing database transaction to add a
// transaction index entry for every transaction in the passed block.
func dbAddOrphanTxIndexEntries(dbTx database.Tx, block *jaxutil.Block, blockID uint32) error {
	// The offset and length of the transactions within the serialized
	// block.
	txLocs, err := block.TxLoc()
	if err != nil {
		return err
	}

	// As an optimization, allocate a single slice big enough to hold all
	// of the serialized transaction index entries for the block and
	// serialize them directly into the slice.  Then, pass the appropriate
	// subslice to the database to be written.  This approach significantly
	// cuts down on the number of required allocations.
	offset := 0
	serializedValues := make([]byte, len(block.Transactions())*txEntrySize)
	for i, tx := range block.Transactions() {
		target := serializedValues[offset:]
		byteOrder.PutUint32(target, blockID)
		byteOrder.PutUint32(target[4:], uint32(txLocs[i].TxStart))
		byteOrder.PutUint32(target[8:], uint32(txLocs[i].TxLen))

		endOffset := offset + txEntrySize
		txIndex := dbTx.Metadata().Bucket(orphanTxIndexKey)
		err := txIndex.Put(tx.Hash()[:], serializedValues[offset:endOffset:endOffset])
		if err != nil {
			return err
		}
		offset += txEntrySize
	}

	return nil
}

// dbPutBlockIDOrphanTxIndexEntry uses an existing database transaction to update or add
// the index entries for the hash to id and id to hash mappings for the provided
// values.
func dbPutBlockIDOrphanTxIndexEntry(dbTx database.Tx, hash *chainhash.Hash, id uint32) error {
	// Serialize the height for use in the index entries.
	var serializedID [4]byte
	byteOrder.PutUint32(serializedID[:], id)

	// Add the block hash to ID mapping to the index.
	meta := dbTx.Metadata()
	hashIndex := meta.Bucket(idByHashOrphanTxIndexBucketName)
	if err := hashIndex.Put(hash[:], serializedID[:]); err != nil {
		return err
	}

	// Add the block ID to hash mapping to the index.
	idIndex := meta.Bucket(hashByIDOrphanTxIndexBucketName)
	return idIndex.Put(serializedID[:], hash[:])
}

// dbFetchOrphanTxBlockHashByID uses an existing database transaction to retrieve the
// hash for the provided block id from the index.
func dbFetchOrphanTxBlockHashByID(dbTx database.Tx, id uint32) (*chainhash.Hash, error) {
	var serializedID [4]byte
	byteOrder.PutUint32(serializedID[:], id)
	return dbFetchOrphanTxBlockHashBySerializedID(dbTx, serializedID[:])
}

// dbFetchBlockHashBySerializedID uses an existing database transaction to
// retrieve the hash for the provided serialized block id from the index.
func dbFetchOrphanTxBlockHashBySerializedID(dbTx database.Tx, serializedID []byte) (*chainhash.Hash, error) {
	idIndex := dbTx.Metadata().Bucket(hashByIDOrphanTxIndexBucketName)
	hashBytes := idIndex.Get(serializedID)
	if hashBytes == nil {
		return nil, errNoBlockIDEntry
	}

	var hash chainhash.Hash
	copy(hash[:], hashBytes)
	return &hash, nil
}

// dbFetchTxIndexEntry uses an existing database transaction to fetch the block
// region for the provided transaction hash from the transaction index.  When
// there is no entry for the provided hash, nil will be returned for the both
// the region and the error.
func dbFetchOrphanTxIndexEntry(dbTx database.Tx, txHash *chainhash.Hash) (*database.BlockRegion, error) {
	// Load the record from the database and return now if it doesn't exist.
	txIndex := dbTx.Metadata().Bucket(orphanTxIndexKey)
	serializedData := txIndex.Get(txHash[:])
	if len(serializedData) == 0 {
		return nil, nil
	}

	// Ensure the serialized data has enough bytes to properly deserialize.
	if len(serializedData) < 12 {
		return nil, database.Error{
			ErrorCode:   database.ErrCorruption,
			Description: fmt.Sprintf("corrupt transaction index entry for %s", txHash),
		}
	}

	// Load the block hash associated with the block ID.
	hash, err := dbFetchOrphanTxBlockHashBySerializedID(dbTx, serializedData[0:4])
	if err != nil {
		return nil, database.Error{
			ErrorCode:   database.ErrCorruption,
			Description: fmt.Sprintf("corrupt transaction index entry for %s: %v", txHash, err),
		}
	}

	// Deserialize the final entry.
	region := database.BlockRegion{Hash: &chainhash.Hash{}}
	copy(region.Hash[:], hash[:])
	region.Offset = byteOrder.Uint32(serializedData[4:8])
	region.Len = byteOrder.Uint32(serializedData[8:12])

	return &region, nil
}

// DropOrphanTxIndex drops the transaction index from the provided database if it
// exists.  Since the address index relies on it, the address index will also be
// dropped when it exists.
func DropOrphanTxIndex(db database.DB, interrupt <-chan struct{}) error {
	return dropIndex(db, orphanTxIndexKey, orphanTxIndexName, interrupt)
}
