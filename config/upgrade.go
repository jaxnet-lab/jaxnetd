// Copyright (c) 2013-2014 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package config

import (
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"gitlab.com/jaxnet/jaxnetd/node"
)

const (
	// blockDBNamePrefix is the prefix for the block database name.  The
	// database type is appended to this value to form the full block
	// database name.
	blockDBNamePrefix = "blocks"
)

// DoUpgrades performs upgrades to jaxnetd as new versions require it.
func DoUpgrades(cfg *node.Config) error {
	err := upgradeDBPaths(cfg)
	if err != nil {
		return err
	}
	return upgradeDataPaths()
}

// dirEmpty returns whether or not the specified directory path is empty.
func dirEmpty(dirPath string) (bool, error) {
	f, err := os.Open(dirPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Read the names of a max of one entry from the directory.  When the
	// directory is empty, an io.EOF error will be returned, so allow it.
	names, err := f.Readdirnames(1)
	if err != nil && err != io.EOF {
		return false, err
	}

	return len(names) == 0, nil
}

// oldBtcdHomeDir returns the OS specific home directory jaxnetd used prior to
// version 0.3.3.  This has since been replaced with jaxutil.AppDataDir, but
// this function is still provided for the automatic upgrade path.
func oldBtcdHomeDir() string {
	// Search for Windows APPDATA first.  This won't exist on POSIX OSes.
	appData := os.Getenv("APPDATA")
	if appData != "" {
		return filepath.Join(appData, "jaxnetd")
	}

	// Fall back to standard HOME directory that works for most POSIX OSes.
	home := os.Getenv("HOME")
	if home != "" {
		return filepath.Join(home, ".jaxnetd")
	}

	// In the worst case, use the current directory.
	return "."
}

// upgradeDBPathNet moves the database for a specific network from its
// location prior to jaxnetd version 0.2.0 and uses heuristics to ascertain the old
// database type to rename to the new format.
func upgradeDBPathNet(cfg *node.Config, oldDBPath, netName string) error {
	// Prior to version 0.2.0, the database was named the same thing for
	// both sqlite and leveldb.  Use heuristics to figure out the type
	// of the database and move it to the new path and name introduced with
	// version 0.2.0 accordingly.
	fi, err := os.Stat(oldDBPath)
	if err == nil {
		oldDBType := "sqlite"
		if fi.IsDir() {
			oldDBType = "leveldb"
		}

		// The new database name is based on the database type and
		// resides in a directory named after the network type.
		newDBRoot := filepath.Join(filepath.Dir(cfg.DataDir), netName)
		newDBName := blockDBNamePrefix + "_" + oldDBType
		if oldDBType == "sqlite" {
			newDBName += ".db"
		}
		newDBPath := filepath.Join(newDBRoot, newDBName)

		// Create the new path if needed.
		//nolint: gomnd
		err = os.MkdirAll(newDBRoot, 0o700)
		if err != nil {
			return err
		}

		// Move and rename the old database.
		err := os.Rename(oldDBPath, newDBPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// upgradeDBPaths moves the databases from their locations prior to jaxnetd
// version 0.2.0 to their new locations.
func upgradeDBPaths(cfg *node.Config) error {
	// Prior to version 0.2.0, the databases were in the "db" directory and
	// their names were suffixed by "testnet" and "regtest" for their
	// respective networks.  Check for the old database and update it to the
	// new path introduced with version 0.2.0 accordingly.
	oldDBRoot := filepath.Join(oldBtcdHomeDir(), "db")
	if err := upgradeDBPathNet(cfg, filepath.Join(oldDBRoot, "jaxnetd.db"), "mainnet"); err != nil {
		log.Error().Err(err).Msg("cannot upgrade dbpath for mainnet")
	}
	if err := upgradeDBPathNet(cfg, filepath.Join(oldDBRoot, "jaxnetd_testnet.db"), "testnet"); err != nil {
		log.Error().Err(err).Msg("cannot upgrade dbpath for testnet")
	}
	if err := upgradeDBPathNet(cfg, filepath.Join(oldDBRoot, "jaxnetd_regtest.db"), "regtest"); err != nil {
		log.Error().Err(err).Msg("cannot upgrade dbpath for regtestnet")
	}

	// Remove the old db directory.
	return os.RemoveAll(oldDBRoot)
}

// upgradeDataPaths moves the application data from its location prior to jaxnetd
// version 0.3.3 to its new location.
func upgradeDataPaths() error {
	// No need to migrate if the old and new home paths are the same.
	oldHomePath := oldBtcdHomeDir()
	newHomePath := defaultHomeDir
	if oldHomePath == newHomePath {
		return nil
	}

	// Only migrate if the old path exists and the new one doesn't.
	if fileExists(oldHomePath) && !fileExists(newHomePath) {
		// Create the new path.
		Log.Info().Msgf("Migrating application home path from '%s' to '%s'",
			oldHomePath, newHomePath)
		//nolint: gomnd
		err := os.MkdirAll(newHomePath, 0o700)
		if err != nil {
			return err
		}

		// Move old jaxnetd.conf into new location if needed.
		oldConfPath := filepath.Join(oldHomePath, defaultConfigFilename)
		newConfPath := filepath.Join(newHomePath, defaultConfigFilename)
		if fileExists(oldConfPath) && !fileExists(newConfPath) {
			err := os.Rename(oldConfPath, newConfPath)
			if err != nil {
				return err
			}
		}

		// Move old data directory into new location if needed.
		oldDataPath := filepath.Join(oldHomePath, defaultDataDirname)
		newDataPath := filepath.Join(newHomePath, defaultDataDirname)
		if fileExists(oldDataPath) && !fileExists(newDataPath) {
			err := os.Rename(oldDataPath, newDataPath)
			if err != nil {
				return err
			}
		}

		// Remove the old home if it is empty or show a warning if not.
		ohpEmpty, err := dirEmpty(oldHomePath)
		if err != nil {
			return err
		}
		if ohpEmpty {
			err := os.Remove(oldHomePath)
			if err != nil {
				return err
			}
		} else {
			Log.Warn().Msgf("Not removing '%s' since it contains files "+
				"not created by this application.  You may "+
				"want to manually move them or delete them.",
				oldHomePath)
		}
	}

	return nil
}
