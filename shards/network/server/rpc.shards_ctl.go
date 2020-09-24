package server

import (
	"errors"

	"gitlab.com/jaxnet/core/shard.core.git/btcjson"
)

// handleAddNode handles addnode commands.
func (server *ChainRPC) handleManageShards(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.ManageShardsCmd)

	var err error
	switch c.Action {
	case "new":
		if c.InitialHeight == nil {
			err = errors.New("initialHeight must be provided")
			break
		}
		err = server.node.ShardsMgr.NewShard(c.ShardID, *c.InitialHeight)
	case "stop":
		err = server.node.ShardsMgr.DisableShard(c.ShardID)
	case "run":
		err = server.node.ShardsMgr.EnableShard(c.ShardID)
	default:
		err = errors.New("invalid actions for manageshards")
	}

	if err != nil {
		return nil, &btcjson.RPCError{
			Code:    btcjson.ErrRPCInvalidParameter,
			Message: err.Error(),
		}
	}

	// no data returned unless an error.
	return nil, nil
}

func (server *ChainRPC) handleListShards(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	shards := server.node.ShardsMgr.ListShards()
	// no data returned unless an error.
	return shards, nil
}
