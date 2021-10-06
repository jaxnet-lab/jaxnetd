/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaincfg

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

const (
	BeaconEpochLength = 2048
	BeaconTimeDelta   = 600 // in seconds

	BeaconRewardLockPeriod = 36000 // in blocks
	BeaconBaseReward       = 20    // In JXN
	ShardTestnetBaseReward = 10    // In JAX

	// ShardEpochLength = 2304  // (1.6 blocks/per hour * 1 day) in blocks
	ShardEpochLength = 2048
	ShardTimeDelta   = 37500 // 37.5s in milliseconds
)
