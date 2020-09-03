// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"gitlab.com/jaxnet/core/shard.core.git/limits"
	"gitlab.com/jaxnet/core/shard.core.git/shards"
	"go.uber.org/zap"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"syscall"
)

const (
	// blockDbNamePrefix is the prefix for the block database name.  The
	// database type is appended to this value to form the full block
	// database name.
	blockDbNamePrefix = "blocks"
)

//var (
//	cfg *config
//)
//
// winServiceMain is only invoked on Windows.  It detects when btcd is running
// as a service and reacts accordingly.
var winServiceMain func() (bool, error)

// btcdMain is the real main function for btcd.  It is necessary to work around
// the fact that deferred functions do not run when os.Exit() is called.  The
// optional serverChan parameter is mainly used by the service code to be
// notified with the server once it is setup so it can gracefully stop it when
// requested from the service control manager.
func btcdMain() error {

	// Load configuration and parse command line.  This function also
	// initializes logging and configures it accordingly.
	cfg, _, err := loadConfig()
	fmt.Println(cfg, err)
	if err != nil {
		return err
	}
	defer func() {
		if logRotator != nil {
			logRotator.Close()
		}
	}()

	// Get a channel that will be closed when a shutdown signal has been
	// triggered either from an OS signal such as SIGINT (Ctrl+C) or from
	// another subsystem such as the RPC server.

	defer btcdLog.Info("Shutdown complete")

	// Show version at startup.
	btcdLog.Infof("Version %s", version())

	// Enable http profiling server if requested.
	if cfg.Profile != "" {
		go func() {
			listenAddr := net.JoinHostPort("", cfg.Profile)
			btcdLog.Infof("Profile server listening on %s", listenAddr)
			profileRedirect := http.RedirectHandler("/debug/pprof",
				http.StatusSeeOther)
			http.Handle("/", profileRedirect)
			btcdLog.Errorf("%v", http.ListenAndServe(listenAddr, nil))
		}()
	}

	// Write cpu profile if requested.
	if cfg.CPUProfile != "" {
		f, err := os.Create(cfg.CPUProfile)
		if err != nil {
			btcdLog.Errorf("Unable to create cpu profile: %v", err)
			return err
		}
		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}

	// Perform upgrades to btcd as new versions require it.
	if err := doUpgrades(cfg); err != nil {
		btcdLog.Errorf("%v", err)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	fmt.Println("Run Beacon")
	//go runBeaconChain(ctx, cfg)
	//go runShardChain(1, cfg)
	//go runShardChain(2, cfg)

	l, err := zap.NewDevelopment()
	if err != nil {
		l.Error("Can't init logger", zap.Error(err))
		os.Exit(1)
	}

	sc := shards.Controller(l)
	if err := sc.Run(ctx, cfg); err != nil {
		l.Error("Can't run Chains", zap.Error(err))
		os.Exit(2)
	}

	done := make(chan bool)
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			log.Println("exiting")
			cancel()
			done <- true
		}
	}()
	<-done

	return nil
}

//// removeRegressionDB removes the existing regression test database if running
//// in regression test mode and it already exists.
//func removeRegressionDB(dbPath string) error {
//	// Don't do anything if not in regression test mode.
//	if !cfg.RegressionTest {
//		return nil
//	}
//
//	// Remove the old regression test database if it already exists.
//	fi, err := os.Stat(dbPath)
//	if err == nil {
//		btcdLog.Infof("Removing regression test database from '%s'", dbPath)
//		if fi.IsDir() {
//			err := os.RemoveAll(dbPath)
//			if err != nil {
//				return err
//			}
//		} else {
//			err := os.Remove(dbPath)
//			if err != nil {
//				return err
//			}
//		}
//	}
//
//	return nil
//}

