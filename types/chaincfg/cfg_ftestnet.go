// Copyright (c) 2014-2016 The btcsuite developers
// Copyright (c) 2020-2021 The JAX.Network developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"math"
	"math/big"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// fastNetPowLimit is the highest proof of work value a Bitcoin block
// can have for the test network (version 3).  It is the value
// 2^255 - 1.
var (
	fastNetPowLimit = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 255), bigOne)

	// fastnetShardPoWBits is basic target for shard chain.
	fastnetShardPoWBits uint32 = 0x1e0dffff
)

// FastNetParams defines the network parameters for the development test network but with low PoW params.
var FastNetParams = Params{
	Name:             "fastnet",
	Net:              wire.FastTestNet,
	DefaultPort:      "18333",
	IsBeacon:         true,
	ChainID:          0,
	ChainName:        "beacon",
	DefaultP2PPort:   "18444",
	DNSSeeds:         []DNSSeed{},
	CoinbaseMaturity: 5,

	PowParams: PowParams{
		PowLimit:                 fastNetPowLimit,
		PowLimitBits:             fastnetShardPoWBits,
		TargetTimespan:           time.Second * 60 * 60 * 24, // 1 day
		TargetTimePerBlock:       time.Second * 15,           // 15 seconds
		RetargetAdjustmentFactor: 4,                          // 25% less, 400% more
		ReduceMinDifficulty:      false,
		MinDiffReductionTime:     time.Second * 30, // TargetTimePerBlock * 2
		GenerateSupported:        true,
		HashSorting:              false,
		HashSortingSlotNumber:    4096, // 2^12
	},

	// Shards Expansion policy
	AutoExpand:            true,
	InitialExpansionRule:  2,
	InitialExpansionLimit: 4,

	// Checkpoints ordered from oldest to newest.
	Checkpoints: nil,

	// Consensus rule change deployments.
	//
	// The miner confirmation window is defined as:
	//   target proof of work timespan / target proof of work spacing
	// RuleChangeActivationThreshold: 1512, // 75% of MinerConfirmationWindow
	// MinerConfirmationWindow:       2016,
	RuleChangeActivationThreshold: 75, // 75% of MinerConfirmationWindow
	MinerConfirmationWindow:       100,

	Deployments: [DefinedDeployments]ConsensusDeployment{
		DeploymentCSV: {
			BitNumber:  0,
			StartTime:  0,             // Always available for vote
			ExpireTime: math.MaxInt64, // Never expires
		},
		DeploymentSegwit: {
			BitNumber:  1,
			StartTime:  0,             // Always available for vote
			ExpireTime: math.MaxInt64, // Never expires.
		},
	},

	// Mempool parameters
	RelayNonStdTxs: true,

	// Human-readable part for Bech32 encoded segwit addresses, as defined in
	// BIP 173.
	Bech32HRPSegwit: "tb", // always tb for test net

	// Address encoding magics
	PubKeyHashAddrID:        0x6f, // starts with m or n
	ScriptHashAddrID:        0xc4, // starts with 2
	WitnessPubKeyHashAddrID: 0x03, // starts with QW
	WitnessScriptHashAddrID: 0x28, // starts with T7n
	PrivateKeyID:            0xef, // starts with 9 (uncompressed) or c (compressed)
	EADAddressID:            0xd8, // starts with B
	HTLCAddressID:           0x05, // starts with H

	// BIP32 hierarchical deterministic extended key magics
	HDPrivateKeyID: [4]byte{0x04, 0x35, 0x83, 0x94}, // starts with tprv
	HDPublicKeyID:  [4]byte{0x04, 0x35, 0x87, 0xcf}, // starts with tpub

	// BIP44 coin type used in the hierarchical deterministic path for
	// address generation.
	HDCoinType: 0x6A, // ASCII for j

}
