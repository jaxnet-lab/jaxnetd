// Copyright (c) 2014-2017 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers
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
)

// TestChainSvrWsCmds tests all of the chain server websocket-specific commands
// marshal and unmarshal into valid results include handling of optional fields
// being omitted in the marshalled command, while optional fields with defaults
// have the default assigned on unmarshalled commands.
func TestChainSvrWsCmds(t *testing.T) {
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
			name: "authenticate",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("authenticate", "user", "pass")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewAuthenticateCmd("user", "pass")
			},
			marshalled:   `{"jsonrpc":"1.0","method":"authenticate","params":["user","pass"],"id":1}`,
			unmarshalled: &jaxjson.AuthenticateCmd{Username: "user", Passphrase: "pass"},
		},
		{
			name: "notifyblocks",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("notifyblocks")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewNotifyBlocksCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"notifyblocks","params":[],"id":1}`,
			unmarshalled: &jaxjson.NotifyBlocksCmd{},
		},
		{
			name: "stopnotifyblocks",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("stopnotifyblocks")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewStopNotifyBlocksCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"stopnotifyblocks","params":[],"id":1}`,
			unmarshalled: &jaxjson.StopNotifyBlocksCmd{},
		},
		{
			name: "notifynewtransactions",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("notifynewtransactions")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewNotifyNewTransactionsCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"notifynewtransactions","params":[],"id":1}`,
			unmarshalled: &jaxjson.NotifyNewTransactionsCmd{
				Verbose: jaxjson.Bool(false),
			},
		},
		{
			name: "notifynewtransactions optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("notifynewtransactions", true)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewNotifyNewTransactionsCmd(jaxjson.Bool(true))
			},
			marshalled: `{"jsonrpc":"1.0","method":"notifynewtransactions","params":[true],"id":1}`,
			unmarshalled: &jaxjson.NotifyNewTransactionsCmd{
				Verbose: jaxjson.Bool(true),
			},
		},
		{
			name: "stopnotifynewtransactions",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("stopnotifynewtransactions")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewStopNotifyNewTransactionsCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"stopnotifynewtransactions","params":[],"id":1}`,
			unmarshalled: &jaxjson.StopNotifyNewTransactionsCmd{},
		},
		{
			name: "notifyreceived",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("notifyreceived", []string{"1Address"})
			},
			staticCmd: func() interface{} {
				return jaxjson.NewNotifyReceivedCmd([]string{"1Address"})
			},
			marshalled: `{"jsonrpc":"1.0","method":"notifyreceived","params":[["1Address"]],"id":1}`,
			unmarshalled: &jaxjson.NotifyReceivedCmd{
				Addresses: []string{"1Address"},
			},
		},
		{
			name: "stopnotifyreceived",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("stopnotifyreceived", []string{"1Address"})
			},
			staticCmd: func() interface{} {
				return jaxjson.NewStopNotifyReceivedCmd([]string{"1Address"})
			},
			marshalled: `{"jsonrpc":"1.0","method":"stopnotifyreceived","params":[["1Address"]],"id":1}`,
			unmarshalled: &jaxjson.StopNotifyReceivedCmd{
				Addresses: []string{"1Address"},
			},
		},
		{
			name: "notifyspent",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("notifyspent", `[{"hash":"123","index":0}]`)
			},
			staticCmd: func() interface{} {
				ops := []jaxjson.OutPoint{{Hash: "123", Index: 0}}
				return jaxjson.NewNotifySpentCmd(ops)
			},
			marshalled: `{"jsonrpc":"1.0","method":"notifyspent","params":[[{"hash":"123","index":0}]],"id":1}`,
			unmarshalled: &jaxjson.NotifySpentCmd{
				OutPoints: []jaxjson.OutPoint{{Hash: "123", Index: 0}},
			},
		},
		{
			name: "stopnotifyspent",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("stopnotifyspent", `[{"hash":"123","index":0}]`)
			},
			staticCmd: func() interface{} {
				ops := []jaxjson.OutPoint{{Hash: "123", Index: 0}}
				return jaxjson.NewStopNotifySpentCmd(ops)
			},
			marshalled: `{"jsonrpc":"1.0","method":"stopnotifyspent","params":[[{"hash":"123","index":0}]],"id":1}`,
			unmarshalled: &jaxjson.StopNotifySpentCmd{
				OutPoints: []jaxjson.OutPoint{{Hash: "123", Index: 0}},
			},
		},
		{
			name: "rescan",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("rescan", "123", `["1Address"]`, `[{"hash":"0000000000000000000000000000000000000000000000000000000000000123","index":0}]`)
			},
			staticCmd: func() interface{} {
				addrs := []string{"1Address"}
				ops := []jaxjson.OutPoint{{
					Hash:  "0000000000000000000000000000000000000000000000000000000000000123",
					Index: 0,
				}}
				return jaxjson.NewRescanCmd("123", addrs, ops, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"rescan","params":["123",["1Address"],[{"hash":"0000000000000000000000000000000000000000000000000000000000000123","index":0}]],"id":1}`,
			unmarshalled: &jaxjson.RescanCmd{
				BeginBlock: "123",
				Addresses:  []string{"1Address"},
				OutPoints:  []jaxjson.OutPoint{{Hash: "0000000000000000000000000000000000000000000000000000000000000123", Index: 0}},
				EndBlock:   nil,
			},
		},
		{
			name: "rescan optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("rescan", "123", `["1Address"]`, `[{"hash":"123","index":0}]`, "456")
			},
			staticCmd: func() interface{} {
				addrs := []string{"1Address"}
				ops := []jaxjson.OutPoint{{Hash: "123", Index: 0}}
				return jaxjson.NewRescanCmd("123", addrs, ops, jaxjson.String("456"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"rescan","params":["123",["1Address"],[{"hash":"123","index":0}],"456"],"id":1}`,
			unmarshalled: &jaxjson.RescanCmd{
				BeginBlock: "123",
				Addresses:  []string{"1Address"},
				OutPoints:  []jaxjson.OutPoint{{Hash: "123", Index: 0}},
				EndBlock:   jaxjson.String("456"),
			},
		},
		{
			name: "loadtxfilter",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("loadtxfilter", false, `["1Address"]`, `[{"hash":"0000000000000000000000000000000000000000000000000000000000000123","index":0}]`)
			},
			staticCmd: func() interface{} {
				addrs := []string{"1Address"}
				ops := []jaxjson.OutPoint{{
					Hash:  "0000000000000000000000000000000000000000000000000000000000000123",
					Index: 0,
				}}
				return jaxjson.NewLoadTxFilterCmd(false, addrs, ops)
			},
			marshalled: `{"jsonrpc":"1.0","method":"loadtxfilter","params":[false,["1Address"],[{"hash":"0000000000000000000000000000000000000000000000000000000000000123","index":0}]],"id":1}`,
			unmarshalled: &jaxjson.LoadTxFilterCmd{
				Reload:    false,
				Addresses: []string{"1Address"},
				OutPoints: []jaxjson.OutPoint{{Hash: "0000000000000000000000000000000000000000000000000000000000000123", Index: 0}},
			},
		},
		{
			name: "rescanblocks",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("rescanblocks", `["0000000000000000000000000000000000000000000000000000000000000123"]`)
			},
			staticCmd: func() interface{} {
				blockhashes := []string{"0000000000000000000000000000000000000000000000000000000000000123"}
				return jaxjson.NewRescanBlocksCmd(blockhashes)
			},
			marshalled: `{"jsonrpc":"1.0","method":"rescanblocks","params":[["0000000000000000000000000000000000000000000000000000000000000123"]],"id":1}`,
			unmarshalled: &jaxjson.RescanBlocksCmd{
				BlockHashes: []string{"0000000000000000000000000000000000000000000000000000000000000123"},
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
