/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package merged_mining_tree

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/big"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// Slot for the shard's hash.
// (leaf of the merkle tree).
type slot = chainhash.Hash

// slotsSpace and levelsSpace are defined as custom types
// to ensure right correlation between them in case of refactoring in the future.
type slotsSpace uint32
type levelsSpace uint16

var (
	ErrInvalidShardPosition   = errors.New("specified shard position does not correspond to the expected tree conf")
	ErrValidation             = errors.New("validation error")
	ErrRootIsNil              = errors.New("root is nil")
	ErrLeafIsNil              = errors.New("leaf is nil")
	ErrInvalidMerkleProofPath = errors.New("invalid merkle proof path")
)

var (
	// MH - MagicHash.
	// Used to bound the orange tree.
	// See the WP for the details.
	MH = chainhash.Hash{}
)

// node represents the node of the sparse Merkle Tree.
type node struct {
	// Sparse Merkle Tree could have some nodes empty.
	// Data contains node's hash, or nil in case if node is empty.
	data *chainhash.Hash

	parent     *node
	leftChild  *node
	rightChild *node

	// Flag that specifies if this node is a part of a
	// special subset of the tree nodes called orange tree.
	// By default is set to false.
	isOrange bool
}

// SetMH fills node's data with magic hash.
func (n *node) SetMH() {
	if n.data == nil {
		n.data = &chainhash.Hash{}
	}

	copy(n.data[:], MH[:])
}

// IsMH returns true in case if current node's data is magic hash.
func (n *node) IsMH() bool {
	if !n.IsNil() {
		return bytes.Compare(n.data[:], MH[:]) == 0
	}

	return false
}

func (n *node) SetNil() {
	n.data = nil
}

func (n *node) IsNil() bool {
	return n.data == nil
}

// ContainsData returns true in case if node's data is not nil and is not MH.
// (MH is not considered as data)
func (n *node) ContainsData() bool {
	return !n.IsNil() && !n.IsMH()
}

func (n *node) SetOrange() {
	n.isOrange = true
}

func (n *node) UnsetOrange() {
	n.isOrange = false
}

// pair represents the pair of adjacent nodes,
// that are located on the same level and have common parent node.
type pair struct {
	parent        *pair
	childrenLeft  *pair
	childrenRight *pair

	leftNode  *node
	rightNode *node
}

// UpdateCorrespondingHash updates the data of the parent node with the hash,
// that is generated using the data of nodes that belong to current pair.
//
// "concatBuffer" is an optional argument that is used as a memory buffer
// to prevent redundant memory allocations and to minimize the pressure on the GC.
// In case if it is nil - the buffer would be allocated internally.
func (p *pair) UpdateCorrespondingHash(concatBuffer []byte) {
	concatBufferSize := len(chainhash.Hash{}) * 2
	concatBuffer = make([]byte, 0, concatBufferSize)

	concatBuffer = append(concatBuffer, p.leftNode.data[:]...)
	concatBuffer = append(concatBuffer, p.rightNode.data[:]...)

	hash := chainhash.HashH(concatBuffer)
	p.leftNode.parent.data = &chainhash.Hash{}
	copy(p.leftNode.parent.data[:], hash[:])
	return
}

var (
	topologySet    = &big.Int{}
	topologyOffset = 0

	nodesTypesSet    = &big.Int{}
	nodesTypesOffset = 0
)

type SparseMerkleTree struct {
	root   *node
	levels [][]*pair

	// Go's standard big.Int is used as a bitset here.
	// This index stores one bit per each hash slot of the tree.
	// In case if slot has been changed - the corresponding bit is set to 1.
	// Initially all the bits are set to 0 and this is optimized by the big.Int impl.,
	// (zero bits are compressed, so no redundant memory usage occurs until some bit set 1 occurs)
	// After each root calculation - all the bits are going to be reset to 0.
	modifiedShardsIndex big.Int

	slotsCount      slotsSpace
	slotsLevelIndex levelsSpace

	// Buffer that is used for hashing the levels of the tree.
	// Allocated once to prevent redundant allocations between root calculations.
	nodesHashBuffer [len(chainhash.Hash{}) * 2]byte

	// How many times calculation of the root has been called.
	// This field is used only for the tests purposes
	// and does not harms the production execution at all.
	rootCalculationExecutionTimes uint64
}

