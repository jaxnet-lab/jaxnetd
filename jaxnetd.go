// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
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

	"gitlab.com/jaxnet/jaxnetd/config"
	"gitlab.com/jaxnet/jaxnetd/internal/limits"
	"gitlab.com/jaxnet/jaxnetd/node"
	"gitlab.com/jaxnet/jaxnetd/version"
)

// winServiceMain is only invoked on Windows.  It detects when jaxnetd is running
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
			fmt.Println("FATAL:", err)
			os.Exit(1)
		}
		if isService {
			os.Exit(0)
		}
	}

	// Work around defer not working after os.Exit()
	if err := shardCoreMain(); err != nil {
		fmt.Println("FATAL:", err)
		os.Exit(1)
	}
}

// shardCoreMain is the real main function for jaxnetd.  It is necessary to work around
// the fact that deferred functions do not run when os.Exit() is called.  The
// optional serverChan parameter is mainly used by the service code to be
// notified with the server once it is setup so it can gracefully stop it when
// requested from the service control manager.
func shardCoreMain() error {

	// Load configuration and parse command line.  This function also
	// initializes logging and configures it accordingly.
	cfg, _, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// Get a channel that will be closed when a shutdown signal has been
	// triggered either from an OS signal such as SIGINT (Ctrl+C) or from
	// another subsystem such as the RPC server.

	defer config.Log.Info().Msg("Shutdown complete")

	// Show version at startup.
	config.Log.Info().Msgf("Version %s", version.GetVersion())

	// Enable http profiling server if requested.
	if cfg.Profile != "" {
		go func() {
			listenAddr := net.JoinHostPort("", cfg.Profile)
			config.Log.Info().Msgf("Profile server listening on %s", listenAddr)
			profileRedirect := http.RedirectHandler("/debug/pprof",
				http.StatusSeeOther)
			http.Handle("/", profileRedirect)
			err = http.ListenAndServe(listenAddr, nil)
			if err != nil {
				config.Log.Error().Err(err).Msg("listen and serve failed")
			}
		}()
	}

	// Write cpu profile if requested.
	if cfg.CPUProfile != "" {
		f, err := os.Create(cfg.CPUProfile)
		if err != nil {
			config.Log.Error().Err(err).Msg("Unable to create cpu profile")
			return err
		}
		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}

	// Perform upgrades to jaxnetd as new versions require it.
	if err := config.DoUpgrades(cfg); err != nil {
		config.Log.Error().
			Err(err).
			Msg("can not do upgrade")
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := interruptListener(config.Log.With().Str("ctx", "interruptListener").Logger())
	go func() {
		select {
		case <-sigChan:
			config.Log.Info().Msg("propagate stop signal")
			cancel()
		}
	}()

	controller := node.Controller(config.Log.With().Str("ctx", "NodeController").Logger())
	if err := controller.Run(ctx, cfg); err != nil {
		config.Log.Error().Err(err).Msg("Can't run Chains")
		os.Exit(2)
	}

	return nil
}
