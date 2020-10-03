package rpc

import (
	"errors"
	"math/big"
	"time"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/node/cprovider"
	"gitlab.com/jaxnet/core/shard.core/node/mining/cpuminer"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"gitlab.com/jaxnet/core/shard.core/types/pow"
	"go.uber.org/zap"
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
	chainProvider cprovider.ChainProvider
}

func NewNodeRPC(shardsMgr ShardManager, logger *zap.Logger) *NodeRPC {
	rpc := &NodeRPC{
		Mux:       NewRPCMux(logger),
		shardsMgr: shardsMgr,
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
		btcjson.ScopedMethod("node", "getNetworkInfo"): server.handleGetnetworkinfo,
		btcjson.ScopedMethod("node", "uptime"):         server.handleUptime,

		btcjson.ScopedMethod("node", "manageShards"): server.handleManageShards,
		btcjson.ScopedMethod("node", "listShards"):   server.handleListShards,

		btcjson.ScopedMethod("node", "generate"):         server.handleGenerate,
		btcjson.ScopedMethod("node", "getDifficulty"):    server.handleGetDifficulty,
		btcjson.ScopedMethod("node", "getMiningInfo"):    server.handleGetMiningInfo,
		btcjson.ScopedMethod("node", "getNetworkHashPS"): server.handleGetNetworkHashPS,

		btcjson.ScopedMethod("node", "setGenerate"):     server.handleSetGenerate,
		btcjson.ScopedMethod("node", "getBlockStats"):   server.handleGetBlockStats,
		btcjson.ScopedMethod("node", "getChainTxStats"): server.handleGetChaintxStats,
		btcjson.ScopedMethod("node", "debugLevel"):      server.handleDebugLevel,
		btcjson.ScopedMethod("node", "stop"):            server.handleStop,
		btcjson.ScopedMethod("node", "help"):            server.handleHelp,
	}
}

// handleVersion implements the version command.
//
// NOTE: This is a btcsuite extension ported from github.com/decred/dcrd.
func (server *NodeRPC) handleVersion(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	result := map[string]btcjson.VersionResult{
		"btcdjsonrpcapi": {
			VersionString: jsonrpcSemverString,
			Major:         jsonrpcSemverMajor,
			Minor:         jsonrpcSemverMinor,
			Patch:         jsonrpcSemverPatch,
		},
	}
	return result, nil
}

