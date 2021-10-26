// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package p2p

import (
	"gitlab.com/jaxnet/jaxnetd/network/peer"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// RelayMsg packages an inventory vector along with the newly discovered
// inventory so the relay has access to that information.
type RelayMsg struct {
	InvVect *wire.InvVect
	Data    interface{}
}

// UpdatePeerHeightsMsg is a message sent from the blockmanager to the server
// after a new block has been accepted. The purpose of the message is to update
// the heights of peers that were known to announce the block before we
// connected it to the main BlockChain or recognized it as an orphan. With these
// updates, peer heights will be kept up to date, allowing for fresh data when
// selecting sync peer candidacy.
type UpdatePeerHeightsMsg struct {
	NewHash    *chainhash.Hash
	NewHeight  int32
	OriginPeer *peer.Peer
}

type GetConnCountMsg struct {
	Reply chan int32
}

type GetPeerStatsMsg struct {
	Reply chan PeerStateStats
}

type GetPeersMsg struct {
	Reply chan []*serverPeer
}

type GetOutboundGroup struct {
	Key   string
	Reply chan int
}

type GetAddedNodesMsg struct {
	Reply chan []*serverPeer
}

type DisconnectNodeMsg struct {
	Cmp   func(*serverPeer) bool
	Reply chan error
}

type ConnectNodeMsg struct {
	Addr      string
	Permanent bool
	Reply     chan error
}

type RemoveNodeMsg struct {
	Cmp   func(*serverPeer) bool
	Reply chan error
}
