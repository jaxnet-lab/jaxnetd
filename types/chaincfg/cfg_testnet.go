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

var (
	// testNetPowLimit is the highest proof of work value a Bitcoin block
	// can have for the test network (version 3).  It is the value
	// 2^256 / 2^(28-12) = 2^240 - 12 bits for hash-sorting here don't present.
	testNetPowLimitBeacon            = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 241), bigOne)
	testNetPowLimitBitsBeacon uint32 = 0x1f01fff0 // 2^28 target=0001fff000000000000000000000000000000000000000000000000000000000

	// 2^256 / 2^(24-12) = 2^244 - 12 bits for hash-sorting here don't present.
	testNetPowLimitShard            = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 245), bigOne)
	testNetPowLimitBitsShard uint32 = 0x1f1ff000 // 2^24 target=001ff00000000000000000000000000000000000000000000000000000000000
)

// TestNet3Params defines the network parameters for the test Bitcoin network
// (version 3).  Not to be confused with the regression test network, this
// network is sometimes simply called "testnet".
var TestNet3Params = TestNetParams

var TestNetParams = Params{
	Name:           "testnet",
	Net:            wire.TestNet,
	DefaultPort:    "18333",
	DefaultP2PPort: "18444",
	DNSSeeds:       []DNSSeed{
		// {"dnsseed.testnet.jaxdevz.space", false}
	},
	IsBeacon:         true,
	ChainID:          0,
	ChainName:        "beacon",
	CoinbaseMaturity: 10,

	PowParams: PowParams{
		PowLimit:                 testNetPowLimitBeacon,
		PowLimitBits:             testNetPowLimitBitsBeacon,
		TargetTimespan:           time.Hour * 24 * 14, // 14 days
		TargetTimePerBlock:       time.Minute * 10,    // 10 minutes
		RetargetAdjustmentFactor: 4,                   // 25% less, 400% more
		ReduceMinDifficulty:      false,
		MinDiffReductionTime:     time.Second * 30, // TargetTimePerBlock * 2
		GenerateSupported:        true,
		HashSorting:              true,
		// HashSortingSlotNumber:    4096, // 2^12
		HashSortingSlotNumber: 16, // 2^4

	},

	// Shards Expansion policy
	AutoExpand:            false,
	InitialExpansionRule:  1,
	InitialExpansionLimit: 0,

	// Checkpoints ordered from oldest to newest.
	Checkpoints: nil,

	// Consensus rule change deployments.
	//
	// The miner confirmation window is defined as:
	//   target proof of work timespan / target proof of work spacing
	RuleChangeActivationThreshold: 1512, // 75% of MinerConfirmationWindow
	MinerConfirmationWindow:       2016,
	Deployments: [DefinedDeployments]ConsensusDeployment{
		DeploymentCSV: {
			BitNumber:  0,
			StartTime:  0,             // Always available for vote
			ExpireTime: math.MaxInt64, // Never expires
		},
		DeploymentSegwit: {
			BitNumber:  0,
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
