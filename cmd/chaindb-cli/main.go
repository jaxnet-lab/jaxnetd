package main

import (
	"flag"
	"log"
	"path"
	"strconv"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/corelog"

	"gitlab.com/jaxnet/jaxnetd/database"
	_ "gitlab.com/jaxnet/jaxnetd/database/ffldb"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

var (
	flagDataDir = flag.String("path", "", "path to data directory")
	flagNetName = flag.String("net", "mainnet", "Name of network: [mainnet|testnet|fastnet]")
	flagChainID = flag.Int64("chain", 2, "0 for Beacon, 1...MAX_UINT32 for shards")
)

func main() {
	flag.Parse()

	logger := corelog.New("CHAN", zerolog.TraceLevel, corelog.Config{}.Default())
	blockchain.UseLogger(logger)

	var (
		err          error
		beaconDBPath                    = path.Join(*flagDataDir, *flagNetName, "beacon", "blocks_ffldb")
		dbPath                          = beaconDBPath
		chain        chainctx.IChainCtx = chainctx.NewBeaconChain(chaincfg.NetName(*flagNetName).Params())
	)
	if *flagChainID != 0 {
		dbPath = path.Join(*flagDataDir, *flagNetName, "shard_"+strconv.Itoa(int(*flagChainID)), "blocks_ffldb")
		chain, err = getChainCtx(chain, beaconDBPath)
		checkError(err)
	}

	db, err := database.Open("ffldb", chain, dbPath)
	checkError(err)

	err = blockchain.VerifyStateSanity(db)
	checkError(err)

	checkError(db.Close())
}

func getChainCtx(beacon chainctx.IChainCtx, beaconDBPath string) (chainctx.IChainCtx, error) {
	beaconDB, err := database.Open("ffldb", beacon, beaconDBPath)
	if err != nil {
		return nil, err
	}
	defer beaconDB.Close()

	shardID := uint32(*flagChainID)

	// Load the block from the database and return it.
	var block *jaxutil.Block
	err = beaconDB.View(func(dbTx database.Tx) error {
		var err error
		shardsData, _ := chaindata.DBGetShardGenesisInfo(dbTx)
		info := shardsData[shardID]

		block, err = chaindata.DBFetchBlockByHash(dbTx, info.ExpansionHash)
		return err
	})
	if err != nil {
		return nil, err
	}

	shardChainCtx := chainctx.ShardChain(shardID, chaincfg.NetName(*flagNetName).Params(), block.MsgBlock(), block.Height())
	return shardChainCtx, nil
}

func checkError(err error) {
	if err != nil {
		log.Fatal("ERROR:", err)
	}
}
