// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"math"
	"math/big"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// fastNetPowLimit is the highest proof of work value a Bitcoin block
// can have for the test network (version 3).  It is the value
// 2^255 - 1.
var fastNetPowLimit = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 255), bigOne)

// FastNetParams defines the network parameters for the development test network but with low PoW params.
var FastNetParams = Params{
	Name:        "fastnet",
	Net:         types.FastTestNet,
	DefaultPort: "18333",
	DNSSeeds:    []DNSSeed{},

	// Chain parameters
	GenesisBlock: GenesisBlockOpts{
		Version:    1,
		PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
		MerkleRoot: genesisMerkleRoot,        // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
		Timestamp:  time.Unix(1296688602, 0), // 2011-02-02 23:16:42 +0000 UTC
		Bits:       0x1e0fffff,               // 486604799 [00000000ffff0000000000000000000000000000000000000000000000000000]
		Nonce:      0x18aea41a,
	},
	GenesisHash: &genesisHash,

	CoinbaseMaturity:         5,
	SubsidyReductionInterval: 210000,

	PowParams: PowParams{
		PowLimit:                 fastNetPowLimit,
		PowLimitBits:             0x1e0dffff,
		TargetTimespan:           time.Second * 60 * 60 * 24, // 1 day
		TargetTimePerBlock:       time.Second * 15,           // 15 seconds
		RetargetAdjustmentFactor: 4,                          // 25% less, 400% more
		ReduceMinDifficulty:      false,
		MinDiffReductionTime:     time.Second * 30, // TargetTimePerBlock * 2
		GenerateSupported:        true,
	},

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
	EADAddressID:            0x25, // starts with todo
	PrivateKeyID:            0xef, // starts with 9 (uncompressed) or c (compressed)

	// BIP32 hierarchical deterministic extended key magics
	HDPrivateKeyID: [4]byte{0x04, 0x35, 0x83, 0x94}, // starts with tprv
	HDPublicKeyID:  [4]byte{0x04, 0x35, 0x87, 0xcf}, // starts with tpub

	// BIP44 coin type used in the hierarchical deterministic path for
	// address generation.
	HDCoinType: 0x6A, // ASCII for j
}
