// Copyright (c) 2015-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/chaindata"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// blockRegionCmd defines the configuration options for the fetchblockregion
// command.
type blockRegionCmd struct{}

// blockRegionCfg defines the configuration options for the command.
var blockRegionCfg = blockRegionCmd{}

// Execute is the main entry point for the command.  It's invoked by the parser.
func (cmd *blockRegionCmd) Execute(args []string) error {
	// Setup the global config options and ensure they are valid.
	if err := setupGlobalConfig(); err != nil {
		return err
	}

	// Ensure expected arguments.
	if len(args) < 2 {
		return errors.New("required shard ID and block hash parameters are not specified")
	}
	if len(args) < 3 {
		return errors.New("required shard ID and start offset parameters are not " +
			"specified")
	}
	if len(args) < 4 {
		return errors.New("required shard ID region length parameters are not " +
			"specified")
	}

	// Parse arguments.
	shardID, err := parseShardID(args[0])
	if err != nil {
		return errors.New("wrong shardID format specified")
	}
	blockHash, err := chainhash.NewHashFromStr(args[1])
	if err != nil {
		return err
	}
	startOffset, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil {
		return err
	}
	regionLen, err := strconv.ParseUint(args[3], 10, 32)
	if err != nil {
		return err
	}

	// Load the block database.
	db, err := loadBlockDB(relevantChain(shardID))
	if err != nil {
		return err
	}
	defer db.Close()

	return db.View(func(tx database.Tx) error {
		log.Infof("Fetching block region %s<%d:%d>", blockHash,
			startOffset, startOffset+regionLen-1)
		region := database.BlockRegion{
			Hash:   blockHash,
			Offset: uint32(startOffset),
			Len:    uint32(regionLen),
		}
		startTime := time.Now()
		regionBytes, err := chaindata.RepoTx(tx).FetchBlockRegion(&region)
		if err != nil {
			return err
		}
		log.Infof("Loaded block region in %v", time.Since(startTime))
		log.Infof("Double Hash: %s", chainhash.DoubleHashH(regionBytes))
		log.Infof("Region Hex: %s", hex.EncodeToString(regionBytes))
		return nil
	})
}

// Usage overrides the usage display for the command.
func (cmd *blockRegionCmd) Usage() string {
	return "<block-hash> <start-offset> <length-of-region>"
}
