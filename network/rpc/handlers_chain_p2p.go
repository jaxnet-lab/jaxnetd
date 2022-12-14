// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
// nolint: forcetypeassert
package rpc

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// handleGetNetTotals implements the getnettotals command.
func (server *CommonChainRPC) handleGetNetTotals(ctx CmdCtx) (interface{}, error) {
	totalBytesRecv, totalBytesSent := server.connMgr.NetTotals()
	reply := &jaxjson.GetNetTotalsResult{
		TotalBytesRecv: totalBytesRecv,
		TotalBytesSent: totalBytesSent,
		TimeMillis:     time.Now().UTC().UnixNano() / int64(time.Millisecond),
	}
	return reply, nil
}

// handleGetConnectionCount implements the getconnectioncount command.
func (server *CommonChainRPC) handleGetConnectionCount(ctx CmdCtx) (interface{}, error) {
	return server.connMgr.ConnectedCount(), nil
}

// handleAddNode handles addnode commands.
func (server *CommonChainRPC) handleAddNode(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.AddNodeCmd)

	addr := normalizeAddress(c.Addr, server.chainProvider.ChainParams.DefaultP2PPort)
	var err error
	switch c.SubCmd {
	case "add":
		err = server.connMgr.Connect(addr, true)
	case "remove":
		err = server.connMgr.RemoveByAddr(addr)
	case "onetry":
		err = server.connMgr.Connect(addr, false)
	default:
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInvalidParameter,
			Message: "invalid subcommand for addnode",
		}
	}

	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInvalidParameter,
			Message: err.Error(),
		}
	}

	// no data returned unless an error.
	return nil, nil
}

// handleGetAddedNodeInfo handles getaddednodeinfo commands.
func (server *CommonChainRPC) handleGetAddedNodeInfo(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.GetAddedNodeInfoCmd)

	// Retrieve a list of persistent (added) peers from the Server and
	// filter the list of peers per the specified address (if any).
	peers := server.connMgr.PersistentPeers()
	if c.Node != nil {
		node := *c.Node
		found := false
		for i, peer := range peers {
			if peer.ToPeer().Addr() == node {
				peers = peers[i : i+1]
				found = true
			}
		}
		if !found {
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCClientNodeNotAdded,
				Message: "Node has not been added",
			}
		}
	}

	// Without the dns flag, the result is just a slice of the addresses as
	// strings.
	if !c.DNS {
		results := make([]string, 0, len(peers))
		for _, peer := range peers {
			results = append(results, peer.ToPeer().Addr())
		}
		return results, nil
	}

	// With the dns flag, the result is an array of JSON objects which
	// include the result of DNS lookups for each Server.
	results := make([]*jaxjson.GetAddedNodeInfoResult, 0, len(peers))
	for _, rpcPeer := range peers {
		// Set the "address" of the Server which could be an ip address
		// or a domain name.
		peer := rpcPeer.ToPeer()
		var result jaxjson.GetAddedNodeInfoResult
		result.AddedNode = peer.Addr()
		result.Connected = jaxjson.Bool(peer.Connected())

		// Split the address into host and port portions so we can do
		// a DNS lookup against the host.  When no port is specified in
		// the address, just use the address as the host.
		host, _, err := net.SplitHostPort(peer.Addr())
		if err != nil {
			host = peer.Addr()
		}

		var ipList []string
		switch {
		case net.ParseIP(host) != nil, strings.HasSuffix(host, ".onion"):
			ipList = make([]string, 1)
			ipList[0] = host
		default:
			// Do a DNS lookup for the address.  If the lookup fails, just
			// use the host.
			// ips, err := jaxnetdLookup(host)
			// if err != nil {
			//	ipList = make([]string, 1)
			//	ipList[0] = host
			//	break
			// }
			// ipList = make([]string, 0, len(ips))
			// for _, ip := range ips {
			//	ipList = append(ipList, ip.String())
			// }
		}

		// Add the addresses and connection info to the result.
		addrs := make([]jaxjson.GetAddedNodeInfoResultAddr, 0, len(ipList))
		for _, ip := range ipList {
			var addr jaxjson.GetAddedNodeInfoResultAddr
			addr.Address = ip
			addr.Connected = "false"
			if ip == host && peer.Connected() {
				addr.Connected = directionString(peer.Inbound())
			}
			addrs = append(addrs, addr)
		}
		result.Addresses = &addrs
		results = append(results, &result)
	}
	return results, nil
}

const microsecsInSecond = 1_000

