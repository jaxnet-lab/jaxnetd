// Copyright (c) 2013-2014 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package jaxutil

import "gitlab.com/jaxnet/jaxnetd/types/chaincfg"

const (
	// SatoshiPerBitcoin is the number of satoshi in one bitcoin (1 BTC).
	SatoshiPerBitcoin = chaincfg.SatoshiPerBitcoin

	// MaxSatoshi is the maximum transaction amount allowed in satoshi.
	MaxSatoshi = chaincfg.MaxSatoshi

	// HaberStornettaPerJAXNETCoin is the number of satoshi in one Beacon Chain coin (1 JAXNET).
	HaberStornettaPerJAXNETCoin = chaincfg.HaberStornettaPerJAXNETCoin

	// JuroPerJAXCoin is the number of satoshi in one Shard Chain coin (1 JAX).
	JuroPerJAXCoin = chaincfg.JuroPerJAXCoin
)
