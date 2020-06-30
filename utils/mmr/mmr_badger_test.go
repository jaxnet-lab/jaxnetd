package mmr

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/sha3"
	"math/big"
	"testing"
)

func TestBadgerProof(t *testing.T) {
	mmrDB, err := BadgerDB("./data/mmr")
	assert.NoError(t, err)

	mmrInstance := Mmr(sha3.New256, mmrDB)

	var i uint64
	for i < 10 {
		data := make([]byte, 32)
		data[0] = byte(i)
		//rand.Read(data[:32])
		mmr := mmrInstance.Append(i, big.NewInt(int64(i)), data[:])
		i++
		fmt.Printf("\t%d. mmr: %x\n", i, mmr)
	}

	fmt.Println("Blocks")
	for _, id := range mmrDB.Blocks() {
		item, _ := mmrDB.GetBlock(id)
		fmt.Printf("\t%d. hash: %x %v\n", id, item.Hash, item.Weight)
	}
	fmt.Println()
	fmt.Println("Nodes")
	for _, id := range mmrDB.Nodes() {
		item, _ := mmrDB.GetNode(id)
		fmt.Printf("\t%d. hash: %x %v\n", id, item.Hash, item.Weight)
	}

}
