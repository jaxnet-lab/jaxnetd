// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package node

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/network/addrmgr"
	"gitlab.com/jaxnet/jaxnetd/network/p2p"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx/btcd"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
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
	cfg := beaconCtl.cfg
	params := cfg.Node.ChainParams()
	if params.Net != wire.MainNet {
		params.AutoExpand = cfg.Node.BeaconChain.AutoExpand
		params.InitialExpansionRule = cfg.Node.BeaconChain.ExpansionRule
		params.InitialExpansionLimit = cfg.Node.BeaconChain.ExpansionLimit
	}

	params.IsBeacon = true
	beaconChain := chainctx.NewBeaconChain(params)

	// initialize chainProvider instance
	{
		// Load the block database.
		db, err := beaconCtl.dbCtl.loadBlockDB(cfg.DataDir, beaconChain, cfg.Node)
		if err != nil {
			beaconCtl.log.Error().Err(err).Msg("Can't load Block db")
			return err
		}

		mAddreses, err := cfg.Node.BeaconChain.ParseMiningAddresses(params)
		if err != nil {
			beaconCtl.log.Error().Err(err).Msg("Can't parse mining addresses")
			return err
		}

		btcdProvider, err := btcd.NewBlockProvider(beaconCtl.cfg.BTCD, mAddreses[0])
		if err != nil {
			beaconCtl.log.Error().Err(err).Msg("Can't init jaxnetdProvider")
			return err
		}

		bsp := chaindata.StateProvider{
			ShardCount: func() (uint32, error) {
				if beaconCtl.chainProvider != nil && beaconCtl.chainProvider.BlockChain() != nil {
					return beaconCtl.chainProvider.ShardCount()
				}
				return 0, nil
			},
			BTCGen: btcdProvider,
		}

		blockGen := chaindata.NewBeaconBlockGen(bsp, params.PowParams)

		chainProvider, err := cprovider.NewChainProvider(beaconCtl.ctx,
			cfg.Node.BeaconChain, beaconChain, blockGen, db, beaconCtl.log)
		if err != nil {
			beaconCtl.log.Error().Err(err).Msg("unable to init ChainProvider for beacon")
			return err
		}

		beaconCtl.chainProvider = chainProvider
	}

	// initialize p2pServer instance
	{
		addrManager := addrmgr.New(cfg.DataDir, beaconChain.Params().ChainName, cfg.Node.P2P.Lookup)
		port, _ := strconv.ParseInt(beaconChain.Params().DefaultPort, 10, 16)

		// Create p2pServer.
		p2pServer, err := p2p.NewServer(&cfg.Node.P2P,
			beaconCtl.chainProvider,
			addrManager,
			p2p.ListenOpts{
				DefaultPort: int(port),
				Listeners:   beaconCtl.cfg.Node.P2P.Listeners,
			},
		)
		if err != nil {
			beaconCtl.log.Error().Msgf("Unable to start p2pServer on %v: %v", beaconCtl.cfg.Node.P2P.Listeners, err)
			return err
		}

		beaconCtl.p2pServer = p2pServer
	}

	// todo: improve
	return beaconCtl.chainProvider.SetP2PProvider(beaconCtl.p2pServer)
}

func (beaconCtl *BeaconCtl) ChainCtx() chainctx.IChainCtx {
	return beaconCtl.ChainProvider().ChainCtx
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

	err = beaconCtl.dbCtl.refillIndexes(ctx, beaconCtl.cfg, beaconCtl.chainProvider.DB)
	if err != nil {
		beaconCtl.log.Error().Err(err).Msg("failed to refill indexes")
		return
	}

	beaconCtl.p2pServer.Run(ctx)

	<-ctx.Done()

	if beaconCtl.cfg.Node.DumpMMR {
		tree := beaconCtl.chainProvider.BlockChain().MMRTree()
		data, err := json.Marshal(tree)
		if err != nil {
			beaconCtl.log.Error().Err(err).Msg("Can't serialize MMT Tree")
		} else {
			filePath := filepath.Join(beaconCtl.cfg.DataDir, "beacon_mmr.json")
			err = ioutil.WriteFile(filePath, data, 0755)
			if err != nil {
				beaconCtl.log.Error().Err(err).Msg("Can't serialize MMT Tree")
			}
		}
	}

	beaconCtl.log.Info().Msg("Gracefully shutting down the database...")
	if err := beaconCtl.chainProvider.DB.Close(); err != nil {
		beaconCtl.log.Error().Err(err).Msg("Can't close db")
	}
}

func (beaconCtl *BeaconCtl) Stats() map[string]float64 {
	chainStats := beaconCtl.chainProvider.Stats()

	chainStats["p2p_total_connected"] = float64(beaconCtl.p2pServer.ConnectedCount())
	bytesReceived, bytesSent := beaconCtl.p2pServer.NetTotals()
	chainStats["p2p_bytes_received"] = float64(bytesReceived)
	chainStats["p2p_bytes_sent"] = float64(bytesSent)

	stats := beaconCtl.p2pServer.PeerStateStats()
	chainStats["p2p_peer_state_in"] = float64(stats.InboundPeers)
	chainStats["p2p_peer_state_out"] = float64(stats.OutboundPeers)
	chainStats["p2p_peer_state_banned"] = float64(stats.Banned)
	chainStats["p2p_peer_state_outgroups"] = float64(stats.OutboundGroups)
	chainStats["p2p_peer_state_total"] = float64(stats.Total)

	return chainStats
}
