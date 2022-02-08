// Copyright (c) 2014 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package jaxjson_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// TestChainSvrCmds tests all of the chain server commands marshal and unmarshal
// into valid results include handling of optional fields being omitted in the
// marshalled command, while optional fields with defaults have the default
// assigned on unmarshalled commands.
func TestChainSvrCmds(t *testing.T) {
	t.Parallel()

	testID := int(1)
	tests := []struct {
		name         string
		newCmd       func() (interface{}, error)
		staticCmd    func() interface{}
		marshalled   string
		unmarshalled interface{}
	}{
		{
			name: "addnode",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("addnode", "127.0.0.1", jaxjson.ANRemove)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewAddNodeCmd("127.0.0.1", jaxjson.ANRemove)
			},
			marshalled:   `{"jsonrpc":"1.0","method":"addnode","params":["127.0.0.1","remove"],"id":1}`,
			unmarshalled: &jaxjson.AddNodeCmd{Addr: "127.0.0.1", SubCmd: jaxjson.ANRemove},
		},
		{
			name: "createrawtransaction",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("createrawtransaction", `[{"txid":"123","vout":1}]`,
					`{"456":0.0123}`)
			},
			staticCmd: func() interface{} {
				txInputs := []jaxjson.TransactionInput{
					{Txid: "123", Vout: 1},
				}
				amounts := map[string]float64{"456": .0123}
				return jaxjson.NewCreateRawTransactionCmd(txInputs, amounts, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"createrawtransaction","params":[[{"txid":"123","vout":1}],{"456":0.0123}],"id":1}`,
			unmarshalled: &jaxjson.CreateRawTransactionCmd{
				Inputs:  []jaxjson.TransactionInput{{Txid: "123", Vout: 1}},
				Amounts: map[string]float64{"456": .0123},
			},
		},
		{
			name: "createrawtransaction - no inputs",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("createrawtransaction", `[]`, `{"456":0.0123}`)
			},
			staticCmd: func() interface{} {
				amounts := map[string]float64{"456": .0123}
				return jaxjson.NewCreateRawTransactionCmd(nil, amounts, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"createrawtransaction","params":[[],{"456":0.0123}],"id":1}`,
			unmarshalled: &jaxjson.CreateRawTransactionCmd{
				Inputs:  []jaxjson.TransactionInput{},
				Amounts: map[string]float64{"456": .0123},
			},
		},
		{
			name: "createrawtransaction optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("createrawtransaction", `[{"txid":"123","vout":1}]`,
					`{"456":0.0123}`, int64(12312333333))
			},
			staticCmd: func() interface{} {
				txInputs := []jaxjson.TransactionInput{
					{Txid: "123", Vout: 1},
				}
				amounts := map[string]float64{"456": .0123}
				return jaxjson.NewCreateRawTransactionCmd(txInputs, amounts, jaxjson.Int64(12312333333))
			},
			marshalled: `{"jsonrpc":"1.0","method":"createrawtransaction","params":[[{"txid":"123","vout":1}],{"456":0.0123},12312333333],"id":1}`,
			unmarshalled: &jaxjson.CreateRawTransactionCmd{
				Inputs:   []jaxjson.TransactionInput{{Txid: "123", Vout: 1}},
				Amounts:  map[string]float64{"456": .0123},
				LockTime: jaxjson.Int64(12312333333),
			},
		},

		{
			name: "decoderawtransaction",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("decoderawtransaction", "123")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewDecodeRawTransactionCmd("123")
			},
			marshalled:   `{"jsonrpc":"1.0","method":"decoderawtransaction","params":["123"],"id":1}`,
			unmarshalled: &jaxjson.DecodeRawTransactionCmd{HexTx: "123"},
		},
		{
			name: "decodescript",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("decodescript", "00")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewDecodeScriptCmd("00")
			},
			marshalled:   `{"jsonrpc":"1.0","method":"decodescript","params":["00"],"id":1}`,
			unmarshalled: &jaxjson.DecodeScriptCmd{HexScript: "00"},
		},
		{
			name: "getaddednodeinfo",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getaddednodeinfo", true)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetAddedNodeInfoCmd(true, nil)
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getaddednodeinfo","params":[true],"id":1}`,
			unmarshalled: &jaxjson.GetAddedNodeInfoCmd{DNS: true, Node: nil},
		},
		{
			name: "getaddednodeinfo optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getaddednodeinfo", true, "127.0.0.1")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetAddedNodeInfoCmd(true, jaxjson.String("127.0.0.1"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getaddednodeinfo","params":[true,"127.0.0.1"],"id":1}`,
			unmarshalled: &jaxjson.GetAddedNodeInfoCmd{
				DNS:  true,
				Node: jaxjson.String("127.0.0.1"),
			},
		},
		{
			name: "getbestblockhash",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getbestblockhash")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBestBlockHashCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getbestblockhash","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetBestBlockHashCmd{},
		},
		{
			name: "getblock",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblock", "123", jaxjson.Int(0))
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBeaconBlockCmd("123", jaxjson.Int(0))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblock","params":["123",0],"id":1}`,
			unmarshalled: &jaxjson.GetBeaconBlockCmd{
				Hash:      "123",
				Verbosity: jaxjson.Int(0),
			},
		},
		{
			name: "getblock default verbosity",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblock", "123")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBeaconBlockCmd("123", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblock","params":["123"],"id":1}`,
			unmarshalled: &jaxjson.GetBeaconBlockCmd{
				Hash:      "123",
				Verbosity: jaxjson.Int(1),
			},
		},
		{
			name: "getblock required optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblock", "123", jaxjson.Int(1))
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBeaconBlockCmd("123", jaxjson.Int(1))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblock","params":["123",1],"id":1}`,
			unmarshalled: &jaxjson.GetBeaconBlockCmd{
				Hash:      "123",
				Verbosity: jaxjson.Int(1),
			},
		},
		{
			name: "getblock required optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblock", "123", jaxjson.Int(2))
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBeaconBlockCmd("123", jaxjson.Int(2))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblock","params":["123",2],"id":1}`,
			unmarshalled: &jaxjson.GetBeaconBlockCmd{
				Hash:      "123",
				Verbosity: jaxjson.Int(2),
			},
		},
		{
			name: "getblockchaininfo",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblockchaininfo")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBlockChainInfoCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getblockchaininfo","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetBlockChainInfoCmd{},
		},
		{
			name: "getblockcount",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblockcount")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBlockCountCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getblockcount","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetBlockCountCmd{},
		},
		{
			name: "getblockhash",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblockhash", 123)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBlockHashCmd(123)
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getblockhash","params":[123],"id":1}`,
			unmarshalled: &jaxjson.GetBlockHashCmd{Index: 123},
		},
		{
			name: "getblockheader",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblockheader", "123")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBeaconBlockHeaderCmd("123", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblockheader","params":["123"],"id":1}`,
			unmarshalled: &jaxjson.GetBeaconBlockHeaderCmd{
				Hash:    "123",
				Verbose: jaxjson.Bool(true),
			},
		},
		{
			name: "getblockstats height",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblockstats", jaxjson.HashOrHeight{Value: 123})
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBlockStatsCmd(jaxjson.HashOrHeight{Value: 123}, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblockstats","params":[123],"id":1}`,
			unmarshalled: &jaxjson.GetBlockStatsCmd{
				HashOrHeight: jaxjson.HashOrHeight{Value: 123},
			},
		},
		{
			name: "getblockstats hash",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblockstats", jaxjson.HashOrHeight{Value: "deadbeef"})
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBlockStatsCmd(jaxjson.HashOrHeight{Value: "deadbeef"}, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblockstats","params":["deadbeef"],"id":1}`,
			unmarshalled: &jaxjson.GetBlockStatsCmd{
				HashOrHeight: jaxjson.HashOrHeight{Value: "deadbeef"},
			},
		},
		{
			name: "getblockstats height optional stats",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblockstats", jaxjson.HashOrHeight{Value: 123}, []string{"avgfee", "maxfee"})
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBlockStatsCmd(jaxjson.HashOrHeight{Value: 123}, &[]string{"avgfee", "maxfee"})
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblockstats","params":[123,["avgfee","maxfee"]],"id":1}`,
			unmarshalled: &jaxjson.GetBlockStatsCmd{
				HashOrHeight: jaxjson.HashOrHeight{Value: 123},
				Stats:        &[]string{"avgfee", "maxfee"},
			},
		},
		{
			name: "getblockstats hash optional stats",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblockstats", jaxjson.HashOrHeight{Value: "deadbeef"}, []string{"avgfee", "maxfee"})
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBlockStatsCmd(jaxjson.HashOrHeight{Value: "deadbeef"}, &[]string{"avgfee", "maxfee"})
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblockstats","params":["deadbeef",["avgfee","maxfee"]],"id":1}`,
			unmarshalled: &jaxjson.GetBlockStatsCmd{
				HashOrHeight: jaxjson.HashOrHeight{Value: "deadbeef"},
				Stats:        &[]string{"avgfee", "maxfee"},
			},
		},
		{
			name: "getblocktemplate",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblocktemplate")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBeaconBlockTemplateCmd(nil)
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getblocktemplate","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetBeaconBlockTemplateCmd{Request: nil},
		},
		{
			name: "getblocktemplate optional - template request",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblocktemplate", `{"mode":"template","capabilities":["longpoll","coinbasetxn"]}`)
			},
			staticCmd: func() interface{} {
				template := jaxjson.TemplateRequest{
					Mode:         "template",
					Capabilities: []string{"longpoll", "coinbasetxn"},
				}
				return jaxjson.NewGetBeaconBlockTemplateCmd(&template)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblocktemplate","params":[{"mode":"template","capabilities":["longpoll","coinbasetxn"]}],"id":1}`,
			unmarshalled: &jaxjson.GetBeaconBlockTemplateCmd{
				Request: &jaxjson.TemplateRequest{
					Mode:         "template",
					Capabilities: []string{"longpoll", "coinbasetxn"},
				},
			},
		},
		{
			name: "getblocktemplate optional - template request with tweaks",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblocktemplate", `{"mode":"template","capabilities":["longpoll","coinbasetxn"],"sigoplimit":500,"sizelimit":100000000,"maxversion":2}`)
			},
			staticCmd: func() interface{} {
				template := jaxjson.TemplateRequest{
					Mode:         "template",
					Capabilities: []string{"longpoll", "coinbasetxn"},
					SigOpLimit:   500,
					SizeLimit:    100000000,
					MaxVersion:   2,
				}
				return jaxjson.NewGetBeaconBlockTemplateCmd(&template)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblocktemplate","params":[{"mode":"template","capabilities":["longpoll","coinbasetxn"],"sigoplimit":500,"sizelimit":100000000,"maxversion":2}],"id":1}`,
			unmarshalled: &jaxjson.GetBeaconBlockTemplateCmd{
				Request: &jaxjson.TemplateRequest{
					Mode:         "template",
					Capabilities: []string{"longpoll", "coinbasetxn"},
					SigOpLimit:   int64(500),
					SizeLimit:    int64(100000000),
					MaxVersion:   2,
				},
			},
		},
		{
			name: "getblocktemplate optional - template request with tweaks 2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getblocktemplate", `{"mode":"template","capabilities":["longpoll","coinbasetxn"],"sigoplimit":true,"sizelimit":100000000,"maxversion":2}`)
			},
			staticCmd: func() interface{} {
				template := jaxjson.TemplateRequest{
					Mode:         "template",
					Capabilities: []string{"longpoll", "coinbasetxn"},
					SigOpLimit:   true,
					SizeLimit:    100000000,
					MaxVersion:   2,
				}
				return jaxjson.NewGetBeaconBlockTemplateCmd(&template)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getblocktemplate","params":[{"mode":"template","capabilities":["longpoll","coinbasetxn"],"sigoplimit":true,"sizelimit":100000000,"maxversion":2}],"id":1}`,
			unmarshalled: &jaxjson.GetBeaconBlockTemplateCmd{
				Request: &jaxjson.TemplateRequest{
					Mode:         "template",
					Capabilities: []string{"longpoll", "coinbasetxn"},
					SigOpLimit:   true,
					SizeLimit:    int64(100000000),
					MaxVersion:   2,
				},
			},
		},
		{
			name: "getcfilter",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getcfilter", "123",
					wire.GCSFilterRegular)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetCFilterCmd("123",
					wire.GCSFilterRegular)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getcfilter","params":["123",0],"id":1}`,
			unmarshalled: &jaxjson.GetCFilterCmd{
				Hash:       "123",
				FilterType: wire.GCSFilterRegular,
			},
		},
		{
			name: "getcfilterheader",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getcfilterheader", "123",
					wire.GCSFilterRegular)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetCFilterHeaderCmd("123",
					wire.GCSFilterRegular)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getcfilterheader","params":["123",0],"id":1}`,
			unmarshalled: &jaxjson.GetCFilterHeaderCmd{
				Hash:       "123",
				FilterType: wire.GCSFilterRegular,
			},
		},
		{
			name: "getchaintips",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getchaintips")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetChainTipsCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getchaintips","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetChainTipsCmd{},
		},
		{
			name: "getconnectioncount",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getconnectioncount")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetConnectionCountCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getconnectioncount","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetConnectionCountCmd{},
		},
		{
			name: "getdifficulty",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getdifficulty")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetDifficultyCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getdifficulty","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetDifficultyCmd{},
		},
		{
			name: "getgenerate",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getgenerate")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetGenerateCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getgenerate","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetGenerateCmd{},
		},
		{
			name: "gethashespersec",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("gethashespersec")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetHashesPerSecCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"gethashespersec","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetHashesPerSecCmd{},
		},
		{
			name: "getinfo",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getinfo")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetInfoCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getinfo","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetInfoCmd{},
		},
		{
			name: "getmempoolentry",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getmempoolentry", "txhash")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetMempoolEntryCmd("txhash")
			},
			marshalled: `{"jsonrpc":"1.0","method":"getmempoolentry","params":["txhash"],"id":1}`,
			unmarshalled: &jaxjson.GetMempoolEntryCmd{
				TxID: "txhash",
			},
		},
		{
			name: "getmempoolinfo",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getmempoolinfo")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetMempoolInfoCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getmempoolinfo","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetMempoolInfoCmd{},
		},
		{
			name: "getmininginfo",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getmininginfo")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetMiningInfoCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getmininginfo","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetMiningInfoCmd{},
		},
		{
			name: "getnetworkinfo",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getnetworkinfo")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetNetworkInfoCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getnetworkinfo","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetNetworkInfoCmd{},
		},
		{
			name: "getnettotals",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getnettotals")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetNetTotalsCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getnettotals","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetNetTotalsCmd{},
		},
		{
			name: "getnetworkhashps",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getnetworkhashps")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetNetworkHashPSCmd(nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getnetworkhashps","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetNetworkHashPSCmd{
				Blocks: jaxjson.Int(120),
				Height: jaxjson.Int(-1),
			},
		},
		{
			name: "getnetworkhashps optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getnetworkhashps", 200)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetNetworkHashPSCmd(jaxjson.Int(200), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getnetworkhashps","params":[200],"id":1}`,
			unmarshalled: &jaxjson.GetNetworkHashPSCmd{
				Blocks: jaxjson.Int(200),
				Height: jaxjson.Int(-1),
			},
		},
		{
			name: "getnetworkhashps optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getnetworkhashps", 200, 123)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetNetworkHashPSCmd(jaxjson.Int(200), jaxjson.Int(123))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getnetworkhashps","params":[200,123],"id":1}`,
			unmarshalled: &jaxjson.GetNetworkHashPSCmd{
				Blocks: jaxjson.Int(200),
				Height: jaxjson.Int(123),
			},
		},
		{
			name: "getpeerinfo",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getpeerinfo")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetPeerInfoCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getpeerinfo","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetPeerInfoCmd{},
		},
		{
			name: "getrawmempool",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getrawmempool")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetRawMempoolCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getrawmempool","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetRawMempoolCmd{
				Verbose: jaxjson.Bool(false),
			},
		},
		{
			name: "getrawmempool optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getrawmempool", false)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetRawMempoolCmd(jaxjson.Bool(false))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getrawmempool","params":[false],"id":1}`,
			unmarshalled: &jaxjson.GetRawMempoolCmd{
				Verbose: jaxjson.Bool(false),
			},
		},
		{
			name: "getrawtransaction",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getrawtransaction", "123")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetRawTransactionCmd("123", nil, false)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getrawtransaction","params":["123"],"id":1}`,
			unmarshalled: &jaxjson.GetRawTransactionCmd{
				Txid:    "123",
				Verbose: jaxjson.Int(0),
			},
		},
		{
			name: "getrawtransaction optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getrawtransaction", "123", 1, false)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetRawTransactionCmd("123", jaxjson.Int(1), false)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getrawtransaction","params":["123",1],"id":1}`,
			unmarshalled: &jaxjson.GetRawTransactionCmd{
				Txid:    "123",
				Verbose: jaxjson.Int(1),
			},
		},
		{
			name: "gettxout",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("gettxout", "123", 1)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetTxOutCmd("123", 1, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"gettxout","params":["123",1],"id":1}`,
			unmarshalled: &jaxjson.GetTxOutCmd{
				Txid:           "123",
				Vout:           1,
				IncludeMempool: jaxjson.Bool(true),
			},
		},
		{
			name: "gettxout optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("gettxout", "123", 1, true, true)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetTxOutCmd("123", 1, jaxjson.Bool(true), jaxjson.Bool(true))
			},
			marshalled: `{"jsonrpc":"1.0","method":"gettxout","params":["123",1,true],"id":1}`,
			unmarshalled: &jaxjson.GetTxOutCmd{
				Txid:           "123",
				Vout:           1,
				IncludeMempool: jaxjson.Bool(true),
			},
		},
		{
			name: "gettxoutproof",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("gettxoutproof", []string{"123", "456"})
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetTxOutProofCmd([]string{"123", "456"}, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"gettxoutproof","params":[["123","456"]],"id":1}`,
			unmarshalled: &jaxjson.GetTxOutProofCmd{
				TxIDs: []string{"123", "456"},
			},
		},
		{
			name: "gettxoutproof optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("gettxoutproof", []string{"123", "456"},
					jaxjson.String("000000000000034a7dedef4a161fa058a2d67a173a90155f3a2fe6fc132e0ebf"))
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetTxOutProofCmd([]string{"123", "456"},
					jaxjson.String("000000000000034a7dedef4a161fa058a2d67a173a90155f3a2fe6fc132e0ebf"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"gettxoutproof","params":[["123","456"],` +
				`"000000000000034a7dedef4a161fa058a2d67a173a90155f3a2fe6fc132e0ebf"],"id":1}`,
			unmarshalled: &jaxjson.GetTxOutProofCmd{
				TxIDs:     []string{"123", "456"},
				BlockHash: jaxjson.String("000000000000034a7dedef4a161fa058a2d67a173a90155f3a2fe6fc132e0ebf"),
			},
		},
		{
			name: "gettxoutsetinfo",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("gettxoutsetinfo")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetTxOutSetInfoCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"gettxoutsetinfo","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetTxOutSetInfoCmd{},
		},
		{
			name: "getwork",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getwork")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetWorkCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getwork","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetWorkCmd{
				Data: nil,
			},
		},
		{
			name: "getwork optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getwork", "00112233")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetWorkCmd(jaxjson.String("00112233"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getwork","params":["00112233"],"id":1}`,
			unmarshalled: &jaxjson.GetWorkCmd{
				Data: jaxjson.String("00112233"),
			},
		},
		{
			name: "help",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("help")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewHelpCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"help","params":[],"id":1}`,
			unmarshalled: &jaxjson.HelpCmd{
				Command: nil,
			},
		},
		{
			name: "help optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("help", "getblock")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewHelpCmd(jaxjson.String("getblock"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"help","params":["getblock"],"id":1}`,
			unmarshalled: &jaxjson.HelpCmd{
				Command: jaxjson.String("getblock"),
			},
		},
		{
			name: "invalidateblock",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("invalidateblock", "123")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewInvalidateBlockCmd("123")
			},
			marshalled: `{"jsonrpc":"1.0","method":"invalidateblock","params":["123"],"id":1}`,
			unmarshalled: &jaxjson.InvalidateBlockCmd{
				BlockHash: "123",
			},
		},
		{
			name: "ping",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("ping")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewPingCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"ping","params":[],"id":1}`,
			unmarshalled: &jaxjson.PingCmd{},
		},
		{
			name: "preciousblock",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("preciousblock", "0123")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewPreciousBlockCmd("0123")
			},
			marshalled: `{"jsonrpc":"1.0","method":"preciousblock","params":["0123"],"id":1}`,
			unmarshalled: &jaxjson.PreciousBlockCmd{
				BlockHash: "0123",
			},
		},
		{
			name: "reconsiderblock",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("reconsiderblock", "123")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewReconsiderBlockCmd("123")
			},
			marshalled: `{"jsonrpc":"1.0","method":"reconsiderblock","params":["123"],"id":1}`,
			unmarshalled: &jaxjson.ReconsiderBlockCmd{
				BlockHash: "123",
			},
		},
		{
			name: "searchrawtransactions",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("searchrawtransactions", "1Address")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSearchRawTransactionsCmd("1Address", nil, nil, nil, nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"searchrawtransactions","params":["1Address"],"id":1}`,
			unmarshalled: &jaxjson.SearchRawTransactionsCmd{
				Address:     "1Address",
				Verbose:     jaxjson.Int(1),
				Skip:        jaxjson.Int(0),
				Count:       jaxjson.Int(100),
				VinExtra:    jaxjson.Int(0),
				Reverse:     jaxjson.Bool(false),
				FilterAddrs: nil,
			},
		},
		{
			name: "searchrawtransactions",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("searchrawtransactions", "1Address", 0)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSearchRawTransactionsCmd("1Address",
					jaxjson.Int(0), nil, nil, nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"searchrawtransactions","params":["1Address",0],"id":1}`,
			unmarshalled: &jaxjson.SearchRawTransactionsCmd{
				Address:     "1Address",
				Verbose:     jaxjson.Int(0),
				Skip:        jaxjson.Int(0),
				Count:       jaxjson.Int(100),
				VinExtra:    jaxjson.Int(0),
				Reverse:     jaxjson.Bool(false),
				FilterAddrs: nil,
			},
		},
		{
			name: "searchrawtransactions",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("searchrawtransactions", "1Address", 0, 5)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSearchRawTransactionsCmd("1Address",
					jaxjson.Int(0), jaxjson.Int(5), nil, nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"searchrawtransactions","params":["1Address",0,5],"id":1}`,
			unmarshalled: &jaxjson.SearchRawTransactionsCmd{
				Address:     "1Address",
				Verbose:     jaxjson.Int(0),
				Skip:        jaxjson.Int(5),
				Count:       jaxjson.Int(100),
				VinExtra:    jaxjson.Int(0),
				Reverse:     jaxjson.Bool(false),
				FilterAddrs: nil,
			},
		},
		{
			name: "searchrawtransactions",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("searchrawtransactions", "1Address", 0, 5, 10)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSearchRawTransactionsCmd("1Address",
					jaxjson.Int(0), jaxjson.Int(5), jaxjson.Int(10), nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"searchrawtransactions","params":["1Address",0,5,10],"id":1}`,
			unmarshalled: &jaxjson.SearchRawTransactionsCmd{
				Address:     "1Address",
				Verbose:     jaxjson.Int(0),
				Skip:        jaxjson.Int(5),
				Count:       jaxjson.Int(10),
				VinExtra:    jaxjson.Int(0),
				Reverse:     jaxjson.Bool(false),
				FilterAddrs: nil,
			},
		},
		{
			name: "searchrawtransactions",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("searchrawtransactions", "1Address", 0, 5, 10, 1)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSearchRawTransactionsCmd("1Address",
					jaxjson.Int(0), jaxjson.Int(5), jaxjson.Int(10), jaxjson.Int(1), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"searchrawtransactions","params":["1Address",0,5,10,1],"id":1}`,
			unmarshalled: &jaxjson.SearchRawTransactionsCmd{
				Address:     "1Address",
				Verbose:     jaxjson.Int(0),
				Skip:        jaxjson.Int(5),
				Count:       jaxjson.Int(10),
				VinExtra:    jaxjson.Int(1),
				Reverse:     jaxjson.Bool(false),
				FilterAddrs: nil,
			},
		},
		{
			name: "searchrawtransactions",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("searchrawtransactions", "1Address", 0, 5, 10, 1, true)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSearchRawTransactionsCmd("1Address",
					jaxjson.Int(0), jaxjson.Int(5), jaxjson.Int(10), jaxjson.Int(1), jaxjson.Bool(true), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"searchrawtransactions","params":["1Address",0,5,10,1,true],"id":1}`,
			unmarshalled: &jaxjson.SearchRawTransactionsCmd{
				Address:     "1Address",
				Verbose:     jaxjson.Int(0),
				Skip:        jaxjson.Int(5),
				Count:       jaxjson.Int(10),
				VinExtra:    jaxjson.Int(1),
				Reverse:     jaxjson.Bool(true),
				FilterAddrs: nil,
			},
		},
		{
			name: "searchrawtransactions",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("searchrawtransactions", "1Address", 0, 5, 10, 1, true, []string{"1Address"})
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSearchRawTransactionsCmd("1Address",
					jaxjson.Int(0), jaxjson.Int(5), jaxjson.Int(10), jaxjson.Int(1), jaxjson.Bool(true), &[]string{"1Address"})
			},
			marshalled: `{"jsonrpc":"1.0","method":"searchrawtransactions","params":["1Address",0,5,10,1,true,["1Address"]],"id":1}`,
			unmarshalled: &jaxjson.SearchRawTransactionsCmd{
				Address:     "1Address",
				Verbose:     jaxjson.Int(0),
				Skip:        jaxjson.Int(5),
				Count:       jaxjson.Int(10),
				VinExtra:    jaxjson.Int(1),
				Reverse:     jaxjson.Bool(true),
				FilterAddrs: &[]string{"1Address"},
			},
		},
		{
			name: "sendrawtransaction",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendrawtransaction", "1122")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSendRawTransactionCmd("1122", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendrawtransaction","params":["1122"],"id":1}`,
			unmarshalled: &jaxjson.SendRawTransactionCmd{
				HexTx:         "1122",
				AllowHighFees: jaxjson.Bool(false),
			},
		},
		{
			name: "sendrawtransaction optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendrawtransaction", "1122", false)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSendRawTransactionCmd("1122", jaxjson.Bool(false))
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendrawtransaction","params":["1122",false],"id":1}`,
			unmarshalled: &jaxjson.SendRawTransactionCmd{
				HexTx:         "1122",
				AllowHighFees: jaxjson.Bool(false),
			},
		},
		{
			name: "setgenerate",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("setgenerate", true)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSetGenerateCmd(true, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"setgenerate","params":[true],"id":1}`,
			unmarshalled: &jaxjson.SetGenerateCmd{
				Generate:     true,
				GenProcLimit: jaxjson.Int(-1),
			},
		},
		{
			name: "setgenerate optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("setgenerate", true, 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSetGenerateCmd(true, jaxjson.Int(6))
			},
			marshalled: `{"jsonrpc":"1.0","method":"setgenerate","params":[true,6],"id":1}`,
			unmarshalled: &jaxjson.SetGenerateCmd{
				Generate:     true,
				GenProcLimit: jaxjson.Int(6),
			},
		},
		{
			name: "stop",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("stop")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewStopCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"stop","params":[],"id":1}`,
			unmarshalled: &jaxjson.StopCmd{},
		},
		{
			name: "submitblock",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("submitblock", "112233")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSubmitBlockCmd("112233", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"submitblock","params":["112233"],"id":1}`,
			unmarshalled: &jaxjson.SubmitBlockCmd{
				HexBlock: "112233",
				Options:  nil,
			},
		},
		{
			name: "submitblock optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("submitblock", "112233", `{"workid":"12345"}`)
			},
			staticCmd: func() interface{} {
				options := jaxjson.SubmitBlockOptions{
					WorkID: "12345",
				}
				return jaxjson.NewSubmitBlockCmd("112233", &options)
			},
			marshalled: `{"jsonrpc":"1.0","method":"submitblock","params":["112233",{"workid":"12345"}],"id":1}`,
			unmarshalled: &jaxjson.SubmitBlockCmd{
				HexBlock: "112233",
				Options: &jaxjson.SubmitBlockOptions{
					WorkID: "12345",
				},
			},
		},
		{
			name: "uptime",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("uptime")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewUptimeCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"uptime","params":[],"id":1}`,
			unmarshalled: &jaxjson.UptimeCmd{},
		},
		{
			name: "validateaddress",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("validateaddress", "1Address")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewValidateAddressCmd("1Address")
			},
			marshalled: `{"jsonrpc":"1.0","method":"validateaddress","params":["1Address"],"id":1}`,
			unmarshalled: &jaxjson.ValidateAddressCmd{
				Address: "1Address",
			},
		},
		{
			name: "verifychain",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("verifychain")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewVerifyChainCmd(nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"verifychain","params":[],"id":1}`,
			unmarshalled: &jaxjson.VerifyChainCmd{
				CheckLevel: jaxjson.Int32(3),
				CheckDepth: jaxjson.Int32(288),
			},
		},
		{
			name: "verifychain optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("verifychain", 2)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewVerifyChainCmd(jaxjson.Int32(2), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"verifychain","params":[2],"id":1}`,
			unmarshalled: &jaxjson.VerifyChainCmd{
				CheckLevel: jaxjson.Int32(2),
				CheckDepth: jaxjson.Int32(288),
			},
		},
		{
			name: "verifychain optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("verifychain", 2, 500)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewVerifyChainCmd(jaxjson.Int32(2), jaxjson.Int32(500))
			},
			marshalled: `{"jsonrpc":"1.0","method":"verifychain","params":[2,500],"id":1}`,
			unmarshalled: &jaxjson.VerifyChainCmd{
				CheckLevel: jaxjson.Int32(2),
				CheckDepth: jaxjson.Int32(500),
			},
		},
		{
			name: "verifymessage",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("verifymessage", "1Address", "301234", "test")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewVerifyMessageCmd("1Address", "301234", "test")
			},
			marshalled: `{"jsonrpc":"1.0","method":"verifymessage","params":["1Address","301234","test"],"id":1}`,
			unmarshalled: &jaxjson.VerifyMessageCmd{
				Address:   "1Address",
				Signature: "301234",
				Message:   "test",
			},
		},
		{
			name: "verifytxoutproof",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("verifytxoutproof", "test")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewVerifyTxOutProofCmd("test")
			},
			marshalled: `{"jsonrpc":"1.0","method":"verifytxoutproof","params":["test"],"id":1}`,
			unmarshalled: &jaxjson.VerifyTxOutProofCmd{
				Proof: "test",
			},
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Marshal the command as created by the new static command
		// creation function.
		marshalled, err := jaxjson.MarshalCmd(testID, 0, test.staticCmd())
		if err != nil {
			t.Errorf("MarshalCmd #%d (%s) unexpected error: %v", i,
				test.name, err)
			continue
		}

		if !bytes.Equal(marshalled, []byte(test.marshalled)) {
			t.Errorf("Test #%d (%s) unexpected marshalled data - "+
				"got %s, want %s", i, test.name, marshalled,
				test.marshalled)
			t.Errorf("\n%s\n%s", marshalled, test.marshalled)
			continue
		}

		// Ensure the command is created without error via the generic
		// new command creation function.
		cmd, err := test.newCmd()
		if err != nil {
			t.Errorf("Test #%d (%s) unexpected NewCmd error: %v ",
				i, test.name, err)
		}

		// Marshal the command as created by the generic new command
		// creation function.
		marshalled, err = jaxjson.MarshalCmd(testID, 0, cmd)
		if err != nil {
			t.Errorf("MarshalCmd #%d (%s) unexpected error: %v", i,
				test.name, err)
			continue
		}

		if !bytes.Equal(marshalled, []byte(test.marshalled)) {
			t.Errorf("Test #%d (%s) unexpected marshalled data - "+
				"got %s, want %s", i, test.name, marshalled,
				test.marshalled)
			continue
		}

		var request jaxjson.Request
		if err := json.Unmarshal(marshalled, &request); err != nil {
			t.Errorf("Test #%d (%s) unexpected error while "+
				"unmarshalling JSON-RPC request: %v", i,
				test.name, err)
			continue
		}

		cmd, err = jaxjson.UnmarshalCmd(&request)
		if err != nil {
			t.Errorf("UnmarshalCmd #%d (%s) unexpected error: %v", i,
				test.name, err)
			continue
		}

		if !reflect.DeepEqual(cmd, test.unmarshalled) {
			t.Errorf("Test #%d (%s) unexpected unmarshalled command "+
				"- got %s, want %s", i, test.name,
				fmt.Sprintf("(%T) %+[1]v", cmd),
				fmt.Sprintf("(%T) %+[1]v\n", test.unmarshalled))
			continue
		}
	}
}

