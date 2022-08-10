package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/mmr"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// cpu: Intel(R) Core(TM) i7-9700K CPU @ 3.60GHz
// 1000		Time elapsed: 0.854485 s
// 10_000	Time elapsed: 60.039169 s
// 20_000   Time elapsed: 242.663918 s
// 100_000	Time elapsed: 6296.853522 s
const defaultNumBlocks = 10_000

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

	writePPROF()

	// this trick is needed so that merkleTreeStore won't be reclaimed by GC
	log.SetOutput(ioutil.Discard)
	log.Println(c)

	fmt.Printf("Time elapsed: %f s\n", float64(time.Now().UnixNano()-start)/1e9)
}

func writePPROF() {
	for _, profile := range pprof.Profiles() {
		outFile, err := os.Create(profile.Name() + ".out")
		if err != nil {
			log.Println("Cannot create out file", err)
			return
		}
		err = profile.WriteTo(outFile, 0)
		if err != nil {
			log.Println("cannot write  profile", err)
			return
		}
	}
}
