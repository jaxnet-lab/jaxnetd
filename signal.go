// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
)

// shutdownRequestChannel is used to initiate shutdown from one of the
// subsystems using the same code paths as when an interrupt signal is received.
var shutdownRequestChannel = make(chan struct{})

// interruptSignals defines the default signals to catch in order to do a proper
// shutdown.  This may be modified during init depending on the platform.
var interruptSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

// interruptListener listens for OS Signals such as SIGINT (Ctrl+C) and shutdown
// requests from shutdownRequestChannel.  It returns a channel that is closed
// when either signal is received.
func interruptListener(log zerolog.Logger) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		interruptChannel := make(chan os.Signal, 1)
		signal.Notify(interruptChannel, interruptSignals...)

		// Listen for initial shutdown signal and close the returned
		// channel to notify the caller.
		select {
		case sig := <-interruptChannel:
			log.Info().Msg("Received signal " + sig.String() + ". Shutting down...")

		case <-shutdownRequestChannel:
			log.Info().Msg("Shutdown requested. Shutting down...")
		}
		close(done)

		// Listen for repeated signals and display a message so the user
		// knows the shutdown is in progress and the process is not
		// hung.
		for {
			select {
			case sig := <-interruptChannel:
				log.Info().Msg("Received signal " + sig.String() + ". Already shutting down...")

			case <-shutdownRequestChannel:
				log.Info().Msg("Shutdown requested.  Already shutting down...")
			}
		}
	}()

	return done
}