func NewSparseMerkleTree(shardsCount uint32) (tree *SparseMerkleTree) {
	// Merkle tree requires binary structure of it's levels/leafs.
	// That means, that with each one next levelIndex - the amount of leafs doubles.
	// Passed "shardsCount" with some probability would not fit exactly the amount of slots that are possible,
	// and in this case the structure of the tree must be aligned to the appropriate slots count.
	var internalAlignedShardsSlotsCount slotsSpace = 2
	for {
		if slotsSpace(shardsCount) <= internalAlignedShardsSlotsCount {
			break
		}

		internalAlignedShardsSlotsCount *= 2
	}

	levelsCountExceptRoot := levelsCount(internalAlignedShardsSlotsCount) - 1
	tree = &SparseMerkleTree{
		root:   &node{},
		levels: make([][]*pair, levelsCountExceptRoot),
	}

	// Structure initialisation
	{
		levelIndex := 0
		levelPairsCount := 1

		for {
			if levelsSpace(levelIndex) == levelsCountExceptRoot {
				break
			}

			tree.levels[levelIndex] = make([]*pair, levelPairsCount)
			for pairIndex := 0; pairIndex < levelPairsCount; pairIndex++ {
				tree.levels[levelIndex][pairIndex] = &pair{}
			}

			levelIndex++
			levelPairsCount *= 2
		}
	}

	// Creating the first level
	p00 := tree.levels[0][0]
	p00.leftNode = &node{
		parent: tree.root,
	}
	p00.rightNode = &node{
		parent: tree.root,
	}

	tree.root.leftChild = p00.leftNode
	tree.root.rightChild = p00.rightNode

	// Note: Root could not be set here.
	//       Root is defined as a node, but pair expects nodes pair as a root (in general case).
	//       The first level pair of nodes are an edge case.

	if levelsCountExceptRoot > 1 {
		p00.childrenLeft = tree.levels[1][0]
		p00.childrenRight = tree.levels[1][1]

		// Initialising the rest nodes on rest levels.
		var (
			parentLevelIndex     levelsSpace = 0
			levelIndex           levelsSpace = 1
			levelPairIndex       slotsSpace  = 0
			parentIndex          slotsSpace  = 0
			totalLevelPairsCount slotsSpace  = 2
		)

		// For each level in range 1..LevelsCount
		for {
			if levelIndex == levelsCountExceptRoot {
				break
			}

			level := tree.levels[levelIndex]

			// For each nodes pair on this level
			for {
				if levelPairIndex == totalLevelPairsCount {
					break
				}

				// Each level has 2^n amount of levels.
				// For efficient parent indexation - levels are created 2 by 2.

				parent := tree.levels[parentLevelIndex][parentIndex]

				{
					p := &pair{
						leftNode:  &node{},
						rightNode: &node{},
						parent:    parent,
					}

					p.leftNode.parent = parent.leftNode
					p.rightNode.parent = parent.leftNode

					parent.childrenLeft = p
					parent.leftNode.leftChild = p.leftNode
					parent.leftNode.rightChild = p.rightNode

					level[levelPairIndex] = p
					levelPairIndex++

				}
				{
					p := &pair{
						leftNode:  &node{},
						rightNode: &node{},
						parent:    parent,
					}

					p.leftNode.parent = parent.rightNode
					p.rightNode.parent = parent.rightNode

					parent.childrenRight = p
					parent.rightNode.leftChild = p.leftNode
					parent.rightNode.rightChild = p.rightNode

					level[levelPairIndex] = p
					levelPairIndex++
				}

				parentIndex++
			}

			// Level processing finished.
			levelIndex++
			parentLevelIndex++

			parentIndex = 0
			levelPairIndex = 0
			totalLevelPairsCount *= 2
		}
	}

	tree.slotsLevelIndex = levelsCountExceptRoot - 1
	tree.slotsCount = slotsSpace(len(tree.levels[tree.slotsLevelIndex]) * 2)
	for i := slotsSpace(0); i < tree.slotsCount; i++ {
		_ = tree.SetShardHash(uint32(i), MH)
	}

	// Force initial root calculation on the first call of Root method
	// (initial cache invalidation)
	tree.markSlotAsModified(0)
	return
}

// Height returns height of the tree including root level.
func (tree *SparseMerkleTree) Height() uint16 {
	// +1 because root does not belong to the levels structure,
	// but it is considered as a level too.
	return uint16(tree.slotsLevelIndex) + 2
}

func (tree *SparseMerkleTree) HeightWithoutRoot() uint16 {
	return tree.Height() - 1
}

// SetShardHash copies shard's hash into slot with position = "position".
// Returns ErrInvalidShardPosition in case if passed position does not correspond to current tree structure.
//
// Note: this method is lazy.
// Setting the shard's hash would not lead to root recalculation until Root() would not be called.
func (tree *SparseMerkleTree) SetShardHash(position uint32, hash [32]byte) (err error) {
	correspondingNode, err := tree.getNode(position)
	if err != nil {
		return
	}

	if correspondingNode.data != nil && bytes.Compare(correspondingNode.data[:], hash[:]) == 0 {
		// Data is the same in both places.
		// No need to update the tree and provoke hashes re-calculation.
		return
	}

	if correspondingNode.data == nil {
		correspondingNode.data = &chainhash.Hash{}
	}

	copy(correspondingNode.data[:], hash[:])
	tree.markSlotAsModified(slotsSpace(position))

	return
}

