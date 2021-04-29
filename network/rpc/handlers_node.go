// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpc

import (
	"errors"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/node/mining/cpuminer"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"gitlab.com/jaxnet/core/shard.core/version"
)

type NodeRPC struct {
	Mux

	// shardsMgr provides ability to manipulate running shards.
	shardsMgr ShardManager

	// fixme: this fields are empty
	helpCache     *helpCacher
	StartupTime   int64
	MiningAddrs   []btcutil.Address
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

func (server *NodeRPC) OwnHandlers() map[btcjson.MethodName]CommandHandler {
	return map[btcjson.MethodName]CommandHandler{
		btcjson.ScopedMethod("node", "version"):        server.handleVersion,
		btcjson.ScopedMethod("node", "getInfo"):        server.handleGetInfo,
		btcjson.ScopedMethod("node", "uptime"):         server.handleUptime,

		btcjson.ScopedMethod("node", "manageShards"): server.handleManageShards,
		btcjson.ScopedMethod("node", "listShards"):   server.handleListShards,

		btcjson.ScopedMethod("node", "generate"):         server.handleGenerate,
		btcjson.ScopedMethod("node", "setGenerate"):     server.handleSetGenerate,

		btcjson.ScopedMethod("node", "debugLevel"):      server.handleDebugLevel,
		btcjson.ScopedMethod("node", "stop"):            server.handleStop,
		btcjson.ScopedMethod("node", "help"):            server.handleHelp,
	}
}

// handleVersion implements the version command.
//
// NOTE: This is a btcsuite extension ported from github.com/decred/dcrd.
func (server *NodeRPC) handleVersion(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	return btcjson.NodeVersion{
		Node: version.GetExtendedVersion(),
		RPC: btcjson.VersionResult{
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
	// ret := &btcjson.InfoChainResult{
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
		return nil, &btcjson.RPCError{
			Code: btcjson.ErrRPCInternal.Code,
			Message: "No payment addresses specified " +
				"via --miningaddr",
		}
	}

	// Respond with an error if there's virtually 0 chance of mining a block
	// with the CPU.
	// if !s.cfg.ChainParams.GenerateSupported {
	// 	return nil, &btcjson.RPCError{
	// 		Code: btcjson.ErrRPCDifficulty,
	// 		Message: fmt.Sprintf("No support for `generate` on "+
	// 			"the current network, %s, as it's unlikely to "+
	// 			"be possible to mine a block with the CPU.",
	// 			s.cfg.ChainParams.Net),
	// 	}
	// }

	c := cmd.(*btcjson.GenerateCmd)

	// Respond with an error if the client is requesting 0 blocks to be generated.
	if c.NumBlocks == 0 {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInternal.Code,
			Message: "Please request a nonzero number of blocks to generate.",
		}
	}

	// Create a reply
	reply := make([]string, c.NumBlocks)

	blockHashes, err := server.CPUMiner.GenerateNBlocks(c.NumBlocks)
	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInternal.Code,
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
	c := cmd.(*btcjson.ManageShardsCmd)

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
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: err.Error(),
		}
	}

	return nil, nil
}

func (server *NodeRPC) handleListShards(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	shards := server.shardsMgr.ListShards()
	return shards, nil
}

// handleSetGenerate implements the setgenerate command.
func (server *NodeRPC) handleSetGenerate(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	//	c := cmd.(*btcjson.SetGenerateCmd)
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
	//			return nil, &btcjson.RPCError{
	//				Code: btcjson.ErrRPCInternal.Code,
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
	// c := cmd.(*btcjson.DebugLevelCmd)

	// Special show command to list supported subsystems.
	// if c.LevelSpec == "show" {
	//	return fmt.Sprintf("Supported subsystems %v",
	//		supportedSubsystems()), nil
	// }

	// err := parseAndSetDebugLevels(c.LevelSpec)
	// if err != nil {
	//	return nil, &btcjson.RPCError{
	//		Code:    btcjson.ErrRPCInvalidParams.Code,
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
	return "btcd stopping.", nil
}

// handleHelp implements the help command.
func (server *NodeRPC) handleHelp(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.HelpCmd)

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
	method := btcjson.ScopedMethod(scope, command)

	// Check that the command asked for is supported and implemented.  Only
	// search the main list of handlers since help should not be provided
	// for commands that are unimplemented or related to wallet
	// functionality.
	if _, ok := server.handlers[method]; !ok {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
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
