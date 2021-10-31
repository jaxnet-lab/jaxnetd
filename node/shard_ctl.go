// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/network/addrmgr"
	"gitlab.com/jaxnet/jaxnetd/network/p2p"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type ShardInfo struct {
	ID            uint32         `json:"id"`
	LastVersion   wire.BVersion  `json:"last_version"`
	GenesisHeight int32          `json:"genesis_height"`
	GenesisHash   string         `json:"genesis_hash"`
	Enabled       bool           `json:"enabled"`
	P2PInfo       p2p.ListenOpts `json:"p2p_info"`
}

type Index struct {
	LastShardID      uint32               `json:"last_shard_id"`
	LastBeaconHeight int32                `json:"last_beacon_height"`
	Shards           map[uint32]ShardInfo `json:"shards"`
}

func (index *Index) AddShard(block *jaxutil.Block, opts p2p.ListenOpts) uint32 {
	if index.LastBeaconHeight < block.Height() {
		index.LastBeaconHeight = block.Height()
	}
	shardID := block.MsgBlock().Header.BeaconHeader().Shards()
	if index.LastShardID < shardID {
		index.LastShardID = shardID
	}

	index.Shards[shardID] = ShardInfo{
		ID:            shardID,
		LastVersion:   block.MsgBlock().Header.Version(),
		GenesisHeight: block.Height(),
		GenesisHash:   block.Hash().String(),
		Enabled:       true,
		P2PInfo:       opts,
	}

	return shardID
}

type shardRO struct {
	ctl    *ShardCtl
	port   int
	cancel context.CancelFunc
}

type ShardCtl struct {
	log   zerolog.Logger
	chain chainctx.IChainCtx
	cfg   *Config
	ctx   context.Context

	dbCtl         DBCtl
	p2pServer     *p2p.Server
	chainProvider *cprovider.ChainProvider
	listenCfg     p2p.ListenOpts
}

func NewShardCtl(ctx context.Context, log zerolog.Logger, cfg *Config,
	chain chainctx.IChainCtx, listenCfg p2p.ListenOpts) *ShardCtl {
	log = log.With().Str("chain", chain.Name()).Logger()

	return &ShardCtl{
		ctx:       ctx,
		cfg:       cfg,
		log:       log,
		chain:     chain,
		dbCtl:     DBCtl{logger: log},
		listenCfg: listenCfg,
	}
}

func (shardCtl *ShardCtl) Init(beaconBlockGen chaindata.BeaconBlockProvider) error {
	// Load the block database.
	db, err := shardCtl.dbCtl.loadBlockDB(shardCtl.cfg.DataDir, shardCtl.chain, shardCtl.cfg.Node)
	if err != nil {
		shardCtl.log.Error().Err(err).Msg("Can't load Block db")
		return err
	}

	blockGen := chaindata.NewShardBlockGen(shardCtl.chain, beaconBlockGen)

	shardCtl.chainProvider, err = cprovider.NewChainProvider(shardCtl.ctx,
		shardCtl.cfg.Node.BeaconChain, shardCtl.chain, blockGen, db, shardCtl.log)
	if err != nil {
		shardCtl.log.Error().Err(err).Msg("unable to init ChainProvider for shard")
		return err
	}

	addrManager := addrmgr.New(shardCtl.cfg.DataDir, shardCtl.chain.Name(),
		func(host string) ([]net.IP, error) {
			if strings.HasSuffix(host, ".onion") {
				return nil, fmt.Errorf("attempt to resolve tor address %s", host)
			}
			return shardCtl.cfg.Node.P2P.Lookup(host)
		})

	shardCtl.log.Info().Interface("Listeners", shardCtl.cfg.Node.P2P.Listeners).Msg("Run P2P Listener ")

	// Create p2pServer and start it.
	shardCtl.p2pServer, err = p2p.NewServer(&shardCtl.cfg.Node.P2P, shardCtl.chainProvider,
		addrManager, shardCtl.listenCfg)
	if err != nil {
		shardCtl.log.Error().Err(err).
			Msg("Unable to start p2pServer")
		return err
	}

	return shardCtl.chainProvider.SetP2PProvider(shardCtl.p2pServer)
}

func (shardCtl *ShardCtl) ChainProvider() *cprovider.ChainProvider {
	return shardCtl.chainProvider
}

func (shardCtl *ShardCtl) Run(ctx context.Context) {
	cleanIndexes, err := shardCtl.dbCtl.cleanIndexes(ctx, shardCtl.cfg, shardCtl.chainProvider.DB)
	if cleanIndexes {
		shardCtl.log.Info().Msg("clean db indexes")
		return
	}

	if err != nil {
		shardCtl.log.Error().Err(err).Msg("failed to clean indexes")
		return
	}

	err = shardCtl.dbCtl.refillIndexes(ctx, shardCtl.cfg, shardCtl.chainProvider.DB)
	if err != nil {
		shardCtl.log.Error().Err(err).Msg("failed to refill indexes")
		return
	}
	shardCtl.p2pServer.Run(ctx)

	<-ctx.Done()

	if shardCtl.cfg.Node.DumpMMR {
		tree := shardCtl.chainProvider.BlockChain().MMRTree()
		data, err := json.Marshal(tree)
		if err != nil {
			shardCtl.log.Error().Err(err).Msg("Can't serialize MMT Tree")
		} else {
			filePath := filepath.Join(shardCtl.cfg.DataDir, shardCtl.chain.Name()+"_mmr.json")
			err = ioutil.WriteFile(filePath, data, 0755)
			if err != nil {
				shardCtl.log.Error().Err(err).Msg("Can't serialize MMT Tree")
			}
		}
	}

	shardCtl.log.Info().Msg("ShardChain p2p server shutdown complete")
	shardCtl.log.Info().Msg("Gracefully shutting down the database...")
	if err := shardCtl.chainProvider.DB.Close(); err != nil {
		shardCtl.log.Error().Err(err).Msg("Can't close db")
	}
}

func (shardCtl *ShardCtl) ChainCtx() chainctx.IChainCtx {
	return shardCtl.ChainProvider().ChainCtx
}

func (shardCtl *ShardCtl) Stats() map[string]float64 {
	chainStats := shardCtl.chainProvider.Stats()

	chainStats["p2p_total_connected"] = float64(shardCtl.p2pServer.ConnectedCount())
	bytesReceived, bytesSent := shardCtl.p2pServer.NetTotals()
	chainStats["p2p_bytes_received"] = float64(bytesReceived)
	chainStats["p2p_bytes_sent"] = float64(bytesSent)

	stats := shardCtl.p2pServer.PeerStateStats()
	chainStats["p2p_peer_state_in"] = float64(stats.InboundPeers)
	chainStats["p2p_peer_state_out"] = float64(stats.OutboundPeers)
	chainStats["p2p_peer_state_banned"] = float64(stats.Banned)
	chainStats["p2p_peer_state_outgroups"] = float64(stats.OutboundGroups)
	chainStats["p2p_peer_state_total"] = float64(stats.Total)

	return chainStats
}
