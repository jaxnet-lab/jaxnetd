/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/node/mmr"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"

	// this is done fo fill the supported drivers
	_ "gitlab.com/jaxnet/jaxnetd/database/ffldb"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

func main() {
	chain := chainctx.NewBeaconChain(&chaincfg.TestNetParams)
	dataDir := "../data/testnet"
	db, err := loadBlockDB(chain, dataDir)
	if err != nil {
		log.Fatalln(err)
		return
	}

	err = db.View(func(dbTx database.Tx) error {
		serializedData := dbTx.Metadata().Get(chaindata.ChainStateKeyName)
		state, err := chaindata.DeserializeBestChainState(serializedData)
		if err != nil {
			return err
		}
		fmt.Println("Best State:")
		fmt.Printf("Genesis=%s\nLastBlock=%s\nMMRRoot=%s\nLastSerialID=%d\n",
			chain.Params().GenesisHash(), state.Hash, state.MMRRoot, state.LastSerialID)

		bestChain, err := chaindata.RepoTx(dbTx).GetBestChainSerialIDs()
		if err != nil {
			return err
		}

		blocksMMRRoots, err := chaindata.RepoTx(dbTx).GetBlocksMMRRoots()
		if err != nil {
			return errors.Wrap(err, "can't get blocks mmr roots")
		}

		buf := bytes.NewBuffer(nil)
		_, err = fmt.Fprintln(buf, "#,Height,SerialID,Hash,PrevBlockHash,ActualMMRRoot,PrevMMRRoot")
		if err != nil {
			return err
		}
		tree := mmr.NewTree()
		tree.PreAllocateTree(len(bestChain))

		for i, record := range bestChain {
			rawBlock, err := chaindata.RepoTx(dbTx).FetchBlock(record.Hash)
			if err != nil {
				return errors.Wrap(err, "can't fetch header")
			}

			block, err := wire.DecodeBlock(bytes.NewBuffer(rawBlock))
			if err != nil {
				return errors.Wrap(err, "can't decode header")
			}
			header := block.Header
			root := blocksMMRRoots[*record.Hash]
			_, err = fmt.Fprintf(buf, "%d,%d,%d,%s,%s,%s,%s\n", i, header.Height(), record.SerialID, record.Hash, header.PrevBlockHash(), root, header.PrevBlocksMMRRoot())
			if err != nil {
				return err
			}
			if root.IsZero() {
				tree.AddBlockWithoutRebuild(*record.Hash, *record.Hash, header.Height(), pow.CalcWork(header.Bits()))
			} else {
				tree.AddBlockWithoutRebuild(*record.Hash, root, header.Height(), pow.CalcWork(header.Bits()))
			}
		}
		// nolint:gofumpt,gomnd
		err = os.WriteFile("GetBestChainSerialIDs.csv", buf.Bytes(), 0644)
		if err != nil {
			return err
		}

		err = tree.RebuildTreeAndAssert()
		if err != nil {
			return err
		}
		fmt.Println("tree.CurrentRoot()", tree.CurrentRoot())
		fmt.Println("bestChain.last", bestChain[len(bestChain)-1].Hash)

		return nil
	})
	if err != nil {
		log.Fatalln(err)
		return
	}
}

// loadBlockDB opens the block database and returns a handle to it.
// nolint: gomnd
func loadBlockDB(chain chainctx.IChainCtx, dataDir string) (database.DB, error) {
	// The database name is based on the database type.
	dbName := "blocks_ffldb"
	dbPath := filepath.Join(dataDir, chain.Name(), dbName)
	log.Printf("Loading block database from '%s'\n", dbPath)
	db, err := database.Open("ffldb", chain, dbPath)
	if err != nil {
		return nil, err
	}

	log.Println("Block database loaded")
	return db, nil
}
