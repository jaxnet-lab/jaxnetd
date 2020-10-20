// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package p2p

import (
	"fmt"
	"time"

	"gitlab.com/jaxnet/core/shard.core/network/peer"
	"gitlab.com/jaxnet/core/shard.core/node/chaindata"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

const (
	// defaultServices describes the default services that are supported by
	// the server.
	defaultServices = wire.SFNodeNetwork | wire.SFNodeBloom |
		wire.SFNodeWitness | wire.SFNodeCF

	// defaultRequiredServices describes the default services that are
	// required to be supported by outbound peers.
	defaultRequiredServices = wire.SFNodeNetwork

	// defaultTargetOutbound is the default number of outbound peers to target.
	defaultTargetOutbound = 8

	// connectionRetryInterval is the base amount of time to wait in between
	// retries when connecting to persistent peers.  It is adjusted by the
	// number of retries such that there is a retry backoff.
	connectionRetryInterval = time.Second * 5

	defaultConfigFilename        = "shard.yaml"
	defaultDataDirname           = "data"
	defaultLogLevel              = "info"
	defaultLogDirname            = "logs"
	defaultLogFilename           = "btcd.log"
	defaultMaxPeers              = 125
	defaultBanDuration           = time.Hour * 24
	defaultBanThreshold          = 100
	defaultConnectTimeout        = time.Second * 30
	defaultMaxRPCClients         = 10
	defaultMaxRPCWebsockets      = 25
	defaultMaxRPCConcurrentReqs  = 20
	defaultDbType                = "ffldb"
	defaultFreeTxRelayLimit      = 15.0
	defaultTrickleInterval       = peer.DefaultTrickleInterval
	defaultBlockMinSize          = 0
	defaultBlockMaxSize          = 750000
	defaultBlockMinWeight        = 0
	defaultBlockMaxWeight        = 3000000
	blockMaxSizeMin              = 1000
	blockMaxSizeMax              = chaindata.MaxBlockBaseSize - 1000
	blockMaxWeightMin            = 4000
	blockMaxWeightMax            = chaindata.MaxBlockWeight - 4000
	defaultGenerate              = false
	defaultMaxOrphanTransactions = 100
	defaultSigCacheMaxSize       = 100000
	sampleConfigFilename         = "sample-btcd.yaml"
	defaultTxIndex               = false
	defaultAddrIndex             = false
)

var (
	// userAgentName is the user agent name and is used to help identify
	// ourselves to other bitcoin peers.
	userAgentName = "btcd"

	// userAgentVersion is the user agent version and is used to help
	// identify ourselves to other bitcoin peers.
	userAgentVersion = fmt.Sprintf("%d.%d.%d", 1, 0, 0)
)
