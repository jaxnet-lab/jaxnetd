// Copyright (c) 2015-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package ffldb

import (
	"fmt"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/corelog"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"

	"gitlab.com/jaxnet/jaxnetd/database"
)

var log = corelog.Disabled

const (
	dbType = "ffldb"
)

// parseArgs parses the arguments from the database Open/Create methods.
func parseArgs(funcName string, args ...interface{}) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("invalid arguments to %s.%s -- "+
			"expected database path and block network", dbType,
			funcName)
	}

	dbPath, ok := args[0].(string)
	if !ok {
		return "", fmt.Errorf("first argument to %s.%s is invalid -- "+
			"expected database path string", dbType, funcName)
	}

	return dbPath, nil
}

// openDBDriver is the callback provided during driver registration that opens
// an existing database for use.
func openDBDriver(chain chainctx.IChainCtx, args ...interface{}) (database.DB, error) {
	dbPath, err := parseArgs("Open", args...)
	if err != nil {
		return nil, err
	}

	return openDB(dbPath, chain, false)
}

// createDBDriver is the callback provided during driver registration that
// creates, initializes, and opens a database for use.
func createDBDriver(chain chainctx.IChainCtx, args ...interface{}) (database.DB, error) {
	dbPath, err := parseArgs("Create", args...)
	if err != nil {
		return nil, err
	}

	return openDB(dbPath, chain, true)
}

// useLogger is the callback provided during driver registration that sets the
// current logger to the provided one.
func useLogger(logger zerolog.Logger) {
	log = logger
}

func init() {
	// Register the driver.
	driver := database.Driver{
		DbType:    dbType,
		Create:    createDBDriver,
		Open:      openDBDriver,
		UseLogger: useLogger,
	}
	if err := database.RegisterDriver(driver); err != nil {
		panic(fmt.Sprintf("Failed to regiser database driver '%s': %v",
			dbType, err))
	}
}