// DropShardHash deletes shard's hash into slot with position = "position", making slot empty.
// Returns ErrInvalidShardPosition in case if passed position does not correspond to current tree structure.
//
// Note: this method is lazy.
// Setting the shard's hash would not lead to root recalculation until Root() would not be called.
func (tree *SparseMerkleTree) DropShardHash(position uint32) (err error) {
	correspondingNode, err := tree.getNode(position)
	if err != nil {
		return
	}

	correspondingNode.data = nil
	tree.markSlotAsModified(slotsSpace(position))

	return
}

func (tree *SparseMerkleTree) Root() (root chainhash.Hash, err error) {
	defer func() {
		// Prevent redundant re-calculations
		// until some changes would be done on the leafs level.
		tree.markWholeTreeAsCached()
	}()

	setNodeNilOrMH := func(destNodeLevel levelsSpace, destNode *node, destGroup *pair) {
		if destNode == tree.root {
			// Edge case #1
			destNode.SetMH()
			return
		}

		// Checking for the case when corresponding to the destination node is not nil.
		if destNode == destGroup.leftNode {
			correspondingNode := destGroup.rightNode
			if !correspondingNode.IsNil() && !correspondingNode.IsMH() {
				destNode.SetMH()
				return
			}
		}

		if destNode == destGroup.rightNode {
			correspondingNode := destGroup.leftNode
			if !correspondingNode.IsNil() && !correspondingNode.IsMH() {
				destNode.SetMH()
				return
			}
		}

		destNode.SetNil()
		return
	}

	processPair := func(p *pair, levelIndex levelsSpace) {
		if p.leftNode.IsNil() && p.rightNode.IsNil() {
			setNodeNilOrMH(levelIndex-1, p.leftNode.parent, p.parent)
			// No need to reset p.right.parent (the parent of this node is the same as the left node)
			return
		}

		if p.leftNode.ContainsData() && p.rightNode.ContainsData() {
			p.UpdateCorrespondingHash(tree.nodesHashBuffer[:])

			if p.leftNode.isOrange || p.rightNode.isOrange {
				p.leftNode.parent.SetOrange()
			}

			return
		}

		if p.leftNode.ContainsData() && p.rightNode.IsNil() {
			p.rightNode.SetMH()
			p.UpdateCorrespondingHash(tree.nodesHashBuffer[:])

			if p.rightNode.IsMH() {
				p.leftNode.parent.SetOrange()
			}
			return
		}

		if p.leftNode.IsNil() && p.rightNode.ContainsData() {
			p.leftNode.SetMH()
			p.UpdateCorrespondingHash(tree.nodesHashBuffer[:])

			if p.leftNode.IsMH() {
				p.leftNode.parent.SetOrange()
			}
			return
		}

		if p.leftNode.ContainsData() && p.rightNode.IsMH() {
			p.UpdateCorrespondingHash(tree.nodesHashBuffer[:])

			p.leftNode.parent.SetOrange()
			return
		}

		if p.leftNode.IsMH() && p.rightNode.ContainsData() {
			p.UpdateCorrespondingHash(tree.nodesHashBuffer[:])
			p.leftNode.parent.SetOrange()
			return
		}
	}

	var slotIndex slotsSpace = 0
	if !tree.containsChanges() {
		goto Exit
	}

	// For tests purposes only.
	tree.rootCalculationExecutionTimes++

	// Drop previous orange tree and build it once again from scratch.
	tree.markAllNodesAsRegular()

	// Slots traversing
	for _, p := range tree.levels[tree.slotsLevelIndex] {
		if tree.isSlotChanged(slotIndex) || tree.isSlotChanged(slotIndex+1) {
			processPair(p, tree.slotsLevelIndex)
		}

		// Pair processing is done.
		// Slots index must be increased by 2 (pair contains 2 slots).
		slotIndex += 2
	}

	// Levels traversing
	if tree.slotsLevelIndex > 0 {
		levelIndex := tree.slotsLevelIndex - 1

		for {
			for _, p := range tree.levels[levelIndex] {
				processPair(p, levelIndex)
			}

			if levelIndex == 0 {
				break
			}

			levelIndex--
		}
	}

Exit:
	if tree.root.IsNil() {
		err = ErrRootIsNil
		return
	}

	copy(root[:], tree.root.data[:])
	return
}