// handleGetPeerInfo implements the getpeerinfo command.
func (server *CommonChainRPC) handleGetPeerInfo(ctx CmdCtx) (interface{}, error) {
	peers := server.connMgr.ConnectedPeers()
	syncPeerID := server.chainProvider.SyncManager.SyncPeerID()
	infos := make([]*jaxjson.GetPeerInfoResult, 0, len(peers))
	for _, p := range peers {
		statsSnap := p.ToPeer().StatsSnapshot()
		info := &jaxjson.GetPeerInfoResult{
			ID:             statsSnap.ID,
			Addr:           statsSnap.Addr,
			AddrLocal:      p.ToPeer().LocalAddr().String(),
			Services:       fmt.Sprintf("%08d", uint64(statsSnap.Services)),
			RelayTxes:      !p.IsTxRelayDisabled(),
			LastSend:       statsSnap.LastSend.Unix(),
			LastRecv:       statsSnap.LastRecv.Unix(),
			BytesSent:      statsSnap.BytesSent,
			BytesRecv:      statsSnap.BytesRecv,
			ConnTime:       statsSnap.ConnTime.Unix(),
			PingTime:       float64(statsSnap.LastPingMicros),
			TimeOffset:     statsSnap.TimeOffset,
			Version:        statsSnap.Version,
			SubVer:         statsSnap.UserAgent,
			Inbound:        statsSnap.Inbound,
			StartingHeight: statsSnap.StartingHeight,
			CurrentHeight:  statsSnap.LastBlock,
			BanScore:       int32(p.GetBanScore()),
			FeeFilter:      p.GetFeeFilter(),
			SyncNode:       statsSnap.ID == syncPeerID,
		}
		if p.ToPeer().LastPingNonce() != 0 {
			wait := float64(time.Since(statsSnap.LastPingTime).Nanoseconds())
			// We actually want microseconds.
			info.PingWait = wait / microsecsInSecond
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// handleNode handles chainProvider commands.
func (server *CommonChainRPC) handleNode(ctx CmdCtx) (interface{}, error) {
	c := ctx.Cmd.(*jaxjson.NodeCmd)

	var addr string
	var nodeID uint64
	var errN, err error
	params := server.chainProvider.ChainParams
	switch c.SubCmd {
	case "disconnect":
		// If we have a valid uint disconnect by beaconChain id. Otherwise,
		// attempt to disconnect by address, returning an error if a
		// valid IP address is not supplied.
		if nodeID, errN = strconv.ParseUint(c.Target, 10, 32); errN == nil {
			err = server.connMgr.DisconnectByID(int32(nodeID))
		} else {
			if _, _, errP := net.SplitHostPort(c.Target); errP == nil || net.ParseIP(c.Target) != nil {
				addr = normalizeAddress(c.Target, params.DefaultP2PPort)
				err = server.connMgr.DisconnectByAddr(addr)
			} else {
				return nil, &jaxjson.RPCError{
					Code:    jaxjson.ErrRPCInvalidParameter,
					Message: "invalid address or beaconChain ID",
				}
			}
		}
		if err != nil && peerExists(server.connMgr, addr, int32(nodeID)) {
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCMisc,
				Message: "can't disconnect a permanent Server, use remove",
			}
		}

	case "remove":
		// If we have a valid uint disconnect by beaconChain id. Otherwise,
		// attempt to disconnect by address, returning an error if a
		// valid IP address is not supplied.
		if nodeID, errN = strconv.ParseUint(c.Target, 10, 32); errN == nil {
			err = server.connMgr.RemoveByID(int32(nodeID))
		} else {
			if _, _, errP := net.SplitHostPort(c.Target); errP == nil || net.ParseIP(c.Target) != nil {
				addr = normalizeAddress(c.Target, params.DefaultP2PPort)
				err = server.connMgr.RemoveByAddr(addr)
			} else {
				return nil, &jaxjson.RPCError{
					Code:    jaxjson.ErrRPCInvalidParameter,
					Message: "invalid address or beaconChain ID",
				}
			}
		}
		if err != nil && peerExists(server.connMgr, addr, int32(nodeID)) {
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCMisc,
				Message: "can't remove a temporary Server, use disconnect",
			}
		}

	case "connect":
		addr = normalizeAddress(c.Target, params.DefaultP2PPort)

		// Default to temporary connections.
		subCmd := "temp"
		if c.ConnectSubCmd != nil {
			subCmd = *c.ConnectSubCmd
		}

		switch subCmd {
		case "perm", "temp":
			err = server.connMgr.Connect(addr, subCmd == "perm")
		default:
			return nil, &jaxjson.RPCError{
				Code:    jaxjson.ErrRPCInvalidParameter,
				Message: "invalid subcommand for beaconChain connect",
			}
		}
	default:
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInvalidParameter,
			Message: "invalid subcommand for beaconChain",
		}
	}

	if err != nil {
		return nil, &jaxjson.RPCError{
			Code:    jaxjson.ErrRPCInvalidParameter,
			Message: err.Error(),
		}
	}

	// no data returned unless an error.
	return nil, nil
}

// handlePing implements the ping command.
func (server *CommonChainRPC) handlePing(ctx CmdCtx) (interface{}, error) {
	// Ask server to ping \o_
	nonce, err := wire.RandomUint64()
	if err != nil {
		return nil, server.InternalRPCError("Not sending ping - failed to "+
			"generate nonce: "+err.Error(), "")
	}
	server.connMgr.BroadcastMessage(wire.NewMsgPing(nonce))

	return nil, nil
}
