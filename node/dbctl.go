package node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gitlab.com/jaxnet/core/shard.core/database"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain/indexers"
	"gitlab.com/jaxnet/core/shard.core/node/chain"
	"go.uber.org/zap"
)

type DBCtl struct {
	logger *zap.Logger
}

// loadBlockDB loads (or creates when needed) the block database taking into
// account the selected database backend and returns a handle to it.  It also
// contains additional logic such warning the user if there are multiple
// databases which consume space on the file system and ensuring the regression
// test database is clean when in regression test mode.
func (ctrl *DBCtl) loadBlockDB(dataDir string, chain chain.IChainCtx, cfg NodeConfig) (database.DB, error) {
	// The memdb backend does not have a file path associated with it, so
	// handle it uniquely.  We also don't want to worry about the multiple
	// database type warnings when running with the memory database.
	if cfg.DbType == "memdb" {
		ctrl.logger.Info("Creating block database in memory.")
		db, err := database.Create(cfg.DbType, chain)
		if err != nil {
			return nil, err
		}
		return db, nil
	}
	chainName := chain.Params().Name

	ctrl.warnMultipleDBs(dataDir, chainName, cfg)

	// The database name is based on the database type.
	dbPath := ctrl.blockDbPath(dataDir, chainName, cfg.DbType)
	ctrl.logger.Debug("dbPath", zap.String("path", dbPath))

	// The regression test is special in that it needs a clean database for
	// each run, so remove it now if it already exists.
	// removeRegressionDB(cfg, dbPath)

	ctrl.logger.Info(fmt.Sprintf("Loading block database from '%s'", dbPath))
	db, err := database.Open(cfg.DbType, chain, dbPath, cfg.ChainParams().Net)
	if err != nil {
		// Return the error if it's not because the database doesn't exist.
		if dbErr, ok := err.(database.Error); !ok || dbErr.ErrorCode !=
			database.ErrDbDoesNotExist {

			return nil, err
		}

		// Create the db if it does not exist.
		err = os.MkdirAll(dataDir, 0700)
		if err != nil {
			return nil, err
		}

		db, err = database.Create(cfg.DbType, chain, dbPath, cfg.ChainParams().Net)
		if err != nil {
			return nil, err
		}
	}

	ctrl.logger.Info("Block database loaded")
	return db, nil
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
	}

	if cfg.DropCfIndex {
		if err := indexers.DropCfIndex(db, ctx.Done()); err != nil {
			return cleanPerformed, err
		}
	}

	return cleanPerformed, nil
}

// dbPath returns the path to the block database given a database type.
func (ctrl *DBCtl) blockDbPath(dataDir string, chain string, dbType string) string {
	// The database name is based on the database type.
	dbName := blockDbNamePrefix + "_" + dbType
	if dbType == "sqlite" {
		dbName = dbName + ".db"
	}
	dbPath := filepath.Join(dataDir, chain, dbName)
	return dbPath
}

// warnMultipleDBs shows a warning if multiple block database types are detected.
// This is not a situation most users want.  It is handy for development however
// to support multiple side-by-side databases.
func (ctrl *DBCtl) warnMultipleDBs(dataDir string, chain string, cfg NodeConfig) {
	// This is intentionally not using the known db types which depend
	// on the database types compiled into the binary since we want to
	// detect legacy db types as well.
	dbTypes := []string{"ffldb", "leveldb", "sqlite"}
	duplicateDbPaths := make([]string, 0, len(dbTypes)-1)
	for _, dbType := range dbTypes {
		if dbType == cfg.DbType {
			continue
		}

		// Store db path as a duplicate db if it exists.
		dbPath := ctrl.blockDbPath(dataDir, chain, dbType)
		if fileExists(dbPath) {
			duplicateDbPaths = append(duplicateDbPaths, dbPath)
		}
	}

	// Warn if there are extra databases.
	if len(duplicateDbPaths) > 0 {
		selectedDbPath := ctrl.blockDbPath(dataDir, chain, cfg.DbType)
		ctrl.logger.Info(fmt.Sprintf("WARNING: There are multiple block chain databases "+
			"using different database types.\nYou probably don't "+
			"want to waste disk space by having more than one.\n"+
			"Your current database is located at [%v].\nThe "+
			"additional database is located at %v", selectedDbPath,
			duplicateDbPaths))
	}
}
