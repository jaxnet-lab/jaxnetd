// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"gitlab.com/jaxnet/jaxnetd/database"
	_ "gitlab.com/jaxnet/jaxnetd/database/ffldb"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const (
	minCandidates        = 1
	maxCandidates        = 20
	defaultNumCandidates = 5
	defaultDBType        = "ffldb"
)

var (
	jaxnetdHomeDir  = jaxutil.AppDataDir("jaxnetd", false)
	defaultDataDir  = filepath.Join(jaxnetdHomeDir, "data") //nolint
	knownDbTypes    = database.SupportedDrivers()           //nolint
	activeNetParams = &chaincfg.MainNetParams               //nolint
)

// config defines the configuration options for findcheckpoint.
//
// See loadConfig for details on the configuration load process.
// nolint
type config struct {
	DataDir        string `short:"b" long:"datadir" description:"Location of the jaxnetd data directory"`
	DBType         string `long:"dbtype" description:"Database backend to use for the Block Chain"`
	UseGoOutput    bool   `short:"g" long:"gooutput" description:"Display the candidates using Go syntax that is ready to insert into the btcchain checkpoint list"`
	NumCandidates  int    `short:"n" long:"numcandidates" description:"Max num of checkpoint candidates to show {1-20}"`
	RegressionTest bool   `long:"regtest" description:"Use the regression test network"`
	SimNet         bool   `long:"simnet" description:"Use the simulation test network"`
	TestNet3       bool   `long:"testnet" description:"Use the test network"`
}

// validDbType returns whether or not dbType is a supported database type.
// nolint
func validDbType(dbType string) bool {
	for _, knownType := range knownDbTypes {
		if dbType == knownType {
			return true
		}
	}

	return false
}

// netName returns the name used when referring to a bitcoin network.  At the
// time of writing, jaxnetd currently places blocks for testnet version 3 in the
// data and log directory "testnet", which does not match the Name field of the
// chaincfg parameters.  This function can be used to override this directory name
// as "testnet" when the passed active network matches types.TestNet.
//
// A proper upgrade to move the data and log directories for this network to
// "testnet" is planned for the future, at which point this function can be
// removed and the network parameter's name used instead.
// nolint
func netName(chainParams *chaincfg.Params) string {
	switch chainParams.Net {
	case wire.TestNet:
		return "testnet"
	default:
		return chainParams.Name
	}
}

// loadConfig initializes and parses the config using command line options
// nolint
func loadConfig() (*config, []string, error) {
	// Default config.
	cfg := config{
		DataDir:       defaultDataDir,
		DBType:        defaultDBType,
		NumCandidates: defaultNumCandidates,
	}

	// Parse command line options.
	parser := flags.NewParser(&cfg, flags.Default)
	remainingArgs, err := parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		}
		return nil, nil, err
	}

	// Multiple networks can't be selected simultaneously.
	funcName := "loadConfig"
	numNets := 0
	// Count number of network flags passed; assign active network params
	// while we're at it
	if cfg.TestNet3 {
		numNets++
		activeNetParams = &chaincfg.TestNet3Params
	}
	if cfg.SimNet {
		numNets++
		activeNetParams = &chaincfg.SimNetParams
	}
	if numNets > 1 {
		str := "%s: The testnet, regtest, and simnet params can't be " +
			"used together -- choose one of the three"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		parser.WriteHelp(os.Stderr)
		return nil, nil, err
	}

	// Validate database type.
	if !validDbType(cfg.DBType) {
		str := "%s: The specified database type [%v] is invalid -- " +
			"supported types %v"
		err := fmt.Errorf(str, "loadConfig", cfg.DBType, knownDbTypes)
		fmt.Fprintln(os.Stderr, err)
		parser.WriteHelp(os.Stderr)
		return nil, nil, err
	}

	// Append the network type to the data directory so it is "namespaced"
	// per network.  In addition to the block database, there are other
	// pieces of data that are saved to disk such as address manager state.
	// All data is specific to a network, so namespacing the data directory
	// means each individual piece of serialized data does not have to
	// worry about changing names per network and such.
	cfg.DataDir = filepath.Join(cfg.DataDir, netName(activeNetParams))

	// Validate the number of candidates.
	if cfg.NumCandidates < minCandidates || cfg.NumCandidates > maxCandidates {
		str := "%s: The specified number of candidates is out of " +
			"range -- parsed [%v]"
		err = fmt.Errorf(str, "loadConfig", cfg.NumCandidates)
		fmt.Fprintln(os.Stderr, err)
		parser.WriteHelp(os.Stderr)
		return nil, nil, err
	}

	return &cfg, remainingArgs, nil
}
