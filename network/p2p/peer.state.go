// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package p2p

import "time"

// peerState maintains state of inbound, persistent, outbound peers as well
// as banned peers and outbound groups.
type peerState struct {
	// todo(mike)
	inboundPeers    map[int32]*serverPeer
	outboundPeers   map[int32]*serverPeer
	persistentPeers map[int32]*serverPeer
	banned          map[string]time.Time
	outboundGroups  map[string]int
}

// Count returns the count of all known peers.
func (ps *peerState) Count() int {
	return len(ps.inboundPeers) + len(ps.outboundPeers) +
		len(ps.persistentPeers)
}

// forAllOutboundPeers is a helper function that runs closure on all outbound
// peers known to peerState.
func (ps *peerState) forAllOutboundPeers(closure func(sp *serverPeer)) {
	for _, e := range ps.outboundPeers {
		closure(e)
	}
	for _, e := range ps.persistentPeers {
		closure(e)
	}
}

// forAllPeers is a helper function that runs closure on all peers known to
// peerState.
func (ps *peerState) forAllPeers(closure func(sp *serverPeer)) {
	for _, e := range ps.inboundPeers {
		closure(e)
	}
	ps.forAllOutboundPeers(closure)
}

func (ps *peerState) Stats() PeerStateStats {
	return PeerStateStats{
		Total:           len(ps.inboundPeers) + len(ps.outboundPeers) + len(ps.persistentPeers),
		InboundPeers:    len(ps.inboundPeers),
		OutboundPeers:   len(ps.outboundPeers),
		PersistentPeers: len(ps.persistentPeers),
		Banned:          len(ps.banned),
		OutboundGroups:  len(ps.outboundGroups),
	}
}

type PeerStateStats struct {
	Total           int
	InboundPeers    int
	OutboundPeers   int
	PersistentPeers int
	Banned          int
	OutboundGroups  int
}
