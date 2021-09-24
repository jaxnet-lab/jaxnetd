package main

import (
	"errors"
	"strconv"
)

// ExportIndexesCmd defines the configuration options for the fetchblockregion
// command.
type ExportIndexesCmd struct{}

var (
	// ExportIndexesCfg defines the configuration options for the command.
	ExportIndexesCfg = ExportIndexesCmd{}
)

// Execute is the main entry point for the command.  It's invoked by the parser.
func (cmd *ExportIndexesCmd) Execute(args []string) error {
	// Setup the global config options and ensure they are valid.
	err := setupGlobalConfig()
	if err != nil {
		return err
	}
	log.Info(args)
	if len(args) < 2 {
		return errors.New("<shardID> <scanner-type> not passed; Usage: " + cmd.Usage())
	}

	shardID, err := parseShardID(args[0])
	if err != nil {
		return errors.New("wrong shardID format specified")
	}

	var offset = 0
	if len(args) >= 3 {
		offset, err = strconv.Atoi(args[2])
		if err != nil {
			return err
		}
	}
	var limit *int
	if len(args) >= 4 {
		lmt, err := strconv.Atoi(args[3])
		if err != nil {
			return err
		}
		limit = &lmt
	}

	switch args[1] {
	case "raw":
		return rawScanner(offset, shardID)
	case "history":
		return historyScanner(offset, limit, shardID)
	}
	return nil
}

// Usage overrides the usage display for the command.
func (cmd *ExportIndexesCmd) Usage() string {
	return "<scanner-type> <start-offset> <block-limit>"
}
