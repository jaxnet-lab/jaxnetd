// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"gitlab.com/jaxnet/core/shard.core.git/btcwallet/jaxnet"
	"gitlab.com/jaxnet/core/shard.core.git/btcwallet/netparams"
)

var activeNet = &JaxNetParams

// MainNetParams contains parameters specific running btcwallet and
// btcd on the main network (wire.MainNet).
var JaxNetParams = netparams.Params{
	Params:        &jaxnet.JaxNetParams,
	RPCClientPort: "8334",
	RPCServerPort: "8332",
}