func (server *NodeRPC) handleGetnetworkinfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// result := btcjson.NewGetNetworkInfoCmd()
	// fmt.Println("NetworkInfo: ", result)
	return struct {
		Subversion string `json:"subversion"`
	}{
		Subversion: "/Satoshi:0.18.0/",
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

// handleGetDifficulty implements the getdifficulty command.
func (server *NodeRPC) handleGetDifficulty(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	best := server.chainProvider.BlockChain.BestSnapshot()
	return server.GetDifficultyRatio(best.Bits, server.chainProvider.ChainParams)
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

// handleGetMiningInfo implements the getmininginfo command. We only return the
// fields that are not related to wallet functionality.
func (server *NodeRPC) handleGetMiningInfo(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Create a default getnetworkhashps command to use defaults and make
	// use of the existing getnetworkhashps handler.
	gnhpsCmd := btcjson.NewGetNetworkHashPSCmd(nil, nil)
	networkHashesPerSecIface, err := server.handleGetNetworkHashPS(gnhpsCmd, closeChan)
	if err != nil {
		return nil, err
	}
	networkHashesPerSec, ok := networkHashesPerSecIface.(int64)
	if !ok {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInternal.Code,
			Message: "networkHashesPerSec is not an int64",
		}
	}

	best := server.chainProvider.BlockChain.BestSnapshot()
	diff, err := server.GetDifficultyRatio(best.Bits, server.chainProvider.ChainParams)
	if err != nil {
		return nil, err
	}
	result := btcjson.GetMiningInfoResult{
		Blocks:             int64(best.Height),
		CurrentBlockSize:   best.BlockSize,
		CurrentBlockWeight: best.BlockWeight,
		CurrentBlockTx:     best.NumTxns,
		Difficulty:         diff,
		NetworkHashPS:      networkHashesPerSec,
		PooledTx:           uint64(server.chainProvider.TxMemPool.Count()),
		// TestNet:            server.cfg.TestNet3,
	}
	return &result, nil
}

// handleGetNetworkHashPS implements the getnetworkhashps command.
func (server *NodeRPC) handleGetNetworkHashPS(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// Note: All valid error return paths should return an int64.
	// Literal zeros are inferred as int, and won't coerce to int64
	// because the return value is an interface{}.

	c := cmd.(*btcjson.GetNetworkHashPSCmd)

	// When the passed height is too high or zero, just return 0 now
	// since we can't reasonably calculate the number of network hashes
	// per second from invalid values.  When it'server negative, use the current
	// best block height.
	best := server.chainProvider.BlockChain.BestSnapshot()
	endHeight := int32(-1)
	if c.Height != nil {
		endHeight = int32(*c.Height)
	}
	if endHeight > best.Height || endHeight == 0 {
		return int64(0), nil
	}
	if endHeight < 0 {
		endHeight = best.Height
	}

	// Calculate the number of blocks per retarget interval based on the
	// BlockChain parameters.
	blocksPerRetarget := int32(server.chainProvider.ChainParams.TargetTimespan /
		server.chainProvider.ChainParams.TargetTimePerBlock)

	// Calculate the starting block height based on the passed number of
	// blocks.  When the passed value is negative, use the last block the
	// difficulty changed as the starting height.  Also make sure the
	// starting height is not before the beginning of the BlockChain.
	numBlocks := int32(120)
	if c.Blocks != nil {
		numBlocks = int32(*c.Blocks)
	}
	var startHeight int32
	if numBlocks <= 0 {
		startHeight = endHeight - ((endHeight % blocksPerRetarget) + 1)
	} else {
		startHeight = endHeight - numBlocks
	}
	if startHeight < 0 {
		startHeight = 0
	}
	server.Log.Debugf("Calculating network hashes per second %v %v", startHeight, endHeight)

	// Find the min and max block timestamps as well as calculate the total
	// amount of work that happened between the start and end blocks.
	var minTimestamp, maxTimestamp time.Time
	totalWork := big.NewInt(0)
	for curHeight := startHeight; curHeight <= endHeight; curHeight++ {
		hash, err := server.chainProvider.BlockChain.BlockHashByHeight(curHeight)
		if err != nil {
			context := "Failed to fetch block hash"
			return nil, server.InternalRPCError(err.Error(), context)
		}

		// Fetch the header from BlockChain.
		header, err := server.chainProvider.BlockChain.HeaderByHash(hash)
		if err != nil {
			context := "Failed to fetch block header"
			return nil, server.InternalRPCError(err.Error(), context)
		}

		if curHeight == startHeight {
			minTimestamp = header.Timestamp()
			maxTimestamp = minTimestamp
		} else {
			totalWork.Add(totalWork, pow.CalcWork(header.Bits()))

			if minTimestamp.After(header.Timestamp()) {
				minTimestamp = header.Timestamp()
			}
			if maxTimestamp.Before(header.Timestamp()) {
				maxTimestamp = header.Timestamp()
			}
		}
	}

	// Calculate the difference in seconds between the min and max block
	// timestamps and avoid division by zero in the case where there is no
	// time difference.
	timeDiff := int64(maxTimestamp.Sub(minTimestamp) / time.Second)
	if timeDiff == 0 {
		return int64(0), nil
	}

	hashesPerSec := new(big.Int).Div(totalWork, big.NewInt(timeDiff))
	return hashesPerSec.Int64(), nil
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

func (server *NodeRPC) handleGetBlockStats(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	// c := cmd.(*btcjson.GetBlockStatsCmd)
	res := btcjson.GetBlockStatsResult{}
	return res, nil
}

func (server *NodeRPC) handleGetChaintxStats(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	_ = cmd.(*btcjson.GetChainStatsCmd)
	res := btcjson.GetChainStatsResult{}
	return res, nil
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
