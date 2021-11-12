package main

import (
	"context"
	"path/filepath"
	"strconv"
)

type UniqueIndex struct {
	lastID  int
	index   map[string]int
	input   chan row
	storage *CSVStorage
}

func NewUniqueIndex(name string, shardID uint32) (UniqueIndex, error) {
	storage, err := NewCSVStorage(filepath.Join(cfg.DataDir, getChainDir(shardID), "archive"+name+".csv"))
	if err != nil {
		return UniqueIndex{}, err
	}

	return UniqueIndex{
		index:   map[string]int{},
		storage: storage,
		input:   make(chan row),
	}, nil
}

func (ui *UniqueIndex) Add(val string) (int, bool) {
	id, ok := ui.index[val]
	if ok {
		return id, ok
	}
	ui.lastID++
	ui.index[val] = ui.lastID
	return ui.lastID, false
}

func (ui *UniqueIndex) Write(ctx context.Context) error {
	return ui.storage.WriteData(ctx, ui.input)
}

type AddressUniqueIndex struct {
	UniqueIndex
}

func NewAddressIndex(shardID uint32) AddressUniqueIndex {
	i, _ := NewUniqueIndex("addresses", shardID)
	return AddressUniqueIndex{UniqueIndex: i}
}

func (ui *AddressUniqueIndex) Add(val string) int {
	id, exist := ui.UniqueIndex.Add(val)
	if !exist {
		ui.input <- row{
			flush: id%1000 == 0,
			data:  Address{ID: id, Address: val},
		}
	}
	return id
}

type HashUniqueIndex struct {
	UniqueIndex
}

func NewHashIndex(shardID uint32) HashUniqueIndex {
	i, _ := NewUniqueIndex("hashes", shardID)
	return HashUniqueIndex{UniqueIndex: i}
}

func (ui *HashUniqueIndex) Add(val string) int {
	id, exist := ui.UniqueIndex.Add(val)
	if !exist {
		ui.input <- row{
			flush: id%1000 == 0,
			data:  Hash{ID: id, Hash: val},
		}
	}
	return id
}

type UTXOIndex struct {
	scriptIndex map[string]struct {
		value  int64
		script []byte
	}
}

func NewUTXOIndex() UTXOIndex {
	return UTXOIndex{
		scriptIndex: map[string]struct {
			value  int64
			script []byte
		}{},
	}
}

func (ui *UTXOIndex) Add(hash, id int, script []byte, value int64) {
	ui.scriptIndex[strconv.Itoa(hash)+strconv.Itoa(id)] = struct {
		value  int64
		script []byte
	}{value: value, script: script}
}

func (ui *UTXOIndex) Get(hash, id int) ([]byte, int64) {
	r := ui.scriptIndex[strconv.Itoa(hash)+strconv.Itoa(id)]
	return r.script, r.value
}
