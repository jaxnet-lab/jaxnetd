package chaincfg

import (
	"math"
	"math/big"
	"time"

	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/types"
)

// testNet3PowLimit is the highest proof of work value a Bitcoin block
// can have for the test network (version 3).  It is the value
// 2^224 - 1.
var testNet3PowLimit = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 240), bigOne)

// testNet3GenesisHash is the hash of the first block in the block chain for the
// test network (version 3).
var testNet3GenesisHash = chainhash.Hash([chainhash.HashSize]byte{ // Make go vet happy.
	0x54, 0x22, 0x34, 0xe1, 0x91, 0xd3, 0xdb, 0xed,
	0x04, 0xdc, 0x01, 0xaa, 0x2c, 0x79, 0x29, 0xf6,
	0x19, 0xed, 0xd4, 0x2f, 0x70, 0x6e, 0xb1, 0x45,
	0xde, 0xb6, 0x20, 0xb6, 0x93, 0xae, 0x19, 0x26,
})

//
// // testNet3GenesisMerkleRoot is the hash of the first transaction in the genesis
// // block for the test network (version 3).  It is the same as the merkle root
// // for the main network.
// var testNet3GenesisMerkleRoot = genesisMerkleRoot
//
// // testNet3GenesisBlock defines the genesis block of the block chain which
// // serves as the public transaction ledger for the test network (version 3).
// var testNet3GenesisBlock = wire.MsgBlock{
// 	Header: shard.NewBlockHeader(
// 		1,
// 		chainhash.Hash{},          // 0000000000000000000000000000000000000000000000000000000000000000
// 		testNet3GenesisMerkleRoot, // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
// 		chainhash.Hash{},
// 		time.Unix(1296688602, 0), // 2011-02-02 23:16:42 +0000 UTC
// 		0x1e0fffff,               // 486604799 [00000000ffff0000000000000000000000000000000000000000000000000000]
// 		// 0x1d00ffff,                // 486604799 [00000000ffff0000000000000000000000000000000000000000000000000000]
// 		0x18aea41a, // 414098458
// 	),
// 	Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
// }

// TestNet3Params defines the network parameters for the test Bitcoin network
// (version 3).  Not to be confused with the regression test network, this
// network is sometimes simply called "testnet".
var TestNet3Params = Params{
	Name:        "testnet3",
	Net:         types.TestNet3,
	DefaultPort: "18333",
	DNSSeeds:    []DNSSeed{},

	// Chain parameters
	// GenesisBlock: &testNet3GenesisBlock,
	GenesisHash: &testNet3GenesisHash,
	PowLimit:    testNet3PowLimit,
	// PowLimitBits:             0x1d00ffff,
	PowLimitBits: 0x1e0fffff,
	// PowLimitBits:             0x207fffff,
	BIP0034Height:            0,
	BIP0065Height:            0,
	BIP0066Height:            0,
	CoinbaseMaturity:         100,
	SubsidyReductionInterval: 210000,

	TargetTimespan:     time.Second * 60 * 60 * 24,
	TargetTimePerBlock: time.Second * 15,
	// TargetTimespan:           time.Hour * 24 * 14, // 14 days
	// TargetTimePerBlock:       time.Minute * 10,    // 10 minutes
	RetargetAdjustmentFactor: 4, // 25% less, 400% more
	ReduceMinDifficulty:      true,
	MinDiffReductionTime:     time.Second * 30, // TargetTimePerBlock * 2
	GenerateSupported:        true,

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
		DeploymentTestDummy: {
			BitNumber:  28,
			StartTime:  0,             // Always available for vote
			ExpireTime: math.MaxInt64, // Never expires
		},
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

	// BIP32 hierarchical deterministic extended key magics
	HDPrivateKeyID: [4]byte{0x04, 0x35, 0x83, 0x94}, // starts with tprv
	HDPublicKeyID:  [4]byte{0x04, 0x35, 0x87, 0xcf}, // starts with tpub

	// BIP44 coin type used in the hierarchical deterministic path for
	// address generation.
	HDCoinType: 1,
}
