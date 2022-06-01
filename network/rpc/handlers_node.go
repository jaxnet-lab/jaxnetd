// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
// nolint: forcetypeassert
package rpc

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/node/mining/cpuminer"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
	"gitlab.com/jaxnet/jaxnetd/version"
)

type NodeRPC struct {
	Mux

	// shardsMgr provides ability to manipulate running shards.
	shardsMgr   ShardManager
	metricsMgr  MetricsManager
	beaconChain *cprovider.ChainProvider

	// fixme: this fields are empty
	helpCache   *helpCacher
	StartupTime int64
	MiningAddrs []jaxutil.Address
	CPUMiner    *cpuminer.CPUMiner
}

func NewNodeRPC(beaconChain *cprovider.ChainProvider, shardsMgr ShardManager, logger zerolog.Logger, metricsMgr MetricsManager,
	cpuMiner *cpuminer.CPUMiner,
) *NodeRPC {
	rpc := &NodeRPC{
		Mux:         NewRPCMux(logger),
		shardsMgr:   shardsMgr,
		metricsMgr:  metricsMgr,
		beaconChain: beaconChain,
		MiningAddrs: beaconChain.MiningAddrs,
		CPUMiner:    cpuMiner,
	}
	rpc.ComposeHandlers()

	return rpc
}

func (server *NodeRPC) ComposeHandlers() {
	server.SetCommands(server.OwnHandlers())
}

func (server *NodeRPC) OwnHandlers() map[jaxjson.MethodName]CommandHandler {
	return map[jaxjson.MethodName]CommandHandler{
		jaxjson.ScopedMethod("node", "version"): server.handleVersion,
		jaxjson.ScopedMethod("node", "getInfo"): server.handleGetInfo,
		jaxjson.ScopedMethod("node", "uptime"):  server.handleUptime,

		jaxjson.ScopedMethod("node", "manageShards"):         server.handleManageShards,
		jaxjson.ScopedMethod("node", "listShards"):           server.handleListShards,
		jaxjson.ScopedMethod("node", "estimateSwapLockTime"): server.handleEstimateLockTime,

		jaxjson.ScopedMethod("node", "generate"):    server.handleGenerate,
		jaxjson.ScopedMethod("node", "setGenerate"): server.handleSetGenerate,

		jaxjson.ScopedMethod("node", "debugLevel"):      server.handleDebugLevel,
		jaxjson.ScopedMethod("node", "stop"):            server.handleStop,
		jaxjson.ScopedMethod("node", "help"):            server.handleHelp,
		jaxjson.ScopedMethod("node", "getnodemetrics"):  server.handleGetNodeMetrics,
		jaxjson.ScopedMethod("node", "getchainmetrics"): server.handleGetChainMetrics,
	}
}

// handleVersion implements the version command.
//
// NOTE: This is a btcsuite extension ported from github.com/decred/dcrd.
func (server *NodeRPC) handleVersion(ctx CmdCtx) (interface{}, error) {
	return jaxjson.NodeVersion{
		Node: version.GetExtendedVersion(),
		RPC: jaxjson.VersionResult{
			VersionString: jsonrpcSemverString,
			Major:         jsonrpcSemverMajor,
			Minor:         jsonrpcSemverMinor,
			Patch:         jsonrpcSemverPatch,
		},
	}, nil
}

// handleUptime implements the uptime command.
func (server *NodeRPC) handleUptime(ctx CmdCtx) (interface{}, error) {
	return time.Now().Unix() - server.StartupTime, nil
}

// handleGetInfo implements the getinfo command. We only return the fields
// that are not related to wallet functionality.
func (server *NodeRPC) handleGetInfo(ctx CmdCtx) (interface{}, error) {
	// best := server.beaconChain.BlockChain.BestSnapshot()
	// ratio, err := server.GetDifficultyRatio(best.Bits, server.beaconChain.ChainParams)
	// if err != nil {
	// 	return nil, err
	//
	// }
	//
	// ret := &jaxjson.InfoChainResult{
	// 	// Version:         int32(1000000*appMajor + 10000*appMinor + 100*appPatch),
	// 	ProtocolVersion: int32(maxProtocolVersion),
	// 	Blocks:          best.Height,
	// 	TimeOffset:      int64(server.beaconChain.TimeSource.Offset().Seconds()),
	// 	Connections:     server.connMgr.ConnectedCount(),
	// 	Difficulty:      ratio,
	// 	// Proxy:           cfg.Proxy,
	// 	// TestNet:         cfg.TestNet3,
	// 	// RelayFee:        cfg.MinRelayTxFeeValues.ToBTC(),
	// }

	return nil, nil
	// return ret, nil
}

