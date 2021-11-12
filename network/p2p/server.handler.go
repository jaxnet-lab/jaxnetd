// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package p2p

import (
	"net"
	"sync/atomic"
	"time"

	"gitlab.com/jaxnet/jaxnetd/network/addrmgr"
	"gitlab.com/jaxnet/jaxnetd/network/connmgr"
	"gitlab.com/jaxnet/jaxnetd/node/mempool"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// handleUpdatePeerHeight updates the heights of all peers who were known to
// announce a block we recently accepted.
func (server *Server) handleUpdatePeerHeights(state *peerState, umsg UpdatePeerHeightsMsg) {
	state.forAllPeers(func(sp *serverPeer) {
		// The origin peer should already have the updated height.
		if sp.Peer == umsg.OriginPeer {
			return
		}

		// This is a pointer to the underlying memory which doesn't
		// change.
		latestBlkHash := sp.LastAnnouncedBlock()

		// Skip this peer if it hasn't recently announced any new blocks.
		if latestBlkHash == nil {
			return
		}

		// If the peer has recently announced a block, and this block
		// matches our newly accepted block, then update their block
		// height.
		if *latestBlkHash == *umsg.NewHash {
			sp.UpdateLastBlockHeight(umsg.NewHeight)
			sp.UpdateLastAnnouncedBlock(nil)
		}
	})
}

// handleAddPeerMsg deals with adding new peers.  It is invoked from the
// peerHandler goroutine.
func (server *Server) handleAddPeerMsg(state *peerState, sp *serverPeer) bool {
	if sp == nil || !sp.Connected() {
		return false
	}

	// Disconnect peers with unwanted user agents.
	if sp.HasUndesiredUserAgent(server.agentBlacklist, server.agentWhitelist) {
		sp.Disconnect()
		return false
	}

	// Ignore new peers if we're shutting down.
	if atomic.LoadInt32(&server.shutdown) != 0 {
		server.logger.Info().Msgf("New peer %s ignored - Server is shutting down", sp)
		sp.Disconnect()
		return false
	}

	// Disconnect banned peers.
	host, _, err := net.SplitHostPort(sp.Addr())
	if err != nil {
		server.logger.Debug().Msgf("can't split hostport %v", err)
		sp.Disconnect()
		return false
	}
	if banEnd, ok := state.banned[host]; ok {
		if time.Now().Before(banEnd) {
			server.logger.Debug().Msgf("Peer %s is banned for another %v - disconnecting",
				host, time.Until(banEnd))
			sp.Disconnect()
			return false
		}

		server.logger.Info().Msgf("Peer %s is no longer banned", host)
		delete(state.banned, host)
	}

	// TODO: Check for max peers from a single IP.

	// Limit max number of total peers.
	if state.Count() >= server.chain.Config().MaxPeers {
		server.logger.Info().Msgf("Max peers reached [%d] - disconnecting peer %s", server.chain.Config().MaxPeers, sp)
		sp.Disconnect()
		// TODO: how to handle permanent peers here?
		// they should be rescheduled.
		return false
	}

	// Add the new peer and start it.
	server.logger.Debug().Msgf("New peer %s", sp)
	if sp.Inbound() {
		state.inboundPeers[sp.ID()] = sp
	} else {
		state.outboundGroups[addrmgr.GroupKey(sp.NA())]++
		if sp.persistent {
			state.persistentPeers[sp.ID()] = sp
		} else {
			state.outboundPeers[sp.ID()] = sp
		}
	}

	// Update the address' last seen time if the peer has acknowledged
	// our version and has sent us its version as well.
	if sp.VerAckReceived() && sp.VersionKnown() && sp.NA() != nil {
		server.addrManager.Connected(sp.NA())
	}

	// Signal the sync manager this peer is a new sync candidate.
	server.chain.SyncManager.NewPeer(sp.Peer)

	// Update the address manager and request known addresses from the
	// remote peer for outbound connections. This is skipped when running on
	// the simulation test network since it is only intended to connect to
	// specified peers and actively avoids advertising and connecting to
	// discovered peers.
	if !sp.Inbound() {
		// Advertise the local address when the Server accepts incoming
		// connections and it believes itself to be close to the best
		// known tip.
		if !server.cfg.DisableListen && server.chain.SyncManager.IsCurrent() {
			// Get address that best matches.
			lna := server.addrManager.GetBestLocalAddress(sp.NA())
			if addrmgr.IsRoutable(lna) {
				// Filter addresses the peer already knows about.
				addresses := []*wire.NetAddress{lna}
				sp.pushAddrMsg(addresses)
			}
		}

		// Request known addresses if the Server address manager needs
		// more and the peer has a protocol version new enough to
		// include a timestamp with addresses.
		// hasTimestamp := sp.ProtocolVersion() >= wire.NetAddressTimeVersion
		if server.addrManager.NeedMoreAddresses() {
			sp.QueueMessage(wire.NewMsgGetAddr(), nil)
		}

		// Mark the address as a known good address.
		server.addrManager.Good(sp.NA())
	}

	return true
}

// handleDonePeerMsg deals with peers that have signalled they are done.  It is
// invoked from the peerHandler goroutine.
// nolint: gocritic
func (server *Server) handleDonePeerMsg(state *peerState, sp *serverPeer) {
	var list map[int32]*serverPeer
	if sp.persistent {
		list = state.persistentPeers
	} else if sp.Inbound() {
		list = state.inboundPeers
	} else {
		list = state.outboundPeers
	}

	if sp.Peer.RedirectRequested() && sp.Peer.NewAddress() != nil {
		newAddress := sp.Peer.NewAddress()
		server.addrManager.Replace(sp.Peer.NA(), sp.Peer.NewAddress())
		server.ConnManager.Remove(sp.connReq.ID())

		netAddr := &net.TCPAddr{
			IP:   newAddress.IP,
			Port: int(newAddress.Port),
		}
		go server.ConnManager.Connect(&connmgr.ConnReq{
			Addr:      netAddr,
			ShardID:   server.chain.ChainCtx.ShardID(),
			Permanent: sp.persistent,
		})
	} else if !sp.Inbound() {
		// Regardless of whether the peer was found in our list, we'll inform
		// our connection manager about the disconnection. This can happen if we
		// process a peer's `done` message before its `add`.
		if sp.persistent {
			server.ConnManager.Disconnect(sp.connReq.ID())
		} else {
			server.ConnManager.Remove(sp.connReq.ID())
			go server.ConnManager.NewConnReq()
		}
	}

	if _, ok := list[sp.ID()]; ok {
		if !sp.Inbound() && sp.VersionKnown() {
			state.outboundGroups[addrmgr.GroupKey(sp.NA())]--
		}
		delete(list, sp.ID())
		server.logger.Debug().Msgf("Removed peer %s", sp)
		return
	}
}

// handleBanPeerMsg deals with banning peers.  It is invoked from the
// peerHandler goroutine.
func (server *Server) handleBanPeerMsg(state *peerState, sp *serverPeer) {
	host, _, err := net.SplitHostPort(sp.Addr())
	if err != nil {
		server.logger.Debug().Msgf("can't split ban peer %s %v", sp.Addr(), err)
		return
	}
	direction := directionString(sp.Inbound())
	server.logger.Info().Msgf("Banned peer %s (%s) for %v", host, direction,
		server.cfg.BanDuration)
	state.banned[host] = time.Now().Add(server.cfg.BanDuration)
}

// directionString is a helper function that returns a string that represents
// the direction of a connection (inbound or outbound).
func directionString(inbound bool) string {
	if inbound {
		return "inbound"
	}
	return "outbound"
}

// handleRelayInvMsg deals with relaying inventory to peers that are not already
// known to have it.  It is invoked from the peerHandler goroutine.
func (server *Server) handleRelayInvMsg(state *peerState, msg RelayMsg) {
	state.forAllPeers(func(sp *serverPeer) {
		if !sp.Connected() {
			return
		}

		// If the inventory is a block and the peer prefers headers,
		// generate and send a headers message instead of an inventory
		// message.
		if msg.InvVect.Type == wire.InvTypeBlock && sp.WantsHeaders() {
			headerBox, ok := msg.Data.(wire.HeaderBox)
			if !ok {
				server.logger.Warn().
					Msgf("Underlying data for headers is not a block header")
				return
			}
			msgHeaders := wire.NewMsgHeaders()
			if err := msgHeaders.AddBlockHeader(headerBox.Header, headerBox.ActualMMRRoot); err != nil {
				server.logger.Error().Err(err).Msg("Failed to add block header")
				return
			}
			sp.QueueMessage(msgHeaders, nil)
			return
		}

		if msg.InvVect.Type == wire.InvTypeTx {
			// Don't relay the transaction to the peer when it has
			// transaction relaying disabled.
			if sp.relayTxDisabled() {
				return
			}

			txD, ok := msg.Data.(*mempool.TxDesc)
			if !ok {
				server.logger.Warn().Msgf("Underlying data for tx inv "+
					"relay is not a *mempool.TxDesc: %T", msg.Data)
				return
			}

			// Don't relay the transaction if the transaction fee-per-kb
			// is less than the peer's feefilter.
			feeFilter := atomic.LoadInt64(&sp.FeeFilter)
			if feeFilter > 0 && txD.FeePerKB < feeFilter {
				return
			}

			// Don't relay the transaction if there is a bloom
			// filter loaded and the transaction doesn't match it.
			if sp.filter.IsLoaded() {
				if !sp.filter.MatchTxAndUpdate(txD.Tx) {
					return
				}
			}
		}

		// Queue the inventory to be relayed with the next batch.
		// It will be ignored if the peer is already known to
		// have the inventory.
		sp.QueueInventory(msg.InvVect)
	})
}

// handleBroadcastMsg deals with broadcasting messages to peers.  It is invoked
// from the peerHandler goroutine.
func (server *Server) handleBroadcastMsg(state *peerState, bmsg *broadcastMsg) {
	state.forAllPeers(func(sp *serverPeer) {
		if !sp.Connected() {
			return
		}

		for _, ep := range bmsg.excludePeers {
			if sp == ep {
				return
			}
		}

		sp.QueueMessage(bmsg.message, nil)
	})
}
