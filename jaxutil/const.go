// Copyright (c) 2013-2014 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package jaxutil

const (
	// SatoshiPerBitcoin is the number of satoshi in one bitcoin (1 BTC).
	SatoshiPerBitcoin = 1e8

	// MaxSatoshi is the maximum transaction amount allowed in satoshi.
	MaxSatoshi = 21e6 * SatoshiPerBitcoin

	// HaberStornettaPerJAXNETCoin is the number of satoshi in one Beacon Chain coin (1 JAXNET).
	HaberStornettaPerJAXNETCoin = 1e8

	// JuroPerJAXCoin is the number of satoshi in one Shard Chain coin (1 JAX).
	JuroPerJAXCoin = 1e4
)