//func runRpc(ctx context.Context, rpcConfig *server2.Config) (error){
//	if !rpcConfig.Disable {
//		// Setup listeners for the configured RPC listen addresses and
//		// TLS settings.
//		rpcListeners, err := setupRPCListeners()
//		if err != nil {
//			return  err
//		}
//		if len(rpcListeners) == 0 {
//			return errors.New("RPCS: No valid listen address")
//		}
//
//		fmt.Println("rpc server", rpcListeners[0].Addr())
//		rpcServer, err := server2.Server(&server2.Config{
//			Listeners:    rpcListeners,
//			StartupTime:  s.startupTime,
//			ConnMgr:      &rpcConnManager{&s},
//			SyncMgr:      &rpcSyncMgr{&s, s.syncManager},
//			TimeSource:   s.timeSource,
//			Chain:        s.chain,
//			ChainParams:  chainParams,
//			DB:           db,
//			TxMemPool:    s.txMemPool,
//			Generator:    blockTemplateGenerator,
//			TxIndex:      s.txIndex,
//			AddrIndex:    s.addrIndex,
//			CfIndex:      s.cfIndex,
//			FeeEstimator: s.feeEstimator,
//		})
//		if err != nil {
//			return nil, err
//		}
//
//		// Signal process shutdown when the RPC server requests it.
//		go func() {
//			<-s.rpcServer.RequestedProcessShutdown()
//			shutdownRequestChannel <- struct{}{}
//		}()
//	}
//}

// setupRPCListeners returns a slice of listeners that are configured for use
// with the RPC server depending on the configuration settings for listen
// addresses and TLS.
//func setupRPCListeners() ([]net.Listener, error) {
//	// Setup TLS if not disabled.
//	listenFunc := net.Listen
//	//if !s.cfg.DisableTLS {
//	//	// Generate the TLS cert and key file if both don't already
//	//	// exist.
//	//	if !fileExists(cfg.RPCKey) && !fileExists(cfg.RPCCert) {
//	//		err := genCertPair(cfg.RPCCert, cfg.RPCKey)
//	//		if err != nil {
//	//			return nil, err
//	//		}
//	//	}
//	//	keypair, err := tls.LoadX509KeyPair(cfg.RPCCert, cfg.RPCKey)
//	//	if err != nil {
//	//		return nil, err
//	//	}
//	//
//	//	tlsConfig := tls.Config{
//	//		Certificates: []tls.Certificate{keypair},
//	//		MinVersion:   tls.VersionTLS12,
//	//	}
//	//
//	//	// Change the standard net.Listen function to the tls one.
//	//	listenFunc = func(net string, laddr string) (net.Listener, error) {
//	//		return tls.Listen(net, laddr, &tlsConfig)
//	//	}
//	//}
//
//	netAddrs, err := parseListeners(s.cfg.Listeners)
//	if err != nil {
//		return nil, err
//	}
//
//	listeners := make([]net.Listener, 0, len(netAddrs))
//	for _, addr := range netAddrs {
//		listener, err := listenFunc(addr.Network(), addr.String())
//		if err != nil {
//			s.logger.Warnf("Can't listen on %s: %v", addr, err)
//			continue
//		}
//		listeners = append(listeners, listener)
//	}
//
//	return listeners, nil
//}
//
//func runBeaconChain(ctx context.Context, cfg *shards.Config) {
//	fmt.Println("Run Beacon")
//	interrupt := interruptListener()
//	// Return now if an interrupt signal was triggered.
//	if interruptRequested(interrupt) {
//		return
//	}
//
//	chain := beacon.Chain()
//	//chain.SetChain(shard.Chain())
//	// Load the block database.
//	db, err := loadBlockDB(cfg.DataDir, "beacon", chain, cfg.Node)
//	if err != nil {
//		btcdLog.Errorf("%v", err)
//	}
//
//	defer func() {
//		// Ensure the database is sync'd and closed on shutdown.
//		btcdLog.Infof("Gracefully shutting down the database...")
//		if err := db.Close(); err != nil {
//			btcdLog.Errorf("%v", err)
//		}
//	}()
//
//	// Return now if an interrupt signal was triggered.
//	//if interruptRequested(interrupt) {
//	//	return nil
//	//}
//
//	// Drop indexes and exit if requested.
//	//
//	// NOTE: The order is important here because dropping the tx index also
//	// drops the address index since it relies on it.
//	if cfg.DropAddrIndex {
//		if err := indexers.DropAddrIndex(db, interrupt); err != nil {
//			btcdLog.Errorf("%v", err)
//			return
//		}
//		return
//	}
//	if cfg.DropTxIndex {
//		if err := indexers.DropTxIndex(db, interrupt); err != nil {
//			btcdLog.Errorf("%v", err)
//		}
//
//		return
//	}
//	if cfg.DropCfIndex {
//		if err := indexers.DropCfIndex(db, interrupt); err != nil {
//			btcdLog.Errorf("%v", err)
//		}
//
//		return
//	}
//
//	l, err := zap.NewProduction()
//
//	amgr := addrmgr.New(cfg.DataDir, func(host string) ([]net.IP, error) {
//		if strings.HasSuffix(host, ".onion") {
//			return nil, fmt.Errorf("attempt to resolve tor address %s", host)
//		}
//
//		return cfg.Node.P2P.Lookup(host)
//	})
//
//	fmt.Println("P2P Listener ", cfg.Node.P2P.Listeners)
//	// Create server and start it.
//	server, err := server2.Server(&cfg.Node.P2P, amgr, chain, cfg.Node.P2P.Listeners, cfg.Node.P2P.AgentBlacklist,
//		cfg.Node.P2P.AgentWhitelist, db, activeNetParams.Params, interrupt, l)
//	if err != nil {
//		// TODO: this logging could do with some beautifying.
//		btcdLog.Errorf("Unable to start server on %v: %v",
//			cfg.Node.P2P.Listeners, err)
//		return
//	}
//	defer func() {
//		btcdLog.Infof("Gracefully shutting down the server...")
//		server.Stop()
//		server.WaitForShutdown()
//		srvrLog.Infof("Server shutdown complete")
//	}()
//	server.Start()
//
//	<-interrupt
//
//	// Wait until the interrupt signal is received from an OS signal or
//	// shutdown is requested through one of the subsystems such as the RPC
//	// server.
//	//<-interrupt
//	return
//}

