// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package node

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"gitlab.com/jaxnet/core/shard.core/network/addrmgr"
	"gitlab.com/jaxnet/core/shard.core/network/p2p"
	"gitlab.com/jaxnet/core/shard.core/node/chain/beacon"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/types"
	"go.uber.org/zap"
)

type BeaconCtl struct {
	cfg *Config
	ctx context.Context
	log *zap.Logger

	dbCtl DBCtl

	p2pServer     *p2p.Server
	chainProvider *cprovider.ChainProvider
}

func NewBeaconCtl(ctx context.Context, logger *zap.Logger, cfg *Config) BeaconCtl {
	logger = logger.With(zap.String("chain", "beacon"))
	return BeaconCtl{
		cfg:   cfg,
		ctx:   ctx,
		log:   logger,
		dbCtl: DBCtl{logger: logger},
	}

}
func (beaconCtl *BeaconCtl) Init() error {
	params := beaconCtl.cfg.Node.ChainParams()
	params.AutoExpand = params.Net != types.MainNet && beaconCtl.cfg.Node.BeaconChain.AutoExpand
	params.ExpansionRule = beaconCtl.cfg.Node.BeaconChain.ExpansionRule

	chain := beacon.Chain(params)

	// Load the block database.
	db, err := beaconCtl.dbCtl.loadBlockDB(beaconCtl.cfg.DataDir, chain, beaconCtl.cfg.Node)
	if err != nil {
		beaconCtl.log.Error("Can't load Block db", zap.Error(err))
		return err
	}

	blockGen := beacon.NewChainBlockGenerator(beacon.StateProvider{ShardCount: func() (uint32, error) {
		if beaconCtl.chainProvider != nil && beaconCtl.chainProvider.BlockChain() != nil {
			return beaconCtl.chainProvider.ShardCount()
		}
		return 0, nil
	}})

	beaconCtl.chainProvider, err = cprovider.NewChainProvider(beaconCtl.ctx,
		beaconCtl.cfg.Node.BeaconChain, chain, blockGen, db, beaconCtl.log)
	if err != nil {
		beaconCtl.log.Error("unable to init ChainProvider for beacon", zap.Error(err))
		return err
	}

	addrManager := addrmgr.New(beaconCtl.cfg.DataDir, chain.Params().Name, func(host string) ([]net.IP, error) {
		if strings.HasSuffix(host, ".onion") {
			return nil, fmt.Errorf("attempt to resolve tor address %s", host)
		}

		return beaconCtl.cfg.Node.P2P.Lookup(host)
	})

	beaconCtl.log.Info("P2P Listener ", zap.Any("Listeners", beaconCtl.cfg.Node.P2P.Listeners))
	port, _ := strconv.ParseInt(chain.Params().DefaultPort, 10, 16)
	// Create p2pServer.
	beaconCtl.p2pServer, err = p2p.NewServer(&beaconCtl.cfg.Node.P2P, beaconCtl.chainProvider, addrManager, p2p.ListenOpts{
		DefaultPort: int(port),
		Listeners:   beaconCtl.cfg.Node.P2P.Listeners,
	})
	if err != nil {
		beaconCtl.log.Error(fmt.Sprintf("Unable to start p2pServer on %v: %v",
			beaconCtl.cfg.Node.P2P.Listeners, err))
		return err
	}

	// todo: improve
	return beaconCtl.chainProvider.SetP2PProvider(beaconCtl.p2pServer)
}

func (beaconCtl *BeaconCtl) ChainProvider() *cprovider.ChainProvider {
	return beaconCtl.chainProvider
}

func (beaconCtl *BeaconCtl) Run(ctx context.Context) {
	cleanIndexes, err := beaconCtl.dbCtl.cleanIndexes(ctx, beaconCtl.cfg, beaconCtl.chainProvider.DB)
	if cleanIndexes {
		beaconCtl.log.Info("clean db indexes")
		return
	}

	if err != nil {
		beaconCtl.log.Error("failed to clean indexes", zap.Error(err))
		return
	}

	beaconCtl.p2pServer.Run(ctx)

	<-ctx.Done()

	beaconCtl.log.Info("Gracefully shutting down the database...")
	if err := beaconCtl.chainProvider.DB.Close(); err != nil {
		beaconCtl.log.Error("Can't close db", zap.Error(err))
	}
}
