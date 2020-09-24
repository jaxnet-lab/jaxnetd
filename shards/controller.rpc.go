package shards

import (
	"context"

	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
)

func (chainCtl *chainController) runRpc(ctx context.Context, cfg *Config) error {
	beaconActor, err := chainCtl.beacon.ChainActor()
	if err != nil {
		return err
	}

	var shardsActors = map[uint32]*server.ChainActor{}
	for id, ro := range chainCtl.shardsCtl {
		actor, err := ro.ctl.ChainActor()
		if err != nil {
			return err
		}
		shardsActors[id] = actor
	}

	srv := server.NewMultiChainRPC(&cfg.Node.RPC, chainCtl.logger, beaconActor, shardsActors)
	chainCtl.wg.Add(1)

	go func() {
		srv.Run(ctx)
		chainCtl.wg.Done()

	}()

	return nil
}

func (chainCtl *chainController) runShards() error {
	if err := chainCtl.syncShardsIndex(); err != nil {
		return err
	}

	defer chainCtl.updateShardsIndex()

	chainCtl.beacon.blockchain.Subscribe(func(not *blockchain.Notification) {
		if not.Type != blockchain.NTBlockConnected {
			return
		}
		block, ok := not.Data.(*btcutil.Block)
		if !ok {
			chainCtl.logger.Warn("block notification data is not a *btcutil.Block")
			return
		}

		msgBlock := block.MsgBlock()
		version := msgBlock.Header.Version()

		if !version.ExpansionMade() {
			return
		}

		chainCtl.shardsIndex.AddShard(block)
		chainCtl.runShardRoutine(chainCtl.shardsIndex.LastShardID, msgBlock, block.Height())

	})

	for shardID, info := range chainCtl.shardsIndex.Shards {
		err := chainCtl.NewShard(shardID, info.GenesisHeight)
		if err != nil {
			return err
		}
	}
	return nil
}
