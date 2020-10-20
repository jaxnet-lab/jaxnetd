// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package mmr

import (
	"errors"
	"fmt"
	"hash"
	"log"
	"math/big"
	"sync"
)

type Hash [32]byte

func (h *Hash) String() string {
	return fmt.Sprintf("%x", (*h)[:])
}

type Hasher func() hash.Hash

type MmrProof struct {
	Nodes map[uint64]*BlockData
}

type IMountainRange interface {
	Set(index uint64, weight *big.Int, hash []byte) (root Hash)
	Append(weight *big.Int, hash []byte) (root Hash)
	Index() int
	Root() (root Hash)
	Copy(db IStore) IMountainRange
}

type mmr struct {
	sync.RWMutex
	hasher Hasher
	db     IStore
}

func Mmr(hasher Hasher, db IStore) *mmr {
	h := hasher().Sum([]byte{})
	if len(h) != 32 {
		log.Println("Invalid Hash function size: ")
		return nil
	}
	return &mmr{hasher: hasher, db: db}
}

func (m *mmr) Copy(db IStore) IMountainRange {
	res := Mmr(m.hasher, db)
	m.Lock()
	blocks := m.db.Blocks()
	for _, index := range blocks {
		if block, ok := m.db.GetBlock(index); ok {
			res.db.SetBlock(index, block)
		}
	}

	nodes := m.db.Nodes()
	for _, index := range nodes {
		if node, ok := m.db.GetNode(index); ok {
			res.db.SetNode(index, node)
		}
	}
	m.Unlock()

	return res
}

func (m *mmr) MmrFromProofs(hasher Hasher, db IStore, proof MmrProof) *mmr {
	res := Mmr(hasher, db)

	for index, data := range proof.Nodes {
		db.SetNode(index, data)
	}
	return res
}

func (m *mmr) Index() int {
	return len(m.db.Blocks())
}

func (m *mmr) Root() (root Hash) {
	return m.GetRoot(uint64(len(m.db.Blocks())))
}

func (m *mmr) Append(weight *big.Int, hash []byte) (root Hash) {
	m.Lock()
	defer m.Unlock()
	index := uint64(len(m.db.Blocks()))

	blockHash := Hash{}
	copy(blockHash[:], hash[:])

	node := LeafIndex(index)
	node.AppendValue(m, &BlockData{
		Weight: weight,
		Hash:   blockHash,
	})
	return m.GetRoot(index)
}

// Append mmr with the block data by index
func (m *mmr) Set(index uint64, weight *big.Int, hash []byte) (root Hash) {
	m.Lock()
	defer m.Unlock()

	blockHash := Hash{}
	copy(blockHash[:], hash[:])

	node := LeafIndex(index)
	node.AppendValue(m, &BlockData{
		Weight: weight,
		Hash:   blockHash,
	})
	return m.GetRoot(index)
}

func (m *mmr) GetProofs(length uint64, indexes ...uint64) (result *MmrProof, err error) {
	result = &MmrProof{
		Nodes: make(map[uint64]*BlockData),
	}
	for _, index := range indexes {
		fmt.Println(index)
		var currentIndex IBlockIndex = LeafIndex(index)
		for {
			fmt.Println(currentIndex.Index())
			if currentIndex == nil || currentIndex.Index() > length {
				break
			}

			value, ok := currentIndex.Value(m)
			if !ok {
				break
				//return nil, errors.New(fmt.Sprintf("Can't construct the Proof. Node not found for index %d", currentIndex.Index()))
			}

			sib := currentIndex.GetSibling()
			if sib == nil || sib.Index() > length {
				break
			}

			result.Nodes[sib.Index()] = value
			//fmt.Printf("currentIndex %d is Right %t\n", currentIndex.Index(), currentIndex.IsRight())
			if currentIndex.IsRight() {
				currentIndex = currentIndex.RightUp()
			} else {
				currentIndex = sib.RightUp()
			}
		}

		//Go by Tops
		last := LeafIndex(length)
		peaks := last.GetPeaks()
		for _, peak := range peaks {
			peakValue, ok := peak.Value(m)
			if !ok {
				return nil, errors.New(fmt.Sprintf("Can't construct the Proof. Peak not found %d", peak.Index()))
			}
			result.Nodes[peak.Index()] = peakValue
		}
	}
	return
}

// Build an MMR Proof by MMR length and indexes
func (m *mmr) Proofs(length uint64, indexes ...uint64) (result *mmr, err error) {
	result = Mmr(m.hasher, MemoryDb())
	for _, index := range indexes {
		var currentIndex IBlockIndex = LeafIndex(index)

		//Go By current branch
		for {
			if currentIndex == nil || currentIndex.Index() > length {
				break
			}
			value, ok := currentIndex.Value(m)
			if !ok {
				return nil, errors.New("Can't construct the Proof")
			}

			sib := currentIndex.GetSibling()
			if sib == nil || sib.Index() > length {
				break
			}
			sib.SetValue(result, value)
			if currentIndex.IsRight() {
				currentIndex = currentIndex.RightUp()
			} else {
				currentIndex = sib.RightUp()
			}
		}

		//Go by Tops
		last := LeafIndex(length)
		peaks := last.GetPeaks()
		for _, peak := range peaks {
			peakValue, ok := peak.Value(m)
			if !ok {
				return nil, errors.New("Can't construct the Proof")
			}
			peak.SetValue(result, peakValue)
		}
	}
	return
}

