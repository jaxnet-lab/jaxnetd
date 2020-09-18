package shards

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"gitlab.com/jaxnet/core/shard.core.git/addrmgr"
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
	actor := &server2.NodeActor{
		ShardsMgr:    ctrl,
		Chain:        server.BlockChain(),
		ConnMgr:      nil,
		SyncMgr:      nil,
		TimeSource:   nil,
		ChainParams:  nil,
		DB:           nil,
		TxMemPool:    nil,
		Generator:    nil,
		TxIndex:      nil,
		AddrIndex:    nil,
		CfIndex:      nil,
		FeeEstimator: nil,
	}

	go ctrl.runRpc(ctx, cfg, actor)

	<-ctx.Done()

	return nil
}