// CatalanNumbersCoding returns binary (BigEndian) coded orange tree, that is a subset of the current merkle tree.
// The coding format is the next: <orange tree structure coding>, <sequence of hashes of the orange tree>.
// Orange tree structure consists of ~2 bits info per node and 1 bit that defines if the node is MH or not.
//    This data is packed into bits slice and is aligned to the 8 bits (one byte).
//
// Sequence of hashes of the orange tree is a simple bytes slice
// which is filled with the hashes of the nodes of the orange tree
// in the same order as they are located in the structure coding sequence.
func (tree *SparseMerkleTree) CatalanNumbersCoding() (coding []byte, bitsCoded uint32, err error) {

	// Seems like current version of Golang has no native way to convert byte <-> bool.
	// This method implements exactly this conversion.
	boolToUint8 := func(b bool) (u uint8) {
		if b == true {
			return 1
		}

		return 0
	}

	isShardMined := func(n *node) bool {
		return !n.IsMH() && !n.IsNil()
	}

	// Returns "true" if all shards in tree are NOT mined
	// (nil or set to MH, by default).
	allShardsAreNotMined := func() bool {
		for _, pair := range tree.levels[tree.slotsLevelIndex] {
			if isShardMined(pair.leftNode) || isShardMined(pair.rightNode) {
				return false
			}
		}

		return true
	}

	// Returns "true" if all shards in tree ARE mined.
	allShardsAreMined := func() bool {
		for _, pair := range tree.levels[tree.slotsLevelIndex] {
			if !isShardMined(pair.leftNode) || !isShardMined(pair.rightNode) {
				return false
			}
		}

		return true
	}

	if allShardsAreNotMined() || allShardsAreMined() {
		// Corner case: no coding should be generated.
		return
	}

	// Force tree recalculation.
	// In case if tree has been calculate already and there was no changes -
	// this operation is noop, so it is save for performance to call it here.
	_, err = tree.Root()
	if err != nil {
		return
	}

	// Source data:
	// The sequence of shards that are mined is represented by the slice S
	// which consists of bytes set to "1" (shard is mined) or "0" (shard is not mined).
	si := 0
	S := make([]uint8, tree.slotsCount)
	for _, pair := range tree.levels[tree.slotsLevelIndex] {
		S[si] = boolToUint8(isShardMined(pair.leftNode))
		si++

		S[si] = boolToUint8(isShardMined(pair.rightNode))
		si++
	}

	// The tree depth is d, the amount of shards in the tree is N, 2^{d-1}<N<=2^d.
	//
	// Creating slice Shards of size 2^{d+1}.
	// This slice describes the topology of the tree in the next way:
	//
	// Item with index 0=“0…00” is ignored.
	// Item with index 1=“0…01” is root.
	//
	// Let's assume the X is the index of the parent node,
	// then left child index is (2*x), and right child index is (2*x+1).
	//
	// The slice S should be copied into slice Shards, starting from (2^d) up to (including) (2^{d+1}-1)
	// The coding of the items in Shards is the next:
	// - “0” all shards in a subtree are NOT mined. The Magic Hash (MH) will occur here on the hash calculation stage.
	// - “1” all shards in a subtree are mined. The regular hash will occur here on the hash calculation stage.
	// - “2” only some amount of shards are mined. This is a internal node of the Orange Tree.
	d := tree.Height() - 1
	Shards := make([]uint8, int(math.Pow(2, float64(d+1)))+1)

	SOffset := 0
	ShardsOffsetFrom := int(math.Pow(2, float64(d)))
	ShardsOffsetTo := int(math.Pow(2, float64(d+1)) - 1)
	for ; ShardsOffsetFrom <= ShardsOffsetTo; ShardsOffsetFrom++ {
		Shards[ShardsOffsetFrom] = S[SOffset]
		SOffset++
	}

	for i := int(math.Pow(2, float64(d+1)) - 1); i > 1; i -= 2 {
		if Shards[i] == Shards[i-1] {
			// если дети одинаковые то и перент такой же
			Shards[i/2] = Shards[i]
		} else {
			Shards[i/2] = 2 // если дети разные то перент “оранжевый”
		}
	}

	// Topology and nodes types bits calculation.
	topologySet = &big.Int{}
	nodesTypesSet = &big.Int{}
	topologyOffset = 0
	nodesTypesOffset = 0

	addMark(1, Shards)
	if topologySet.BitLen() == 0 {
		return
	}

	// 2 trailing zeroes of the coding could be omitted:
	// each coding regardless of a structure would contain 2 trailing zeroes.
	// (bits are skipped by replacing them with bits from nodes types bits sequence)
	topologyOffset = topologyOffset - 2
	if topologyOffset < 0 {
		topologyOffset = 0
	}

	// Setting trailing guard bit.
	// This bit ensures all bits from topologySet would be included into the bytes sequence,
	// even if there are trailing zeroes in it.
	// Trailing bit will replace one last bit of topologySet with 1.
	// The original bit value would be remembered and restored further in binary representation.
	trailingBitGuardIndex := topologyOffset - 1 // last bit.
	if trailingBitGuardIndex < 0 {
		trailingBitGuardIndex = 0
	}

	trailingBitGuardSet := false
	if topologySet.Bit(trailingBitGuardIndex) == 0 {
		topologySet = topologySet.SetBit(topologySet, trailingBitGuardIndex, 1)
		trailingBitGuardSet = true
	}

	// Copying bits from nodes types bits sequence
	// (chaining 2 sequences into one)
	for i := uint(0); i < uint(nodesTypesOffset); i++ {
		bit := nodesTypesSet.Bit(int(i))
		topologySet.SetBit(topologySet, topologyOffset, bit)
		topologyOffset++
	}

	coding = topologySet.Bytes()
	bitsCoded = uint32(topologyOffset)

	if trailingBitGuardSet {
		// Restoring original trailing guard bit value.
		destinationByteIndex := trailingBitGuardIndex / 8
		destinationBitOffset := trailingBitGuardIndex - destinationByteIndex*8

		// Bytes being generated from topologySet are mirrored in reverse order.
		// To patch some specific byte in result sequence - some additional indexing is needed.
		mirroredDestinationByteIndex := len(coding) - destinationByteIndex - 1
		b := coding[mirroredDestinationByteIndex]
		b = clearBit(b, uint(destinationBitOffset))
		coding[mirroredDestinationByteIndex] = b
	}

	return
}