// handleGenerate handles generate commands.
func (server *NodeRPC) handleGenerate(ctx CmdCtx) (interface{}, error) {
	// Respond with an error if there are no addresses to pay the
	// created blocks to.
	if len(server.MiningAddrs) == 0 {
		return nil, &jaxjson.RPCError{
			Code: jaxjson.ErrRPCInternal.Code,
			Message: "No payment addresses specified " +
				"via --miningaddr",
		}
	}

	// Respond with an error if there's virtually 0 chance of mining a block
	// with the CPU.
	if !server.beaconChain.ChainParams.PowParams.GenerateSupported {
		return nil, &jaxjson.RPCError{
			Code: jaxjson.ErrRPCDifficulty,
			Message: fmt.Sprintf("No support for `generate` on "+
				"the current network, %s, as it's unlikely to "+
				"be possible to mine a block with the CPU.",
				server.beaconChain.ChainParams.Net),
		}
	}

	c := ctx.Cmd.(*jaxjson.GenerateCmd)

	// Respond with an error if the client is requesting 0 blocks to be generated.
	if c.NumBlocks == 0 {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInternal.Code,
			Message: "Please request a nonzero number of blocks to generate.",
		}
	}

	// Create a reply
	reply := make([]string, c.NumBlocks)
	quit := make(chan struct{})
	blockHashes, err := server.CPUMiner.GenerateNBlocks(quit, c.NumBlocks)
	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInternal.Code,
			Message: err.Error(),
		}
	}

	// Mine the correct number of blocks, assigning the hex representation of the
	// hash of each one to its place in the reply.
	for i, hash := range blockHashes {
		reply[i] = hash.String()
	}

	return reply, nil
}

func (server *NodeRPC) handleManageShards(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.ManageShardsCmd)

	var err error
	switch c.Action {
	case "stop":
		err = server.shardsMgr.DisableShard(c.ShardID)
	case "run":
		err = server.shardsMgr.EnableShard(c.ShardID)
	default:
		err = errors.New("invalid actions for manageshards")
	}

	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInvalidParameter,
			Message: err.Error(),
		}
	}

	return nil, nil
}

func (server *NodeRPC) handleListShards(ctx CmdCtx) (interface{}, error) {
	shards := server.shardsMgr.ListShards()
	return shards, nil
}

// handleGetDifficulty implements the getdifficulty command.
func (server *NodeRPC) handleEstimateLockTime(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.EstimateSwapLockTime)

	isMainNet := server.beaconChain.ChainParams.Net == wire.MainNet

	sourceShard, ok := server.shardsMgr.ShardCtl(c.SourceShard)
	if !ok {
		return nil, jaxjson.NewRPCError(jaxjson.ErrRPCInvalidParameter,
			fmt.Sprintf("shard %d not exist", c.SourceShard))
	}
	sourceBest := sourceShard.BlockChain().BestSnapshot()
	sN, err := EstimateLockInChain(sourceBest.Bits, sourceBest.K, c.Amount)
	if err != nil {
		if isMainNet {
			return nil, err
		}
		sN = chaincfg.ShardEpochLength
	}

	if !isMainNet && sN > chaincfg.ShardEpochLength/32 {
		sN = chaincfg.ShardEpochLength / 32
	}

	destinationShard, ok := server.shardsMgr.ShardCtl(c.DestinationShard)
	if !ok {
		return nil, jaxjson.NewRPCError(jaxjson.ErrRPCInvalidParameter,
			fmt.Sprintf("shard %d not exist", c.SourceShard))
	}

	destBest := destinationShard.BlockChain().BestSnapshot()
	dN, err := EstimateLockInChain(destBest.Bits, destBest.K, c.Amount)
	if err != nil {
		if isMainNet {
			return nil, err
		}
		dN = chaincfg.ShardEpochLength / 32
	}
	if !isMainNet && dN > chaincfg.ShardEpochLength/32 {
		dN = chaincfg.ShardEpochLength / 32
	}

	if sN < dN*2 {
		sN = dN * 2
	}

	return jaxjson.EstimateSwapLockTimeResult{
		NBlocksAtSource: sN,
		NBlocksAtDest:   dN,
	}, nil
}

