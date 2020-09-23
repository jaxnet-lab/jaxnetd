package server

import (
	"gitlab.com/jaxnet/core/shard.core.git/mempool"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
	"gitlab.com/jaxnet/core/shard.core.git/shards/types"
)

// RPCConnManager provides a connection manager for use with the RPC Server and
// implements the rpcserverConnManager interface.
type RPCConnManager struct {
	Server *server
}

// Ensure RPCConnManager implements the rpcserverConnManager interface.
var _ rpcserverConnManager = &RPCConnManager{}

// Connect adds the provided address as a new outbound peer.  The permanent flag
// indicates whether or not to make the peer persistent and reconnect if the
// connection is lost.  Attempting to connect to an already existing peer will
// return an error.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) Connect(addr string, permanent bool) error {
	replyChan := make(chan error)
	cm.Server.Query(connectNodeMsg{
		addr:      addr,
		permanent: permanent,
		reply:     replyChan,
	})
	return <-replyChan
}

// RemoveByID removes the peer associated with the provided id from the list of
// persistent peers.  Attempting to remove an id that does not exist will return
// an error.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) RemoveByID(id int32) error {
	replyChan := make(chan error)
	cm.Server.query <- removeNodeMsg{
		cmp:   func(sp *serverPeer) bool { return sp.ID() == id },
		reply: replyChan,
	}
	return <-replyChan
}

// RemoveByAddr removes the peer associated with the provided address from the
// list of persistent peers.  Attempting to remove an address that does not
// exist will return an error.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) RemoveByAddr(addr string) error {
	replyChan := make(chan error)
	cm.Server.query <- removeNodeMsg{
		cmp:   func(sp *serverPeer) bool { return sp.Addr() == addr },
		reply: replyChan,
	}
	return <-replyChan
}

// DisconnectByID disconnects the peer associated with the provided id.  This
// applies to both inbound and outbound peers.  Attempting to remove an id that
// does not exist will return an error.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) DisconnectByID(id int32) error {
	replyChan := make(chan error)
	cm.Server.query <- disconnectNodeMsg{
		cmp:   func(sp *serverPeer) bool { return sp.ID() == id },
		reply: replyChan,
	}
	return <-replyChan
}

// DisconnectByAddr disconnects the peer associated with the provided address.
// This applies to both inbound and outbound peers.  Attempting to remove an
// address that does not exist will return an error.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) DisconnectByAddr(addr string) error {
	replyChan := make(chan error)
	cm.Server.query <- disconnectNodeMsg{
		cmp:   func(sp *serverPeer) bool { return sp.Addr() == addr },
		reply: replyChan,
	}
	return <-replyChan
}

// ConnectedCount returns the number of currently connected peers.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) ConnectedCount() int32 {
	return cm.Server.ConnectedCount()
}

// NetTotals returns the sum of all bytes received and sent across the network
// for all peers.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) NetTotals() (uint64, uint64) {
	return cm.Server.NetTotals()
}

// ConnectedPeers returns an array consisting of all connected peers.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) ConnectedPeers() []rpcserverPeer {
	replyChan := make(chan []*serverPeer)
	cm.Server.query <- getPeersMsg{reply: replyChan}
	serverPeers := <-replyChan

	// Convert to RPC Server peers.
	peers := make([]rpcserverPeer, 0, len(serverPeers))
	for _, sp := range serverPeers {
		peers = append(peers, (*rpcPeer)(sp))
	}
	return peers
}

// PersistentPeers returns an array consisting of all the added persistent
// peers.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) PersistentPeers() []rpcserverPeer {
	replyChan := make(chan []*serverPeer)
	cm.Server.query <- getAddedNodesMsg{reply: replyChan}
	serverPeers := <-replyChan

	// Convert to generic peers.
	peers := make([]rpcserverPeer, 0, len(serverPeers))
	for _, sp := range serverPeers {
		peers = append(peers, (*rpcPeer)(sp))
	}
	return peers
}

// BroadcastMessage sends the provided message to all currently connected peers.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) BroadcastMessage(msg wire.Message) {
	cm.Server.BroadcastMessage(msg)
}

// AddRebroadcastInventory adds the provided inventory to the list of
// inventories to be rebroadcast at random intervals until they show up in a
// block.
//
// This function is safe for concurrent access and is part of the
// rpcserverConnManager interface implementation.
func (cm *RPCConnManager) AddRebroadcastInventory(iv *types.InvVect, data interface{}) {
	cm.Server.AddRebroadcastInventory(iv, data)
}

// RelayTransactions generates and relays inventory vectors for all of the
// passed transactions to all connected peers.
func (cm *RPCConnManager) RelayTransactions(txns []*mempool.TxDesc) {
	cm.Server.relayTransactions(txns)
}
