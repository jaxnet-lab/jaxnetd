// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package node

import (
	"context"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain/indexers"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
)

type DBCtl struct {
	logger zerolog.Logger
}

// loadBlockDB loads (or creates when needed) the block database taking into
// account the selected database backend and returns a handle to it.  It also
// contains additional logic such warning the user if there are multiple
// databases which consume space on the file system and ensuring the regression
// test database is clean when in regression test mode.
// nolint: gomnd
func (ctrl *DBCtl) loadBlockDB(dataDir string, chain chainctx.IChainCtx, cfg InstanceConfig) (database.DB, error) {
	// The memdb backend does not have a file path associated with it, so
	// handle it uniquely.  We also don't want to worry about the multiple
	// database type warnings when running with the memory database.
	if cfg.DBType == "memdb" {
		ctrl.logger.Info().Msg("Creating block database in memory.")
		db, err := database.Create(cfg.DBType, chain)
		if err != nil {
			return nil, err
		}
		return db, nil
	}
	chainName := chain.Name()

	ctrl.warnMultipleDBs(dataDir, chainName, cfg)

	// The database name is based on the database type.
	dbPath := ctrl.blockDBPath(dataDir, chainName, cfg.DBType)
	ctrl.logger.Debug().Str("path", dbPath).Msg("dbPath")

	// The regression test is special in that it needs a clean database for
	// each run, so remove it now if it already exists.
	// removeRegressionDB(cfg, dbPath)

	ctrl.logger.Info().Msgf("Loading block database from '%s'", dbPath)

	db, err := database.Open(cfg.DBType, chain, dbPath)
	if err != nil {
		// Return the error if it's not because the database doesn't exist.
		if dbErr, ok := err.(database.Error); !ok || dbErr.ErrorCode !=
			database.ErrDBDoesNotExist {

			return nil, err
		}

		// Create the db if it does not exist.
		err = os.MkdirAll(dataDir, 0o700)
		if err != nil {
			return nil, err
		}

		db, err = database.Create(cfg.DBType, chain, dbPath)
		if err != nil {
			return nil, err
		}
	}

	ctrl.logger.Info().Msg("Block database loaded")
	return db, nil
}

func (ctrl *DBCtl) sanitizeState(ctx context.Context, cfg *Config, db database.DB) bool {
	if cfg.VerifyState {
		err := blockchain.VerifyStateSanity(db)
		if err != nil {
			ctrl.logger.Error().Err(err).Msg("Can't load Block db")
			return false
		}
		return false
	}

	cleanIndexes, err := ctrl.cleanIndexes(ctx, cfg, db)
	if cleanIndexes {
		ctrl.logger.Info().Msg("clean db indexes")
		return false
	}

	if err != nil {
		ctrl.logger.Error().Err(err).Msg("failed to clean indexes")
		return false
	}

	err = ctrl.refillIndexes(ctx, cfg, db)
	if err != nil {
		ctrl.logger.Error().Err(err).Msg("failed to refill indexes")
		return false
	}

	return true
}

// cleanIndexes drops indexes and exit if requested.
//
// NOTE: The order is important here because dropping the tx index also
// drops the address index since it relies on it.
func (ctrl *DBCtl) cleanIndexes(ctx context.Context, cfg *Config, db database.DB) (bool, error) {
	cleanPerformed := cfg.DropAddrIndex || cfg.DropTxIndex || cfg.DropCfIndex
	if cfg.DropAddrIndex {
		if err := indexers.DropAddrIndex(db, ctx.Done()); err != nil {
			return cleanPerformed, err
		}
	}

	if cfg.DropTxIndex {
		if err := indexers.DropTxIndex(db, ctx.Done()); err != nil {
			return cleanPerformed, err
		}
		if err := indexers.DropOrphanTxIndex(db, ctx.Done()); err != nil {
			return cleanPerformed, err
		}
	}

	if cfg.DropCfIndex {
		if err := indexers.DropCfIndex(db, ctx.Done()); err != nil {
			return cleanPerformed, err
		}
	}

	return cleanPerformed, nil
}

// refillIndexes drops indexes and exit if requested.
//
// NOTE: The order is important here because dropping the tx index also
// drops the address index since it relies on it.
func (ctrl *DBCtl) refillIndexes(ctx context.Context, cfg *Config, db database.DB) error {
	// if cfg.DropAddrIndex {
	// 	if err := indexers.DropAddrIndex(db, ctx.Done()); err != nil {
	// 		return cleanPerformed, err
	// 	}
	// }

	if cfg.RefillTxIndex {
		// 	if err := indexers.DropTxIndex(db, ctx.Done()); err != nil {
		// 		return err
		// 	}
		if err := indexers.ReinitOrphanTxIndex(db, ctx.Done()); err != nil {
			return err
		}
	}

	return nil
}

// dbPath returns the path to the block database given a database type.
func (ctrl *DBCtl) blockDBPath(dataDir string, chain string, dbType string) string {
	// The database name is based on the database type.
	dbName := blockDBNamePrefix + "_" + dbType
	if dbType == "sqlite" {
		dbName += ".db"
	}
	dbPath := filepath.Join(dataDir, chain, dbName)
	return dbPath
}

// warnMultipleDBs shows a warning if multiple block database types are detected.
// This is not a situation most users want.  It is handy for development however
// to support multiple side-by-side databases.
func (ctrl *DBCtl) warnMultipleDBs(dataDir string, chain string, cfg InstanceConfig) {
	// This is intentionally not using the known db types which depend
	// on the database types compiled into the binary since we want to
	// detect legacy db types as well.
	dbTypes := []string{"ffldb", "leveldb", "sqlite"}
	duplicateDBPaths := make([]string, 0, len(dbTypes)-1)
	for _, dbType := range dbTypes {
		if dbType == cfg.DBType {
			continue
		}

		// Store db path as a duplicate db if it exists.
		dbPath := ctrl.blockDBPath(dataDir, chain, dbType)
		if fileExists(dbPath) {
			duplicateDBPaths = append(duplicateDBPaths, dbPath)
		}
	}

	// Warn if there are extra databases.
	if len(duplicateDBPaths) > 0 {
		selectedDBPath := ctrl.blockDBPath(dataDir, chain, cfg.DBType)
		ctrl.logger.Info().Msgf("WARNING: There are multiple block unit databases "+
			"using different database types.\nYou probably don't "+
			"want to waste disk space by having more than one.\n"+
			"Your current database is located at [%v].\nThe "+
			"additional database is located at %v", selectedDBPath,
			duplicateDBPaths)
	}
}
