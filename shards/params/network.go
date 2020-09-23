package params

import (
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
)

// params is used to group parameters for various networks such as the main
// network and test networks.
type NetParams struct {
	*chain.Params
	rpcPort string
}