// Build an MMR Proof by MMR length and index
// Algorithm:
//  1. Get current block.
//  2. If left - go to Right. Take it
//  3. Go up. If block exists  - take it
//  4. Go to Step 2
func (m *mmr) Proof(index uint64, length uint64) (result *mmr, err error) {
	result = Mmr(m.hasher, MemoryDb())
	var current IBlockIndex = LeafIndex(index)

	// я так понимаю "куррент" инициализируется как индекс болка в чейне
	// но при построении пруфа мы двигаемся по узлам ммр дерева - "ммр нодам"
	// тоесть правильно, наверное, инициализировать куррент как индекс ммр ноды
	// Еще одно. Для лучшего понимания что есть "пруф" блока надо погуглить и почитать
	// про Merkle Tree и Merkle Proof. Идея здесь и там абсолютна идентичная.
	// Но про Merkle Tree написано в разы больше чем про ммр.

	//if value, ok := current.Value(m); ok {
	//	current.SetValue(result, value)
	//}

	//Go By current branch
	for {
		if current == nil || current.Index() > length {
			break
		}

		sib := current.GetSibling()
		if sib == nil || sib.Index() > length {
			break
		}
		value, ok := sib.Value(m)
		if !ok {
			return nil, errors.New("Can't construct the Proof")
		}

		sib.SetValue(result, value)
		if current.IsRight() {
			current = current.RightUp()
		} else {
			current = sib.RightUp()
		}
	}

	//Go by Tops
	// Надо добавить только те "пики" которые идут на пути к "руту".
	// Тоесть если считать что ММР это "цепь маунтингов" и наш куррент индекс находится маунтинге номер m,
	// то необходимо добавить топы только тех маунтингов которые находятся слева от m.
	// Это топы маунтингов 1,2,...,m-1.

	// Отбой. Место лучше не экономить и просто добавить все ммр топы без разбору.
	// Так даже эффективнее будет. Всё правильно.

	last := LeafIndex(length)
	peaks := last.GetPeaks()
	for _, peak := range peaks {
		value, ok := peak.Value(m)
		if !ok {
			return nil, errors.New("Can't construct the Proof")
		}
		peak.SetValue(result, value)
	}
	return
}

// Returns MMR Root hash
// Algorithm:
//  1. Take peaks
//  2. Aggregate starting from end to start
func (m *mmr) GetRoot(length uint64) (root Hash) {
	last := LeafIndex(length)
	peaks := last.GetPeaks()
	peaksCount := len(peaks)
	hashes := make([]*BlockData, peaksCount)
	str := ""
	for i, peak := range peaks {
		str += fmt.Sprintf("%d ", i)
		value, _ := peak.Value(m)
		hashes[peaksCount-i-1] = value
	}
	if len(hashes) == 0 {
		return
	}

	rootNode := hashes[0]
	if peaksCount > 1 {
		for _, h := range hashes[1:] {
			rootNode = m.aggregate(h, rootNode)
		}
	}
	return rootNode.Digest(m.hasher)
}

// Validate MMR proof
func (m *mmr) ValidateProof(index, length uint64, fullMmr *mmr) (ok bool) {

	var current IBlockIndex = LeafIndex(index)
	var parent IBlockIndex
	for {
		if current == nil || current.Index() > length {
			fmt.Printf("Breaking")
			break
		}

		originlvalue, ok := current.Value(fullMmr)
		fmt.Printf("Current %d [%d %x]\n", current.Index(), originlvalue.Weight, originlvalue.Hash)
		if !ok {
			break
		}
		sib := current.GetSibling()

		sibValue, ok := sib.Value(m)
		if !ok {
			break
		}

		fmt.Printf("Sibling %d [%d %x] \n", sib.Index(), sibValue.Weight, sibValue.Hash)

		if current.IsRight() {
			parent = current.RightUp()
			if parent == nil {
				break
			}
			aggregated := m.aggregate(sibValue, originlvalue)
			fmt.Printf("Parent %d [%d %x] \n\n", parent.Index(), aggregated.Weight, aggregated.Hash)
			parent.SetValue(m, aggregated)
			current = parent
		} else {

			parent = sib.RightUp()
			if parent == nil {
				break
			}
			aggregated := m.aggregate(originlvalue, sibValue)
			fmt.Printf("Parent %d [%d %x] \n\n", parent.Index(), aggregated.Weight, aggregated.Hash)
			parent.SetValue(m, aggregated)
			current = parent
		}
	}

	fmt.Printf("Proofs %x %x\n", m.GetRoot(length), fullMmr.GetRoot(length))

	return m.GetRoot(length) == fullMmr.GetRoot(length)
}

func (m *mmr) aggregate(left, right *BlockData) (result *BlockData) {
	h := m.hasher()
	digest := right.Digest(m.hasher)
	h.Write(digest[:])
	digest = left.Digest(m.hasher)
	h.Write(digest[:])
	result = &BlockData{
		Weight: big.NewInt(0),
	}
	result.Weight.Add(right.Weight, left.Weight)
	copy(result.Hash[:], h.Sum([]byte{}))
	return
}
