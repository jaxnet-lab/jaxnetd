/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package pow

import (
	"math/big"
	"strconv"
	"strings"
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

	// SupplementaryK1 is supplementary reward coefficient for the first mining epoch.
	// SupplementaryK1 = Lambda ^ 12
	SupplementaryK1 = LambdaPow12
	// K1 is inflation coefficient for the first mining epoch.
	K1 = 3.552713678800501e-15 // 2^−48

	BeaconEpochLen = 2048
	ShardEpochLen  = 4 * 60 * 24
)

func BeaconEpoch(height int32) int32 {
	return (height / BeaconEpochLen) + 1
}

func ShardEpoch(height int32) int32 {
	return (height / ShardEpochLen) + 1
}

// ValidateVoteK checks that new voteK value between  between -3% TO + 1% of previous_epoch’s k_value.
//
func ValidateVoteK(prevK, newK uint32) int64 {
	bPrevK := CompactToBig(prevK)
	bNewK := CompactToBig(newK)
	// ( (prevK - newK) * 100 ) / prevK = deltaPercent

	// (prevK - newK) * 100 = delta
	delta := new(big.Int).Mul(new(big.Int).Sub(bPrevK, bNewK), big.NewInt(100))
	// delta / prevK
	deltaPercent := new(big.Int).Div(delta, bPrevK).Int64()
	return deltaPercent
}

func CalcKCoefficient(height int32, prevK uint32) uint32 {
	if BeaconEpoch(height) == 1 {
		return PackRat(new(big.Rat).SetFloat64(K1 * SupplementaryK1))
	}

	k := UnpackRat(prevK)
	//   -1 if x <  y
	//    0 if x == y
	//   +1 if x >  y
	if k.Cmp(new(big.Rat).SetFloat64(1)) >= 0 {
		return PackRat(new(big.Rat).SetFloat64(1))
	}

	return prevK
}

const kValMask = "10000000000000000000"

func KValRatToInt(val *big.Rat) *big.Int {
	mult, _ := new(big.Rat).SetString(kValMask)
	nVal := new(big.Rat).Mul(val, mult)
	intPart := strings.Split(nVal.FloatString(18), ".")[0]

	bigVal, _ := new(big.Int).SetString(intPart, 10)
	return bigVal
}

func PackRat(val *big.Rat) uint32 {
	return BigToCompact(KValRatToInt(val))
}

func KValIntToRat(nVal *big.Int) *big.Rat {
	mult, _ := new(big.Rat).SetString(kValMask)
	rVal, _ := new(big.Rat).SetString(nVal.String())
	return new(big.Rat).Quo(rVal, mult)
}

func UnpackRat(val uint32) *big.Rat {
	return KValIntToRat(CompactToBig(val))
}

func MultBitsAndK(genesisBits, bits, k uint32) float64 {
	// (Di * Ki) * jaxutil.SatoshiPerJAXCoin
	d := GetDifficulty(genesisBits, bits)
	k1 := UnpackRat(k)
	rewardStr := new(big.Rat).Mul(new(big.Rat).SetInt64(d.Int64()), k1).FloatString(4)
	reward, err := strconv.ParseFloat(rewardStr, 64)
	if err != nil {
		return 20
	}
	return reward
}

// CalcShardBlockSubsidy returns reward for shard block.
// - height is block height;
// - shards is a number of shards that were mined by a miner at the time;
// - bits is current target;
// - k is inflation-fix-coefficient.
func CalcShardBlockSubsidy(height int32, shards, genesisBits, bits, k uint32) int64 {
	switch ShardEpoch(height) {
	case 1:
		// (Di * Ki) * jaxutil.SatoshiPerJAXCoin
		d := GetDifficulty(genesisBits, bits)
		k1 := UnpackRat(k)

		// fixme
		rewardStr := new(big.Rat).Mul(new(big.Rat).SetInt64(d.Int64()), k1).FloatString(4)
		reward, err := strconv.ParseFloat(rewardStr, 64)
		if err != nil {
			return (20 * 1000) >> uint(height/210000)
		}
		return int64(reward * 1000)

	default:
		// Equivalent to: baseSubsidy / 2^(height/subsidyHalvingInterval)
		return (20 * 1000) >> uint(height/210000)
	}
}
