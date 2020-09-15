package shards

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"gitlab.com/jaxnet/core/shard.core.git/addrmgr"
	"gitlab.com/jaxnet/core/shard.core.git/blockchain/indexers"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/shard"
	server2 "gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
	"go.uber.org/zap"
)

func (c *chainController) runShard(ctx context.Context, cfg *Config, shardId uint32) error {
	interrupt := interruptListener()
	// Return now if an interrupt signal was triggered.
	if interruptRequested(interrupt) {
		return errors.New("can't create interrupt request")
	}

	chain := shard.Chain(shardId, cfg.Node.ChainParams())
	c.shards[shardId] = chain

	// Load the block database.
	db, err := c.loadBlockDB(cfg.DataDir, chain, cfg.Node)
	if err != nil {
		c.logger.Error("Can't load Block db", zap.Error(err))
		return err
	}

	defer func() {
		// Ensure the database is sync'd and closed on shutdown.
		c.logger.Info("Gracefully shutting down the database...")
		if err := db.Close(); err != nil {
			c.logger.Error("Can't close db", zap.Error(err))
		}
	}()

	// Drop indexes and exit if requested.
	//
	// NOTE: The order is important here because dropping the tx index also
	// drops the address index since it relies on it.
	if cfg.DropAddrIndex {
		if err := indexers.DropAddrIndex(db, interrupt); err != nil {
			c.logger.Error(fmt.Sprintf("%v", err))
			return err
		}
		return nil
	}
	if cfg.DropTxIndex {
		if err := indexers.DropTxIndex(db, interrupt); err != nil {
			c.logger.Error(fmt.Sprintf("%v", err))
		}

		return nil
	}
	if cfg.DropCfIndex {
		if err := indexers.DropCfIndex(db, interrupt); err != nil {
			c.logger.Error(fmt.Sprintf("%v", err))
		}

		return nil
	}

	amgr := addrmgr.New(cfg.DataDir, func(host string) ([]net.IP, error) {
		if strings.HasSuffix(host, ".onion") {
			return nil, fmt.Errorf("attempt to resolve tor address %s", host)
		}

		return cfg.Node.P2P.Lookup(host)
	})

	c.logger.Info("P2P Listener ", zap.Any("Listeners", cfg.Node.P2P.Listeners))
	// Create server and start it.
	server, err := server2.ShardServer(&cfg.Node.P2P, amgr, chain, db, chain.Params(), interrupt,
		c.logger.With(zap.String("server", "Shard P2P")))
	if err != nil {
		// TODO: this logging could do with some beautifying.
		c.logger.Error(fmt.Sprintf("Unable to start server on %v: %v",
			cfg.Node.P2P.Listeners, err))
		return err
	}
	l := c.logger
	defer func() {
		fmt.Println("Closing... ", l, c.logger)
		l.Info("Gracefully shutting down the server...")
		if err := server.Stop(); err != nil {
			l.Error("Can't stop server ", zap.Error(err))
		}
		server.WaitForShutdown()
		l.Info("Server shutdown complete")
	}()
	server.Start()

	<-interrupt

	return nil
}
