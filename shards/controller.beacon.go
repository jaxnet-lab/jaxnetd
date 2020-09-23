package shards

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"gitlab.com/jaxnet/core/shard.core.git/addrmgr"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/mining"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/beacon"
	server2 "gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
	"go.uber.org/zap"
)

func (ctrl *chainController) runBeacon(ctx context.Context, cfg *Config) error {
	if interruptRequested(ctx) {
		return errors.New("can't create interrupt request")
	}

	chain := beacon.Chain(cfg.Node.ChainParams())
	ctrl.beacon = chain

	// Load the block database.
	db, err := ctrl.loadBlockDB(cfg.DataDir, chain, cfg.Node)
	if err != nil {
		ctrl.logger.Error("Can't load Block db", zap.Error(err))
		return err
	}
	defer func() {
		// Ensure the database is sync'd and closed on shutdown.
		ctrl.logger.Info("Gracefully shutting down the database...")
		if err := db.Close(); err != nil {
			ctrl.logger.Error("Can't close db", zap.Error(err))
		}
	}()

	cleanSmth, err := ctrl.cleanIndexes(ctx, cfg, db)
	if cleanSmth || err != nil {
		return err
	}

	amgr := addrmgr.New(cfg.DataDir, func(host string) ([]net.IP, error) {
		if strings.HasSuffix(host, ".onion") {
			return nil, fmt.Errorf("attempt to resolve tor address %s", host)
		}

		return cfg.Node.P2P.Lookup(host)
	})

	ctrl.logger.Info("P2P Listener ", zap.Any("Listeners", cfg.Node.P2P.Listeners))
	// Create server and start it.
	server, err := server2.Server(
		ctx,
		&cfg.Node.P2P,
		amgr,
		chain,
		cfg.Node.P2P.Listeners,
		cfg.Node.P2P.AgentBlacklist,
		cfg.Node.P2P.AgentWhitelist,
		db,
		ctrl.logger.With(zap.String("server", "Beacon P2P")),
	)
	if err != nil {
		// TODO: this logging could do with some beautifying.
		ctrl.logger.Error(fmt.Sprintf("Unable to start server on %v: %v",
			cfg.Node.P2P.Listeners, err))
		return err
	}

	l := ctrl.logger
	defer func() {
		l.Info("Gracefully shutting down the server...")
		if err := server.Stop(); err != nil {
			l.Error("Can't stop server ", zap.Error(err))
		}
		server.WaitForShutdown()
		l.Info("Server shutdown complete")
	}()
	server.Start()

	// todo(mike)
	policy := mining.Policy{
		BlockMinWeight:    cfg.Node.P2P.BlockMinWeight,
		BlockMaxWeight:    cfg.Node.P2P.BlockMaxWeight,
		BlockMinSize:      cfg.Node.P2P.BlockMinSize,
		BlockMaxSize:      cfg.Node.P2P.BlockMaxSize,
		BlockPrioritySize: cfg.Node.P2P.BlockPrioritySize,
		TxMinFreeFee:      cfg.Node.P2P.MinRelayTxFeeValues,
	}
	blockTemplateGenerator := mining.NewBlkTmplGenerator(&policy,
		chain.Params(), server.TxMemPool, server.BlockChain(), server.TimeSource,
		server.SigCache, server.HashCache)

	listeners, err := setupRPCListeners(cfg.Node.RPC.ListenerAddresses)
	if err != nil {
		return err
	}

	miningAddrs := make([]btcutil.Address, 0, len(cfg.Node.MiningAddresses))
	for _, address := range cfg.Node.MiningAddresses {
		addr, err := btcutil.DecodeAddress(address, chain.Params())
		if err != nil {
			return err
		}

		miningAddrs = append(miningAddrs, addr)
	}

	actor := &server2.NodeActor{
		StartupTime:  server.StartupTime,
		Listeners:    listeners,
		ConnMgr:      &server2.RPCConnManager{Server: server},
		SyncMgr:      &server2.RPCSyncMgr{Server: server, SyncMgr: server.SyncManager},
		TimeSource:   server.TimeSource,
		DB:           db,
		Generator:    blockTemplateGenerator,
		TxIndex:      server.TxIndex,
		AddrIndex:    server.AddrIndex,
		CfIndex:      server.CfIndex,
		FeeEstimator: server.FeeEstimator,
		MiningAddrs:  miningAddrs,

		ShardsMgr:   ctrl,
		Chain:       server.BlockChain(),
		ChainParams: chain.Params(),
		TxMemPool:   server.TxMemPool,
	}

	go func() {
		ctrl.runRpc(ctx, cfg, actor)
	}()

	<-ctx.Done()

	return nil
}