// TestChainSvrCmdErrors ensures any errors that occur in the command during
// custom mashal and unmarshal are as expected.
func TestChainSvrCmdErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     interface{}
		marshalled string
		err        error
	}{
		{
			name:       "template request with invalid type",
			result:     &jaxjson.TemplateRequest{},
			marshalled: `{"mode":1}`,
			err:        &json.UnmarshalTypeError{},
		},
		{
			name:       "invalid template request sigoplimit field",
			result:     &jaxjson.TemplateRequest{},
			marshalled: `{"sigoplimit":"invalid"}`,
			err:        jaxjson.Error{ErrorCode: jaxjson.ErrInvalidType},
		},
		{
			name:       "invalid template request sizelimit field",
			result:     &jaxjson.TemplateRequest{},
			marshalled: `{"sizelimit":"invalid"}`,
			err:        jaxjson.Error{ErrorCode: jaxjson.ErrInvalidType},
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		err := json.Unmarshal([]byte(test.marshalled), &test.result)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("Test #%d (%s) wrong error - got %T (%v), "+
				"want %T", i, test.name, err, err, test.err)
			continue
		}

		if terr, ok := test.err.(jaxjson.Error); ok {
			gotErrorCode := err.(jaxjson.Error).ErrorCode
			if gotErrorCode != terr.ErrorCode {
				t.Errorf("Test #%d (%s) mismatched error code "+
					"- got %v (%v), want %v", i, test.name,
					gotErrorCode, terr, terr.ErrorCode)
				continue
			}
		}
	}
}
