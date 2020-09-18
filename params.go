// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/shards/types"
)

// activeNetParams is a pointer to the parameters specific to the
// currently active bitcoin network.
var activeNetParams = &mainNetParams

// params is used to group parameters for various networks such as the main
// network and test networks.
type params struct {
	*chaincfg.Params
	rpcPort string
}

// mainNetParams contains parameters specific to the main network
// (types.TestNet3).  NOTE: The RPC port is intentionally different than the
// reference implementation because btcd does not handle wallet requests.  The
// separate wallet process listens on the well-known port and forwards requests
// it does not handle on to btcd.  This approach allows the wallet process
// to emulate the full reference implementation RPC API.
var mainNetParams = params{
	Params:  &chaincfg.MainNetParams,
	rpcPort: "8334",
}

// regressionNetParams contains parameters specific to the regression test
// network (wire.TestNet).  NOTE: The RPC port is intentionally different
// than the reference implementation - see the mainNetParams comment for
// details.
var regressionNetParams = params{
	Params:  &chaincfg.RegressionNetParams,
	rpcPort: "18334",
}

// testNet3Params contains parameters specific to the test network (version 3)
// (types.TestNet3).  NOTE: The RPC port is intentionally different than the
// reference implementation - see the mainNetParams comment for details.
var testNet3Params = params{
	Params:  &chaincfg.TestNet3Params,
	rpcPort: "18334",
}

// simNetParams contains parameters specific to the simulation test network
// (wire.SimNet).
var simNetParams = params{
	Params:  &chaincfg.SimNetParams,
	rpcPort: "18556",
}

// netName returns the name used when referring to a bitcoin network.  At the
// time of writing, btcd currently places blocks for testnet version 3 in the
// data and log directory "testnet", which does not match the Name field of the
// chaincfg parameters.  This function can be used to override this directory
// name as "testnet" when the passed active network matches types.TestNet3.
//
// A proper upgrade to move the data and log directories for this network to
//"testnet" is planned for the future, at which point this function can be
// removed and the network parameter's name used instead.
func netName(chainParams *params) string {
	switch chainParams.Net {
	case types.TestNet3:
		return "testnet"
	default:
		return chainParams.Name
	}
}