func (tree *SparseMerkleTree) MarshalOrangeTreeLeafs() (data []chainhash.Hash) {

	var fillHashes func(n *node)
	fillHashes = func(n *node) {
		if n.isOrange == false {
			return
		}

		if n.leftChild.isOrange {
			fillHashes(n.leftChild)
		} else if n.leftChild.IsMH() == false {
			data = append(data, *n.leftChild.data)
		}

		if n.rightChild.isOrange {
			fillHashes(n.rightChild)
		} else if n.rightChild.IsMH() == false {
			data = append(data, *n.rightChild.data)
		}
	}

	data = make([]chainhash.Hash, 0, int(tree.slotsCount))
	n := tree.root
	fillHashes(n)
	return
}

func (tree *SparseMerkleTree) MerkleProofPath(position uint32) (pathData []chainhash.Hash, err error) {
	node, err := tree.getNode(position)
	if err != nil {
		return
	}

	if node.IsNil() || node.IsMH() {
		// Leaf is not initialised.
		// No merkle path could be built.
		err = ErrLeafIsNil
		return
	}

	// Merkle proof path can't be build without tree's data set.
	// tree.Root() caches tree's internal state, so no root recalculation would happen
	// each time when MerkleProofPath() would be called.
	_, err = tree.Root()
	if err != nil {
		return
	}

	var (
		correspondingPairIndex = position
		correspondingPair      *pair
	)

	totalLevelsCount := tree.HeightWithoutRoot()
	pathHashesCount := totalLevelsCount
	pathSize := int(pathHashesCount)
	pathData = make([]chainhash.Hash, 0, pathSize)

	for level := totalLevelsCount; level > 0; level-- {
		correspondingPairIndex = correspondingPairIndex / 2
		levelIndex := level - 1
		correspondingPair = tree.levels[levelIndex][correspondingPairIndex]

		isProcessedItemLeft := position%2 == 0

		if isProcessedItemLeft {
			pathData = append(pathData, *correspondingPair.rightNode.data)
		} else {
			pathData = append(pathData, *correspondingPair.leftNode.data)
		}

		position = position / 2
	}

	return
}

