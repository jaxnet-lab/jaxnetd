// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mmr

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"math/big"
	"sync"

	"github.com/pkg/errors"
)

type Hash [32]byte

func (h *Hash) String() string {
	return fmt.Sprintf("%x", (*h)[:])
}

type Hasher func() hash.Hash

type MmrProof struct {
	Nodes map[uint64]*BlockData
}

// IShardsMergedMiningTree ...
type IShardsMergedMiningTree interface {
	Index() int
	Root() (root Hash, err error)
	Set(index uint64, weight *big.Int, hash []byte) (root Hash, err error)
	Append(weight *big.Int, hash []byte) (err error)
	Copy(db IStore) (IShardsMergedMiningTree, error)
}

type ShardsMergedMiningTree struct {
	sync.RWMutex
	hasher  Hasher
	db      IStore
	genesis []byte
}

func MergedMiningTree(db IStore, genesis []byte) *ShardsMergedMiningTree {
	return &ShardsMergedMiningTree{hasher: sha256.New, db: db, genesis: genesis}
}

func (mmTree *ShardsMergedMiningTree) Copy(db IStore) (IShardsMergedMiningTree, error) {
	treeCopy := MergedMiningTree(db, mmTree.genesis)
	mmTree.RLock()
	defer mmTree.RUnlock()

	blocks, err := mmTree.db.Blocks()
	if err != nil {
		return nil, err
	}

	for _, index := range blocks {
		node, err := mmTree.db.GetBlock(index)
		if err != nil {
			return nil, err
		}

		err = treeCopy.db.SetBlock(index, node)
		if err != nil {
			return nil, err
		}
	}

	nodes, err := mmTree.db.Nodes()
	if err != nil {
		return nil, err
	}

	for _, index := range nodes {
		node, err := mmTree.db.GetNode(index)
		if err != nil {
			return nil, err
		}

		err = treeCopy.db.SetNode(index, node)
		if err != nil {
			return nil, err
		}
	}

	return treeCopy, nil
}

