// Copyright (c) 2015-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"gitlab.com/jaxnet/jaxnetd/node/chain/beacon"
	"gitlab.com/jaxnet/jaxnetd/node/chain/shard"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/btcsuite/btclog"
	"github.com/jessevdk/go-flags"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/node/chain"
)

const (
	// blockDbNamePrefix is the prefix for the jaxnetd block database.
	blockDbNamePrefix = "blocks"
)

var (
	log             btclog.Logger
	shutdownChannel = make(chan error)
)

// loadBlockDB opens the block database and returns a handle to it.
func loadBlockDB(chain chain.IChainCtx) (database.DB, error) {
	// The database name is based on the database type.
	dbName := blockDbNamePrefix + "_" + cfg.DbType
	dbPath := filepath.Join(cfg.DataDir, getChainDir(chain.ShardID()), dbName)
	log.Infof("Loading block database from '%s'", dbPath)
	db, err := database.Open(cfg.DbType, chain, dbPath)
	if err != nil {
		// Return the error if it's not because the database doesn't
		// exist.
		if dbErr, ok := err.(database.Error); !ok || dbErr.ErrorCode !=
			database.ErrDbDoesNotExist {

			return nil, err
		}

		// Create the db if it does not exist.
		err = os.MkdirAll(cfg.DataDir, 0700)
		if err != nil {
			return nil, err
		}
		db, err = database.Create(cfg.DbType, chain, dbPath)
		if err != nil {
			return nil, err
		}
	}

	log.Info("Block database loaded")
	return db, nil
}

// realMain is the real main function for the utility.  It is necessary to work
// around the fact that deferred functions do not run when os.Exit() is called.
func realMain() error {
	// Setup logging.
	backendLogger := btclog.NewBackend(os.Stdout)
	defer os.Stdout.Sync()
	log = backendLogger.Logger("MAIN")
	// database.UseLogger(dbLog)

	// Setup the parser options and commands.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	parserFlags := flags.Options(flags.HelpFlag | flags.PassDoubleDash)
	parser := flags.NewNamedParser(appName, parserFlags)
	parser.AddGroup("Global Options", "", cfg)
	parser.AddCommand("insecureimport",
		"Insecurely import bulk block data from bootstrap.dat",
		"Insecurely import bulk block data from bootstrap.dat.  "+
			"WARNING: This is NOT secure because it does NOT "+
			"verify chain rules.  It is only provided for testing "+
			"purposes.", &importCfg)
	parser.AddCommand("loadheaders",
		"Time how long to load headers for all blocks in the database",
		"", &headersCfg)
	parser.AddCommand("fetchblock",
		"Fetch the specific block hash from the database", "",
		&fetchBlockCfg)
	parser.AddCommand("fetchblockregion",
		"Fetch the specified block region from the database", "",
		&blockRegionCfg)
	parser.AddCommand("export-indexes",
		"Exports blockchain data in csv format", "",
		&ExportIndexesCfg)

	// Parse command line and invoke the Execute function for the specified
	// command.
	if _, err := parser.Parse(); err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		} else {
			log.Error(err)
		}

		return err
	}

	return nil
}

func main() {
	// Use all processor cores.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Work around defer not working after os.Exit()
	if err := realMain(); err != nil {
		os.Exit(1)
	}
}

func parseShardID(cliArg string) (uint32, error) {
	shardID, err := strconv.ParseUint(cliArg, 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(shardID), nil
}

func relevantChain(shardID uint32) chain.IChainCtx {
	if shardID > 0 {
		return shard.Chain(shardID, activeNetParams, beacon.Chain(activeNetParams).GenesisBlock().Header.BeaconHeader(), beacon.Chain(activeNetParams).GenesisBlock().Transactions[0])
	}

	return beacon.Chain(activeNetParams)
}

func getChainDir(shardID uint32) string {
	chainDir := "beacon"
	if shardID > 0 {
		return fmt.Sprintf("shard_%d", shardID)
	}

	return chainDir
}
