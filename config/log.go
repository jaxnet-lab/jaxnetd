// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2017 The Decred developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jrick/logrotate/rotator"
	"gitlab.com/jaxnet/core/shard.core/corelog"
	"gitlab.com/jaxnet/core/shard.core/database"
	"gitlab.com/jaxnet/core/shard.core/network/addrmgr"
	"gitlab.com/jaxnet/core/shard.core/network/connmgr"
	"gitlab.com/jaxnet/core/shard.core/network/netsync"
	"gitlab.com/jaxnet/core/shard.core/network/peer"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain/indexers"
	"gitlab.com/jaxnet/core/shard.core/node/mempool"
	"gitlab.com/jaxnet/core/shard.core/node/mining"
	"gitlab.com/jaxnet/core/shard.core/txscript"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// logWriter implements an io.Writer that outputs to both standard output and
// the write-end pipe of an initialized log rotator.
type logWriter struct{}

func (logWriter) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	LogRotator.Write(p)
	return len(p), nil
}

// Loggers per subsystem.  A single backend logger is created and all subsytem
// loggers created from it will write to the backend.  When adding new
// subsystems, add the subsystem logger variable here and to the
// subsystemLoggers map.
//
// Loggers can not be used before the log rotator has been initialized with a
// log file.  This must be performed early during application startup by calling
// initLogRotator.
var (
	// backendLog is the logging backend used to create all subsystem loggers.
	// The backend must not be used before the log rotator has been initialized,
	// or data races and/or nil pointer dereferences will occur.
	backendLog = corelog.New(corelog.DefaultLevel, corelog.DefaultLogFile, false)

	// LogRotator is one of the logging outputs.  It should be closed on
	// application shutdown.
	LogRotator *rotator.Rotator

	BtcdLog = backendLog.With(zap.String("app.unit", "BTCD"))

	// subsystemLoggers maps each subsystem identifier to its associated logger.
	subsystemLoggers = map[string]*zap.Logger{
		logUnitADXR: backendLog.With(zap.String("app.unit", logUnitADXR)),
		logUnitAMGR: backendLog.With(zap.String("app.unit", logUnitAMGR)),
		logUnitCMGR: backendLog.With(zap.String("app.unit", logUnitCMGR)),
		logUnitBCDB: backendLog.With(zap.String("app.unit", logUnitBCDB)),
		logUnitBTCD: backendLog.With(zap.String("app.unit", logUnitBTCD)),
		logUnitCHAN: backendLog.With(zap.String("app.unit", logUnitCHAN)),
		logUnitDISC: backendLog.With(zap.String("app.unit", logUnitDISC)),
		logUnitINDX: backendLog.With(zap.String("app.unit", logUnitINDX)),
		logUnitMINR: backendLog.With(zap.String("app.unit", logUnitMINR)),
		logUnitPEER: backendLog.With(zap.String("app.unit", logUnitPEER)),
		logUnitRPCS: backendLog.With(zap.String("app.unit", logUnitRPCS)),
		logUnitSCRP: backendLog.With(zap.String("app.unit", logUnitSCRP)),
		logUnitSRVR: backendLog.With(zap.String("app.unit", logUnitSRVR)),
		logUnitSYNC: backendLog.With(zap.String("app.unit", logUnitSYNC)),
		logUnitTXMP: backendLog.With(zap.String("app.unit", logUnitTXMP)),
	}
)

const (
	logUnitADXR = "ADXR"
	logUnitAMGR = "AMGR"
	logUnitCMGR = "CMGR"
	logUnitBCDB = "BCDB"
	logUnitBTCD = "BTCD"
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
	addrmgr.UseLogger(corelog.Adapter(subsystemLoggers[logUnitAMGR]))
	connmgr.UseLogger(corelog.Adapter(subsystemLoggers[logUnitCMGR]))
	database.UseLogger(corelog.Adapter(subsystemLoggers[logUnitBCDB]))
	blockchain.UseLogger(corelog.Adapter(subsystemLoggers[logUnitCHAN]))
	indexers.UseLogger(corelog.Adapter(subsystemLoggers[logUnitINDX]))
	mining.UseLogger(corelog.Adapter(subsystemLoggers[logUnitMINR]))
	peer.UseLogger(corelog.Adapter(subsystemLoggers[logUnitPEER]))
	txscript.UseLogger(corelog.Adapter(subsystemLoggers[logUnitSRVR]))
	netsync.UseLogger(corelog.Adapter(subsystemLoggers[logUnitSYNC]))
	mempool.UseLogger(corelog.Adapter(subsystemLoggers[logUnitTXMP]))
}

// initLogRotator initializes the logging rotater to write logs to logFile and
// create roll files in the same directory.  It must be called before the
// package-global log rotater variables are used.
func initLogRotator(logFile string) {
	logDir, _ := filepath.Split(logFile)
	err := os.MkdirAll(logDir, 0700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
		os.Exit(1)
	}
	r, err := rotator.New(logFile, 10*1024, false, 3)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file rotator: %v\n", err)
		os.Exit(1)
	}

	LogRotator = r
}

// setLogLevel sets the logging level for provided subsystem.  Invalid
// subsystems are ignored.  Uninitialized subsystems are dynamically created as
// needed.
func setLogLevel(subsystemID, logLevel, file string, disableStdOut bool) {
	// Ignore invalid subsystems.
	logger, ok := subsystemLoggers[subsystemID]
	if !ok {
		return
	}

	level := zapcore.DebugLevel
	_ = level.Set(logLevel)
	logger = corelog.New(level, file, disableStdOut)
	subsystemLoggers[subsystemID] = logger.With(zap.String("app.unit", subsystemID))
}

// setLogLevels sets the log level for all subsystem loggers to the passed
// level.  It also dynamically creates the subsystem loggers as needed, so it
// can be used to initialize the logging system.
func setLogLevels(logLevel, file string, disableStdOut bool) {
	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for subsystemID := range subsystemLoggers {
		setLogLevel(subsystemID, logLevel, file, disableStdOut)
	}
}
