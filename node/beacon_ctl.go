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

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/core/shard.core/network/addrmgr"
	"gitlab.com/jaxnet/core/shard.core/network/p2p"
	"gitlab.com/jaxnet/core/shard.core/node/chain/beacon"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/types"
)

type BeaconCtl struct {
	cfg *Config
	ctx context.Context
	log zerolog.Logger

	dbCtl DBCtl

	p2pServer     *p2p.Server
	chainProvider *cprovider.ChainProvider
}

func NewBeaconCtl(ctx context.Context, logger zerolog.Logger, cfg *Config) BeaconCtl {
	logger = logger.With().Str("chain", "beacon").Logger()
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
		beaconCtl.log.Error().Err(err).Msg("Can't load Block db")
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
		beaconCtl.log.Error().Err(err).Msg("unable to init ChainProvider for beacon")
		return err
	}

	addrManager := addrmgr.New(beaconCtl.cfg.DataDir, chain.Params().Name, func(host string) ([]net.IP, error) {
		if strings.HasSuffix(host, ".onion") {
			return nil, fmt.Errorf("attempt to resolve tor address %s", host)
		}

		return beaconCtl.cfg.Node.P2P.Lookup(host)
	})

	port, _ := strconv.ParseInt(chain.Params().DefaultPort, 10, 16)
	// Create p2pServer.
	beaconCtl.p2pServer, err = p2p.NewServer(&beaconCtl.cfg.Node.P2P, beaconCtl.chainProvider, addrManager,
		p2p.ListenOpts{DefaultPort: int(port),
			Listeners: beaconCtl.cfg.Node.P2P.Listeners,
		})
	if err != nil {
		beaconCtl.log.Error().Msgf("Unable to start p2pServer on %v: %v", beaconCtl.cfg.Node.P2P.Listeners, err)
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
		beaconCtl.log.Info().Msg("clean db indexes")
		return
	}

	if err != nil {
		beaconCtl.log.Error().Err(err).Msg("failed to clean indexes")
		return
	}

	beaconCtl.p2pServer.Run(ctx)

	<-ctx.Done()

	beaconCtl.log.Info().Msg("Gracefully shutting down the database...")
	if err := beaconCtl.chainProvider.DB.Close(); err != nil {
		beaconCtl.log.Error().Err(err).Msg("Can't close db")
	}
}
