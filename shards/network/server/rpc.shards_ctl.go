package server

import (
	"errors"

	"gitlab.com/jaxnet/core/shard.core.git/btcjson"
)

// handleAddNode handles addnode commands.
func (s *RPCServer) handleManageShards(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.ManageShardsCmd)

	var err error
	switch c.Action {
	case "new":
		if c.InitialHeight == nil {
			err = errors.New("initialHeight must be provided")
			break
		}
		err = s.node.ShardsMgr.NewShard(c.ShardID, *c.InitialHeight)
	case "stop":
		err = s.node.ShardsMgr.DisableShard(c.ShardID)
	case "run":
		err = s.node.ShardsMgr.EnableShard(c.ShardID)
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

func (s *RPCServer) handleListShards(cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	shards := s.node.ShardsMgr.ListShards()
	// no data returned unless an error.
	return shards, nil
}
