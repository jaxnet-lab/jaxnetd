package shards

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"gitlab.com/jaxnet/core/shard.core.git/addrmgr"
	"gitlab.com/jaxnet/core/shard.core.git/mining"
	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/database"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/beacon"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/server"
	"go.uber.org/zap"
)

type BeaconCtl struct {
	cfg *Config
	ctx context.Context
	log *zap.Logger

	db        database.DB
	dbCtl     DBCtl
	chain     chain.IChain
	shardsMgr server.ShardManager

	actor      *server.NodeActor
	p2pServer  *server.P2PServer
	rpcServer  *server.RPCServer
	blockchain *blockchain.BlockChain
}

func NewBeaconCtl(ctx context.Context, logger *zap.Logger, cfg *Config, shardsMgr server.ShardManager) BeaconCtl {
	logger = logger.With(zap.String("chain", "beacon"))
	return BeaconCtl{
		cfg:       cfg,
		ctx:       ctx,
		log:       logger,
		dbCtl:     DBCtl{logger: logger},
		shardsMgr: shardsMgr,
	}

}
func (beaconCtl *BeaconCtl) Init() error {

	beaconCtl.chain = beacon.Chain(beaconCtl.cfg.Node.ChainParams())

	var err error
	// Load the block database.
	beaconCtl.db, err = beaconCtl.dbCtl.loadBlockDB(beaconCtl.cfg.DataDir, beaconCtl.chain, beaconCtl.cfg.Node)
	if err != nil {
		beaconCtl.log.Error("Can't load Block db", zap.Error(err))
		return err
	}

	addrManager := addrmgr.New(beaconCtl.cfg.DataDir, func(host string) ([]net.IP, error) {
		if strings.HasSuffix(host, ".onion") {
			return nil, fmt.Errorf("attempt to resolve tor address %s", host)
		}

		return beaconCtl.cfg.Node.P2P.Lookup(host)
	})

	beaconCtl.log.Info("P2P Listener ", zap.Any("Listeners", beaconCtl.cfg.Node.P2P.Listeners))
	// Create p2pServer and start it.
	beaconCtl.p2pServer, err = server.Server(
		beaconCtl.ctx,
		&beaconCtl.cfg.Node.P2P,
		addrManager,
		beaconCtl.chain,
		beaconCtl.cfg.Node.P2P.Listeners,
		beaconCtl.cfg.Node.P2P.AgentBlacklist,
		beaconCtl.cfg.Node.P2P.AgentWhitelist,
		beaconCtl.db,
		beaconCtl.log.With(zap.String("p2pServer", "Beacon P2P")),
	)
	if err != nil {
		// TODO: this logging could do with some beautifying.
		beaconCtl.log.Error(fmt.Sprintf("Unable to start p2pServer on %v: %v",
			beaconCtl.cfg.Node.P2P.Listeners, err))
		return err
	}
	beaconCtl.blockchain = beaconCtl.p2pServer.BlockChain()

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
	beaconCtl.actor = &server.NodeActor{
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

		ShardsMgr:   ctrl,
		Chain:       server.BlockChain(),
		ChainParams: chain.Params(),
		TxMemPool:   server.TxMemPool,
	}

	beaconCtl.rpcServer, err = server.RpcServer(&beaconCtl.cfg.Node.RPC, beaconCtl.actor, beaconCtl.log)
	if err != nil {
		return err
	}
	return nil
}

func (beaconCtl *BeaconCtl) Run(ctx context.Context) {
	cleanIndexes, err := beaconCtl.dbCtl.cleanIndexes(ctx, beaconCtl.cfg, beaconCtl.db)
	if cleanIndexes {
		beaconCtl.log.Info("clean db indexes")
		return
	}

	if err != nil {
		beaconCtl.log.Error("failed to clean indexes", zap.Error(err))
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		beaconCtl.p2pServer.Run(ctx)
	}()

	go func() {
		defer wg.Done()
		beaconCtl.rpcServer.Start(ctx)
	}()

	<-ctx.Done()
	wg.Wait()

	beaconCtl.log.Info("Gracefully shutting down the database...")
	if err := beaconCtl.db.Close(); err != nil {
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
	if err := beaconCtl.db.Close(); err != nil {
		beaconCtl.log.Error("Can't close db", zap.Error(err))
	}
}

func (chainCtl *chainController) runBeacon(ctx context.Context, cfg *Config) error {
	if interruptRequested(ctx) {
		return errors.New("can't create interrupt request")
	}

	chainCtl.beacon = NewBeaconCtl(ctx, chainCtl.logger, cfg, chainCtl)
	if err := chainCtl.beacon.Init(); err != nil {
		chainCtl.logger.Error("Can't init Beacon chainCtl", zap.Error(err))
		return err
	}

	go chainCtl.beacon.Run(ctx)

	<-ctx.Done()

	return nil
}