func (mmTree *ShardsMergedMiningTree) MmrFromProofs(db IStore, proof MmrProof) (*ShardsMergedMiningTree, error) {
	res := MergedMiningTree(db, mmTree.genesis)

	for index, data := range proof.Nodes {
		err := db.SetNode(index, data)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

func (mmTree *ShardsMergedMiningTree) Index() int {
	blocks, _ := mmTree.db.Blocks()
	return len(blocks)
}

func (mmTree *ShardsMergedMiningTree) Root() (root Hash, err error) {
	blocks, err := mmTree.db.Blocks()
	return mmTree.GetRoot(uint64(len(blocks))), err
}

func (mmTree *ShardsMergedMiningTree) Append(weight *big.Int, hash []byte) (
	// root Hash,
	err error) {
	mmTree.Lock()
	defer mmTree.Unlock()
	blocks, err := mmTree.db.Blocks()
	if err != nil {
		// return root, err
		return err
	}
	index := uint64(len(blocks))

	if index == 0 {
		_, err = mmTree.set(0, big.NewInt(0), mmTree.genesis)
		if err != nil {
			// return Hash{}, errors.Wrap(err, "unable to init mountainRange for shard")
			return errors.Wrap(err, "unable to init mountainRange for shard")
		}
		index += 1
	}

	blockHash := Hash{}
	copy(blockHash[:], hash[:])

	node := LeafIndex(index)
	err = node.AppendValue(mmTree, &BlockData{Weight: weight, Hash: blockHash})

	// return mmTree.GetRoot(index), err
	return err
}

// Set appends ShardsMergedMiningTree with the block data by index
func (mmTree *ShardsMergedMiningTree) Set(index uint64, weight *big.Int, hash []byte) (Hash, error) {
	mmTree.Lock()
	root, err := mmTree.set(index, weight, hash)
	mmTree.Unlock()
	return root, err

}

// Set appends ShardsMergedMiningTree with the block data by index
func (mmTree *ShardsMergedMiningTree) set(index uint64, weight *big.Int, hash []byte) (root Hash, err error) {
	blockHash := Hash{}
	copy(blockHash[:], hash[:])

	node := LeafIndex(index)
	err = node.AppendValue(mmTree, &BlockData{
		Weight: weight,
		Hash:   blockHash,
	})
	if err != nil {
		return Hash{}, err
	}
	return mmTree.GetRoot(index), nil
}

func (mmTree *ShardsMergedMiningTree) GetProofs(length uint64, indexes ...uint64) (result *MmrProof, err error) {
	result = &MmrProof{
		Nodes: make(map[uint64]*BlockData),
	}
	for _, index := range indexes {
		var currentIndex IBlockIndex = LeafIndex(index)
		for {
			if currentIndex == nil || currentIndex.Index() > length {
				break
			}

			value, ok := currentIndex.Value(mmTree)
			if !ok {
				break
				// return nil, errors.New(fmt.Sprintf("Can't construct the Proof. Node not found for index %d", currentIndex.Index()))
			}

			sib := currentIndex.GetSibling()
			if sib == nil || sib.Index() > length {
				break
			}

			result.Nodes[sib.Index()] = value
			// fmt.Printf("currentIndex %d is Right %t\n", currentIndex.Index(), currentIndex.IsRight())
			if currentIndex.IsRight() {
				currentIndex = currentIndex.RightUp()
			} else {
				currentIndex = sib.RightUp()
			}
		}

		// Go by Tops
		last := LeafIndex(length)
		peaks := last.GetPeaks()
		for _, peak := range peaks {
			peakValue, ok := peak.Value(mmTree)
			if !ok {
				return nil, errors.New(fmt.Sprintf("Can't construct the Proof. Peak not found %d", peak.Index()))
			}
			result.Nodes[peak.Index()] = peakValue
		}
	}
	return
}

// Proofs build an MMR Proof by MMR length and indexes
func (mmTree *ShardsMergedMiningTree) Proofs(length uint64, indexes ...uint64) (result *ShardsMergedMiningTree, err error) {
	result = MergedMiningTree(MemoryDb(), mmTree.genesis)
	for _, index := range indexes {
		var currentIndex IBlockIndex = LeafIndex(index)

		// Go By current branch
		for {
			if currentIndex == nil || currentIndex.Index() > length {
				break
			}
			value, ok := currentIndex.Value(mmTree)
			if !ok {
				return nil, errors.New("can't construct the Proof")
			}

			sib := currentIndex.GetSibling()
			if sib == nil || sib.Index() > length {
				break
			}
			err = sib.SetValue(result, value)
			if err != nil {
				return nil, err
			}
			if currentIndex.IsRight() {
				currentIndex = currentIndex.RightUp()
			} else {
				currentIndex = sib.RightUp()
			}
		}

		// Go by Tops
		last := LeafIndex(length)
		peaks := last.GetPeaks()
		for _, peak := range peaks {
			peakValue, ok := peak.Value(mmTree)
			if !ok {
				return nil, errors.New("can't construct the Proof")
			}

			err = peak.SetValue(result, peakValue)
			if err != nil {
				return nil, err
			}
		}
	}
	return
}

// Proof build an MMR Proof by MMR length and index
// Algorithm:
//  1. Get current block.
//  2. If left - go to Right. Take it
//  3. Go up. If block exists  - take it
//  4. Go to Step 2
func (mmTree *ShardsMergedMiningTree) Proof(index uint64, length uint64) (result *ShardsMergedMiningTree, err error) {
	result = MergedMiningTree(MemoryDb(), mmTree.genesis)
	var current IBlockIndex = LeafIndex(index)

	// я так понимаю "куррент" инициализируется как индекс болка в чейне
	// но при построении пруфа мы двигаемся по узлам ммр дерева - "ммр нодам"
	// тоесть правильно, наверное, инициализировать куррент как индекс ммр ноды
	// Еще одно. Для лучшего понимания что есть "пруф" блока надо погуглить и почитать
	// про Merkle Tree и Merkle Proof. Идея здесь и там абсолютна идентичная.
	// Но про Merkle Tree написано в разы больше чем про ммр.

	// if value, ok := current.Value(mmTree); ok {
	//	current.SetValue(result, value)
	// }

	// Go By current branch
	for {
		if current == nil || current.Index() > length {
			break
		}

		sib := current.GetSibling()
		if sib == nil || sib.Index() > length {
			break
		}
		value, ok := sib.Value(mmTree)
		if !ok {
			return nil, errors.New("can't construct the Proof")
		}

		err = sib.SetValue(result, value)
		if err != nil {
			return nil, err
		}
		if current.IsRight() {
			current = current.RightUp()
		} else {
			current = sib.RightUp()
		}
	}

	// Go by Tops
	// Надо добавить только те "пики" которые идут на пути к "руту".
	// Тоесть если считать что ММР это "цепь маунтингов" и наш куррент индекс находится маунтинге номер mmTree,
	// то необходимо добавить топы только тех маунтингов которые находятся слева от mmTree.
	// Это топы маунтингов 1,2,...,mmTree-1.

	// Отбой. Место лучше не экономить и просто добавить все ммр топы без разбору.
	// Так даже эффективнее будет. Всё правильно.

	last := LeafIndex(length)
	peaks := last.GetPeaks()
	for _, peak := range peaks {
		value, ok := peak.Value(mmTree)
		if !ok {
			return nil, errors.New("can't construct the Proof")
		}

		err = peak.SetValue(result, value)
		if err != nil {
			return nil, err
		}
	}
	return
}

// GetRoot returns MMR Root hash
// Algorithm:
//  1. Take peaks
//  2. Aggregate starting from end to start
func (mmTree *ShardsMergedMiningTree) GetRoot(length uint64) (root Hash) {
	last := LeafIndex(length)
	peaks := last.GetPeaks()
	peaksCount := len(peaks)
	hashes := make([]*BlockData, peaksCount)
	str := ""
	for i, peak := range peaks {
		str += fmt.Sprintf("%d ", i)
		value, _ := peak.Value(mmTree)
		hashes[peaksCount-i-1] = value
	}
	if len(hashes) == 0 {
		return
	}

	rootNode := hashes[0]
	if peaksCount > 1 {
		for _, h := range hashes[1:] {
			rootNode = mmTree.aggregate(h, rootNode)
		}
	}
	return rootNode.Digest(mmTree.hasher)
}

// ValidateProof validates MMR proof
func (mmTree *ShardsMergedMiningTree) ValidateProof(index, length uint64, fullMmr *ShardsMergedMiningTree) (ok bool) {

	var current IBlockIndex = LeafIndex(index)
	var parent IBlockIndex
	for {
		if current == nil || current.Index() > length {
			fmt.Printf("Breaking")
			break
		}

		originlvalue, ok := current.Value(fullMmr)
		// fmt.Printf("Current %d [%d %x]\n", current.Index(), originlvalue.Weight, originlvalue.Hash)
		if !ok {
			break
		}
		sib := current.GetSibling()

		sibValue, ok := sib.Value(mmTree)
		if !ok {
			break
		}

		// fmt.Printf("Sibling %d [%d %x] \n", sib.Index(), sibValue.Weight, sibValue.Hash)

		if current.IsRight() {
			parent = current.RightUp()
			if parent == nil {
				break
			}
			aggregated := mmTree.aggregate(sibValue, originlvalue)
			// fmt.Printf("Parent %d [%d %x] \n\n", parent.Index(), aggregated.Weight, aggregated.Hash)
			err := parent.SetValue(mmTree, aggregated)
			if err != nil {
				return false
			}

			current = parent
		} else {

			parent = sib.RightUp()
			if parent == nil {
				break
			}
			aggregated := mmTree.aggregate(originlvalue, sibValue)
			// fmt.Printf("Parent %d [%d %x] \n\n", parent.Index(), aggregated.Weight, aggregated.Hash)
			err := parent.SetValue(mmTree, aggregated)
			if err != nil {
				return false
			}
			current = parent
		}
	}

	// fmt.Printf("Proofs %x %x\n", mmTree.GetRoot(length), fullMmr.GetRoot(length))
	return mmTree.GetRoot(length) == fullMmr.GetRoot(length)
}

func (mmTree *ShardsMergedMiningTree) aggregate(left, right *BlockData) (result *BlockData) {
	h := mmTree.hasher()
	digest := right.Digest(mmTree.hasher)
	h.Write(digest[:])
	digest = left.Digest(mmTree.hasher)
	h.Write(digest[:])
	result = &BlockData{
		Weight: big.NewInt(0),
	}
	result.Weight.Add(right.Weight, left.Weight)
	copy(result.Hash[:], h.Sum([]byte{}))
	return
}
