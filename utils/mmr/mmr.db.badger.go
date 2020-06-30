package mmr

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	badger "github.com/dgraph-io/badger"
	"log"
)

type badgerDB struct {
	db *badger.DB
}

func BadgerDB(path string) (res *badgerDB, err error) {
	res = &badgerDB{}
	res.db, err = badger.Open(badger.DefaultOptions(path))
	if err != nil {
		log.Fatal(err)
	}
	return
}

func (b *badgerDB) Close() error {
	return b.db.Close()
}

type keyType []byte

func getObjectIndex(index uint64) (res keyType) {
	res = make([]byte, 9)
	res[0] = 0x01
	binary.LittleEndian.PutUint64(res[1:9], index)
	return
}

func getNodeIndex(index uint64) (res keyType) {
	res = make([]byte, 9)
	res[0] = 0x02
	binary.LittleEndian.PutUint64(res[1:9], index)
	return
}

func (b *badgerDB) getData(key keyType) (res *BlockData, ok bool) {
	trx := b.db.NewTransaction(false)
	defer func() {
		if ok {
			trx.Commit()
		} else {
			trx.Discard()
		}
	}()
	if item, err := trx.Get(key); err != nil {
		return nil, false
	} else {
		if data, err := item.ValueCopy(nil); err != nil {
			return nil, false
		} else {
			res = &BlockData{}
			if err = json.Unmarshal(data, res); err != nil {
				return nil, false
			}
			return res, true
		}
	}
}

func (b *badgerDB) saveData(key keyType, value *BlockData) {
	b.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
	return
}

func (b *badgerDB) GetNode(index uint64) (res *BlockData, ok bool) {
	return b.getData(getNodeIndex(index))
}

func (b *badgerDB) SetNode(index uint64, data *BlockData) {
	b.saveData(getNodeIndex(index), data)
}

func (b *badgerDB) GetBlock(index uint64) (res *BlockData, ok bool) {
	return b.getData(getObjectIndex(index))
}

func (b *badgerDB) SetBlock(index uint64, data *BlockData) {
	b.saveData(getObjectIndex(index), data)
}

func (b *badgerDB) Nodes() (res []uint64) {
	b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			if k[0] == 0x02 {
				res = append(res, binary.LittleEndian.Uint64(k[1:9]))
			}
		}
		return nil
	})
	return
}

func (b *badgerDB) Blocks() (res []uint64) {
	b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			if k[0] == 0x02 {
				res = append(res, binary.LittleEndian.Uint64(k[1:9]))
			}
		}
		return nil
	})
	return
}

func (b *badgerDB) Debug() {
	fmt.Println("Blocks")
	for _, id := range b.Blocks() {
		item, _ := b.GetBlock(id)
		fmt.Printf("\t%d. hash: %x %v\n", id, item.Hash, item.Weight)
	}
	fmt.Println()
	fmt.Println("Nodes")
	for _, id := range b.Nodes() {
		item, _ := b.GetNode(id)
		fmt.Printf("\t%d. hash: %x %v\n", id, item.Hash, item.Weight)
	}
}
