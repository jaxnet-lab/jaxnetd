// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpc

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gitlab.com/jaxnet/jaxnetd/node/mining/cpuminer"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/version"
)

type NodeRPC struct {
	Mux

	// shardsMgr provides ability to manipulate running shards.
	shardsMgr ShardManager

	// fixme: this fields are empty
	helpCache     *helpCacher
	StartupTime   int64
	MiningAddrs   []jaxutil.Address
	CPUMiner      cpuminer.CPUMiner
	chainProvider *cprovider.ChainProvider
}

func NewNodeRPC(provider *cprovider.ChainProvider, shardsMgr ShardManager, logger zerolog.Logger) *NodeRPC {
	rpc := &NodeRPC{
		Mux:           NewRPCMux(logger),
		shardsMgr:     shardsMgr,
		chainProvider: provider,
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

		jaxjson.ScopedMethod("node", "debugLevel"): server.handleDebugLevel,
		jaxjson.ScopedMethod("node", "stop"):       server.handleStop,
		jaxjson.ScopedMethod("node", "help"):       server.handleHelp,
	}
}

// handleVersion implements the version command.
//
// NOTE: This is a btcsuite extension ported from github.com/decred/dcrd.
func (server *NodeRPC) handleVersion(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
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
func (server *NodeRPC) handleUptime(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return time.Now().Unix() - server.StartupTime, nil
}

// handleGetInfo implements the getinfo command. We only return the fields
// that are not related to wallet functionality.
func (server *NodeRPC) handleGetInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// best := server.chainProvider.BlockChain.BestSnapshot()
	// ratio, err := server.GetDifficultyRatio(best.Bits, server.chainProvider.ChainParams)
	// if err != nil {
	// 	return nil, err
	//
	// }
	//
	// ret := &jaxjson.InfoChainResult{
	// 	// Version:         int32(1000000*appMajor + 10000*appMinor + 100*appPatch),
	// 	ProtocolVersion: int32(maxProtocolVersion),
	// 	Blocks:          best.Height,
	// 	TimeOffset:      int64(server.chainProvider.TimeSource.Offset().Seconds()),
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
func (server *NodeRPC) handleGenerate(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
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
	// if !s.cfg.ChainParams.GenerateSupported {
	// 	return nil, &jaxjson.RPCError{
	// 		Code: jaxjson.ErrRPCDifficulty,
	// 		Message: fmt.Sprintf("No support for `generate` on "+
	// 			"the current network, %s, as it's unlikely to "+
	// 			"be possible to mine a block with the CPU.",
	// 			s.cfg.ChainParams.Net),
	// 	}
	// }

	c := cmd.(*jaxjson.GenerateCmd)

	// Respond with an error if the client is requesting 0 blocks to be generated.
	if c.NumBlocks == 0 {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInternal.Code,
			Message: "Please request a nonzero number of blocks to generate.",
		}
	}

	// Create a reply
	reply := make([]string, c.NumBlocks)

	blockHashes, err := server.CPUMiner.GenerateNBlocks(c.NumBlocks)
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

func (server *NodeRPC) handleManageShards(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.ManageShardsCmd)

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

func (server *NodeRPC) handleListShards(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	shards := server.shardsMgr.ListShards()
	return shards, nil
}

// handleGetDifficulty implements the getdifficulty command.
func (server *NodeRPC) handleEstimateLockTime(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.EstimateSwapLockTime)

	sourceShard, ok := server.shardsMgr.ShardCtl(c.SourceShard)
	if !ok {
		return nil, jaxjson.NewRPCError(jaxjson.ErrRPCInvalidParameter,
			fmt.Sprintf("shard %d not exist", c.SourceShard))
	}
	sourceBest := sourceShard.BlockChain().BestSnapshot()
	sN, err := EstimateLockInChain(sourceShard.ChainCtx.Params().PowParams.PowLimitBits,
		sourceBest.Bits, sourceBest.K, c.Amount)
	if err != nil {
		return nil, err
	}

	destinationShard, ok := server.shardsMgr.ShardCtl(c.DestinationShard)
	if !ok {
		return nil, jaxjson.NewRPCError(jaxjson.ErrRPCInvalidParameter,
			fmt.Sprintf("shard %d not exist", c.SourceShard))
	}

	destBest := destinationShard.BlockChain().BestSnapshot()
	dN, err := EstimateLockInChain(destinationShard.ChainCtx.Params().PowParams.PowLimitBits,
		destBest.Bits, destBest.K, c.Amount)
	if err != nil {
		return nil, err
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
func EstimateLockInChain(genesisBits, bits, k uint32, amount int64) (int64, error) {
	kd := pow.MultBitsAndK(genesisBits, bits, k)
	n := amount / int64(kd*jaxutil.SatoshiPerJAXCoin)

	if n < 4 {
		n = 4 * 30
	}
	if n > 20000 {
		return 0, jaxjson.NewRPCError(jaxjson.ErrRPCTxRejected,
			fmt.Sprintf("lock time more than 2000 blocks"))
	}
	return n, nil
}

// handleSetGenerate implements the setgenerate command.
func (server *NodeRPC) handleSetGenerate(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	//	c := cmd.(*jaxjson.SetGenerateCmd)
	//
	//	// Disable generation regardless of the provided generate flag if the
	//	// maximum number of threads (goroutines for our purposes) is 0.
	//	// Otherwise enable or disable it depending on the provided flag.
	//	generate := c.Generate
	//	genProcLimit := -1
	//	if c.GenProcLimit != nil {
	//		genProcLimit = *c.GenProcLimit
	//	}
	//	if genProcLimit == 0 {
	//		generate = false
	//	}
	//
	//	if !generate {
	//		s.chainProvider.CPUMiner.Stop()
	//	} else {
	//		// Respond with an error if there are no addresses to pay the
	//		// created blocks to.
	//		if len(s.cfg.MiningAddrs) == 0 {
	//			return nil, &jaxjson.RPCError{
	//				Code: jaxjson.ErrRPCInternal.Code,
	//				Message: "No payment addresses specified " +
	//					"via --miningaddr",
	//			}
	//		}
	//
	//		// It's safe to call start even if it's already started.
	//		s.chainProvider.CPUMiner.SetNumWorkers(int32(genProcLimit))
	//		s.chainProvider.CPUMiner.Start()
	//	}
	return nil, nil
}

// handleDebugLevel handles debuglevel commands.
func (server *NodeRPC) handleDebugLevel(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// c := cmd.(*jaxjson.DebugLevelCmd)

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
func (server *NodeRPC) handleStop(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// todo
	// select {
	// case server.requestProcessShutdown <- struct{}{}:
	// default:
	// }
	return "jaxnetd stopping.", nil
}

// handleHelp implements the help command.
func (server *NodeRPC) handleHelp(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*jaxjson.HelpCmd)

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
		usage, err := server.helpCache.rpcUsage(false)
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