func (tree *SparseMerkleTree) ValidateOrangeTree(
	codingBitsSize uint32, coding []byte, hashes []chainhash.Hash, mmNumber uint32, expectedRoot chainhash.Hash) (err error) {

	// Shortcut method for wrapping various errors into ErrValidation.
	validationError := func(description string) (err error) {
		return fmt.Errorf(fmt.Sprintf(description)+": %v", ErrValidation)
	}

	// Shortcut method for checking bit positiveness.
	bitIsPositive := func(b byte) bool {
		return b != 0
	}

	if codingBitsSize == 0 {
		return validationError("coding bits size must be greater than zero")
	}

	if len(hashes) == 0 || len(coding) == 0 {
		return validationError("coding bits size is non zero, but hashes or coding is empty")
	}

	// Hashes are received as byte slice.
	// Here is the check that the size of the byte slice is aligned to the hash size.
	// if len(hashes)%chainhash.HashSize != 0 {
	// 	return validationError("invalid hashes slice: it must aligned to the hash size")
	// }

	hashesCount := len(hashes)

	// Coding must always be divisible by 3.
	if codingBitsSize%3 != 0 {
		return validationError("codingBitsSize must be divisible by 3")
	}

	// Checking correspondence of the coding's size to the tree size.
	// The expected correspondence is the next:
	// 	d=1		→  size<=3 bit
	//	d=2		→  size<=6 bit
	//	d=3		→  size<=12 bit
	//	d=4		→  size<=48 bit
	//	d>=5	→  size<=18*d bit
	d := tree.HeightWithoutRoot()
	if d < 1 {
		return validationError("invalid tree structure")
	}

	maxCodingSizeUpTo4 := []uint32{0, 3, 9, 21, 45}
	if d <= 4 {
		if codingBitsSize > maxCodingSizeUpTo4[d] {
			return validationError("invalid coding size (greater than maximal expected)")
		}

	} else {
		// d > 4
		maxCodingSize := uint32(math.Pow(18, float64(d)))
		if codingBitsSize > maxCodingSize {
			return validationError("invalid coding size (greater than maximal expected)")
		}
	}

	k := codingBitsSize / 3
	bits := (&big.Int{}).SetBytes(coding)
	topologyCodingFirstBit := uint32(0)
	topologyCodingLastBitIndex := (2 * k) - 1 - 1

	nodesTypesCodingFirstBitIndex := topologyCodingLastBitIndex + 1
	nodesTypesCodingLastBitIndex := nodesTypesCodingFirstBitIndex + k

	// todo: comment about proxy
	getTopologyBit := func(pos int) byte {
		if uint32(pos) > topologyCodingLastBitIndex && uint32(pos) <= topologyCodingLastBitIndex+2 {
			return 0
		}

		return byte(bits.Bit(pos))
	}

	// Checking the amount of hashes according to the nodes types bits sequence.
	// For each positive bit in nodes coding there must be corresponding hash in hashes slice.
	expectedHashesCount := 0
	for i := nodesTypesCodingFirstBitIndex; i <= nodesTypesCodingLastBitIndex; i++ {
		if bitIsPositive(byte(bits.Bit(int(i)))) {
			expectedHashesCount++
		}
	}

	if expectedHashesCount != hashesCount {
		return validationError("expected hashes count does not correspond to the present hashes count")
	}

	// Checking the topology coding.
	var (
		i = 0
		j = 0

		// The algorithm compares this bit 0, so
		// it must not be equal to zero on first iteration.
		previousBit = byte(1)

		// By default, each topology coding consist 2 trailing zeroes,
		// that are escaped during transporting and must be restored during validation.
		alignedTopologyCodingLastBitIndex = topologyCodingLastBitIndex + 2
	)
	for index := topologyCodingFirstBit; index <= alignedTopologyCodingLastBitIndex; index++ {
		bit := getTopologyBit(int(index))
		if bitIsPositive(bit) {
			i++
			j++

		} else { // bit is negative
			j--

			if previousBit == 0 {
				i--
			}
		}

		if i > int(d) {
			return validationError("orange tree deepness error")
		}
		if j < 0 && index != alignedTopologyCodingLastBitIndex {
			return validationError("orange tree deepness error")
		}

		previousBit = bit
	}

	// Checking the amount of positive bits in the coding.
	positiveBitsCount := 0
	for topologyIndex := topologyCodingFirstBit; topologyIndex <= topologyCodingLastBitIndex; topologyIndex++ {
		bit := bits.Bit(int(topologyIndex))
		if bit != 0 {
			positiveBitsCount++
		}
	}
	if uint32(positiveBitsCount) != k {
		err = fmt.Errorf("invalid tree structure occured: %v", ErrValidation)
		return
	}

	// MM Number check
	j = 0
	t := uint32(math.Pow(2, float64(d)))
	previousBit = 1
	positiveBitsCount = 0
	typesIndex := int(nodesTypesCodingFirstBitIndex)
	for topologyIndex := topologyCodingFirstBit; topologyIndex <= topologyCodingLastBitIndex+2; topologyIndex++ {
		bit := getTopologyBit(int(topologyIndex))
		if bitIsPositive(bit) {
			t /= 2

		} else {
			if previousBit == 0 {
				t *= 2
			}

			typesBit := nodesTypesSet.Bit(typesIndex)
			typesIndex++
			if typesBit == 0 {
				j += int(t)
			}
		}

		previousBit = bit
	}

	if (mmNumber + uint32(j)) < uint32(math.Pow(2, float64(d))) {
		err = fmt.Errorf("invalid MMNumber: %v", ErrValidation)
		return
	}

	// Restoring original hashes count.
	// Merging received hashes with MHs according to nodes topology.
	nextNonMHHashIndex := 0
	restoredHashesSequence := make([]chainhash.Hash, 0, hashesCount)
	for i := nodesTypesCodingFirstBitIndex; i < codingBitsSize; i++ {
		if bits.Bit(int(i)) == 0 { // bit is negative
			restoredHashesSequence = append(restoredHashesSequence, MH)
		} else {
			restoredHashesSequence = append(restoredHashesSequence, hashes[nextNonMHHashIndex])
			nextNonMHHashIndex += 1
		}
	}

	var (
		calculateRoot         func() (hash chainhash.Hash)
		nextRestoredHashIndex = 0
		nextTopologyBiIndex   = 0
	)
	calculateRoot = func() (hash chainhash.Hash) {
		bit := getTopologyBit(nextTopologyBiIndex)
		nextTopologyBiIndex++

		if bit == 0 {
			copy(hash[:], restoredHashesSequence[nextRestoredHashIndex][:])
			nextRestoredHashIndex++
			return

		} else {
			h1 := calculateRoot()
			h2 := calculateRoot()

			concatBuffer := make([]byte, 0, chainhash.HashSize)
			concatBuffer = append(concatBuffer, h1[:]...)
			concatBuffer = append(concatBuffer, h2[:]...)
			h := chainhash.HashH(concatBuffer)
			copy(hash[:], h[:])
			return
		}
	}

	root := calculateRoot()
	if bytes.Compare(root[:], expectedRoot[:]) != 0 {
		err = fmt.Errorf("calculated root does not corresponds to the expected root: %v", ErrValidation)
		return
	}

	return
}