// EstimateLockInChain estimates desired period in block for locking funds
// in shard during the CrossShard Swap Tx.
// nolint: gomnd
func EstimateLockInChain(bits, k uint32, amount int64) (int64, error) {
	kd := pow.MultBitsAndK(bits, k)
	n := float64(amount/chaincfg.JuroPerJAXCoin) / kd

	if n < 4 {
		n = 4 * 30
	}
	if n > lockTimeBlocks {
		return 0, jaxjson.NewRPCError(jaxjson.ErrRPCTxRejected, "lock time more than 2000 blocks")
	}
	return int64(n), nil
}

// handleSetGenerate implements the setgenerate command.
func (server *NodeRPC) handleSetGenerate(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.SetGenerateCmd)

	// Disable generation regardless of the provided generate flag if the
	// maximum number of threads (goroutines for our purposes) is 0.
	// Otherwise enable or disable it depending on the provided flag.
	generate := c.Generate
	genProcLimit := -1
	if c.GenProcLimit != nil {
		genProcLimit = *c.GenProcLimit
	}
	if genProcLimit == 0 {
		generate = false
	}

	if !generate {
		server.CPUMiner.Stop()
	} else {
		// Respond with an error if there are no addresses to pay the
		// created blocks to.
		if len(server.beaconChain.MiningAddrs) == 0 {
			return nil, &jaxjson.RPCError{
				Code: jaxjson.ErrRPCInternal.Code,
				Message: "No payment addresses specified " +
					"via --miningaddr",
			}
		}

		// It's safe to call start even if it's already started.
		server.CPUMiner.SetNumWorkers(int32(genProcLimit))
		server.CPUMiner.Start()
	}
	return nil, nil
}

// handleDebugLevel handles debuglevel commands.
func (server *NodeRPC) handleDebugLevel(ctx CmdCtx) (interface{}, error) {
	// c := ctx.Cmd.(*jaxjson.DebugLevelCmd)

	// Special show command to list supported subsystems.
	// if c.LevelSpec == "show" {
	//	return fmt.Sprintf("Supported subsystems %v",
	//		supportedSubsystems()), nil
	// }

	// err := parseAndSetDebugLevels(c.LevelSpec)
	// if err != nil {
	//	return nil, &jaxjson.RPCError{
	//		Code:    jaxjson.ErrRPCInvalidParams.Code,
	//		Message: err.Error(),
	//	}
	// }
	return "Done.", nil
}

// handleStop implements the stop command.
func (server *NodeRPC) handleStop(ctx CmdCtx) (interface{}, error) {
	// todo
	// select {
	// case server.requestProcessShutdown <- struct{}{}:
	// default:
	// }
	return "jaxnetd stopping.", nil
}

// handleHelp implements the help command.
func (server *NodeRPC) handleHelp(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.HelpCmd)

	// Provide a usage overview of all commands when no specific command
	// was specified.
	var scope, command string
	if c.Scope != nil {
		scope = *c.Scope
	}
	if c.Command != nil {
		command = *c.Command
	}
	if scope == "" || command == "" {
		usage, err := server.helpCache.rpcUsage()
		if err != nil {
			context := "Failed to generate RPC usage"
			return nil, server.InternalRPCError(err.Error(), context)
		}
		return usage, nil
	}
	method := jaxjson.ScopedMethod(scope, command)

	// Check that the command asked for is supported and implemented.  Only
	// search the main list of handlers since help should not be provided
	// for commands that are unimplemented or related to wallet
	// functionality.
	if _, ok := server.handlers[method]; !ok {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInvalidParameter,
			Message: "Unknown command: " + command,
		}
	}

	// Get the help for the command.
	help, err := server.helpCache.rpcMethodHelp(command)
	if err != nil {
		context := "Failed to generate help"
		return nil, server.InternalRPCError(err.Error(), context)
	}
	return help, nil
}

func (server *NodeRPC) handleGetNodeMetrics(ctx CmdCtx) (interface{}, error) {
	res := server.metricsMgr.GetNodeMetrics()
	return res, nil
}

func (server *NodeRPC) handleGetChainMetrics(ctx CmdCtx) (interface{}, error) {
	res := server.metricsMgr.GetChainMetrics()
	return res, nil
}
