// Copyright (c) 2014-2016 The btcsuite developers
// Copyright (c) 2020 The JAX.Network developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"math"
	"math/big"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

var (
	// testNetPowLimit is the highest proof of work value a Bitcoin block
	// can have for the test network (version 3).  It is the value
	// 2^256 / 2^(28-12) = 2^240 - 12 bits for hash-sorting here don't present.
	mainNetPowLimitBeacon            = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 240), bigOne)
	mainNetPowLimitBitsBeacon uint32 = 0x1f01fff0 // 2^28 target=0001fff000000000000000000000000000000000000000000000000000000000

	// 2^256 / 2^(24-12) = 2^244 - 12 bits for hash-sorting here don't present.
	mainNetPowLimitShard            = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 244), bigOne)
	mainNetPowLimitBitsShard uint32 = 0x1f1ff000 // 2^24 target=001ff00000000000000000000000000000000000000000000000000000000000
)

// var mainPowLimit = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 224), bigOne)

// MainNetParams defines the network parameters for the main Bitcoin network.
var MainNetParams = Params{
	Name:           "mainnet",
	Net:            wire.MainNet,
	DefaultPort:    "8333",
	DefaultP2PPort: "8444",
	DNSSeeds:       []DNSSeed{
		// {"dnsseed.testnet.jaxdevz.space", false}
	},
	IsBeacon:         true,
	ChainID:          0,
	ChainName:        "beacon",
	CoinbaseMaturity: 100,

	PowParams: PowParams{
		PowLimit:                 mainNetPowLimitBeacon,
		PowLimitBits:             mainNetPowLimitBitsBeacon,
		TargetTimespan:           time.Hour * 24 * 14, // 14 days
		TargetTimePerBlock:       time.Minute * 10,    // 10 minutes
		RetargetAdjustmentFactor: 4,                   // 25% less, 400% more
		ReduceMinDifficulty:      false,
		MinDiffReductionTime:     0,
		GenerateSupported:        false,
		HashSorting:              true,
		HashSortingSlotNumber:    16, // 2^4
		// HashSortingSlotNumber:    4096, // 2^12
	},

	// Checkpoints ordered from oldest to newest.
	Checkpoints: []Checkpoint{},

	// Consensus rule change deployments.
	//
	// The miner confirmation window is defined as:
	//   target proof of work timespan / target proof of work spacing
	RuleChangeActivationThreshold: 1916, // 95% of MinerConfirmationWindow
	MinerConfirmationWindow:       2016, //
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
	RelayNonStdTxs: false,

	// Human-readable part for Bech32 encoded segwit addresses, as defined in
	// BIP 173.
	Bech32HRPSegwit: "bc", // always bc for main net

	// Address encoding magics
	PubKeyHashAddrID:        0x00, // starts with 1
	ScriptHashAddrID:        0x05, // starts with 3
	PrivateKeyID:            0x80, // starts with 5 (uncompressed) or K (compressed)
	WitnessPubKeyHashAddrID: 0x06, // starts with p2
	WitnessScriptHashAddrID: 0x0A, // starts with 7Xh
	EADAddressID:            0xd8, // starts with B
	HTLCAddressID:           0x05, // starts with H

	// BIP32 hierarchical deterministic extended key magics
	HDPrivateKeyID: [4]byte{0x04, 0x88, 0xad, 0xe4}, // starts with xprv
	HDPublicKeyID:  [4]byte{0x04, 0x88, 0xb2, 0x1e}, // starts with xpub

	// BIP44 coin type used in the hierarchical deterministic path for
	// address generation.
	HDCoinType: 0x4A, // ASCII for J

	// Shards Expansion policy
	AutoExpand:            true,
	InitialExpansionRule:  2,
	InitialExpansionLimit: 4,
}
