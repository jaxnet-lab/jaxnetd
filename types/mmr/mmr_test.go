// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package mmr

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitMmr(t *testing.T) {
	m := MergedMiningTree(nil, nil)
	assert.NotNil(t, m)

}

func TestGetTop(t *testing.T) {
	top := LeafIndex(0).GetTop().Index()
	assert.Equal(t, uint64(0), top)

	top = LeafIndex(1).GetTop().Index()
	assert.Equal(t, uint64(1), top)

	top = LeafIndex(2).GetTop().Index()
	assert.Equal(t, uint64(2), top)

	top = LeafIndex(3).GetTop().Index()
	assert.Equal(t, uint64(2), top)

	top = LeafIndex(4).GetTop().Index()
	assert.Equal(t, uint64(4), top)

	top = LeafIndex(5).GetTop().Index()
	assert.Equal(t, uint64(5), top)

	top = LeafIndex(6).GetTop().Index()
	assert.Equal(t, uint64(6), top)

	top = LeafIndex(7).GetTop().Index()
	assert.Equal(t, uint64(4), top)

	top = LeafIndex(11).GetTop().Index()
	assert.Equal(t, uint64(10), top)

	top = LeafIndex(15).GetTop().Index()
	assert.Equal(t, uint64(8), top)

	top = LeafIndex(16).GetTop().Index()
	assert.Equal(t, uint64(16), top)

	top = LeafIndex(23).GetTop().Index()
	assert.Equal(t, uint64(20), top)

	top = LeafIndex(31).GetTop().Index()
	assert.Equal(t, uint64(16), top)

	top = NodeIndex(6).GetTop().Index()
	assert.Equal(t, uint64(4), top)

	top = NodeIndex(0).GetTop().Index()
	assert.Equal(t, uint64(0), top)

	top = NodeIndex(5).GetTop().Index()
	assert.Equal(t, uint64(5), top)

	top = NodeIndex(1).GetTop().Index()
	assert.Equal(t, uint64(1), top)

	top = NodeIndex(2).GetTop().Index()
	assert.Equal(t, uint64(2), top)

	top = NodeIndex(14).GetTop().Index()
	assert.Equal(t, uint64(8), top)

	top = NodeIndex(12).GetTop().Index()
	assert.Equal(t, uint64(8), top)

	top = NodeIndex(15).GetTop().Index()
	assert.Equal(t, uint64(8), top)

	top = NodeIndex(20).GetTop().Index()
	assert.Equal(t, uint64(20), top)

	top = NodeIndex(22).GetTop().Index()
	assert.Equal(t, uint64(20), top)

	top = NodeIndex(23).GetTop().Index()
	assert.Equal(t, uint64(20), top)

	top = NodeIndex(27).GetTop().Index()
	assert.Equal(t, uint64(26), top)

	top = NodeIndex(31).GetTop().Index()
	assert.Equal(t, uint64(16), top)
}

func TestGetRightTop(t *testing.T) {
	valueNil := LeafIndex(0).RightUp()
	assert.Equal(t, nil, valueNil)

	valueNil = LeafIndex(2).RightUp()
	assert.Equal(t, nil, valueNil)
	//
	valueNil = LeafIndex(4).RightUp()
	assert.Equal(t, nil, valueNil)
	//
	//
	valueNil = NodeIndex(0).RightUp()
	assert.Equal(t, nil, valueNil)
	//
	valueNil = NodeIndex(1).RightUp()
	assert.Equal(t, nil, valueNil)

	valueNil = NodeIndex(2).RightUp()
	assert.Equal(t, nil, valueNil)

	valueNil = NodeIndex(4).RightUp()
	assert.Equal(t, nil, valueNil)
	//
	valueNil = NodeIndex(10).RightUp()
	assert.Equal(t, nil, valueNil)

	valueNil = NodeIndex(20).RightUp()
	assert.Equal(t, nil, valueNil)

	value := LeafIndex(1).RightUp().Index()
	assert.Equal(t, uint64(1), value)

	value = LeafIndex(3).RightUp().Index()
	assert.Equal(t, uint64(3), value)

	value = LeafIndex(5).RightUp().Index()
	assert.Equal(t, uint64(5), value)

	value = NodeIndex(3).RightUp().Index()
	assert.Equal(t, uint64(2), value)
	//
	value = NodeIndex(11).RightUp().Index()
	assert.Equal(t, uint64(10), value)

	value = NodeIndex(15).RightUp().Index()
	assert.Equal(t, uint64(14), value)

	value = NodeIndex(14).RightUp().Index()
	assert.Equal(t, uint64(12), value)

	value = NodeIndex(12).RightUp().Index()
	assert.Equal(t, uint64(8), value)

	value = NodeIndex(11).RightUp().Index()
	assert.Equal(t, uint64(10), value)

	value = NodeIndex(31).RightUp().Index()
	assert.Equal(t, uint64(30), value)

	value = NodeIndex(30).RightUp().Index()
	assert.Equal(t, uint64(28), value)
}

