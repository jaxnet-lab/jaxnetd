/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package pow

import (
	"math/big"
)

const (
	// Lambda is λ = exp(ln(0.8)/144) = 0.99845159 is some parameter that is determined
	// based on our estimates for technological progress in mining hardware.
	Lambda        = 0.99845159  // λ
	LambdaPow2    = 0.996905578 // λ^2
	LambdaPow4    = 0.993820731 // λ^4
	LambdaPow12   = 0.981576506 // λ^12
	LambdaPowMin2 = 1.003104028 // λ^-2

	// M is a number of blocks on SC that correspond to the one block on BC.
	// In other words, in JaxNet blocks on SC are set to be less difficult than blocks BC.
	// On average, blocks on any SC are generated M times more often than on BC.
	M = int64(40)

	// L is a length of the mining epoch on BC.So for m > 0,
	// m-th mining epoch starts with a BC block with index [(m−1)·L+1] and
	// ends with a block [m·L]. Genesis block has an index 0. It belongs to any 0-th epoch.
	// The reward the genesis block is not determined by this algorithm.
	// Subsequent blocks on BC are indexed in ascending order with a step 1.
	L = int64(4096)
	// LM is a length of the mining epoch on any SC.
	LM = L * M

	// SmallK1 is supplementary reward coefficient for the first mining epoch.
	// SmallK1 = Lambda ^ 12
	SmallK1 = LambdaPow12
	// BigK1 is inflation coefficient for the first mining epoch.
	BigK1 = 3.552713678800501e-15 // 2^−48

	DifficultyBeaconEpochLen = 2048
	KBeaconEpochLen          = 4096
	ShardEpochLen            = 16 * 4096
)

// K1 = 1 / (1 << 60 ) == 2^-60
var K1 = new(big.Float).Quo(
	new(big.Float).SetFloat64(1),
	new(big.Float).SetInt(oneLsh60),
)

func BeaconEpoch(height int32) int32 {
	return (height / KBeaconEpochLen) + 1
}

func ShardEpoch(height int32) int32 {
	return (height / ShardEpochLen) + 1
}

// nolint
var (
	// todo: review this
	kValMask   = new(big.Int).Lsh(bigOne, 1000) // 1 << 100 == 2^100
	kPrecision = new(big.Float).SetInt(kValMask)

	//	oneLsh64 = new(big.Int).Lsh(bigOne, 64)
	oneLsh60 = new(big.Int).Lsh(bigOne, 60)

	//jCoefficient = new(big.Float).Quo(
	//	new(big.Float).SetInt64(1),
	//	new(big.Float).SetInt(oneLsh64),
	//)
)

func KValFloatToInt(val *big.Float) *big.Int {
	nVal := new(big.Float).Mul(val, kPrecision)
	res, _ := nVal.Int(nil)
	return res
}

func KValIntToFloat(nVal *big.Int) *big.Float {
	rVal := new(big.Float).SetInt(nVal)
	return new(big.Float).Quo(rVal, kPrecision)
}

func UnpackK(val uint32) *big.Float {
	return KValIntToFloat(CompactToBig(val))
}

func PackK(val *big.Float) uint32 {
	return BigToCompact(KValFloatToInt(val))
}

func MultBitsAndK(bits, k uint32) float64 {
	// (Di * Ki)
	d := CalcWork(bits)
	k1 := UnpackK(k)
	dFloat := new(big.Float).SetInt(d)
	reward, _ := new(big.Float).Mul(dFloat, k1).Float64()
	return reward
}