func AggregateOrangeTree(result []byte, coding []byte, codingSize, nShards uint32) {
	l := chainhash.NextPowerOfTwo(int(nShards))*2 - 1
	if len(result) < l {
		res := make([]byte, l)
		copy(res[:], result[:])
		result = res
	}

	if codingSize == 0 {
		result[0] += 1
		return
	}

	k := codingSize / 3

	topologyCodingFirstBit := uint32(0)
	topologyCodingLastBitIndex := (2 * k) - 1 - 1

	nodesTypesCodingFirstBitIndex := topologyCodingLastBitIndex + 1
	// nodesTypesCodingLastBitIndex := nodesTypesCodingFirstBitIndex + k

	topologyBits := (&big.Int{}).SetBytes(coding)
	getTopologyBit := func(pos uint32) byte {
		if pos > topologyCodingLastBitIndex && pos <= topologyCodingLastBitIndex+2 {
			return 0
		}

		return byte(topologyBits.Bit(int(pos)))
	}

	var v = topologyCodingFirstBit
	var u = nodesTypesCodingFirstBitIndex

	var traverseTreeStep func(nodeIndex uint32)
	treeIndexLimit := uint32(chainhash.NextPowerOfTwo(int(nShards))) + nShards - 2

	traverseTreeStep = func(nodeIndex uint32) {
		if getTopologyBit(v) == 1 {
			v++
			traverseTreeStep(uint32(2*nodeIndex) + 1)
			v++
			traverseTreeStep(uint32(2*nodeIndex) + 2)
		}

		if len(result) > int(2*nodeIndex)+2 {
			return
		}

		if result[(2*nodeIndex)+1] > 0 && result[(2*nodeIndex)+2] > 0 {
			result[(2*nodeIndex)+1] -= 1
			result[(2*nodeIndex)+2] -= 1
			result[nodeIndex] += 1
			return
		}

		bit := byte(topologyBits.Bit(int(u)))
		result[nodeIndex] += bit
		if nodeIndex > treeIndexLimit {
			result[nodeIndex] += 1
			return
		}

		u++
	}

	traverseTreeStep(0)

	return
}

