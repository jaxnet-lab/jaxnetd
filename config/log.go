// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2017 The Decred developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package config

import (
	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/corelog"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/network/addrmgr"
	"gitlab.com/jaxnet/jaxnetd/network/connmgr"
	"gitlab.com/jaxnet/jaxnetd/network/netsync"
	"gitlab.com/jaxnet/jaxnetd/network/peer"
	"gitlab.com/jaxnet/jaxnetd/network/rpc"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain/indexers"
	"gitlab.com/jaxnet/jaxnetd/node/mempool"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
	"gitlab.com/jaxnet/jaxnetd/txscript"
)

// Loggers per subsystem.  A single backend logger is created and all subsytem
// loggers created from it will write to the backend.  When adding new
// subsystems, add the subsystem logger variable here and to the
// unitLogs map.
//
// Loggers can not be used before the log rotator has been initialized with a
// log file.  This must be performed early during application startup by calling
// initLogRotator.
var (
	// backendLog is the logging backend used to create all subsystem loggers.
	// The backend must not be used before the log rotator has been initialized,
	// or data races and/or nil pointer dereferences will occur.
	backendLog = corelog.New(logUnitJXCD, corelog.DefaultLevel, corelog.Config{}.Default())
	Log        = backendLog.With().Str("app.unit", logUnitJXCD).Logger()

	// unitLogs maps each subsystem identifier to its associated logger.
	unitLogs = map[string]zerolog.Logger{
		logUnitADXR: backendLog.With().Str("app.unit", logUnitADXR).Logger(),
		logUnitAMGR: backendLog.With().Str("app.unit", logUnitAMGR).Logger(),
		logUnitCMGR: backendLog.With().Str("app.unit", logUnitCMGR).Logger(),
		logUnitBCDB: backendLog.With().Str("app.unit", logUnitBCDB).Logger(),
		logUnitJXCD: backendLog.With().Str("app.unit", logUnitJXCD).Logger(),
		logUnitCHAN: backendLog.With().Str("app.unit", logUnitCHAN).Logger(),
		logUnitDISC: backendLog.With().Str("app.unit", logUnitDISC).Logger(),
		logUnitINDX: backendLog.With().Str("app.unit", logUnitINDX).Logger(),
		logUnitMINR: backendLog.With().Str("app.unit", logUnitMINR).Logger(),
		logUnitPEER: backendLog.With().Str("app.unit", logUnitPEER).Logger(),
		logUnitRPCS: backendLog.With().Str("app.unit", logUnitRPCS).Logger(),
		logUnitSCRP: backendLog.With().Str("app.unit", logUnitSCRP).Logger(),
		logUnitSRVR: backendLog.With().Str("app.unit", logUnitSRVR).Logger(),
		logUnitSYNC: backendLog.With().Str("app.unit", logUnitSYNC).Logger(),
		logUnitTXMP: backendLog.With().Str("app.unit", logUnitTXMP).Logger(),
	}
)

const (
	logUnitADXR = "ADXR"
	logUnitAMGR = "AMGR"
	logUnitCMGR = "CMGR"
	logUnitBCDB = "BCDB"
	logUnitJXCD = "JXCD"
	logUnitCHAN = "CHAN"
	logUnitDISC = "DISC"
	logUnitINDX = "INDX"
	logUnitMINR = "MINR"
	logUnitPEER = "PEER"
	logUnitRPCS = "RPCS"
	logUnitSCRP = "SCRP"
	logUnitSRVR = "SRVR"
	logUnitSYNC = "SYNC"
	logUnitTXMP = "TXMP"
)

func init() {
	setLoggers()
}

// Initialize package-global logger variables.
func setLoggers() {
	addrmgr.UseLogger(unitLogs[logUnitAMGR])
	connmgr.UseLogger(unitLogs[logUnitCMGR])
	database.UseLogger(unitLogs[logUnitBCDB])
	blockchain.UseLogger(unitLogs[logUnitCHAN])
	indexers.UseLogger(unitLogs[logUnitINDX])
	mining.UseLogger(unitLogs[logUnitMINR])
	peer.UseLogger(unitLogs[logUnitPEER])
	txscript.UseLogger(unitLogs[logUnitSRVR])
	netsync.UseLogger(unitLogs[logUnitSYNC])
	mempool.UseLogger(unitLogs[logUnitTXMP])
	rpc.UseLogger(unitLogs[logUnitRPCS])
}

// setLogLevel sets the logging level for provided subsystem.  Invalid
// subsystems are ignored.  Uninitialized subsystems are dynamically created as
// needed.
func setLogLevel(unit, logLevel string, config corelog.Config) {
	// Ignore invalid subsystems.
	logger, ok := unitLogs[unit]
	if !ok {
		return
	}

	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	logger = corelog.New(unit, level, config)
	unitLogs[unit] = logger.With().Str("app.unit", unit).Logger()
}

// setLogLevels sets the log level for all subsystem loggers to the passed
// level.  It also dynamically creates the subsystem loggers as needed, so it
// can be used to initialize the logging system.
func setLogLevels(logLevel string, config corelog.Config) {
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	backendLog = corelog.New(logUnitJXCD, level, config)
	Log = backendLog.With().Str("app.unit", logUnitJXCD).Logger()

	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for unit := range unitLogs {
		setLogLevel(unit, logLevel, config)
	}
}
