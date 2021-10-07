// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"strconv"
	"strings"
)

// ServiceFlag identifies services supported by a bitcoin server.
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

// JaxNet represents which JAX network a message belongs to.
type JaxNet uint32

// Constants used to indicate the message bitcoin network.  They can also be
// used to seek to the next message when a stream's state is unknown, but
// this package does not provide that functionality since it's generally a
// better idea to simply disconnect clients that are misbehaving over TCP.
const (
	// MainNet represents the main jax network.
	MainNet JaxNet = 0x6a_61_78_64

	// TestNet represents the test network.
	// TestNet JaxNet = 0x0709110b
	TestNet JaxNet = 0x76_6e_64_6d

	// SimNet represents the simulation test network.
	SimNet JaxNet = 0x12141c16

	// FastTestNet represents the development jax network.
	FastTestNet JaxNet = 0x12121212
)

// bnStrings is a map of bitcoin networks back to their constant names for
// pretty printing.
var bnStrings = map[JaxNet]string{
	MainNet:     "MainNet",
	TestNet:     "TestNet",
	SimNet:      "SimNet",
	FastTestNet: "FastTestNet",
}

// String returns the JaxNet in human-readable form.
func (n JaxNet) String() string {
	if s, ok := bnStrings[n]; ok {
		return s
	}

	return fmt.Sprintf("Unknown JaxNet (%d)", uint32(n))
}