//func runShardChain(shardId int, cfg *shards.Config) {
//	fmt.Println("runShardChain: ", shardId)
//	interrupt := interruptListener()
//	// Return now if an interrupt signal was triggered.
//	if interruptRequested(interrupt) {
//		return
//	}
//	chain := shard.Chain()
//	// Load the block database.
//	db, err := loadBlockDB(cfg.DataDir, fmt.Sprintf("shard_%d", shardId), chain, cfg.Node)
//	if err != nil {
//		btcdLog.Errorf("%v", err)
//		return
//	}
//	defer func() {
//		// Ensure the database is sync'd and closed on shutdown.
//		btcdLog.Infof("Gracefully shutting down the database...")
//		if err := db.Close(); err != nil {
//			btcdLog.Errorf("%v", err)
//			return
//		}
//	}()
//
//	// Return now if an interrupt signal was triggered.
//	if interruptRequested(interrupt) {
//		return
//	}
//
//	// Drop indexes and exit if requested.
//	//
//	// NOTE: The order is important here because dropping the tx index also
//	// drops the address index since it relies on it.
//	if cfg.DropAddrIndex {
//		if err := indexers.DropAddrIndex(db, interrupt); err != nil {
//			btcdLog.Errorf("%v", err)
//		}
//
//		return
//	}
//	if cfg.DropTxIndex {
//		if err := indexers.DropTxIndex(db, interrupt); err != nil {
//			btcdLog.Errorf("%v", err)
//			return
//		}
//
//		return
//	}
//	if cfg.DropCfIndex {
//		if err := indexers.DropCfIndex(db, interrupt); err != nil {
//			btcdLog.Errorf("%v", err)
//			return
//		}
//
//		return
//	}
//
//	amgr := addrmgr.New(cfg.DataDir, func(host string) ([]net.IP, error) {
//		if strings.HasSuffix(host, ".onion") {
//			return nil, fmt.Errorf("attempt to resolve tor address %s", host)
//		}
//
//		return cfg.Node.P2P.Lookup(host)
//	})
//
//	l, err := zap.NewProduction()
//	// Create server and start it.
//	server, err := server2.Server(&cfg.Server, amgr, chain, cfg.Beacon.Listeners, cfg.Beacon.AgentBlacklist,
//		cfg.Beacon.AgentWhitelist, db, activeNetParams.Params, interrupt, l)
//	if err != nil {
//		// TODO: this logging could do with some beautifying.
//		btcdLog.Errorf("Unable to start server on %v: %v",
//			cfg.Beacon.Listeners, err)
//		return
//	}
//	defer func() {
//		btcdLog.Infof("Gracefully shutting down the server...")
//		server.Stop()
//		server.WaitForShutdown()
//		srvrLog.Infof("Server shutdown complete")
//	}()
//	server.Start()
//
//	// Wait until the interrupt signal is received from an OS signal or
//	// shutdown is requested through one of the subsystems such as the RPC
//	// server.
//	<-interrupt
//	return
//}

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
