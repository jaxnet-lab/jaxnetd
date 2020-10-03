// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"

	"gitlab.com/jaxnet/core/shard.core/config"
	"gitlab.com/jaxnet/core/shard.core/limits"
	"gitlab.com/jaxnet/core/shard.core/node"
	"gitlab.com/jaxnet/core/shard.core/node/chain"
	"gitlab.com/jaxnet/core/shard.core/node/chain/beacon"
	"go.uber.org/zap"
)

func initChain() bool {
	chain.BeaconChain = beacon.Chain(config.ActiveNetParams.Params)
	return true
}

var _ = initChain()

// winServiceMain is only invoked on Windows.  It detects when btcd is running
// as a service and reacts accordingly.
var winServiceMain func() (bool, error)

func main() {
	// Use all processor cores.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Block and transaction processing can cause bursty allocations.  This
	// limits the garbage collector from excessively overallocating during
	// bursts.  This value was arrived at with the help of profiling live
	// usage.
	debug.SetGCPercent(10)

	// Up some limits.
	if err := limits.SetLimits(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to set limits: %v\n", err)
		os.Exit(1)
	}

	// Call serviceMain on Windows to handle running as a service.  When
	// the return isService flag is true, exit now since we ran as a
	// service.  Otherwise, just fall through to normal operation.
	if runtime.GOOS == "windows" {
		isService, err := winServiceMain()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if isService {
			os.Exit(0)
		}
	}

	// Work around defer not working after os.Exit()
	if err := btcdMain(); err != nil {
		os.Exit(1)
	}
}

// btcdMain is the real main function for btcd.  It is necessary to work around
// the fact that deferred functions do not run when os.Exit() is called.  The
// optional serverChan parameter is mainly used by the service code to be
// notified with the server once it is setup so it can gracefully stop it when
// requested from the service control manager.
func btcdMain() error {

	// Load configuration and parse command line.  This function also
	// initializes logging and configures it accordingly.
	cfg, _, err := config.LoadConfig()
	if err != nil {
		return err
	}

	defer func() {
		if config.LogRotator != nil {
			config.LogRotator.Close()
		}
	}()

	// Get a channel that will be closed when a shutdown signal has been
	// triggered either from an OS signal such as SIGINT (Ctrl+C) or from
	// another subsystem such as the RPC server.

	defer config.BtcdLog.Info("Shutdown complete")

	// Show version at startup.
	config.BtcdLog.Infof("Version %s", version())

	// Enable http profiling server if requested.
	if cfg.Profile != "" {
		go func() {
			listenAddr := net.JoinHostPort("", cfg.Profile)
			config.BtcdLog.Infof("Profile server listening on %s", listenAddr)
			profileRedirect := http.RedirectHandler("/debug/pprof",
				http.StatusSeeOther)
			http.Handle("/", profileRedirect)
			config.BtcdLog.Errorf("%v", http.ListenAndServe(listenAddr, nil))
		}()
	}

	// Write cpu profile if requested.
	if cfg.CPUProfile != "" {
		f, err := os.Create(cfg.CPUProfile)
		if err != nil {
			config.BtcdLog.Errorf("Unable to create cpu profile: %v", err)
			return err
		}
		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}

	// Perform upgrades to btcd as new versions require it.
	if err := config.DoUpgrades(cfg); err != nil {
		config.BtcdLog.Errorf("%v", err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	logger, err := zap.NewDevelopment()
	if err != nil {
		logger.Error("Can't init logger", zap.Error(err))
		os.Exit(1)
	}

	sigChan := interruptListener(logger)
	go func() {
		select {
		case <-sigChan:
			logger.Info("propagate stop signal")
			cancel()
		}
	}()

	if err := node.Controller(logger).Run(ctx, cfg); err != nil {
		logger.Error("Can't run Chains", zap.Error(err))
		os.Exit(2)
	}

	return nil
}
