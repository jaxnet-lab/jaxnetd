// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"strconv"
	"strings"
)

// todo(mike): we will probably need to bump this.
const (
	_                      = iota
	BaseJaxProtocol uint32 = iota

	// ProtocolVersion is the latest protocol version this package supports.
	ProtocolVersion = BaseJaxProtocol

// All features of bitcoin enabled from start.
//
// 	// MultipleAddressVersion is the protocol version which added multiple
// 	// addresses per message (pver >= MultipleAddressVersion).
// 	MultipleAddressVersion uint32 = 209
//
// 	// NetAddressTimeVersion is the protocol version which added the
// 	// timestamp field (pver >= NetAddressTimeVersion).
// 	NetAddressTimeVersion uint32 = 31402
//
// 	// BIP0031Version is the protocol version AFTER which a pong message
// 	// and nonce field in ping were added (pver > BIP0031Version).
// 	BIP0031Version uint32 = 60000
//
// 	// BIP0035Version is the protocol version which added the mempool
// 	// message (pver >= BIP0035Version).
// 	BIP0035Version uint32 = 60002
//
// 	// BIP0037Version is the protocol version which added new connection
// 	// bloom filtering related messages and extended the version message
// 	// with a relay flag (pver >= BIP0037Version).
// 	BIP0037Version uint32 = 70001
//
// 	// RejectVersion is the protocol version which added a new reject
// 	// message.
// 	RejectVersion uint32 = 70002
//
// 	// BIP0111Version is the protocol version which added the SFNodeBloom
// 	// service flag.
// 	BIP0111Version uint32 = 70011
//
// 	// SendHeadersVersion is the protocol version which added a new
// 	// sendheaders message.
// 	SendHeadersVersion uint32 = 70012
//
// 	// FeeFilterVersion is the protocol version which added a new
// 	// feefilter message.
// 	FeeFilterVersion uint32 = 70013
)

// ServiceFlag identifies services supported by a jaxnet server.
type ServiceFlag uint64

const (
	// SFNodeNetwork is a flag used to indicate a server is a full node.
	SFNodeNetwork ServiceFlag = 1 << iota

	// SFNodeGetUTXO is a flag used to indicate a server supports the
	// getutxos and utxos commands (BIP0064).
	SFNodeGetUTXO

	// SFNodeBloom is a flag used to indicate a server supports bloom
	// filtering.
	SFNodeBloom

	// SFNodeWitness is a flag used to indicate a server supports blocks
	// and transactions including witness data (BIP0144).
	SFNodeWitness

	// SFNodeXthin is a flag used to indicate a server supports xthin blocks.
	SFNodeXthin

	// SFNodeBit5 is a flag used to indicate a server supports a service
	// defined by bit 5.
	SFNodeBit5

	// SFNodeCF is a flag used to indicate a server supports committed
	// filters (CFs).
	SFNodeCF

	// SFNode2X is a flag used to indicate a server is running the Segwit2X
	// software.
	SFNode2X
)

// Map of service flags back to their constant names for pretty printing.
var sfStrings = map[ServiceFlag]string{
	SFNodeNetwork: "SFNodeNetwork",
	SFNodeGetUTXO: "SFNodeGetUTXO",
	SFNodeBloom:   "SFNodeBloom",
	SFNodeWitness: "SFNodeWitness",
	SFNodeXthin:   "SFNodeXthin",
	SFNodeBit5:    "SFNodeBit5",
	SFNodeCF:      "SFNodeCF",
	SFNode2X:      "SFNode2X",
}

// orderedSFStrings is an ordered list of service flags from highest to
// lowest.
var orderedSFStrings = []ServiceFlag{
	SFNodeNetwork,
	SFNodeGetUTXO,
	SFNodeBloom,
	SFNodeWitness,
	SFNodeXthin,
	SFNodeBit5,
	SFNodeCF,
	SFNode2X,
}

// String returns the ServiceFlag in human-readable form.
func (f ServiceFlag) String() string {
	// No flags are set.
	if f == 0 {
		return "0x0"
	}

	// Add individual bit flags.
	s := ""
	for _, flag := range orderedSFStrings {
		if f&flag == flag {
			s += sfStrings[flag] + "|"
			f -= flag
		}
	}

	// Add any remaining flags which aren't accounted for as hex.
	s = strings.TrimRight(s, "|")
	if f != 0 {
		s += "|0x" + strconv.FormatUint(uint64(f), 16)
	}
	s = strings.TrimLeft(s, "|")
	return s
}
