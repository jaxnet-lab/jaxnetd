package shards

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"gitlab.com/jaxnet/core/shard.core.git/addrmgr"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/shard"
	server2 "gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
	"go.uber.org/zap"
)

func (ctrl *chainController) EnableShard(shardID uint32) error {
	ctrl.wg.Add(1)

	go ctrl.runShardRoutine(shardID)

	return nil
}

func (ctrl *chainController) ListShards() map[uint32]string {
	ctrl.shardsMutex.RLock()
	list := map[uint32]string{}

	for id, iChain := range ctrl.shards {
		list[id] = iChain.Params().Name
	}

	return list
}

func (ctrl *chainController) DisableShard(shardID uint32) error {
	return nil
}

func (ctrl *chainController) NewShard(shardID uint32, height int64) error {
	ctrl.wg.Add(1)

	go ctrl.runShardRoutine(shardID)

	return nil
}

func (ctrl *chainController) runShardRoutine(shardID uint32) {
	defer func() {
		ctrl.wg.Done()
		ctrl.shardsMutex.Lock()
		delete(ctrl.shards, shardID)
		ctrl.shardsMutex.Unlock()
	}()

	err := ctrl.runShard(ctrl.ctx, ctrl.cfg, shardID)
	if err != nil {
		ctrl.logger.Error("shard run interrupted",
			zap.Uint32("shard_id", shardID), zap.Error(err))
	}
}

func (ctrl *chainController) runShard(ctx context.Context, cfg *Config, shardID uint32) error {
	if interruptRequested(ctx) {
		return errors.New("can't create interrupt request")
	}

	ctrl.shardsMutex.Lock()
	chain := shard.Chain(shardID, cfg.Node.ChainParams())
	ctrl.shards[shardID] = chain
	ctrl.shardsMutex.Unlock()

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

	ctrl.logger.Info("Run P2P Listener ", zap.Any("Listeners", cfg.Node.P2P.Listeners))

	// Create server and start it.
	server, err := server2.ShardServer(ctx, &cfg.Node.P2P, amgr, chain, db,
		ctrl.logger.With(zap.String("server", "Shard P2P")))
	if err != nil {
		ctrl.logger.Error("Unable to start server",
			zap.Any("address", cfg.Node.P2P.Listeners), zap.Error(err))
		return err
	}
	server.Start()

	<-ctx.Done()

	ctrl.logger.Info("Gracefully shutting down the server...")
	if err := server.Stop(); err != nil {
		ctrl.logger.Error("Can't stop server ", zap.Error(err))
	}

	server.WaitForShutdown()
	ctrl.logger.Info("Server shutdown complete")

	return nil
}
