// Copyright (c) 2014-2016 The btcsuite developers
// Copyright (c) 2020-2021 The JAX.Network developers
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

// testNet3PowLimit is the highest proof of work value a Bitcoin block
// can have for the test network (version 3).  It is the value
// 2^224 - 1.
var testNet3PowLimit = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 240), bigOne)

// TestNet3Params defines the network parameters for the test Bitcoin network
// (version 3).  Not to be confused with the regression test network, this
// network is sometimes simply called "testnet".
var TestNet3Params = Params{
	Name:           "testnet",
	Net:            types.TestNet,
	DefaultPort:    "18333",
	DefaultP2PPort: "18444",
	DNSSeeds:       []DNSSeed{{"dnsseed.testnet.jaxdevz.space", false}},

	// Chain parameters
	GenesisBlock: GenesisBlockOpts{
		Version:    1,
		PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
		MerkleRoot: genesisMerkleRoot,        // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
		Timestamp:  time.Unix(1296688602, 0), // 2011-02-02 23:16:42 +0000 UTC
		Bits:       0x1d0ffff0,               // 487587824 [0000000ffff00000000000000000000000000000000000000000000000000000]
		Nonce:      0x18aea41a,               // 414098458
	},
	GenesisHash: &genesisHash,

	CoinbaseMaturity:         10,
	SubsidyReductionInterval: 210000,

	PowParams: PowParams{
		PowLimit:                 testNet3PowLimit,
		PowLimitBits:             0x1d0ffff0,
		TargetTimespan:           time.Hour * 24 * 14, // 14 days
		TargetTimePerBlock:       time.Minute * 10,    // 10 minutes
		RetargetAdjustmentFactor: 4,                   // 25% less, 400% more
		ReduceMinDifficulty:      false,
		MinDiffReductionTime:     time.Second * 30, // TargetTimePerBlock * 2
		GenerateSupported:        true,
		HashSorting:              true,
		ChainIDCount:             4096, // 2^12
	},

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
