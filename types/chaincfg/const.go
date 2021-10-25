/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaincfg

const (
	// MaxCoinAmount is the maximum transaction amount allowed in satoshi.
	MaxCoinAmount = 21e6 * HaberStornettaPerJAXNETCoin // todo: review this value in future

	// HaberStornettaPerJAXNETCoin is the number of satoshi in one Beacon Chain coin (1 JAXNET).
	HaberStornettaPerJAXNETCoin = 1e8

	// JuroPerJAXCoin is the number of satoshi in one Shard Chain coin (1 JAX).
	JuroPerJAXCoin = 1e4

	// NOTE: if in code no difference is this Juro or HaberStornetta, please use `dime` naming.
	// Dime stands for decimals of coin.
)

const (
	BeaconEpochLength = 2048
	BeaconTimeDelta   = 600 // in seconds

	BeaconRewardLockPeriod = 36000 // in blocks
	BeaconBaseReward       = 20    // In JXN
	ShardTestnetBaseReward = 10    // In JAX

	ShardEpochLength = 4096
	ShardTimeDelta   = 37500 // 37.5s in milliseconds

	ExpansionEpochLength = 1024 // in blocks
)