func (tree *SparseMerkleTree) ValidateShardMerkleProofPath(position, shardsCount uint32,
	shardMerkleProof []chainhash.Hash, expectedShardHash, expectedRootHash chainhash.Hash) (err error) {

	height := int(math.Ceil(math.Log2(float64(shardsCount))))
	if len(shardMerkleProof) != height {
		err = fmt.Errorf("%w: the shardMerkleProof(%d) size must have %d members",
			ErrValidation, len(shardMerkleProof), height)
		return
	}

	totalLevelsCount := len(shardMerkleProof)
	correspondingPairIndex := position

	concatBuffer := make([]byte, 0, 2*chainhash.HashSize)
	calculatedHash := expectedShardHash[:]
	nextPathHashIndex := 0

	for level := totalLevelsCount; level > 0; level-- {
		nextHashFromPath := shardMerkleProof[nextPathHashIndex]
		correspondingPairIndex = correspondingPairIndex / 2

		isProcessedItemLeft := position%2 == 0
		if isProcessedItemLeft {
			concatBuffer = append(concatBuffer, calculatedHash[:]...)
			concatBuffer = append(concatBuffer, nextHashFromPath[:]...)
		} else {
			// note: reverse order here
			concatBuffer = append(concatBuffer, nextHashFromPath[:]...)
			concatBuffer = append(concatBuffer, calculatedHash[:]...)
		}

		hash := chainhash.HashH(concatBuffer)
		calculatedHash = hash[:]

		position = position / 2
		concatBuffer = concatBuffer[:0]
		nextPathHashIndex += 1
	}

	if bytes.Compare(expectedRootHash[:], calculatedHash) != 0 {
		err = fmt.Errorf("root does not match: %w", ErrValidation)
		return
	}

	return
}

func (tree *SparseMerkleTree) getNode(position uint32) (correspondingNode *node, err error) {
	if position >= uint32(tree.slotsCount) {
		err = ErrInvalidShardPosition
		return
	}

	correspondingPairIndex := position / 2
	correspondingPair := tree.levels[tree.slotsLevelIndex][correspondingPairIndex]

	if position%2 == 0 {
		correspondingNode = correspondingPair.leftNode

	} else {
		correspondingNode = correspondingPair.rightNode
	}

	return
}

func (tree *SparseMerkleTree) markSlotAsModified(i slotsSpace) {
	tree.setSlotBit(i, 1)
}

func (tree *SparseMerkleTree) markSlotAsNonModified(i slotsSpace) {
	tree.setSlotBit(i, 0)
}

func (tree *SparseMerkleTree) setSlotBit(bit slotsSpace, state uint) {
	tree.modifiedShardsIndex.SetBit(&tree.modifiedShardsIndex, int(bit), state)
}

func (tree *SparseMerkleTree) isSlotChanged(slot slotsSpace) bool {
	return tree.modifiedShardsIndex.Bit(int(slot)) == 1
}

// Sets corresponding bits of each one slot to 0
// to prevent redundant root recalculations.
func (tree *SparseMerkleTree) markWholeTreeAsCached() {
	var i slotsSpace = 0
	for i = 0; i < tree.slotsCount; i++ {
		tree.markSlotAsNonModified(i)
	}
}

func (tree *SparseMerkleTree) containsChanges() (changesPresent bool) {
	// Fast check for the updates in the leafs.
	// If modifiedShardsIndex equals to 0 then it contains no bits set to 1 ->
	// there are no changes has been made in the tree and there is no need for further analise.
	if tree.modifiedShardsIndex.IsInt64() {
		changesPresent = tree.modifiedShardsIndex.Int64() != 0
		return

	} else {
		for i := 0; slotsSpace(i) < tree.slotsCount; i++ {
			if tree.modifiedShardsIndex.Bit(i) == 1 {
				return true
			}
		}
	}

	return
}

// markAllNodesAsRegular marks each one node of the tree as not belonging to the orange subtree.
func (tree *SparseMerkleTree) markAllNodesAsRegular() {
	tree.root.UnsetOrange()
	for _, level := range tree.levels {
		for _, p := range level {
			if !p.leftNode.IsNil() {
				p.leftNode.UnsetOrange()
			}

			if !p.rightNode.IsNil() {
				p.rightNode.UnsetOrange()
			}
		}
	}
}

// levelsCount returns current count of levels of the tree.
// It assumes that tree is aligned by the levels amount.
func levelsCount(slotsCount slotsSpace) (count levelsSpace) {
	count = 2 // At least root and one leafs level.
	var currentLevelLeafsCount slotsSpace = 1

	for {
		if (currentLevelLeafsCount * 2) == slotsCount {
			break
		}

		currentLevelLeafsCount *= 2
		count++
	}
	return
}

func addMark(nodeIndex int, D []uint8) {
	if D[nodeIndex] == 2 {
		// todo: skip first bit also
		topologySet = topologySet.SetBit(topologySet, topologyOffset, 1)
		topologyOffset++

		addMark(2*nodeIndex, D)
		addMark(2*nodeIndex+1, D)

	} else {
		topologySet = topologySet.SetBit(topologySet, topologyOffset, 0)
		topologyOffset++

		nodesTypesSet = nodesTypesSet.SetBit(nodesTypesSet, nodesTypesOffset, uint(D[nodeIndex]))
		nodesTypesOffset++
	}
}

func clearBit(b byte, pos uint) byte {
	mask := byte(^(1 << pos))
	b &= mask
	return b
}
