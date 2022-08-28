package mmr

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

func TestTreeContainer_SetNodeToMmrWithReorganization(t *testing.T) {
	mmrContainer := TreeContainer{
		BlocksMMRTree: NewTree(),
		RootToBlock:   map[chainhash.Hash]chainhash.Hash{},
	}

	tBlockNodes := generateBlockNodeChain(t, &mmrContainer)

	toInsert := &TBlockNode{
		hash:          hash("we_insert_orphan"),
		prevMMRRoot:   tBlockNodes[5].actualMMRRoot,
		actualMMRRoot: chainhash.Hash{},
		height:        6,
		difficulty:    tBlockNodes[5].PowWeight(),
		parent:        tBlockNodes[5],
	}
	ok := mmrContainer.SetNodeToMmrWithReorganization(toInsert)

	assert.Equal(t, true, ok)
	assert.Equal(t, mmrContainer.Current().Hash, toInsert.hash)
	assert.Equal(t, mmrContainer.lastNode.Hash, toInsert.hash)
}

func BenchmarkMMRTreeConstruction(b *testing.B) {
	sizes := []struct {
		name    string
		MMRSize int64
	}{
		{"100 blocks", 100},
		{"500 blocks", 500},
		{"1000 blocks", 1000},
		{"2000 blocks", 2000},
	}

	var c *TreeContainer
	log.SetOutput(ioutil.Discard)
	for _, size := range sizes {
		c = &TreeContainer{
			BlocksMMRTree: NewTree(),
			RootToBlock:   map[chainhash.Hash]chainhash.Hash{},
		}

		b.Run(size.name, func(b *testing.B) {
			GenerateBlockNodeChain(c, size.MMRSize)
		})
		log.Println(c)
	}
}

func generateBlockNodeChain(t *testing.T, c *TreeContainer) []*TBlockNode {
	blocks := getBlocks()

	blockNodes := make([]*TBlockNode, len(blocks))

	genesisBlockNode := &TBlockNode{
		hash:          blocks[0].Hash,
		prevMMRRoot:   chainhash.ZeroHash,
		actualMMRRoot: chainhash.Hash{},
		height:        0,
		difficulty:    blocks[0].Weight,
		parent:        nil,
	}
	c.SetNodeToMmrWithReorganization(genesisBlockNode)
	blockNodes[0] = genesisBlockNode

	for i := 1; i < len(blocks); i++ {
		prevMMRRoot := c.Current().Hash
		blNode := &TBlockNode{
			hash:          blocks[i].Hash,
			prevMMRRoot:   prevMMRRoot,
			actualMMRRoot: chainhash.Hash{},
			height:        int32(i),
			difficulty:    blocks[i].Weight,
			parent:        blockNodes[i-1],
		}
		ok := c.SetNodeToMmrWithReorganization(blNode)
		assert.Equal(t, true, ok)
		blockNodes[i] = blNode
	}

	return blockNodes
}
