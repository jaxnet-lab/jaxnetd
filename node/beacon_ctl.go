package node

import (
	"context"
	"fmt"
	"net"
	"strings"

	"gitlab.com/jaxnet/core/shard.core/network/addrmgr"
	"gitlab.com/jaxnet/core/shard.core/network/p2p"
	"gitlab.com/jaxnet/core/shard.core/node/chain/beacon"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/node/mining"
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
	chain := beacon.Chain(beaconCtl.cfg.Node.ChainParams())

	// Load the block database.
	db, err := beaconCtl.dbCtl.loadBlockDB(beaconCtl.cfg.DataDir, chain, beaconCtl.cfg.Node)
	if err != nil {
		beaconCtl.log.Error("Can't load Block db", zap.Error(err))
		return err
	}

	beaconCtl.chainProvider, err = cprovider.NewChainProvider(beaconCtl.ctx,
		beaconCtl.cfg.Node.BeaconChain, chain, db, beaconCtl.log)
	if err != nil {
		beaconCtl.log.Error("unable to init ChainProvider for beacon", zap.Error(err))
		return err
	}

	addrManager := addrmgr.New(beaconCtl.cfg.DataDir, func(host string) ([]net.IP, error) {
		if strings.HasSuffix(host, ".onion") {
			return nil, fmt.Errorf("attempt to resolve tor address %s", host)
		}

		return beaconCtl.cfg.Node.P2P.Lookup(host)
	})

	beaconCtl.log.Info("P2P Listener ", zap.Any("Listeners", beaconCtl.cfg.Node.P2P.Listeners))

	// Create p2pServer.
	beaconCtl.p2pServer, err = p2p.NewServer(&beaconCtl.cfg.Node.P2P, beaconCtl.chainProvider, addrManager, p2p.ListenOpts{
		DefaultPort: chain.Params().DefaultPort,
		Listeners:   beaconCtl.cfg.Node.P2P.Listeners,
	})
	if err != nil {
		// TODO: this logging could do with some beautifying.
		beaconCtl.log.Error(fmt.Sprintf("Unable to start p2pServer on %v: %v",
			beaconCtl.cfg.Node.P2P.Listeners, err))
		return err
	}

	// todo: improve
	return beaconCtl.chainProvider.SetP2PProvider(beaconCtl.p2pServer)
}

func (beaconCtl *BeaconCtl) BlkTmplGenerator() *mining.BlkTmplGenerator {
	// Create the mining policy and block template generator based on the
	// configuration options.
	policy := mining.Policy{
		BlockMinWeight:    beaconCtl.cfg.Node.BeaconChain.BlockMinWeight,
		BlockMaxWeight:    beaconCtl.cfg.Node.BeaconChain.BlockMaxWeight,
		BlockMinSize:      beaconCtl.cfg.Node.BeaconChain.BlockMinSize,
		BlockMaxSize:      beaconCtl.cfg.Node.BeaconChain.BlockMaxSize,
		BlockPrioritySize: beaconCtl.cfg.Node.BeaconChain.BlockPrioritySize,
		TxMinFreeFee:      beaconCtl.cfg.Node.BeaconChain.MinRelayTxFeeValues,
	}
	return mining.NewBlkTmplGenerator(&policy,
		beaconCtl.chainProvider.ChainCtx,
		beaconCtl.chainProvider.ChainCtx,
		beaconCtl.chainProvider.TxMemPool,
		beaconCtl.chainProvider.BlockChain())
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

func (beaconCtl *BeaconCtl) Shutdown() {
	beaconCtl.log.Info("Gracefully shutting down the p2pServer...")
	if err := beaconCtl.p2pServer.Stop(); err != nil {
		beaconCtl.log.Error("Can't stop p2pServer ", zap.Error(err))
	} else {
		beaconCtl.p2pServer.WaitForShutdown()
		beaconCtl.log.Info("Server shutdown complete")
	}

	// Ensure the database is sync'd and closed on shutdown.
	beaconCtl.log.Info("Gracefully shutting down the database...")
	if err := beaconCtl.chainProvider.DB.Close(); err != nil {
		beaconCtl.log.Error("Can't close db", zap.Error(err))
	}
}
