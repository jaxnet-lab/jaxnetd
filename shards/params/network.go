package params

import (
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
)

// params is used to group parameters for various networks such as the main
// network and test networks.
type NetParams struct {
	*chaincfg.Params
	rpcPort string
}

var JaxNetParams = NetParams{
	Params:  &chaincfg.JaxNetParams,
	rpcPort: "8334",
}
