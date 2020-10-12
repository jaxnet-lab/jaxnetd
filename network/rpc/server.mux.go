package rpc

import (
	"context"
	"net/http"

	"gitlab.com/jaxnet/core/shard.core/network"
	"gitlab.com/jaxnet/core/shard.core/network/rpcutli"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"go.uber.org/zap"
)

type MultiChainRPC struct {
	*ServerCore
	nodeRPC   *NodeRPC
	beaconRPC *BeaconRPC
	shardRPCs map[uint32]*ShardRPC
}

func NewMultiChainRPC(config *Config, logger *zap.Logger,
	nodeRPC *NodeRPC, beaconRPC *BeaconRPC, shardRPCs map[uint32]*ShardRPC) *MultiChainRPC {
	rpc := &MultiChainRPC{
		ServerCore: NewRPCCore(config, logger),
		nodeRPC:    nodeRPC,
		beaconRPC:  beaconRPC,
		shardRPCs:  shardRPCs,
	}

	return rpc
}

func (server *MultiChainRPC) Run(ctx context.Context) {
	rpcServeMux := http.NewServeMux()

	// rpcServeMux.HandleFunc("/ws", server.WSHandleFunc())
	rpcServeMux.HandleFunc("/",
		server.HandleFunc(func(cmd *parsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
			if cmd.scope == "node" {
				return server.nodeRPC.HandleCommand(cmd, closeChan)
			}
			if cmd.shardID == 0 {
				return server.beaconRPC.HandleCommand(cmd, closeChan)
			}
			if _, ok := server.shardRPCs[cmd.shardID]; !ok {
				return nil, &btcjson.RPCError{
					Code:    btcjson.ErrShardIDMismatch,
					Message: "Provided ShardID does not match with any present",
				}
			}
			return server.shardRPCs[cmd.shardID].HandleCommand(cmd, closeChan)

		}))

	server.StartRPC(ctx, rpcServeMux)
}

type Mux struct {
	rpcutli.ToolsXt
	Log      network.ILogger
	handlers map[btcjson.MethodName]CommandHandler
}

func NewRPCMux(logger *zap.Logger) Mux {
	return Mux{
		Log:      network.LogAdapter(logger),
		handlers: map[btcjson.MethodName]CommandHandler{},
	}
}

// HandleCommand checks that a parsed command is a standard Bitcoin JSON-RPC
// command and runs the appropriate handler to reply to the command.  Any
// commands which are not recognized or not implemented will return an error
// suitable for use in replies.
func (server *Mux) HandleCommand(cmd *parsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
	handler, ok := server.handlers[btcjson.ScopedMethod(cmd.scope, cmd.method)]
	if ok {
		return handler(cmd.cmd, closeChan)
	}

	return nil, btcjson.ErrRPCMethodNotFound
}

func (server *Mux) SetCommands(commands map[btcjson.MethodName]CommandHandler) {
	for cmd, handler := range commands {
		server.handlers[cmd] = handler
	}
}

// InternalRPCError is a convenience function to convert an internal error to
// an RPC error with the appropriate code set.  It also logs the error to the
// RPC server subsystem since internal errors really should not occur.  The
// context parameter is only used in the log message and may be empty if it's
// not needed.
func (server *Mux) InternalRPCError(errStr, context string) *btcjson.RPCError {
	logStr := errStr
	if context != "" {
		logStr = context + ": " + errStr
	}
	server.Log.Error(logStr)
	return btcjson.NewRPCError(btcjson.ErrRPCInternal.Code, errStr)
}
