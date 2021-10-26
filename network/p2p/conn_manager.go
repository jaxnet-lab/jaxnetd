// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package p2p

import (
	"gitlab.com/jaxnet/jaxnetd/network/netsync"
	"gitlab.com/jaxnet/jaxnetd/node/mempool"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// ConnManager provides a connection manager for use with the RPC Server and
// implements the P2PConnManager interface.
type ConnManager struct {
	server *Server
}

// Ensure ConnManager implements the P2PConnManager interface.
var _ netsync.P2PConnManager = &ConnManager{}

// Connect adds the provided address as a new outbound peer.  The permanent flag
// indicates whether or not to make the peer persistent and reconnect if the
// connection is lost.  Attempting to connect to an already existing peer will
// return an error.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) Connect(addr string, permanent bool) error {
	replyChan := make(chan error)
	cm.server.Query(ConnectNodeMsg{
		Addr:      addr,
		Permanent: permanent,
		Reply:     replyChan,
	})
	return <-replyChan
}

// RemoveByID removes the peer associated with the provided id from the list of
// persistent peers.  Attempting to remove an id that does not exist will return
// an error.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) RemoveByID(id int32) error {
	replyChan := make(chan error)
	cm.server.Query(RemoveNodeMsg{
		Cmp:   func(sp *serverPeer) bool { return sp.ID() == id },
		Reply: replyChan,
	})
	return <-replyChan
}

// RemoveByAddr removes the peer associated with the provided address from the
// list of persistent peers.  Attempting to remove an address that does not
// exist will return an error.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) RemoveByAddr(addr string) error {
	replyChan := make(chan error)
	cm.server.Query(RemoveNodeMsg{
		Cmp:   func(sp *serverPeer) bool { return sp.Addr() == addr },
		Reply: replyChan,
	})
	return <-replyChan
}

// DisconnectByID disconnects the peer associated with the provided id.  This
// applies to both inbound and outbound peers.  Attempting to remove an id that
// does not exist will return an error.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) DisconnectByID(id int32) error {
	replyChan := make(chan error)
	cm.server.Query(DisconnectNodeMsg{
		Cmp:   func(sp *serverPeer) bool { return sp.ID() == id },
		Reply: replyChan,
	})
	return <-replyChan
}

// DisconnectByAddr disconnects the peer associated with the provided address.
// This applies to both inbound and outbound peers.  Attempting to remove an
// address that does not exist will return an error.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) DisconnectByAddr(addr string) error {
	replyChan := make(chan error)
	cm.server.Query(DisconnectNodeMsg{
		Cmp:   func(sp *serverPeer) bool { return sp.Addr() == addr },
		Reply: replyChan,
	})
	return <-replyChan
}

// ConnectedCount returns the number of currently connected peers.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) ConnectedCount() int32 {
	return cm.server.ConnectedCount()
}

// NetTotals returns the sum of all bytes received and sent across the network
// for all peers.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) NetTotals() (uint64, uint64) {
	return cm.server.NetTotals()
}

// ConnectedPeers returns an array consisting of all connected peers.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) ConnectedPeers() []netsync.ServerPeer {
	replyChan := make(chan []*serverPeer)
	cm.server.Query(GetPeersMsg{Reply: replyChan})
	serverPeers := <-replyChan

	// Convert to RPC Server peers.
	peers := make([]netsync.ServerPeer, 0, len(serverPeers))
	for _, sp := range serverPeers {
		peers = append(peers, sp)
	}
	return peers
}

// PersistentPeers returns an array consisting of all the added persistent
// peers.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) PersistentPeers() []netsync.ServerPeer {
	replyChan := make(chan []*serverPeer)
	cm.server.Query(GetAddedNodesMsg{Reply: replyChan})
	serverPeers := <-replyChan

	// Convert to generic peers.
	peers := make([]netsync.ServerPeer, 0, len(serverPeers))
	for _, sp := range serverPeers {
		peers = append(peers, sp)
	}
	return peers
}

// BroadcastMessage sends the provided message to all currently connected peers.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) BroadcastMessage(msg wire.Message) {
	cm.server.BroadcastMessage(msg)
}

// AddRebroadcastInventory adds the provided inventory to the list of
// inventories to be rebroadcast at random intervals until they show up in a
// block.
//
// This function is safe for concurrent access and is part of the
// P2PConnManager interface implementation.
func (cm *ConnManager) AddRebroadcastInventory(iv *wire.InvVect, data interface{}) {
	cm.server.AddRebroadcastInventory(iv, data)
}

// RelayTransactions generates and relays inventory vectors for all of the
// passed transactions to all connected peers.
func (cm *ConnManager) RelayTransactions(txns []*mempool.TxDesc) {
	cm.server.RelayTransactions(txns)
}
