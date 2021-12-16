/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

// Package chaindata:  Functions related to Block Serial ID feature.
//
// BlockHashToSerialID is the name of the db bucket used to house the mapping of
// block hash to serial id.
// SerialIDToPrevBlock is the name of the db bucket used to house the mapping of
// block serial id to hash and previous serial id.
//
//  | bucket                         | Key        | Value           |
//  | ------------------------------ | ---------- | --------------- |
//  | SerialIDToPrevBlock            | serialID   | {hash; prev_id} |
//  | BlockHashToSerialID            | block_hash | serialID        |
//
package chaindata

import (
	"encoding/binary"
	"errors"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

type SerialIDBlockMeta struct {
	SerialID     int64
	Hash         chainhash.Hash
	PrevSerialID int64
}

func DBFetchAllBlocksHashBySerialID(dbTx database.Tx, serialID int64, onlyOrphan bool) ([]SerialIDBlockMeta, error) {
	meta := dbTx.Metadata()
	blockSerialIDHashPrevSerialID := meta.Bucket(SerialIDToPrevBlock)
	// nolint: gomnd
	dataList := make([]SerialIDBlockMeta, 0, 256)

	for id := serialID; ; id++ {
		res := blockSerialIDHashPrevSerialID.Get(i64ToBytes(id))
		if len(res) < chainhash.HashSize+8 {
			break
		}

		var hash chainhash.Hash
		copy(hash[:], res[:chainhash.HashSize])

		sid := make([]byte, 8)
		copy(sid, res[chainhash.HashSize:])

		prevSerialID := bytesToI64(sid)
		if onlyOrphan && id == prevSerialID+1 {
			continue
		}

		dataList = append(dataList, SerialIDBlockMeta{
			SerialID:     id,
			Hash:         hash,
			PrevSerialID: prevSerialID,
		})
	}

	return dataList, nil
}

func DBFetchBlockHashBySerialID(dbTx database.Tx, serialID int64) (*chainhash.Hash, int64, error) {
	meta := dbTx.Metadata()
	blockSerialIDHashPrevSerialID := meta.Bucket(SerialIDToPrevBlock)
	res := blockSerialIDHashPrevSerialID.Get(i64ToBytes(serialID))
	if len(res) < chainhash.HashSize+8 {
		return nil, 0, errors.New("chain serial id is empty or invalid")
	}

	var hash chainhash.Hash
	copy(hash[:], res[:chainhash.HashSize])

	sid := make([]byte, 8)
	copy(sid, res[chainhash.HashSize:])

	return &hash, bytesToI64(sid), nil
}

func DBFetchBlockSerialID(dbTx database.Tx, hash *chainhash.Hash) (int64, int64, error) {
	meta := dbTx.Metadata()
	blockSerialIDBucket := meta.Bucket(BlockHashToSerialID)
	res := blockSerialIDBucket.Get(hash[:])
	if len(res) < 8 {
		return -1, -1, errors.New("chain last serial id is empty or invalid")
	}

	id := bytesToI64(res)
	_, prevID, err := DBFetchBlockHashBySerialID(dbTx, id)
	return id, prevID, err
}

func DBPutBlockHashToSerialID(dbTx database.Tx, hash chainhash.Hash, serialID int64) error {
	meta := dbTx.Metadata()
	blockSerialIDBucket := meta.Bucket(BlockHashToSerialID)

	return blockSerialIDBucket.Put(hash[:], i64ToBytes(serialID))
}

// DBPutHashToSerialIDWithPrev stores block hash with corresponding serialID and serialID of prev_block.
//  | bucket                         | Key        | Value           |
//  | ------------------------------ | ---------- | --------------- |
//  | SerialIDToPrevBlock            | serialID   | {hash; prev_block_id} |
func DBPutHashToSerialIDWithPrev(dbTx database.Tx, hash chainhash.Hash, serialID, prevSerialID int64) error {
	err := DBPutBlockHashToSerialID(dbTx, hash, serialID)
	if err != nil {
		return err
	}

	meta := dbTx.Metadata()
	blockSerialIDHashPrevSerialID := meta.Bucket(SerialIDToPrevBlock)

	buf := make([]byte, chainhash.HashSize+8)
	copy(buf[:chainhash.HashSize], hash[:])
	copy(buf[chainhash.HashSize:], i64ToBytes(prevSerialID))

	return blockSerialIDHashPrevSerialID.Put(i64ToBytes(serialID), buf)
}

const serialIDByteSize = 8

func DBPutSerialIDsList(dbTx database.Tx, serialIDs []int64) error {
	meta := dbTx.Metadata()
	bucket, err := meta.GetOrCreateBucket(BestChainSerialIDsBucketName)
	if err != nil {
		return err
	}

	count := len(serialIDs)
	buf := make([]byte, serialIDByteSize*(count+1)+1)

	var startOffset int64

	copy(buf[:startOffset+serialIDByteSize], i64ToBytes(int64(count)))
	startOffset += serialIDByteSize

	for i := range serialIDs {
		copy(buf[startOffset:startOffset+serialIDByteSize], i64ToBytes(serialIDs[i]))
		startOffset += serialIDByteSize
	}

	return bucket.Put(BestChainSerialIDsBucketName, buf)
}

type BestChainBlockRecord struct {
	SerialID int64           `json:"serial_id"`
	Hash     *chainhash.Hash `json:"hash"`
}

func DBGetBestChainSerialIDs(dbTx database.Tx) ([]BestChainBlockRecord, error) {
	var res []BestChainBlockRecord
	meta := dbTx.Metadata()
	bucket := meta.Bucket(BestChainSerialIDsBucketName)

	var startOffset int64
	record := bucket.Get(BestChainSerialIDsBucketName)
	if len(record) < serialIDByteSize {
		return nil, nil
	}

	count := bytesToI64(record[:serialIDByteSize])
	startOffset += serialIDByteSize

	for i := int64(0); i < count; i++ {
		serialID := bytesToI64(record[startOffset : startOffset+serialIDByteSize])
		hash, _, err := DBFetchBlockHashBySerialID(dbTx, serialID)
		if err != nil {
			return nil, err
		}

		startOffset += serialIDByteSize
		res = append(res, BestChainBlockRecord{
			SerialID: serialID,
			Hash:     hash,
		})
	}

	return res, nil
}

func i64ToBytes(val int64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(val))
	return buf
}

func bytesToI64(val []byte) int64 {
	num := binary.LittleEndian.Uint64(val)
	return int64(num)
}
