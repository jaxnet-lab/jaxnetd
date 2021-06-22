// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mmr

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
)

var merkleMountainRangeBucket = chaindata.ShardsMMRBucketName

type nativeStorage struct {
	db        database.DB
	lastIndex int64
}

func Storage(db database.DB) IStore {
	return &nativeStorage{db: db, lastIndex: -1}

}

func (b *nativeStorage) getData(key keyType) (res *BlockData, err error) {
	err = b.db.View(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(merkleMountainRangeBucket)
		if bucket == nil {
			return errors.New("bucket not exist: " + string(merkleMountainRangeBucket))
		}
		data := bucket.Get(key)
		if data == nil {
			return errors.New("data not found: " + string(merkleMountainRangeBucket))
		}

		res = &BlockData{}
		if err := json.Unmarshal(data, res); err != nil {
			return err
		}
		return nil
	})

	return
}

func (b *nativeStorage) saveData(key keyType, value *BlockData) error {
	return b.db.Update(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(merkleMountainRangeBucket)
		if bucket == nil {
			return errors.New("bucket not exist: " + string(merkleMountainRangeBucket))
		}

		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return bucket.Put(key, data)
	})
}

func (b *nativeStorage) GetNode(index uint64) (res *BlockData, ok error) {
	return b.getData(getNodeIndexKey(index))
}

func (b *nativeStorage) SetNode(index uint64, data *BlockData) error {
	return b.saveData(getNodeIndexKey(index), data)
}

func (b *nativeStorage) GetBlock(index uint64) (res *BlockData, ok error) {
	return b.getData(getBlockIndexKey(index))
}

func (b *nativeStorage) SetBlock(index uint64, data *BlockData) error {
	return b.saveData(getBlockIndexKey(index), data)
}

func (b *nativeStorage) Nodes() (res []uint64, err error) {
	err = b.db.View(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(merkleMountainRangeBucket)
		if bucket == nil {
			return errors.New("bucket not exist: " + string(merkleMountainRangeBucket))
		}

		return bucket.ForEach(func(k, _ []byte) error {
			if k[0] == nodeKeyID {
				res = append(res, binary.LittleEndian.Uint64(k[1:9]))
			}
			return nil
		})
	})
	return
}

func (b *nativeStorage) Blocks() (res []uint64, err error) {
	err = b.db.View(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket(merkleMountainRangeBucket)
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, _ []byte) error {
			if k[0] == blockKeyID {
				res = append(res, binary.LittleEndian.Uint64(k[1:9]))
			}
			return nil
		})
	})
	return
}

func (b *nativeStorage) Debug() {
	fmt.Println("Blocks")
	blocks, err := b.Blocks()
	if err != nil {
		fmt.Println("unable to fetch blocks: ", err.Error())
		return
	}
	for _, id := range blocks {
		item, _ := b.GetBlock(id)
		fmt.Printf("\t%d. hash: %x %v\n", id, item.Hash, item.Weight)
	}

	fmt.Println()
	fmt.Println("Nodes")
	nodes, err := b.Nodes()
	if err != nil {
		fmt.Println("unable to fetch nodes: ", err.Error())
		return
	}
	for _, id := range nodes {
		item, _ := b.GetNode(id)
		fmt.Printf("\t%d. hash: %x %v\n", id, item.Hash, item.Weight)
	}
}
