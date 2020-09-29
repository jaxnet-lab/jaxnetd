package params

import (
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chaincfg"
)

// params is used to group parameters for various networks such as the main
// network and test networks.
type NetParams struct {
	*chaincfg.Params
	rpcPort string
}
