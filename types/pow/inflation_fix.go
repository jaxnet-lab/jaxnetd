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

	beaconEpoch = 1 << 11
	shardEpoch  = 4 * 60 * 24
)

func BeaconEpoch(height int32) int32 {
	return (height / beaconEpoch) + 1
}

func ShardEpoch(height int32) int32 {
	return (height / shardEpoch) + 1
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

func PackRat(val *big.Rat) uint32 {
	mult, _ := new(big.Rat).SetString("10000000000000000000")
	nVal := new(big.Rat).Mul(val, mult)
	intPart := strings.Split(nVal.FloatString(18), ".")[0]

	bigVal, _ := new(big.Int).SetString(intPart, 10)
	return BigToCompact(bigVal)
}

func UnpackRat(val uint32) *big.Rat {
	nVal := CompactToBig(val)

	mult, _ := new(big.Rat).SetString("10000000000000000000")
	rVal, _ := new(big.Rat).SetString(nVal.String())
	return new(big.Rat).Quo(rVal, mult)
}

func GetDifficultyF(bits uint32) (float64, error) {
	// The minimum difficulty is the max possible proof-of-work limit bits
	// converted back to a number.  Note this is not the same as the proof of
	// work limit directly because the block difficulty is encoded in a block
	// with the compact form which loses precision.
	twoPow256 := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
	target := CompactToBig(bits)

	difficulty := new(big.Rat).SetFrac(twoPow256, target)

	outString := difficulty.FloatString(8)
	diff, err := strconv.ParseFloat(outString, 64)
	if err != nil {
		return 0, err
	}
	return diff, nil
}

func GetDifficulty(bits uint32) *big.Rat {
	twoPow180 := new(big.Int).Exp(big.NewInt(2), big.NewInt(180), nil)

	target := CompactToBig(bits)
	// D =  Target / 2^180
	return new(big.Rat).SetFrac(target, twoPow180)
}

func MultBitsAndK(bits, k uint32) float64 {
	// (Di * Ki) * jaxutil.SatoshiPerJAXCoin
	d := GetDifficulty(bits)
	k1 := UnpackRat(k)
	rewardStr := new(big.Rat).Mul(d, k1).FloatString(4)
	reward, err := strconv.ParseFloat(rewardStr, 64)
	if err != nil {
		return 20
	}
	return reward
}

func CalcShardBlockSubsidy(height int32, bits, k uint32) int64 {
	switch ShardEpoch(height) {
	case 1:
		// (Di * Ki) * jaxutil.SatoshiPerJAXCoin
		d := GetDifficulty(bits)
		k1 := UnpackRat(k)
		rewardStr := new(big.Rat).Mul(d, k1).FloatString(4)
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
