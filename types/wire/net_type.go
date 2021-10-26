// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
)

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
