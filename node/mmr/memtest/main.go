package main

import (
	"flag"
	"fmt"
	"gitlab.com/jaxnet/jaxnetd/node/mmr"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"io/ioutil"
	"log"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

const defaultNumBlocks = 1_000

func main() {
	start := time.Now().UnixNano()
	var numBlocks int64
	flag.Int64Var(&numBlocks, "n", defaultNumBlocks, "number of blocks to insert in mmr tree")
	flag.Parse()

	c := &mmr.TreeContainer{
		BlocksMMRTree: mmr.NewTree(),
		RootToBlock:   map[chainhash.Hash]chainhash.Hash{},
	}

	mmr.GenerateBlockNodeChain(c, numBlocks)
	runtime.GC()
	outFile, err := os.Create("heap.out")
	if err != nil {
		log.Println("Cannot create heap out file", err)
		return
	}

	err = pprof.WriteHeapProfile(outFile)
	if err != nil {
		log.Println("cannot write heap profile", err)
		return
	}

	// this trick is needed so that merkleTreeStore won't be reclaimed by GC
	log.SetOutput(ioutil.Discard)
	log.Println(c)

	fmt.Printf("Time elapsed: %f s\n", float64(time.Now().UnixNano()-start)/1e9)
}
