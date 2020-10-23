// Copyright (c) 2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain"
)

type chainProvider interface {
	BlockChain() *blockchain.BlockChain
	MiningAddresses() []btcutil.Address
}
