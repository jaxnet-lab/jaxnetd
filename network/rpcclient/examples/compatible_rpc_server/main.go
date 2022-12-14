/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

	btcdjson "github.com/btcsuite/btcd/btcjson"
	"gitlab.com/jaxnet/jaxnetd/network/rpc"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
)

func init() {
	// jaxjson.DropAllCmds()
	jaxjson.MustRegisterLegacyCmd("getblocktemplate", (*btcdjson.GetBlockTemplateCmd)(nil), jaxjson.UsageFlag(0))
	jaxjson.RegisterOrReplaceLegacyCmd("submitblock", (*btcdjson.GetBlockTemplateCmd)(nil), jaxjson.UsageFlag(0))
}

func main() {

	cmdHandler := CmdHandler{
		// TODO: ...
	}

	l, err := net.Listen("tcp", "0.0.0.0:18333")
	if err != nil {
		fmt.Println(err)
	}

	coreRPCCfg := rpc.Config{
		ListenerAddresses: []string{"0.0.0.0:18333"},
		MaxClients:        1000,
		User:              "",
		Password:          "",
		Disable:           false,
		RPCCert:           "",
		RPCKey:            "",
		LimitPass:         "",
		LimitUser:         "",
		MaxConcurrentReqs: 1000,
		MaxWebsockets:     1000,
		WSEnable:          false,
		AuthProvider:      authProvider,
		Listeners:         []net.Listener{l},
	}

	serverCore := rpc.NewRPCCore(&coreRPCCfg)

	rpcServeMux := http.NewServeMux()
	rpcServeMux.HandleFunc("/",
		serverCore.HandleFunc(func(cmd *rpc.ParsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
			method := jaxjson.LegacyMethod(cmd.Method)

			handlerFunc, ok := cmdHandler.Handlers()[method]
			if ok {
				return handlerFunc(rpc.CmdCtx{
					ID:        cmd.ID,
					Cmd:       cmd.Cmd,
					CloseChan: closeChan,
					AuthCtx:   cmd.AuthCtx,
				})
			}

			return nil, jaxjson.ErrRPCMethodNotFound.WithMethod(method.String())

		}))
	serverCore.StartRPC(context.TODO(), rpcServeMux)
}

type CmdHandler struct {
	// TODO: ...
}

func (srv *CmdHandler) Handlers() map[jaxjson.MethodName]rpc.CommandHandler {
	return map[jaxjson.MethodName]rpc.CommandHandler{
		jaxjson.LegacyMethod("getblocktemplate"): srv.handleGetBlockTemplate,
	}
}

func (srv *CmdHandler) handleGetBlockTemplate(ctx rpc.CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*btcdjson.GetBlockTemplateCmd)
	request := c.Request

	// TODO: ...

	_ = request
	return nil, nil
}

func authProvider(req *http.Request) (authCtx interface{}, isAuthorized bool, isLimited bool) {
	if req.Header.Get("isAuth") == "true" {
		return authCtx, true, true
	}

	return authCtx, false, false
}
