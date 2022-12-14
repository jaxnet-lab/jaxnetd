// Copyright (c) 2015-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package database

import (
	"fmt"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
)

// Driver defines a structure for backend drivers to use when they registered
// themselves as a backend which implements the DB interface.
type Driver struct {
	// DBType is the identifier used to uniquely identify a specific
	// database driver.  There can be only one driver with the same name.
	DBType string

	// Create is the function that will be invoked with all user-specified
	// arguments to create the database.  This function must return
	// ErrDBExists if the database already exists.
	Create func(chain chainctx.IChainCtx, args ...interface{}) (DB, error)

	// Open is the function that will be invoked with all user-specified
	// arguments to open the database.  This function must return
	// ErrDBDoesNotExist if the database has not already been created.
	Open func(chain chainctx.IChainCtx, args ...interface{}) (DB, error)

	// UseLogger uses a specified Logger to output package logging info.
	UseLogger func(logger zerolog.Logger)
}

// driverList holds all of the registered database backends.
var drivers = make(map[string]*Driver)

// RegisterDriver adds a backend database driver to available interfaces.
// ErrDBTypeRegistered will be returned if the database type for the driver has
// already been registered.
func RegisterDriver(driver Driver) error {
	if _, exists := drivers[driver.DBType]; exists {
		str := fmt.Sprintf("driver %q is already registered",
			driver.DBType)
		return makeError(ErrDBTypeRegistered, str, nil)
	}

	drivers[driver.DBType] = &driver
	return nil
}

// SupportedDrivers returns a slice of strings that represent the database
// drivers that have been registered and are therefore supported.
func SupportedDrivers() []string {
	supportedDBs := make([]string, 0, len(drivers))
	for _, drv := range drivers {
		supportedDBs = append(supportedDBs, drv.DBType)
	}
	return supportedDBs
}

// Create initializes and opens a database for the specified type.  The
// arguments are specific to the database type driver.  See the documentation
// for the database driver for further details.
//
// ErrDBUnknownType will be returned if the the database type is not registered.
func Create(dbType string, chain chainctx.IChainCtx, args ...interface{}) (DB, error) {
	drv, exists := drivers[dbType]
	if !exists {
		str := fmt.Sprintf("driver %q is not registered", dbType)
		return nil, makeError(ErrDBUnknownType, str, nil)
	}

	return drv.Create(chain, args...)
}

// Open opens an existing database for the specified type.  The arguments are
// specific to the database type driver.  See the documentation for the database
// driver for further details.
//
// ErrDBUnknownType will be returned if the the database type is not registered.
func Open(dbType string, chain chainctx.IChainCtx, args ...interface{}) (DB, error) {
	drv, exists := drivers[dbType]
	if !exists {
		str := fmt.Sprintf("driver %q is not registered", dbType)
		return nil, makeError(ErrDBUnknownType, str, nil)
	}

	return drv.Open(chain, args...)
}
