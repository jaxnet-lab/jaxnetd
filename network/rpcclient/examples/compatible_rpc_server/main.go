/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package main

import (
	"context"
	"net/http"

	btcdjson "github.com/btcsuite/btcd/btcjson"
	"gitlab.com/jaxnet/jaxnetd/network/rpc"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
)

func init() {
	jaxjson.MustRegisterLegacyCmd("getblocktemplate", (*btcdjson.GetBlockTemplateCmd)(nil), jaxjson.UsageFlag(0))
}

func main() {

	cmdHandler := CmdHandler{
		// TODO: ...
	}

	coreRPCCfg := rpc.Config{
		ListenerAddresses: nil,
		MaxClients:        0,
		User:              "",
		Password:          "",
		Disable:           false,
		RPCCert:           "",
		RPCKey:            "",
		LimitPass:         "",
		LimitUser:         "",
		MaxConcurrentReqs: 0,
		MaxWebsockets:     0,
		WSEnable:          false,
		Listeners:         nil,
	}

	serverCore := rpc.NewRPCCore(&coreRPCCfg)

	rpcServeMux := http.NewServeMux()
	rpcServeMux.HandleFunc("/",
		serverCore.HandleFunc(func(cmd *rpc.ParsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
			method := jaxjson.LegacyMethod(cmd.Method)

			handlerFunc, ok := cmdHandler.Handlers()[method]
			if ok {
				return handlerFunc(cmd.Cmd, closeChan)
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

func (srv *CmdHandler) handleGetBlockTemplate(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcdjson.GetBlockTemplateCmd)
	request := c.Request

	// TODO: ...

	_ = request
	return nil, nil
}
