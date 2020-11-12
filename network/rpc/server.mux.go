// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package rpc

import (
	"context"
	"fmt"
	"github.com/btcsuite/websocket"
	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/core/shard.core/network/rpcutli"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"net/http"
	"sync"
)

type MultiChainRPC struct {
	*ServerCore
	nodeRPC     *NodeRPC
	beaconRPC   *BeaconRPC
	shardRPCs   map[uint32]*ShardRPC
	chainsMutex sync.RWMutex
	helpCache   *helpCacher
	wsManager   *wsManager
}

func NewMultiChainRPC(config *Config, logger zerolog.Logger,
	nodeRPC *NodeRPC, beaconRPC *BeaconRPC, shardRPCs map[uint32]*ShardRPC) *MultiChainRPC {
	rpc := &MultiChainRPC{
		ServerCore: NewRPCCore(config, logger),
		nodeRPC:    nodeRPC,
		beaconRPC:  beaconRPC,
		shardRPCs:  shardRPCs,
	}
	rpc.wsManager = WebSocketManager(rpc, logger)

	return rpc
}

func (server *MultiChainRPC) AddShard(shardID uint32, rpc *ShardRPC) {
	server.chainsMutex.Lock()
	server.shardRPCs[shardID] = rpc
	server.chainsMutex.Unlock()
}

func (server *MultiChainRPC) WSHandleFunc() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if server.cfg.WSEnable {
			http.Error(w, "WS is Unavailable", http.StatusServiceUnavailable)
			return
		}

		authenticated, isAdmin, err := server.checkAuth(r, false)
		if err != nil {
			jsonAuthFail(w)
			return
		}

		server.logger.Info().Msg("Upgrade To websocket")
		// Attempt to upgrade the connection to a websocket connection
		// using the default size for read/write bufferserver.
		ws, err := websocket.Upgrade(w, r, nil, 0, 0)
		if err != nil {
			if _, ok := err.(websocket.HandshakeError); !ok {
				server.logger.Error().Err(err).Msg("Unexpected websocket")
			}
			http.Error(w, "400 Bad Request.", http.StatusBadRequest)
			return
		}
		_, _, _ = ws, authenticated, isAdmin
		server.logger.Info().Msg("WebsocketHandler")
		server.WebsocketHandler(ws, r.RemoteAddr, authenticated, isAdmin)
	}
}

func (server *MultiChainRPC) Run(ctx context.Context) {
	rpcServeMux := http.NewServeMux()

	rpcServeMux.HandleFunc("/ws", server.WSHandleFunc())

	rpcServeMux.HandleFunc("/",
		server.HandleFunc(func(cmd *parsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
			if cmd.scope == "node" {
				return server.nodeRPC.HandleCommand(cmd, closeChan)
			}
			if cmd.shardID == 0 {
				return server.beaconRPC.HandleCommand(cmd, closeChan)
			}

			server.chainsMutex.RLock()
			prcPtr, ok := server.shardRPCs[cmd.shardID]
			server.chainsMutex.RUnlock()
			if !ok {
				server.logger.Error().Msgf("Provided ShardID (%d) does not match with any present", cmd.shardID)
				return nil, &btcjson.RPCError{
					Code:    btcjson.ErrShardIDMismatch,
					Message: fmt.Sprintf("Provided ShardID (%d) does not match with any present", cmd.shardID),
				}
			}

			return prcPtr.HandleCommand(cmd, closeChan)
		}))

	server.wsManager.Start(ctx)
	server.StartRPC(ctx, rpcServeMux)
}

type Mux struct {
	rpcutli.ToolsXt
	Log      zerolog.Logger
	handlers map[btcjson.MethodName]CommandHandler
}

func NewRPCMux(logger zerolog.Logger) Mux {
	return Mux{
		Log:      logger,
		handlers: map[btcjson.MethodName]CommandHandler{},
	}
}

// HandleCommand checks that a parsed command is a standard Bitcoin JSON-RPC
// command and runs the appropriate handler to reply to the command.  Any
// commands which are not recognized or not implemented will return an error
// suitable for use in replies.
func (server *Mux) HandleCommand(cmd *parsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
	handler, ok := server.handlers[btcjson.ScopedMethod(cmd.scope, cmd.method)]
	server.Log.Debug().Msg("Handle command " + cmd.scope + "." + cmd.method)
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
	server.Log.Error().Msg(logStr)
	return btcjson.NewRPCError(btcjson.ErrRPCInternal.Code, errStr)
}