func TestGetNeighbor(t *testing.T) {
	value := NodeIndex(2).GetSibling().Index()
	assert.Equal(t, uint64(6), value)

	value = NodeIndex(8).GetSibling().Index()
	assert.Equal(t, uint64(24), value)

	value = NodeIndex(12).GetSibling().Index()
	assert.Equal(t, uint64(4), value)

	value = NodeIndex(3).GetSibling().Index()
	assert.Equal(t, uint64(1), value)

	value = NodeIndex(7).GetSibling().Index()
	assert.Equal(t, uint64(5), value)

	value = NodeIndex(28).GetSibling().Index()
	assert.Equal(t, uint64(20), value)

	value = NodeIndex(24).GetSibling().Index()
	assert.Equal(t, uint64(8), value)

	value = NodeIndex(9).GetSibling().Index()
	assert.Equal(t, uint64(11), value)
}

func TestGetLeftSideBranch(t *testing.T) {
	value := LeafIndex(0).GetLeftBranch()
	assert.Equal(t, nil, value)

	value = LeafIndex(1).GetLeftBranch()
	assert.Equal(t, nil, value)

	value = LeafIndex(2).GetLeftBranch()
	assert.Equal(t, uint64(1), value.Index())

	value = LeafIndex(4).GetLeftBranch()
	assert.Equal(t, uint64(3), value.Index())

	value = NodeIndex(5).GetLeftBranch()
	assert.Equal(t, uint64(3), value.Index())

	value = NodeIndex(6).GetLeftBranch()
	assert.Equal(t, uint64(2), value.Index())

	value = NodeIndex(12).GetLeftBranch()
	assert.Equal(t, uint64(4), value.Index())

	value = NodeIndex(4).GetLeftBranch()
	assert.Equal(t, nil, value)

	value = NodeIndex(24).GetLeftBranch()
	assert.Equal(t, uint64(8), value.Index())

	value = NodeIndex(30).GetLeftBranch()
	assert.Equal(t, uint64(26), value.Index())

	value = NodeIndex(31).GetLeftBranch()
	assert.Equal(t, uint64(29), value.Index())

	value = NodeIndex(20).GetLeftBranch()
	assert.Equal(t, uint64(12), value.Index())
}

