/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package merged_mining_tree

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

func p2(x int) int {
	return int(math.Pow(2, float64(x)))
}

func failOnErr(err error, t *testing.T) {
	if err != nil {
		t.Fatal(err)
	}
}

var (
	shardsFixtures = []slot{
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
		{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3},
		{4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
		{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
		{6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6},
		{7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7},
	}

	defaultMH = slot{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
)

func assertCodingsAreEqual(coding []byte, reference string, t *testing.T) {
	var binaryRepresentation string
	for _, n := range coding {
		binaryRepresentation += fmt.Sprintf("%08b", n)
	}

	isOK := strings.Compare(binaryRepresentation, reference) == 0
	if !isOK {
		t.Fatal(binaryRepresentation)
	}
}

func assertBitsCoded(coded, expected uint32, t *testing.T) {
	if coded != expected {
		t.Fatal(fmt.Sprintf(
			"Coded bits does not correspond to the expected. Coded: %v, Expected: %v", coded, expected))
	}
}

func randomHash() (h chainhash.Hash) {
	rand.Read(h[:])
	return
}

func TestTreeNoSlots(t *testing.T) {
	tree := NewSparseMerkleTree(0)

	// Tree can't be created with 0 slots count.
	// In case if 0 is passed - the internal implementation creates minimal tree possible (with 2 slots)
	if tree.slotsCount != 2 {
		t.Fatal()
	}
}

// Generates topology according to assets/topology_a
func topologyA(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(2)
	return tree
}

// Generates topology according to assets/topology_b
func topologyB(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(2)
	err1 := tree.SetShardHash(0, shardsFixtures[2])
	err2 := tree.SetShardHash(1, shardsFixtures[3])
	for _, e := range []error{err1, err2} {
		failOnErr(e, t)
	}
	return tree
}

// Generates topology according to assets/topology_c
func topologyC(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(2)
	err := tree.SetShardHash(0, shardsFixtures[2])
	failOnErr(err, t)
	return tree
}

func topologyD(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(2)
	err := tree.SetShardHash(1, shardsFixtures[2])
	failOnErr(err, t)
	return tree
}

// Generates topology according to assets/topology_L_1
func topologyL1(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(8)
	err1 := tree.SetShardHash(0, shardsFixtures[2])
	err2 := tree.SetShardHash(1, shardsFixtures[2])
	err3 := tree.SetShardHash(2, shardsFixtures[2])
	err4 := tree.SetShardHash(3, shardsFixtures[2])
	for _, e := range []error{err1, err2, err3, err4} {
		failOnErr(e, t)
	}
	return tree
}

// Generates topology according to assets/topology_L_2
func topologyL2(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(8)
	err1 := tree.SetShardHash(0, shardsFixtures[2])
	err2 := tree.SetShardHash(1, shardsFixtures[2])
	err3 := tree.SetShardHash(2, shardsFixtures[2])
	err4 := tree.SetShardHash(3, shardsFixtures[2])
	err5 := tree.SetShardHash(4, shardsFixtures[2])
	err6 := tree.SetShardHash(5, shardsFixtures[2])
	for _, e := range []error{err1, err2, err3, err4, err5, err6} {
		failOnErr(e, t)
	}
	return tree
}

// Generates topology according to assets/topology_L_3
func topologyL3(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(8)
	err1 := tree.SetShardHash(0, shardsFixtures[2])
	err2 := tree.SetShardHash(1, shardsFixtures[2])
	err3 := tree.SetShardHash(2, shardsFixtures[2])
	err4 := tree.SetShardHash(3, shardsFixtures[2])
	err5 := tree.SetShardHash(4, shardsFixtures[2])
	for _, e := range []error{err1, err2, err3, err4, err5} {
		failOnErr(e, t)
	}
	return tree
}

// Generates topology according to assets/topology_S_1
func topologyS1(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(4)
	return tree
}

// Generates topology according to assets/topology_S_2
func topologyS2(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(4)
	err1 := tree.SetShardHash(0, shardsFixtures[2])
	err2 := tree.SetShardHash(1, shardsFixtures[2])
	err3 := tree.SetShardHash(2, shardsFixtures[2])
	err4 := tree.SetShardHash(3, shardsFixtures[2])
	for _, e := range []error{err1, err2, err3, err4} {
		failOnErr(e, t)
	}
	return tree
}

// Generates topology according to assets/topology_S_3
func topologyS3(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(4)
	err := tree.SetShardHash(1, shardsFixtures[2])
	failOnErr(err, t)
	return tree
}

// Generates topology according to assets/topology_S_4
func topologyS4(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(4)
	err1 := tree.SetShardHash(2, shardsFixtures[2])
	err2 := tree.SetShardHash(3, shardsFixtures[2])
	for _, e := range []error{err1, err2} {
		failOnErr(e, t)
	}
	return tree
}

// Generates topology according to assets/topology_S_5
func topologyS5(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(4)
	err1 := tree.SetShardHash(1, shardsFixtures[2])
	err2 := tree.SetShardHash(2, shardsFixtures[2])
	for _, e := range []error{err1, err2} {
		failOnErr(e, t)
	}
	return tree
}

// Generates topology according to assets/topology_S_6
func topologyS6(t *testing.T) *SparseMerkleTree {
	tree := NewSparseMerkleTree(4)
	err1 := tree.SetShardHash(1, shardsFixtures[2])
	err2 := tree.SetShardHash(2, shardsFixtures[2])
	err3 := tree.SetShardHash(3, shardsFixtures[2])
	for _, e := range []error{err1, err2, err3} {
		failOnErr(e, t)
	}
	return tree
}

func TestHeight(t *testing.T) {
	fixtures := [][]int{
		/*  1 item  -> Level 1 */ {1, 2},
		/*  2 items -> Level 2 */ {2, 2},
		/*  3 items -> Level 3 */ {p2(2) - 1, 3},
		/*  4 items -> Level 3 */ {p2(2), 3},

		/*  5 items -> Level 4 */ {p2(2) + 1, 4} /* ..  8 items -> Level 4 */, {p2(3), 4},
		/*  9 items -> Level 5 */ {p2(3) + 1, 5} /* .. 16 items -> Level 5 */, {p2(4), 5},
		/* 17 items -> Level 6 */ {p2(4) + 1, 6} /* .. 32 items -> Level 6 */, {p2(5), 6},
		/* 33 items -> Level 7 */ {p2(5) + 1, 7} /* .. 64 items -> Level 7 */, {p2(6), 7},

		// ...
		{p2(16), 17},
	}

	for _, fixture := range fixtures {
		shardsCount := uint32(fixture[0])
		expectedHeight := uint16(fixture[1])

		tree := NewSparseMerkleTree(shardsCount)
		if tree.Height() != expectedHeight {
			t.Error("Reported levels count ", tree.Height(), " != expected levels count", expectedHeight)
		}
	}
}

func TestLinking(t *testing.T) {
	checkRoot := func(tree *SparseMerkleTree) {
		if tree.root == nil {
			t.Fatal("2 / Tree's root can't be nil")
		}
	}

	checkPair := func(tree *SparseMerkleTree, level, x int) {
		if tree.levels[level][x] == nil {
			t.Fatal("Pair must not be nil")
		}

		if tree.levels[level][x].leftNode == nil {
			t.Error("2 / Pair's left node must not be nil")
		}

		if tree.levels[level][x].rightNode == nil {
			t.Error("2 / Pair's right node must not be nil")
		}
	}

	checkUpsideLinking := func(tree *SparseMerkleTree, level, x, parentX int) {
		fixtureIndex := rand.Intn(len(shardsFixtures))

		tree.levels[level-1][parentX].rightNode.data = &chainhash.Hash{}
		tree.levels[level-1][parentX].leftNode.data = &chainhash.Hash{}

		copy(tree.levels[level-1][parentX].rightNode.data[:], shardsFixtures[fixtureIndex][:])
		copy(tree.levels[level-1][parentX].leftNode.data[:], shardsFixtures[fixtureIndex][:])

		leftOK := bytes.Compare(tree.levels[level][x].parent.leftNode.data[:], shardsFixtures[fixtureIndex][:]) == 0
		rightOK := bytes.Compare(tree.levels[level][x].parent.rightNode.data[:], shardsFixtures[fixtureIndex][:]) == 0

		if !leftOK || !rightOK {
			t.Fatal("Upside linking seems to not to work properly")
		}
	}

	checkDownsideLinking := func(tree *SparseMerkleTree, level, x, leftChildrenX, rightChildrenX int) {
		tree.levels[level+1][leftChildrenX].rightNode.data = &chainhash.Hash{}
		tree.levels[level+1][leftChildrenX].leftNode.data = &chainhash.Hash{}
		leftChildrenFixture := rand.Intn(len(shardsFixtures))
		copy(tree.levels[level+1][leftChildrenX].rightNode.data[:], shardsFixtures[leftChildrenFixture][:])
		copy(tree.levels[level+1][leftChildrenX].leftNode.data[:], shardsFixtures[leftChildrenFixture][:])

		tree.levels[level+1][rightChildrenX].rightNode.data = &chainhash.Hash{}
		tree.levels[level+1][rightChildrenX].leftNode.data = &chainhash.Hash{}
		rightChildrenFixture := rand.Intn(len(shardsFixtures))
		copy(tree.levels[level+1][rightChildrenX].rightNode.data[:], shardsFixtures[rightChildrenFixture][:])
		copy(tree.levels[level+1][rightChildrenX].leftNode.data[:], shardsFixtures[rightChildrenFixture][:])

		leftA := bytes.Compare(
			tree.levels[level][x].childrenLeft.leftNode.data[:], shardsFixtures[leftChildrenFixture][:]) == 0

		leftB := bytes.Compare(
			tree.levels[level][x].childrenLeft.rightNode.data[:], shardsFixtures[leftChildrenFixture][:]) == 0

		rightA := bytes.Compare(
			tree.levels[level][x].childrenRight.leftNode.data[:], shardsFixtures[rightChildrenFixture][:]) == 0

		rightB := bytes.Compare(
			tree.levels[level][x].childrenRight.rightNode.data[:], shardsFixtures[rightChildrenFixture][:]) == 0

		if !leftA || !leftB || !rightA || !rightB {
			t.Fatal("Downside linking seems to not to work properly")
		}
	}

	{
		// 2 Levels
		levelIndex := 0
		tree := NewSparseMerkleTree(2)
		checkRoot(tree)
		checkPair(tree, levelIndex, 0)
	}

	{
		// 3 Levels
		levelIndex := 1
		tree := NewSparseMerkleTree(5)
		checkRoot(tree)

		// First level is not checked
		// (it is covered by the previous test)
		checkPair(tree, levelIndex, 0)
		checkPair(tree, levelIndex, 1)
		checkUpsideLinking(tree, levelIndex, 0, 0)
		checkUpsideLinking(tree, levelIndex, 1, 0)
	}

	{
		// 4 Levels
		levelIndex := 2
		tree := NewSparseMerkleTree(9)
		checkRoot(tree)

		checkDownsideLinking(tree, levelIndex-1, 0, 0, 1)
		// checkDownsideLinking(tree, levelIndex-1, 1, 2, 3)
	}
}

func TestShardsSetting(t *testing.T) {
	{
		// Sets only one shard.
		// Checks that data is going into corresponding shard.
		// Checks that no other shards has been modified.

		tree := NewSparseMerkleTree(2)
		err := tree.SetShardHash(0, shardsFixtures[0])
		if err != nil {
			t.Fatal()
		}

		// Checking data has been set into corresponding node.
		lastLevelIndex := tree.Height() - 1 - 1
		actualShardData := tree.levels[lastLevelIndex][0].leftNode.data
		if bytes.Compare(actualShardData[:], shardsFixtures[0][:]) != 0 {
			t.Fatal()
		}

		// Checking that paired node has not been changed.
		if tree.levels[lastLevelIndex][0].rightNode.IsMH() == false {
			t.Fatal()
		}
	}

	{
		// Sets two shards with different values.
		// Checks that data is going into corresponding shard.

		tree := NewSparseMerkleTree(2)
		err := tree.SetShardHash(0, shardsFixtures[0])
		if err != nil {
			t.Fatal()
		}

		err = tree.SetShardHash(1, shardsFixtures[1])
		if err != nil {
			t.Fatal()
		}

		// Checking data has been set into corresponding node.
		lastLevelIndex := tree.Height() - 1 - 1

		shardAData := tree.levels[lastLevelIndex][0].leftNode.data
		if bytes.Compare(shardAData[:], shardsFixtures[0][:]) != 0 {
			t.Fatal()
		}

		shardBData := tree.levels[lastLevelIndex][0].rightNode.data
		if bytes.Compare(shardBData[:], shardsFixtures[1][:]) != 0 {
			t.Fatal()
		}
	}
}

func TestShardDropping(t *testing.T) {
	{
		// Initializes the tree with 2 shards.
		// then drops both.
		// Checks that corresponding nodes has been updated and data is gone.

		tree := NewSparseMerkleTree(2)
		err := tree.SetShardHash(0, shardsFixtures[0])
		if err != nil {
			t.Fatal()
		}

		err = tree.SetShardHash(1, shardsFixtures[1])
		if err != nil {
			t.Fatal()
		}

		// Note: Shards setting is tested with another test.
		// No need to repeat this logic here.

		err = tree.DropShardHash(0)
		if err != nil {
			t.Fatal()
		}

		err = tree.DropShardHash(1)
		if err != nil {
			t.Fatal()
		}

		lastLevelIndex := tree.Height() - 1 - 1

		if tree.levels[lastLevelIndex][0].leftNode.data != nil {
			t.Fatal()
		}

		if tree.levels[lastLevelIndex][0].rightNode.data != nil {
			t.Fatal()
		}
	}
}

func TestSettingHashMarksSlotAsUpdated(t *testing.T) {
	tree := NewSparseMerkleTree(2)
	err := tree.SetShardHash(0, shardsFixtures[0])
	if err != nil {
		t.Fatal()
	}

	if !tree.isSlotChanged(0) {
		t.Fatal()
	}
}

func TestDroppingHashMarksSlotAsUpdated(t *testing.T) {
	tree := NewSparseMerkleTree(2)
	err := tree.SetShardHash(0, shardsFixtures[0])
	if err != nil {
		t.Fatal()
	}

	tree.markSlotAsNonModified(0)

	err = tree.DropShardHash(0)
	if err != nil {
		t.Fatal()
	}

	if !tree.isSlotChanged(0) {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_a.png
func TestTopologyA(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = shardsFixtures[0]
	}()

	tree := topologyA(t)

	// Both slots are set to nil by default.
	// But to force root recalculation - initial cache of the tree must be dropped.
	tree.markSlotAsModified(0)
	tree.markSlotAsModified(1)

	_, err := tree.Root()
	if err != ErrRootIsNil {
		t.Fatal("root is expected to be nil")
	}
}

// Tests /assets/tests/topology_b.png
func TestTopologyB(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyB(t)
	root, err := tree.Root()
	failOnErr(err, t)

	resultFixture := []byte{
		80, 90, 156, 106, 199, 11, 223, 250,
		70, 36, 142, 32, 37, 72, 63, 159,
		233, 151, 160, 227, 30, 210, 85, 89,
		228, 72, 183, 59, 126, 2, 185, 189,
	}

	if bytes.Compare(root[:], resultFixture[:]) != 0 {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_c.png
func TestTopologyC(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyC(t)
	root, err := tree.Root()
	failOnErr(err, t)

	if bytes.Compare(tree.levels[0][0].rightNode.data[:], MH[:]) != 0 {
		t.Fatal()
	}

	// Checking orange nodes placement
	if !tree.root.isOrange {
		t.Fatal()
	}

	resultFixture := []byte{
		219, 205, 0, 53, 68, 58, 116, 245,
		73, 70, 158, 114, 10, 72, 51, 130,
		131, 76, 217, 218, 155, 150, 15, 139,
		122, 167, 207, 223, 249, 2, 253, 251,
	}
	if bytes.Compare(root[:], resultFixture[:]) != 0 {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_d.png
func TestTopologyD(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyD(t)
	root, err := tree.Root()
	failOnErr(err, t)

	if bytes.Compare(tree.levels[0][0].leftNode.data[:], MH[:]) != 0 {
		t.Fatal()
	}

	// Checking orange nodes placement
	if !tree.root.isOrange {
		t.Fatal()
	}

	resultFixture := []byte{
		39, 243, 47, 187, 250, 194, 251, 187,
		206, 88, 177, 7, 82, 20, 75, 90,
		116, 70, 212, 185, 30, 75, 169, 15,
		253, 238, 48, 94, 145, 89, 128, 232,
	}
	if bytes.Compare(root[:], resultFixture[:]) != 0 {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_S_1.png
func TestTopologyS1(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS1(t)
	_, err := tree.Root()
	failOnErr(err, t)

	// Checking MH node placements
	if bytes.Compare(tree.root.data[:], MH[:]) != 0 {
		t.Fatal()
	}

	// Checking other nodes are not MH
	if tree.levels[0][0].leftNode.IsMH() {
		t.Fatal()
	}

	if tree.levels[0][0].rightNode.IsMH() {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_S_2.png
func TestTopologyS2(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS2(t)

	_, err := tree.Root()
	failOnErr(err, t)

	// Checking no nodes are MH
	if tree.root.IsMH() {
		t.Fatal()
	}

	coords := [][]int{
		{0, 0},
		{1, 0},
		{1, 1},
	}

	for _, c := range coords {
		level := c[0]
		x := c[1]

		if tree.levels[level][x].leftNode.IsMH() {
			t.Fatal()
		}

		if tree.levels[level][x].rightNode.IsMH() {
			t.Fatal()
		}
	}
}

// Tests /assets/tests/topology_S_3.png
func TestTopologyS3(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS3(t)

	_, err := tree.Root()
	failOnErr(err, t)

	// Checking for orange nodes
	if !tree.root.isOrange {
		t.Fatal()
	}

	if !tree.levels[0][0].leftNode.isOrange {
		t.Fatal()
	}

	// Checking for MH nodes
	if !tree.levels[0][0].rightNode.IsMH() {
		t.Fatal()
	}

	if !tree.levels[1][0].leftNode.IsMH() {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_S_4.png
func TestTopologyS4(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS4(t)
	_, err := tree.Root()
	failOnErr(err, t)

	// Checking for orange nodes
	if !tree.root.isOrange {
		t.Fatal()
	}

	// Checking for MH nodes
	if !tree.levels[0][0].leftNode.IsMH() {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_S_5.png
func TestTopologyS5(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS5(t)
	_, err := tree.Root()
	failOnErr(err, t)

	// Checking for orange nodes
	if !tree.root.isOrange {
		t.Fatal()
	}

	if !tree.levels[0][0].leftNode.isOrange {
		t.Fatal()
	}

	if !tree.levels[0][0].rightNode.isOrange {
		t.Fatal()
	}

	// Checking for MH nodes
	if !tree.levels[1][0].leftNode.IsMH() {
		t.Fatal()
	}

	if !tree.levels[1][1].rightNode.IsMH() {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_S_6.png
func TestTopologyS6(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS6(t)
	_, err := tree.Root()
	failOnErr(err, t)

	// Checking for orange nodes
	if !tree.root.isOrange {
		t.Fatal()
	}

	if !tree.levels[0][0].leftNode.isOrange {
		t.Fatal()
	}

	// Checking for MH nodes
	if !tree.levels[1][0].leftNode.IsMH() {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_L_1.png
func TestTopologyL1(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyL1(t)
	root, err := tree.Root()
	failOnErr(err, t)

	// Checking MH node placements
	if bytes.Compare(tree.levels[0][0].rightNode.data[:], MH[:]) != 0 {
		t.Fatal()
	}

	// Checking orange nodes placement
	if !tree.root.isOrange {
		t.Fatal()
	}

	resultFixture := []byte{
		189, 206, 41, 243, 38, 116, 189, 198,
		120, 194, 243, 125, 103, 58, 248, 136,
		16, 209, 65, 176, 34, 218, 83, 192,
		189, 189, 215, 57, 22, 67, 191, 68,
	}
	if bytes.Compare(root[:], resultFixture[:]) != 0 {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_L_2.png
func TestTopologyL2(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyL2(t)
	root, err := tree.Root()
	failOnErr(err, t)

	// Checking MH node placements
	if bytes.Compare(tree.levels[1][1].rightNode.data[:], MH[:]) != 0 {
		t.Fatal()
	}

	// Checking orange nodes placement
	if !tree.root.isOrange {
		t.Fatal()
	}

	if !tree.levels[0][0].rightNode.isOrange {
		t.Fatal()
	}

	// Comparing root
	resultFixture := []byte{
		121, 232, 67, 125, 141, 140, 159, 208, 11, 31, 86, 232, 49, 176,
		22, 142, 170, 29, 74, 230, 3, 1, 53, 97, 199, 125, 144, 156, 243, 185, 45, 184,
	}
	if bytes.Compare(root[:], resultFixture[:]) != 0 {
		t.Fatal()
	}
}

// Tests /assets/tests/topology_L_3.png
func TestTopologyL3(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyL3(t)
	root, err := tree.Root()
	failOnErr(err, t)

	// Checking MH node placements
	if bytes.Compare(tree.levels[1][1].rightNode.data[:], MH[:]) != 0 {
		t.Fatal()
	}

	if bytes.Compare(tree.levels[2][2].rightNode.data[:], MH[:]) != 0 {
		t.Fatal()
	}

	// Checking orange nodes placement
	if !tree.root.isOrange {
		t.Fatal()
	}

	if !tree.levels[0][0].rightNode.isOrange {
		t.Fatal()
	}

	if !tree.levels[1][1].leftNode.isOrange {
		t.Fatal()
	}

	// Comparing root
	resultFixture := []byte{
		233, 123, 96, 120, 70, 16, 75, 108, 1, 77, 64, 207, 173,
		166, 123, 214, 121, 22, 130, 142, 176, 138, 110, 212, 203, 97, 96, 131, 206, 220, 116, 138,
	}
	if bytes.Compare(root[:], resultFixture[:]) != 0 {
		t.Fatal()
	}
}

func TestOrangeTreeCodingTopologyB(t *testing.T) {
	tree := topologyB(t)

	coding, bitsCoded, err := tree.CatalanNumbersCoding()
	if err != nil {
		return
	}

	assertCodingsAreEqual(coding, "", t) // No coding is expected
	assertBitsCoded(bitsCoded, 0, t)
}

// Tests /assets/tests/topology_S_1.png
func TestOrangeTreeCodingTopologyS1(t *testing.T) {
	tree := topologyS1(t)
	coding, bitsCoded, err := tree.CatalanNumbersCoding()
	if err != nil {
		t.Fatal()
	}

	assertCodingsAreEqual(coding, "", t) // No coding is expected
	assertBitsCoded(bitsCoded, 0, t)
}

// Tests /assets/tests/topology_S_2.png
func TestOrangeTreeCodingTopologyS2(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS2(t)
	coding, bitsCoded, err := tree.CatalanNumbersCoding()
	if err != nil {
		t.Fatal()
	}

	assertCodingsAreEqual(coding, "", t) // No coding is expected
	assertBitsCoded(bitsCoded, 0, t)
}

// Tests /assets/tests/topology_S_3.png
func TestOrangeTreeCodingTopologyS3(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS3(t)
	coding, bitsCoded, err := tree.CatalanNumbersCoding()
	if err != nil {
		t.Fatal()
	}

	// Note: BigEndian here
	// (result bits sequence is inverted)
	//
	// Real coding is: <topology: 110><nodes types: 010> / 6 bits in total
	// The expected coding is: 00010011
	// (inverted original coding + trailed nodes types sequence)
	assertCodingsAreEqual(coding, "00010011", t)
	assertBitsCoded(bitsCoded, 6, t)
}

// Tests /assets/tests/topology_S_4.png
func TestOrangeTreeCodingTopologyS4(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS4(t)
	coding, bitsCoded, err := tree.CatalanNumbersCoding()
	if err != nil {
		t.Fatal()
	}

	// Note: BigEndian here
	// (result bits sequence is inverted)
	//
	// Real coding is: <topology: 1><nodes types: 01> / 3 bits in total
	// The expected coding is: 00000101
	// (inverted original coding + trailed nodes types sequence)
	assertCodingsAreEqual(coding, "00000101", t)
	assertBitsCoded(bitsCoded, 3, t)
}

// Tests /assets/tests/topology_S_5.png
func TestOrangeTreeCodingTopologyS5(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS5(t)
	coding, bitsCoded, err := tree.CatalanNumbersCoding()
	if err != nil {
		t.Fatal()
	}

	// Note: BigEndian here
	// (result bits sequence is inverted)
	//
	// Real coding is: <topology: 11001><nodes types: 0110> / 9 bits in total
	// The expected coding is: 11010011
	// (inverted original coding + trailed nodes types sequence)
	assertCodingsAreEqual(coding, "11010011", t)
	assertBitsCoded(bitsCoded, 9, t)
}

// Tests /assets/tests/topology_S_6.png
func TestOrangeTreeCodingTopologyS6(t *testing.T) {

	// By default, MH is set to 0000...0000 that is equal to the default value of hash.
	// There is a erroneous behavior is possible, when root calculation is not done as expected,
	// but the actual root wasn't calculated at all (and default BinHash values is used).
	// To prevent this type of cases -- in some tests MH is replaced by other constant,
	// that differs from default BinHash value.
	MH = shardsFixtures[1]
	defer func() {
		MH = defaultMH
	}()

	tree := topologyS6(t)
	coding, bitsCoded, err := tree.CatalanNumbersCoding()
	if err != nil {
		t.Fatal()
	}

	// Note: BigEndian here
	// (result bits sequence is inverted)
	//
	// Real coding is: <topology: 110><nodes types: 011> / 6 bits in total
	// The expected coding is: 00110011
	// (inverted original coding + trailed nodes types sequence)
	assertCodingsAreEqual(coding, "00110011", t)
	assertBitsCoded(bitsCoded, 6, t)
}

// Tests /assets/tests/topology_L_1.png
func TestOrangeTreeCodingTopologyL1(t *testing.T) {
	tree := topologyL1(t)
	coding, bitsCoded, err := tree.CatalanNumbersCoding()
	if err != nil {
		t.Fatal()
	}

	// Note: BigEndian here
	// (result bits sequence is inverted)
	//
	// Real coding is: <topology: 1><nodes types: 10> / 3 bits in total
	// The expected coding is: 00000011
	// (inverted original coding + trailed nodes types sequence)
	assertCodingsAreEqual(coding, "00000011", t)
	assertBitsCoded(bitsCoded, 3, t)
}

// Tests /assets/tests/topology_L_2.png
func TestOrangeTreeCodingTopologyL2(t *testing.T) {
	tree := topologyL2(t)
	coding, bitsCoded, err := tree.CatalanNumbersCoding()
	if err != nil {
		t.Fatal()
	}

	// Note: BigEndian here
	// (result bits sequence is inverted)
	//
	// Real coding is: <topology: 10100><nodes types: 110> / 6 bits in total
	// The expected coding is: 00011101
	// (inverted original coding + trailed nodes types sequence)
	assertCodingsAreEqual(coding, "00011101", t)
	assertBitsCoded(bitsCoded, 6, t)
}

// Tests /assets/tests/topology_L_3.png
func TestOrangeTreeCodingTopologyL3(t *testing.T) {
	tree := topologyL3(t)
	coding, bitsCoded, err := tree.CatalanNumbersCoding()
	if err != nil {
		t.Fatal()
	}

	// Note: BigEndian here
	// (result bits sequence is inverted)
	//
	// Real coding is: <topology: 10110><nodes types: 1100> / 9 bits in total
	// The expected coding is: 01101101
	// (inverted original coding + trailed nodes types sequence)
	assertCodingsAreEqual(coding, "01101101", t)
	assertBitsCoded(bitsCoded, 9, t)
}

// This test creates several topologies and corresponding codings and ensures
// that validation algorithm works well on correct topologies and data.
func TestValidationOnTopologiesRange(t *testing.T) {
	var topologies = []*SparseMerkleTree{
		// Topology A does not mine any shard (skipped).
		// Topology B generates empty coding, cause all shards are mined.
		topologyC(t),
		topologyD(t),

		// Topology S1 does not mine any shard (skipped).
		// Topology S2 generates empty coding, cause all shards are mined.
		topologyS3(t),
		topologyS4(t),
		topologyS5(t),
		topologyS6(t),

		topologyL1(t),
		topologyL2(t),
		topologyL3(t),
	}

	for _, sourceTree := range topologies {
		root, _ := sourceTree.Root()
		coding, codingLengthBits, _ := sourceTree.CatalanNumbersCoding()
		hashes := sourceTree.MarshalOrangeTreeLeafs()
		mmNumber := uint32(sourceTree.slotsCount)

		validationTree := NewSparseMerkleTree(mmNumber)
		err := validationTree.ValidateOrangeTree(codingLengthBits, coding, hashes, mmNumber, root)
		if err != nil {
			t.Fatal(err)
		}
	}
}

// This test checks several topologies and
// ensures validation algorithm returns coding error on broken codings.
func TestValidationBrokenTopologies(t *testing.T) {

	// Note: deterministic random is used.
	// deterministic random is used to fill the fixtures in a programmatic way,
	// and to store all of them in code.
	rand.Seed(1234)

	mockHashesArray := func() (s []chainhash.Hash) {
		count := rand.Intn(20)
		s = make([]chainhash.Hash, count)
		for i := range s {
			rand.Read(s[i][:])
		}

		return
	}

	hashesFixtures := make([][]chainhash.Hash, 10)
	for i := 0; i < 10; i++ {
		hashesFixtures[i] = mockHashesArray()
	}

	type topology struct {
		n        uint32
		mmNumber uint32
		coding   string
	}

	brokenCodings := []topology{
		{5, 3, "110010"},
		{8, 7, "110101"},
		{7, 4, "110010"},
		{16, 13, "110101"},
		{7, 3, "101010"},
		{13, 5, "101010"},
	}

	for _, topology := range brokenCodings {
		for _, hashSeq := range hashesFixtures {
			bits, ok := (&big.Int{}).SetString(topology.coding, 2)
			if !ok {
				t.Fatal()
			}

			tree := NewSparseMerkleTree(topology.n)
			err := tree.ValidateOrangeTree(uint32(len(topology.coding)), bits.Bytes(), hashSeq, topology.mmNumber, chainhash.Hash{})
			if err == nil {
				t.Fatal("error is expected")
			}
		}
	}
}

// Tree caches results of root calculation for performance reasons.
// This test ensures that cache is implemented right and the original result and cached one are equal.
func TestRepetitiveRootCalculation(t *testing.T) {
	tree := topologyS5(t)
	root1, err1 := tree.Root()
	failOnErr(err1, t)

	root2, err2 := tree.Root()
	failOnErr(err2, t)

	if bytes.Compare(root1[:], root2[:]) != 0 {
		t.Fatal("Calculated hashes must be equal")
	}
}

// This test emulates mining of the trees with 1..n shards sequentially mined.
func TestSequentialShardsMiningAndValidation(t *testing.T) {
	for shardsCount := 1; shardsCount <= 256; shardsCount++ {
		tree := NewSparseMerkleTree(uint32(shardsCount))

		for i := 0; i < shardsCount; i++ {
			err := tree.SetShardHash(uint32(i), shardsFixtures[3])
			failOnErr(err, t)
		}

		root, err := tree.Root()
		failOnErr(err, t)

		hashes := tree.MarshalOrangeTreeLeafs()
		coding, codingSize, err := tree.CatalanNumbersCoding()
		failOnErr(err, t)

		if codingSize == 0 {
			continue
		}

		validationTree := NewSparseMerkleTree(uint32(shardsCount))
		err = validationTree.ValidateOrangeTree(codingSize, coding, hashes, uint32(shardsCount), root)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestMerkleProofPathOnL1(t *testing.T) {
	tree := topologyL1(t)

	var position uint32
	for position = 0; position < 4; position++ {
		data, err := tree.MerkleProofPath(position)
		if err != nil {
			t.Error(err)
		}

		node, _ := tree.getNode(position)
		err = tree.ValidateShardMerkleProofPath(position, 8, data, *node.data, *tree.root.data)
		if err != nil {
			t.Error(err)
		}
	}
}
