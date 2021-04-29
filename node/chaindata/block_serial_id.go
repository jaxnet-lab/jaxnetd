/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

// Package chaindata:  Functions related to Block Serial ID feature.
//
// BlockLastSerialID is the name of the db bucket used to house the
// block last serial id.
//
// BlockHashSerialID is the name of the db bucket used to house the mapping of
// block hash to serial id.
// BlockSerialIDHashPrevSerialID is the name of the db bucket used to house the mapping of
// block serial id to hash and previous serial id.
//
//  | bucket                         | Key        | Value           |
//  | ------------------------------ | ---------- | --------------- |
//  | BlockSerialIDHashPrevSerialID  | serialID   | {hash; prev_id} |
//  | BlockHashSerialID              | block_hash | serialID        |
//  | BlockLastSerialID              | BlockLastSerialID | lastSerialID |
//
package chaindata

import (
	"encoding/binary"
	"encoding/json"
	"errors"

	"gitlab.com/jaxnet/core/shard.core/database"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
)

type SerialValue struct {
	Hash   *chainhash.Hash `json:"hash"`
	PrevID int64           `json:"prev_id"`
}

func DBFetchLastSerialID(dbTx database.Tx) (int64, error) {
	meta := dbTx.Metadata()
	lastSerialIDBucket := meta.Bucket(BlockLastSerialID)
	res := lastSerialIDBucket.Get(BlockLastSerialID)

	if res == nil {
		return -1, errors.New("chain last serial id is nil")
	}

	if len(res) < 8 {
		return -1, errors.New("chain last serial id is empty or invalid")
	}

	lastSerialID := binary.LittleEndian.Uint64(res)

	return int64(lastSerialID), nil
}

func DBPutLastSerialID(dbTx database.Tx, lastSerialID int64) error {
	meta := dbTx.Metadata()
	lastSerialIDBucket := meta.Bucket(BlockLastSerialID)
	return lastSerialIDBucket.Put(BlockLastSerialID, i64ToBytes(lastSerialID))
}

func DBFetchBlockHashBySerialID(dbTx database.Tx, serialID int64) (*chainhash.Hash, int64, error) {
	meta := dbTx.Metadata()
	blockSerialIDHashPrevSerialID := meta.Bucket(BlockSerialIDHashPrevSerialID)
	res := blockSerialIDHashPrevSerialID.Get(i64ToBytes(serialID))
	if res == nil {
		return nil, 0, errors.New("chain serial id does not exist")
	}

	if len(res) < 0 {
		return nil, 0, errors.New("chain serial id is empty")
	}

	value := &SerialValue{}
	err := json.Unmarshal(res, value)
	if err != nil {
		return nil, 0, err
	}
	if value.Hash == nil {
		return nil, 0, errors.New("hash is nil")
	}
	return value.Hash, value.PrevID, nil
}

func DBFetchBlockSerialID(dbTx database.Tx, hash *chainhash.Hash) (int64, int64, error) {
	meta := dbTx.Metadata()
	blockSerialIDBucket := meta.Bucket(BlockHashSerialID)
	res := blockSerialIDBucket.Get(hash[:])

	if res == nil {
		return -1, -1, nil
	}

	if len(res) < 8 {
		return -1, -1, errors.New("chain last serial id is empty or invalid")
	}

	id := bytesToI64(res)
	_, prevID, err := DBFetchBlockHashBySerialID(dbTx, id)
	return id, prevID, err
}

func DBPutBlockHashSerialID(dbTx database.Tx, hash *chainhash.Hash, serialID int64) error {
	meta := dbTx.Metadata()
	blockSerialIDBucket := meta.Bucket(BlockHashSerialID)

	return blockSerialIDBucket.Put(hash[:], i64ToBytes(serialID))
}

func DBPutBlockSerialIDHash(dbTx database.Tx, hash *chainhash.Hash, serialID int64) error {
	meta := dbTx.Metadata()
	blockSerialIDBucket := meta.Bucket(BlockHashSerialID)

	return blockSerialIDBucket.Put(i64ToBytes(serialID), hash[:])
}

func DBPutBlockSerialIDHashPrevSerialID(dbTx database.Tx, hash *chainhash.Hash, serialID, lastSerialID int64) error {
	meta := dbTx.Metadata()
	blockSerialIDHashPrevSerialID := meta.Bucket(BlockSerialIDHashPrevSerialID)
	value := &SerialValue{
		hash,
		lastSerialID,
	}

	valueBytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return blockSerialIDHashPrevSerialID.Put(i64ToBytes(serialID), valueBytes)
}