func TestGetPeaks(t *testing.T) {
	peaks := LeafIndex(0).GetPeaks()
	assert.Equal(t, 1, len(peaks))

	peaks = LeafIndex(1).GetPeaks()
	assert.Equal(t, 1, len(peaks))

	peaks = LeafIndex(2).GetPeaks()
	assert.Equal(t, 2, len(peaks))

	peaks = LeafIndex(3).GetPeaks()
	assert.Equal(t, 1, len(peaks))

	peaks = LeafIndex(4).GetPeaks()
	assert.Equal(t, 2, len(peaks))

	peaks = LeafIndex(5).GetPeaks()
	assert.Equal(t, 2, len(peaks))

	peaks = LeafIndex(6).GetPeaks()
	assert.Equal(t, 3, len(peaks))
	assert.Equal(t, uint64(6), peaks[0].Index())
	assert.Equal(t, uint64(5), peaks[1].Index())
	assert.Equal(t, uint64(2), peaks[2].Index())

	peaks = LeafIndex(7).GetPeaks()
	assert.Equal(t, 1, len(peaks))
	assert.Equal(t, uint64(4), peaks[0].Index())
	//
	peaks = LeafIndex(8).GetPeaks()
	assert.Equal(t, 2, len(peaks))
	assert.Equal(t, uint64(8), peaks[0].Index())
	assert.Equal(t, uint64(4), peaks[1].Index())

	peaks = LeafIndex(9).GetPeaks()
	assert.Equal(t, 2, len(peaks))

	peaks = LeafIndex(10).GetPeaks()
	assert.Equal(t, 3, len(peaks))

	peaks = LeafIndex(12).GetPeaks()
	assert.Equal(t, 3, len(peaks))

	peaks = LeafIndex(14).GetPeaks()
	assert.Equal(t, 4, len(peaks))
	assert.Equal(t, uint64(14), peaks[0].Index())
	assert.Equal(t, uint64(13), peaks[1].Index())
	assert.Equal(t, uint64(10), peaks[2].Index())
	assert.Equal(t, uint64(4), peaks[3].Index())

	peaks = LeafIndex(24).GetPeaks()
	assert.Equal(t, 3, len(peaks))
	assert.Equal(t, uint64(24), peaks[0].Index())
	assert.Equal(t, uint64(20), peaks[1].Index())
	assert.Equal(t, uint64(8), peaks[2].Index())
}

func TestAppendNode(t *testing.T) {
	mmr := MergedMiningTree(MemoryDb(), make([]byte, 32))
	data := make([]byte, 32)
	rand.Read(data[:32])

	var i uint64 = 0
	for i < 20 {
		rand.Read(data[:32])
		mmr.Set(i, big.NewInt(int64(i)), data)
		i++
	}
}

func validateNodes(nodes []uint64, expected ...uint64) bool {
	if len(nodes) != len(expected) {
		return false

	}
	expectedMap := map[uint64]bool{}
	for _, value := range expected {
		expectedMap[value] = true
	}

	for _, value := range nodes {
		if _, ok := expectedMap[value]; !ok {
			return false
		}
	}

	return true

}

func TestGetProof(t *testing.T) {
	db := MemoryDb()
	mmr := MergedMiningTree(db, make([]byte, 32))
	data := make([]byte, 32)
	rand.Read(data[:32])
	var i uint64 = 0
	var err error
	h := Hash{}

	for i <= 20 {
		rand.Read(data[:32])
		h, err = mmr.Set(i, big.NewInt(int64(i)), data)
		assert.NoError(t, err)
		i++
	}
	h2 := mmr.GetRoot(20)
	assert.Equal(t, h2, h)

	fmt.Println("Build proof")
	mmrP, err := mmr.Proof(7, 8)
	//mmrP.db.Debug()

	assert.NoError(t, err)
	assert.NotNil(t, mmrP)
	//

	var try = func(f func() ([]uint64, error)) []uint64 {
		d, e := f()
		assert.NoError(t, e)
		return d
	}

	assert.True(t, validateNodes(try(mmrP.db.Blocks), 6, 8))
	assert.True(t, validateNodes(try(mmrP.db.Nodes), 5, 2, 4))
	assert.False(t, validateNodes(try(mmrP.db.Blocks), 5, 8))

	mmr.db.Debug()
	mmrP, err = mmr.Proof(7, 18)
	mmrP.db.Debug()
	assert.NoError(t, err)
	assert.NotNil(t, mmrP)

	assert.True(t, validateNodes(try(mmrP.db.Blocks), 6, 18))
	//assert.True(t, validateNodes(try(mmrP.db.Nodes), 5, 2, 4))
	assert.False(t, validateNodes(try(mmrP.db.Blocks), 5, 2, 12, 17, 8))

	h = mmr.GetRoot(18)
	h2 = mmrP.GetRoot(18)
	assert.Equal(t, h2, h)

	assert.True(t, mmrP.ValidateProof(7, 18, mmr))
}

