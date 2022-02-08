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
)

// TestWalletSvrCmds tests all of the wallet server commands marshal and
// unmarshal into valid results include handling of optional fields being
// omitted in the marshalled command, while optional fields with defaults have
// the default assigned on unmarshalled commands.
func TestWalletSvrCmds(t *testing.T) {
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
			name: "addmultisigaddress",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("addmultisigaddress", 2, []string{"031234", "035678"})
			},
			staticCmd: func() interface{} {
				keys := []string{"031234", "035678"}
				return jaxjson.NewAddMultisigAddressCmd(2, keys, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"addmultisigaddress","params":[2,["031234","035678"]],"id":1}`,
			unmarshalled: &jaxjson.AddMultisigAddressCmd{
				NRequired: 2,
				Keys:      []string{"031234", "035678"},
				Account:   nil,
			},
		},
		{
			name: "addmultisigaddress optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("addmultisigaddress", 2, []string{"031234", "035678"}, "test")
			},
			staticCmd: func() interface{} {
				keys := []string{"031234", "035678"}
				return jaxjson.NewAddMultisigAddressCmd(2, keys, jaxjson.String("test"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"addmultisigaddress","params":[2,["031234","035678"],"test"],"id":1}`,
			unmarshalled: &jaxjson.AddMultisigAddressCmd{
				NRequired: 2,
				Keys:      []string{"031234", "035678"},
				Account:   jaxjson.String("test"),
			},
		},
		{
			name: "addwitnessaddress",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("addwitnessaddress", "1address")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewAddWitnessAddressCmd("1address")
			},
			marshalled: `{"jsonrpc":"1.0","method":"addwitnessaddress","params":["1address"],"id":1}`,
			unmarshalled: &jaxjson.AddWitnessAddressCmd{
				Address: "1address",
			},
		},
		{
			name: "createmultisig",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("createmultisig", 2, []string{"031234", "035678"})
			},
			staticCmd: func() interface{} {
				keys := []string{"031234", "035678"}
				return jaxjson.NewCreateMultisigCmd(2, keys)
			},
			marshalled: `{"jsonrpc":"1.0","method":"createmultisig","params":[2,["031234","035678"]],"id":1}`,
			unmarshalled: &jaxjson.CreateMultisigCmd{
				NRequired: 2,
				Keys:      []string{"031234", "035678"},
			},
		},
		{
			name: "dumpprivkey",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("dumpprivkey", "1Address")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewDumpPrivKeyCmd("1Address")
			},
			marshalled: `{"jsonrpc":"1.0","method":"dumpprivkey","params":["1Address"],"id":1}`,
			unmarshalled: &jaxjson.DumpPrivKeyCmd{
				Address: "1Address",
			},
		},
		{
			name: "encryptwallet",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("encryptwallet", "pass")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewEncryptWalletCmd("pass")
			},
			marshalled: `{"jsonrpc":"1.0","method":"encryptwallet","params":["pass"],"id":1}`,
			unmarshalled: &jaxjson.EncryptWalletCmd{
				Passphrase: "pass",
			},
		},
		{
			name: "estimatefee",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("estimatefee", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewEstimateFeeCmd(6)
			},
			marshalled: `{"jsonrpc":"1.0","method":"estimatefee","params":[6],"id":1}`,
			unmarshalled: &jaxjson.EstimateFeeCmd{
				NumBlocks: 6,
			},
		},
		{
			name: "estimatesmartfee - no mode",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("estimatesmartfee", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewEstimateSmartFeeCmd(6, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"estimatesmartfee","params":[6],"id":1}`,
			unmarshalled: &jaxjson.EstimateSmartFeeCmd{
				ConfTarget:   6,
				EstimateMode: &jaxjson.EstimateModeConservative,
			},
		},
		{
			name: "estimatesmartfee - economical mode",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("estimatesmartfee", 6, jaxjson.EstimateModeEconomical)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewEstimateSmartFeeCmd(6, &jaxjson.EstimateModeEconomical)
			},
			marshalled: `{"jsonrpc":"1.0","method":"estimatesmartfee","params":[6,"ECONOMICAL"],"id":1}`,
			unmarshalled: &jaxjson.EstimateSmartFeeCmd{
				ConfTarget:   6,
				EstimateMode: &jaxjson.EstimateModeEconomical,
			},
		},
		{
			name: "estimatepriority",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("estimatepriority", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewEstimatePriorityCmd(6)
			},
			marshalled: `{"jsonrpc":"1.0","method":"estimatepriority","params":[6],"id":1}`,
			unmarshalled: &jaxjson.EstimatePriorityCmd{
				NumBlocks: 6,
			},
		},
		{
			name: "getaccount",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getaccount", "1Address")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetAccountCmd("1Address")
			},
			marshalled: `{"jsonrpc":"1.0","method":"getaccount","params":["1Address"],"id":1}`,
			unmarshalled: &jaxjson.GetAccountCmd{
				Address: "1Address",
			},
		},
		{
			name: "getaccountaddress",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getaccountaddress", "acct")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetAccountAddressCmd("acct")
			},
			marshalled: `{"jsonrpc":"1.0","method":"getaccountaddress","params":["acct"],"id":1}`,
			unmarshalled: &jaxjson.GetAccountAddressCmd{
				Account: "acct",
			},
		},
		{
			name: "getaddressesbyaccount",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getaddressesbyaccount", "acct")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetAddressesByAccountCmd("acct")
			},
			marshalled: `{"jsonrpc":"1.0","method":"getaddressesbyaccount","params":["acct"],"id":1}`,
			unmarshalled: &jaxjson.GetAddressesByAccountCmd{
				Account: "acct",
			},
		},
		{
			name: "getbalance",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getbalance")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBalanceCmd(nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getbalance","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetBalanceCmd{
				Account: nil,
				MinConf: jaxjson.Int(1),
			},
		},
		{
			name: "getbalance optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getbalance", "acct")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBalanceCmd(jaxjson.String("acct"), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getbalance","params":["acct"],"id":1}`,
			unmarshalled: &jaxjson.GetBalanceCmd{
				Account: jaxjson.String("acct"),
				MinConf: jaxjson.Int(1),
			},
		},
		{
			name: "getbalance optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getbalance", "acct", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetBalanceCmd(jaxjson.String("acct"), jaxjson.Int(6))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getbalance","params":["acct",6],"id":1}`,
			unmarshalled: &jaxjson.GetBalanceCmd{
				Account: jaxjson.String("acct"),
				MinConf: jaxjson.Int(6),
			},
		},
		{
			name: "getnewaddress",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getnewaddress")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetNewAddressCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getnewaddress","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetNewAddressCmd{
				Account: nil,
			},
		},
		{
			name: "getnewaddress optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getnewaddress", "acct")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetNewAddressCmd(jaxjson.String("acct"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getnewaddress","params":["acct"],"id":1}`,
			unmarshalled: &jaxjson.GetNewAddressCmd{
				Account: jaxjson.String("acct"),
			},
		},
		{
			name: "getrawchangeaddress",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getrawchangeaddress")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetRawChangeAddressCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getrawchangeaddress","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetRawChangeAddressCmd{
				Account: nil,
			},
		},
		{
			name: "getrawchangeaddress optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getrawchangeaddress", "acct")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetRawChangeAddressCmd(jaxjson.String("acct"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getrawchangeaddress","params":["acct"],"id":1}`,
			unmarshalled: &jaxjson.GetRawChangeAddressCmd{
				Account: jaxjson.String("acct"),
			},
		},
		{
			name: "getreceivedbyaccount",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getreceivedbyaccount", "acct")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetReceivedByAccountCmd("acct", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getreceivedbyaccount","params":["acct"],"id":1}`,
			unmarshalled: &jaxjson.GetReceivedByAccountCmd{
				Account: "acct",
				MinConf: jaxjson.Int(1),
			},
		},
		{
			name: "getreceivedbyaccount optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getreceivedbyaccount", "acct", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetReceivedByAccountCmd("acct", jaxjson.Int(6))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getreceivedbyaccount","params":["acct",6],"id":1}`,
			unmarshalled: &jaxjson.GetReceivedByAccountCmd{
				Account: "acct",
				MinConf: jaxjson.Int(6),
			},
		},
		{
			name: "getreceivedbyaddress",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getreceivedbyaddress", "1Address")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetReceivedByAddressCmd("1Address", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getreceivedbyaddress","params":["1Address"],"id":1}`,
			unmarshalled: &jaxjson.GetReceivedByAddressCmd{
				Address: "1Address",
				MinConf: jaxjson.Int(1),
			},
		},
		{
			name: "getreceivedbyaddress optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getreceivedbyaddress", "1Address", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetReceivedByAddressCmd("1Address", jaxjson.Int(6))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getreceivedbyaddress","params":["1Address",6],"id":1}`,
			unmarshalled: &jaxjson.GetReceivedByAddressCmd{
				Address: "1Address",
				MinConf: jaxjson.Int(6),
			},
		},
		{
			name: "gettransaction",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("gettransaction", "123")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetTransactionCmd("123", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"gettransaction","params":["123"],"id":1}`,
			unmarshalled: &jaxjson.GetTransactionCmd{
				Txid:             "123",
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "gettransaction optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("gettransaction", "123", true)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetTransactionCmd("123", jaxjson.Bool(true))
			},
			marshalled: `{"jsonrpc":"1.0","method":"gettransaction","params":["123",true],"id":1}`,
			unmarshalled: &jaxjson.GetTransactionCmd{
				Txid:             "123",
				IncludeWatchOnly: jaxjson.Bool(true),
			},
		},
		{
			name: "getwalletinfo",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("getwalletinfo")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewGetWalletInfoCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getwalletinfo","params":[],"id":1}`,
			unmarshalled: &jaxjson.GetWalletInfoCmd{},
		},
		{
			name: "importprivkey",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("importprivkey", "abc")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewImportPrivKeyCmd("abc", nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"importprivkey","params":["abc"],"id":1}`,
			unmarshalled: &jaxjson.ImportPrivKeyCmd{
				PrivKey: "abc",
				Label:   nil,
				Rescan:  jaxjson.Bool(true),
			},
		},
		{
			name: "importprivkey optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("importprivkey", "abc", "label")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewImportPrivKeyCmd("abc", jaxjson.String("label"), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"importprivkey","params":["abc","label"],"id":1}`,
			unmarshalled: &jaxjson.ImportPrivKeyCmd{
				PrivKey: "abc",
				Label:   jaxjson.String("label"),
				Rescan:  jaxjson.Bool(true),
			},
		},
		{
			name: "importprivkey optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("importprivkey", "abc", "label", false)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewImportPrivKeyCmd("abc", jaxjson.String("label"), jaxjson.Bool(false))
			},
			marshalled: `{"jsonrpc":"1.0","method":"importprivkey","params":["abc","label",false],"id":1}`,
			unmarshalled: &jaxjson.ImportPrivKeyCmd{
				PrivKey: "abc",
				Label:   jaxjson.String("label"),
				Rescan:  jaxjson.Bool(false),
			},
		},
		{
			name: "keypoolrefill",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("keypoolrefill")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewKeyPoolRefillCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"keypoolrefill","params":[],"id":1}`,
			unmarshalled: &jaxjson.KeyPoolRefillCmd{
				NewSize: jaxjson.Uint(100),
			},
		},
		{
			name: "keypoolrefill optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("keypoolrefill", 200)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewKeyPoolRefillCmd(jaxjson.Uint(200))
			},
			marshalled: `{"jsonrpc":"1.0","method":"keypoolrefill","params":[200],"id":1}`,
			unmarshalled: &jaxjson.KeyPoolRefillCmd{
				NewSize: jaxjson.Uint(200),
			},
		},
		{
			name: "listaccounts",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listaccounts")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListAccountsCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listaccounts","params":[],"id":1}`,
			unmarshalled: &jaxjson.ListAccountsCmd{
				MinConf: jaxjson.Int(1),
			},
		},
		{
			name: "listaccounts optional",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listaccounts", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListAccountsCmd(jaxjson.Int(6))
			},
			marshalled: `{"jsonrpc":"1.0","method":"listaccounts","params":[6],"id":1}`,
			unmarshalled: &jaxjson.ListAccountsCmd{
				MinConf: jaxjson.Int(6),
			},
		},
		{
			name: "listaddressgroupings",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listaddressgroupings")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListAddressGroupingsCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"listaddressgroupings","params":[],"id":1}`,
			unmarshalled: &jaxjson.ListAddressGroupingsCmd{},
		},
		{
			name: "listlockunspent",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listlockunspent")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListLockUnspentCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"listlockunspent","params":[],"id":1}`,
			unmarshalled: &jaxjson.ListLockUnspentCmd{},
		},
		{
			name: "listreceivedbyaccount",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listreceivedbyaccount")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListReceivedByAccountCmd(nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaccount","params":[],"id":1}`,
			unmarshalled: &jaxjson.ListReceivedByAccountCmd{
				MinConf:          jaxjson.Int(1),
				IncludeEmpty:     jaxjson.Bool(false),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listreceivedbyaccount optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listreceivedbyaccount", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListReceivedByAccountCmd(jaxjson.Int(6), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaccount","params":[6],"id":1}`,
			unmarshalled: &jaxjson.ListReceivedByAccountCmd{
				MinConf:          jaxjson.Int(6),
				IncludeEmpty:     jaxjson.Bool(false),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listreceivedbyaccount optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listreceivedbyaccount", 6, true)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListReceivedByAccountCmd(jaxjson.Int(6), jaxjson.Bool(true), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaccount","params":[6,true],"id":1}`,
			unmarshalled: &jaxjson.ListReceivedByAccountCmd{
				MinConf:          jaxjson.Int(6),
				IncludeEmpty:     jaxjson.Bool(true),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listreceivedbyaccount optional3",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listreceivedbyaccount", 6, true, false)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListReceivedByAccountCmd(jaxjson.Int(6), jaxjson.Bool(true), jaxjson.Bool(false))
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaccount","params":[6,true,false],"id":1}`,
			unmarshalled: &jaxjson.ListReceivedByAccountCmd{
				MinConf:          jaxjson.Int(6),
				IncludeEmpty:     jaxjson.Bool(true),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listreceivedbyaddress",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listreceivedbyaddress")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListReceivedByAddressCmd(nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaddress","params":[],"id":1}`,
			unmarshalled: &jaxjson.ListReceivedByAddressCmd{
				MinConf:          jaxjson.Int(1),
				IncludeEmpty:     jaxjson.Bool(false),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listreceivedbyaddress optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listreceivedbyaddress", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListReceivedByAddressCmd(jaxjson.Int(6), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaddress","params":[6],"id":1}`,
			unmarshalled: &jaxjson.ListReceivedByAddressCmd{
				MinConf:          jaxjson.Int(6),
				IncludeEmpty:     jaxjson.Bool(false),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listreceivedbyaddress optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listreceivedbyaddress", 6, true)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListReceivedByAddressCmd(jaxjson.Int(6), jaxjson.Bool(true), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaddress","params":[6,true],"id":1}`,
			unmarshalled: &jaxjson.ListReceivedByAddressCmd{
				MinConf:          jaxjson.Int(6),
				IncludeEmpty:     jaxjson.Bool(true),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listreceivedbyaddress optional3",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listreceivedbyaddress", 6, true, false)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListReceivedByAddressCmd(jaxjson.Int(6), jaxjson.Bool(true), jaxjson.Bool(false))
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaddress","params":[6,true,false],"id":1}`,
			unmarshalled: &jaxjson.ListReceivedByAddressCmd{
				MinConf:          jaxjson.Int(6),
				IncludeEmpty:     jaxjson.Bool(true),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listsinceblock",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listsinceblock")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListSinceBlockCmd(nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listsinceblock","params":[],"id":1}`,
			unmarshalled: &jaxjson.ListSinceBlockCmd{
				BlockHash:           nil,
				TargetConfirmations: jaxjson.Int(1),
				IncludeWatchOnly:    jaxjson.Bool(false),
			},
		},
		{
			name: "listsinceblock optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listsinceblock", "123")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListSinceBlockCmd(jaxjson.String("123"), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listsinceblock","params":["123"],"id":1}`,
			unmarshalled: &jaxjson.ListSinceBlockCmd{
				BlockHash:           jaxjson.String("123"),
				TargetConfirmations: jaxjson.Int(1),
				IncludeWatchOnly:    jaxjson.Bool(false),
			},
		},
		{
			name: "listsinceblock optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listsinceblock", "123", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListSinceBlockCmd(jaxjson.String("123"), jaxjson.Int(6), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listsinceblock","params":["123",6],"id":1}`,
			unmarshalled: &jaxjson.ListSinceBlockCmd{
				BlockHash:           jaxjson.String("123"),
				TargetConfirmations: jaxjson.Int(6),
				IncludeWatchOnly:    jaxjson.Bool(false),
			},
		},
		{
			name: "listsinceblock optional3",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listsinceblock", "123", 6, true)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListSinceBlockCmd(jaxjson.String("123"), jaxjson.Int(6), jaxjson.Bool(true))
			},
			marshalled: `{"jsonrpc":"1.0","method":"listsinceblock","params":["123",6,true],"id":1}`,
			unmarshalled: &jaxjson.ListSinceBlockCmd{
				BlockHash:           jaxjson.String("123"),
				TargetConfirmations: jaxjson.Int(6),
				IncludeWatchOnly:    jaxjson.Bool(true),
			},
		},
		{
			name: "listtransactions",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listtransactions")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListTransactionsCmd(nil, nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listtransactions","params":[],"id":1}`,
			unmarshalled: &jaxjson.ListTransactionsCmd{
				Account:          nil,
				Count:            jaxjson.Int(10),
				From:             jaxjson.Int(0),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listtransactions optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listtransactions", "acct")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListTransactionsCmd(jaxjson.String("acct"), nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listtransactions","params":["acct"],"id":1}`,
			unmarshalled: &jaxjson.ListTransactionsCmd{
				Account:          jaxjson.String("acct"),
				Count:            jaxjson.Int(10),
				From:             jaxjson.Int(0),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listtransactions optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listtransactions", "acct", 20)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListTransactionsCmd(jaxjson.String("acct"), jaxjson.Int(20), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listtransactions","params":["acct",20],"id":1}`,
			unmarshalled: &jaxjson.ListTransactionsCmd{
				Account:          jaxjson.String("acct"),
				Count:            jaxjson.Int(20),
				From:             jaxjson.Int(0),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listtransactions optional3",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listtransactions", "acct", 20, 1)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListTransactionsCmd(jaxjson.String("acct"), jaxjson.Int(20),
					jaxjson.Int(1), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listtransactions","params":["acct",20,1],"id":1}`,
			unmarshalled: &jaxjson.ListTransactionsCmd{
				Account:          jaxjson.String("acct"),
				Count:            jaxjson.Int(20),
				From:             jaxjson.Int(1),
				IncludeWatchOnly: jaxjson.Bool(false),
			},
		},
		{
			name: "listtransactions optional4",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listtransactions", "acct", 20, 1, true)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListTransactionsCmd(jaxjson.String("acct"), jaxjson.Int(20),
					jaxjson.Int(1), jaxjson.Bool(true))
			},
			marshalled: `{"jsonrpc":"1.0","method":"listtransactions","params":["acct",20,1,true],"id":1}`,
			unmarshalled: &jaxjson.ListTransactionsCmd{
				Account:          jaxjson.String("acct"),
				Count:            jaxjson.Int(20),
				From:             jaxjson.Int(1),
				IncludeWatchOnly: jaxjson.Bool(true),
			},
		},
		{
			name: "listunspent",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listunspent")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListUnspentCmd(nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listunspent","params":[],"id":1}`,
			unmarshalled: &jaxjson.ListUnspentCmd{
				MinConf:   jaxjson.Int(1),
				MaxConf:   jaxjson.Int(9999999),
				Addresses: nil,
			},
		},
		{
			name: "listunspent optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listunspent", 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListUnspentCmd(jaxjson.Int(6), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listunspent","params":[6],"id":1}`,
			unmarshalled: &jaxjson.ListUnspentCmd{
				MinConf:   jaxjson.Int(6),
				MaxConf:   jaxjson.Int(9999999),
				Addresses: nil,
			},
		},
		{
			name: "listunspent optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listunspent", 6, 100)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListUnspentCmd(jaxjson.Int(6), jaxjson.Int(100), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listunspent","params":[6,100],"id":1}`,
			unmarshalled: &jaxjson.ListUnspentCmd{
				MinConf:   jaxjson.Int(6),
				MaxConf:   jaxjson.Int(100),
				Addresses: nil,
			},
		},
		{
			name: "listunspent optional3",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("listunspent", 6, 100, []string{"1Address", "1Address2"})
			},
			staticCmd: func() interface{} {
				return jaxjson.NewListUnspentCmd(jaxjson.Int(6), jaxjson.Int(100),
					&[]string{"1Address", "1Address2"})
			},
			marshalled: `{"jsonrpc":"1.0","method":"listunspent","params":[6,100,["1Address","1Address2"]],"id":1}`,
			unmarshalled: &jaxjson.ListUnspentCmd{
				MinConf:   jaxjson.Int(6),
				MaxConf:   jaxjson.Int(100),
				Addresses: &[]string{"1Address", "1Address2"},
			},
		},
		{
			name: "lockunspent",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("lockunspent", true, `[{"txid":"123","vout":1}]`)
			},
			staticCmd: func() interface{} {
				txInputs := []jaxjson.TransactionInput{
					{Txid: "123", Vout: 1},
				}
				return jaxjson.NewLockUnspentCmd(true, txInputs)
			},
			marshalled: `{"jsonrpc":"1.0","method":"lockunspent","params":[true,[{"txid":"123","vout":1}]],"id":1}`,
			unmarshalled: &jaxjson.LockUnspentCmd{
				Unlock: true,
				Transactions: []jaxjson.TransactionInput{
					{Txid: "123", Vout: 1},
				},
			},
		},
		{
			name: "move",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("move", "from", "to", 0.5)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewMoveCmd("from", "to", 0.5, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"move","params":["from","to",0.5],"id":1}`,
			unmarshalled: &jaxjson.MoveCmd{
				FromAccount: "from",
				ToAccount:   "to",
				Amount:      0.5,
				MinConf:     jaxjson.Int(1),
				Comment:     nil,
			},
		},
		{
			name: "move optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("move", "from", "to", 0.5, 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewMoveCmd("from", "to", 0.5, jaxjson.Int(6), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"move","params":["from","to",0.5,6],"id":1}`,
			unmarshalled: &jaxjson.MoveCmd{
				FromAccount: "from",
				ToAccount:   "to",
				Amount:      0.5,
				MinConf:     jaxjson.Int(6),
				Comment:     nil,
			},
		},
		{
			name: "move optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("move", "from", "to", 0.5, 6, "comment")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewMoveCmd("from", "to", 0.5, jaxjson.Int(6), jaxjson.String("comment"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"move","params":["from","to",0.5,6,"comment"],"id":1}`,
			unmarshalled: &jaxjson.MoveCmd{
				FromAccount: "from",
				ToAccount:   "to",
				Amount:      0.5,
				MinConf:     jaxjson.Int(6),
				Comment:     jaxjson.String("comment"),
			},
		},
		{
			name: "sendfrom",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendfrom", "from", "1Address", 0.5)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSendFromCmd("from", "1Address", 0.5, nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendfrom","params":["from","1Address",0.5],"id":1}`,
			unmarshalled: &jaxjson.SendFromCmd{
				FromAccount: "from",
				ToAddress:   "1Address",
				Amount:      0.5,
				MinConf:     jaxjson.Int(1),
				Comment:     nil,
				CommentTo:   nil,
			},
		},
		{
			name: "sendfrom optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendfrom", "from", "1Address", 0.5, 6)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSendFromCmd("from", "1Address", 0.5, jaxjson.Int(6), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendfrom","params":["from","1Address",0.5,6],"id":1}`,
			unmarshalled: &jaxjson.SendFromCmd{
				FromAccount: "from",
				ToAddress:   "1Address",
				Amount:      0.5,
				MinConf:     jaxjson.Int(6),
				Comment:     nil,
				CommentTo:   nil,
			},
		},
		{
			name: "sendfrom optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendfrom", "from", "1Address", 0.5, 6, "comment")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSendFromCmd("from", "1Address", 0.5, jaxjson.Int(6),
					jaxjson.String("comment"), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendfrom","params":["from","1Address",0.5,6,"comment"],"id":1}`,
			unmarshalled: &jaxjson.SendFromCmd{
				FromAccount: "from",
				ToAddress:   "1Address",
				Amount:      0.5,
				MinConf:     jaxjson.Int(6),
				Comment:     jaxjson.String("comment"),
				CommentTo:   nil,
			},
		},
		{
			name: "sendfrom optional3",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendfrom", "from", "1Address", 0.5, 6, "comment", "commentto")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSendFromCmd("from", "1Address", 0.5, jaxjson.Int(6),
					jaxjson.String("comment"), jaxjson.String("commentto"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendfrom","params":["from","1Address",0.5,6,"comment","commentto"],"id":1}`,
			unmarshalled: &jaxjson.SendFromCmd{
				FromAccount: "from",
				ToAddress:   "1Address",
				Amount:      0.5,
				MinConf:     jaxjson.Int(6),
				Comment:     jaxjson.String("comment"),
				CommentTo:   jaxjson.String("commentto"),
			},
		},
		{
			name: "sendmany",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendmany", "from", `{"1Address":0.5}`)
			},
			staticCmd: func() interface{} {
				amounts := map[string]float64{"1Address": 0.5}
				return jaxjson.NewSendManyCmd("from", amounts, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendmany","params":["from",{"1Address":0.5}],"id":1}`,
			unmarshalled: &jaxjson.SendManyCmd{
				FromAccount: "from",
				Amounts:     map[string]float64{"1Address": 0.5},
				MinConf:     jaxjson.Int(1),
				Comment:     nil,
			},
		},
		{
			name: "sendmany optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendmany", "from", `{"1Address":0.5}`, 6)
			},
			staticCmd: func() interface{} {
				amounts := map[string]float64{"1Address": 0.5}
				return jaxjson.NewSendManyCmd("from", amounts, jaxjson.Int(6), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendmany","params":["from",{"1Address":0.5},6],"id":1}`,
			unmarshalled: &jaxjson.SendManyCmd{
				FromAccount: "from",
				Amounts:     map[string]float64{"1Address": 0.5},
				MinConf:     jaxjson.Int(6),
				Comment:     nil,
			},
		},
		{
			name: "sendmany optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendmany", "from", `{"1Address":0.5}`, 6, "comment")
			},
			staticCmd: func() interface{} {
				amounts := map[string]float64{"1Address": 0.5}
				return jaxjson.NewSendManyCmd("from", amounts, jaxjson.Int(6), jaxjson.String("comment"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendmany","params":["from",{"1Address":0.5},6,"comment"],"id":1}`,
			unmarshalled: &jaxjson.SendManyCmd{
				FromAccount: "from",
				Amounts:     map[string]float64{"1Address": 0.5},
				MinConf:     jaxjson.Int(6),
				Comment:     jaxjson.String("comment"),
			},
		},
		{
			name: "sendtoaddress",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendtoaddress", "1Address", 0.5)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSendToAddressCmd("1Address", 0.5, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendtoaddress","params":["1Address",0.5],"id":1}`,
			unmarshalled: &jaxjson.SendToAddressCmd{
				Address:   "1Address",
				Amount:    0.5,
				Comment:   nil,
				CommentTo: nil,
			},
		},
		{
			name: "sendtoaddress optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("sendtoaddress", "1Address", 0.5, "comment", "commentto")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSendToAddressCmd("1Address", 0.5, jaxjson.String("comment"),
					jaxjson.String("commentto"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendtoaddress","params":["1Address",0.5,"comment","commentto"],"id":1}`,
			unmarshalled: &jaxjson.SendToAddressCmd{
				Address:   "1Address",
				Amount:    0.5,
				Comment:   jaxjson.String("comment"),
				CommentTo: jaxjson.String("commentto"),
			},
		},
		{
			name: "setaccount",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("setaccount", "1Address", "acct")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSetAccountCmd("1Address", "acct")
			},
			marshalled: `{"jsonrpc":"1.0","method":"setaccount","params":["1Address","acct"],"id":1}`,
			unmarshalled: &jaxjson.SetAccountCmd{
				Address: "1Address",
				Account: "acct",
			},
		},
		{
			name: "settxfee",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("settxfee", 0.0001)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSetTxFeeCmd(0.0001)
			},
			marshalled: `{"jsonrpc":"1.0","method":"settxfee","params":[0.0001],"id":1}`,
			unmarshalled: &jaxjson.SetTxFeeCmd{
				Amount: 0.0001,
			},
		},
		{
			name: "signmessage",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("signmessage", "1Address", "message")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSignMessageCmd("1Address", "message")
			},
			marshalled: `{"jsonrpc":"1.0","method":"signmessage","params":["1Address","message"],"id":1}`,
			unmarshalled: &jaxjson.SignMessageCmd{
				Address: "1Address",
				Message: "message",
			},
		},
		{
			name: "signrawtransaction",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("signrawtransaction", "001122")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewSignRawTransactionCmd("001122", nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"signrawtransaction","params":["001122"],"id":1}`,
			unmarshalled: &jaxjson.SignRawTransactionCmd{
				RawTx:    "001122",
				Inputs:   nil,
				PrivKeys: nil,
				Flags:    jaxjson.String("ALL"),
			},
		},
		{
			name: "signrawtransaction optional1",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("signrawtransaction", "001122", `[{"txid":"123","vout":1,"scriptPubKey":"00","redeemScript":"01"}]`)
			},
			staticCmd: func() interface{} {
				txInputs := []jaxjson.RawTxInput{
					{
						Txid:         "123",
						Vout:         1,
						ScriptPubKey: "00",
						RedeemScript: "01",
					},
				}

				return jaxjson.NewSignRawTransactionCmd("001122", &txInputs, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"signrawtransaction","params":["001122",[{"txid":"123","vout":1,"scriptPubKey":"00","redeemScript":"01"}]],"id":1}`,
			unmarshalled: &jaxjson.SignRawTransactionCmd{
				RawTx: "001122",
				Inputs: &[]jaxjson.RawTxInput{
					{
						Txid:         "123",
						Vout:         1,
						ScriptPubKey: "00",
						RedeemScript: "01",
					},
				},
				PrivKeys: nil,
				Flags:    jaxjson.String("ALL"),
			},
		},
		{
			name: "signrawtransaction optional2",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("signrawtransaction", "001122", `[]`, `["abc"]`)
			},
			staticCmd: func() interface{} {
				txInputs := []jaxjson.RawTxInput{}
				privKeys := []string{"abc"}
				return jaxjson.NewSignRawTransactionCmd("001122", &txInputs, &privKeys, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"signrawtransaction","params":["001122",[],["abc"]],"id":1}`,
			unmarshalled: &jaxjson.SignRawTransactionCmd{
				RawTx:    "001122",
				Inputs:   &[]jaxjson.RawTxInput{},
				PrivKeys: &[]string{"abc"},
				Flags:    jaxjson.String("ALL"),
			},
		},
		{
			name: "signrawtransaction optional3",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("signrawtransaction", "001122", `[]`, `[]`, "ALL")
			},
			staticCmd: func() interface{} {
				txInputs := []jaxjson.RawTxInput{}
				privKeys := []string{}
				return jaxjson.NewSignRawTransactionCmd("001122", &txInputs, &privKeys,
					jaxjson.String("ALL"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"signrawtransaction","params":["001122",[],[],"ALL"],"id":1}`,
			unmarshalled: &jaxjson.SignRawTransactionCmd{
				RawTx:    "001122",
				Inputs:   &[]jaxjson.RawTxInput{},
				PrivKeys: &[]string{},
				Flags:    jaxjson.String("ALL"),
			},
		},
		{
			name: "walletlock",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("walletlock")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewWalletLockCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"walletlock","params":[],"id":1}`,
			unmarshalled: &jaxjson.WalletLockCmd{},
		},
		{
			name: "walletpassphrase",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("walletpassphrase", "pass", 60)
			},
			staticCmd: func() interface{} {
				return jaxjson.NewWalletPassphraseCmd("pass", 60)
			},
			marshalled: `{"jsonrpc":"1.0","method":"walletpassphrase","params":["pass",60],"id":1}`,
			unmarshalled: &jaxjson.WalletPassphraseCmd{
				Passphrase: "pass",
				Timeout:    60,
			},
		},
		{
			name: "walletpassphrasechange",
			newCmd: func() (interface{}, error) {
				return jaxjson.NewCmd("walletpassphrasechange", "old", "new")
			},
			staticCmd: func() interface{} {
				return jaxjson.NewWalletPassphraseChangeCmd("old", "new")
			},
			marshalled: `{"jsonrpc":"1.0","method":"walletpassphrasechange","params":["old","new"],"id":1}`,
			unmarshalled: &jaxjson.WalletPassphraseChangeCmd{
				OldPassphrase: "old",
				NewPassphrase: "new",
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