func TestGetProof200(t *testing.T) {
	db := MemoryDb()
	mmr := MergedMiningTree(db, make([]byte, 32))
	data := make([]byte, 32)
	rand.Read(data[:32])

	var i uint64 = 0
	var err error

	h := Hash{}
	for i <= 203 {
		rand.Read(data[:32])
		h, err = mmr.Set(i, big.NewInt(int64(i)), data)
		assert.NoError(t, err)
		i++
	}
	h2 := mmr.GetRoot(203)
	assert.Equal(t, h2, h)
	//
	fmt.Println("Build proof")
	db.Debug()
	pf, err := mmr.GetProofs(200, 44)
	assert.NoError(t, err)
	fmt.Println(pf)

	//mmrP, err := ShardsMergedMiningTree.Proof(7, 8)
	//mmrP.db.Debug()
	//
	//assert.NoError(t, err)
	//assert.NotNil(t, mmrP)
	//
	//assert.True(t, validateNodes(mmrP.db.Blocks(),  6, 8))
	//assert.True(t, validateNodes(mmrP.db.Nodes(), 5, 2, 4))
	//assert.False(t, validateNodes(mmrP.db.Blocks(), 5, 8))
	//
	//ShardsMergedMiningTree.db.Debug()
	//mmrP, err = ShardsMergedMiningTree.Proof(7, 18)
	//mmrP.db.Debug()
	//assert.NoError(t, err)
	//assert.NotNil(t, mmrP)
	//
	//assert.True(t, validateNodes(mmrP.db.Blocks(),  6, 18))
	////assert.True(t, validateNodes(mmrP.db.Nodes(), 5, 2, 4))
	//assert.False(t, validateNodes(mmrP.db.Blocks(), 5, 2, 12, 17, 8))
	//
	//h = ShardsMergedMiningTree.GetRoot(18)
	//h2 = mmrP.GetRoot(18)
	//assert.Equal(t, h2, h)
	//
	//
	//assert.True(t, mmrP.ValidateProof(7, 18, ShardsMergedMiningTree))
}

func TestGetProofs(t *testing.T) {
	mmr := MergedMiningTree(MemoryDb(), make([]byte, 32))
	data := make([]byte, 32)
	rand.Read(data[:32])
	var i uint64 = 0
	for i < 21 {
		rand.Read(data[:32])
		mmr.Set(i, big.NewInt(int64(i)), data)
		i++
	}

	mmrP, err := mmr.Proofs(8, 4, 7)

	assert.NoError(t, err)
	assert.NotNil(t, mmrP)

	//assert.True(t, mmrP.ValidateProof(4,  ShardsMergedMiningTree.GetRoot(4)))
	//assert.True(t, mmrP.ValidateProof(8, ShardsMergedMiningTree.GetRoot(8)))
	//
	//mmrP, err = ShardsMergedMiningTree.Proofs(3, 1, 2)
	//
	//assert.True(t, validateNodes(mmrP.db.Blocks(), 0, 1, 2, 3))
	//assert.True(t, validateNodes(mmrP.db.Nodes(), 3, 2, 1))
	//
	//assert.NoError(t, err)
	//assert.NotNil(t, mmrP)
	//
	////assert.True(t, mmrP.ValidateProof(4,  ShardsMergedMiningTree.GetRoot(4)))
	//assert.True(t, mmrP.ValidateProof(3, ShardsMergedMiningTree.GetRoot(3)))
	//
	//mmrP, err = ShardsMergedMiningTree.Proofs(20, 5, 10)
	//
	//assert.True(t, validateNodes(mmrP.db.Blocks(), 5, 4, 10, 11, 20))
	//assert.True(t, validateNodes(mmrP.db.Nodes(),  7, 2, 12, 18, 8, 9, 14,4 ))
	//
	//assert.NoError(t, err)
	//assert.NotNil(t, mmrP)
}

//
// func TestGetProofsFileDB(t *testing.T) {
// 	db, err := BadgerDB("./data")
// 	assert.NoError(t, err)
// 	mmr := MergedMiningTree(sha3.New256, db)
// 	data := make([]byte, 32)
// 	rand.Read(data[:32])
// 	var i uint64 = 0
// 	for i < 21 {
// 		rand.Read(data[:32])
// 		mmr.Set(i, big.NewInt(int64(i)), data)
// 		i++
// 	}
//
// 	mmrP, err := mmr.Proofs(8, 4, 7)
//
// 	assert.NoError(t, err)
// 	assert.NotNil(t, mmrP)
// }
