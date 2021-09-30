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
func ValidateVoteK(prevK, newK uint32) bool {
	bPrevK := CompactToBig(prevK)
	bNewK := CompactToBig(newK)
	// ( (prevK - newK) * 100 ) / prevK = deltaPercent

	// (prevK - newK) * 100 = delta
	delta := new(big.Int).Mul(new(big.Int).Sub(bPrevK, bNewK), big.NewInt(100))
	// delta / prevK
	deltaPercent := new(big.Int).Div(delta, bPrevK).Int64()
	return deltaPercent <= 1 && deltaPercent >= -3
}

func CalcKCoefficient(height int32, prevK uint32, prevTarget, currentTarget uint32) uint32 {
	if BeaconEpoch(height) == 1 {
		return PackK(new(big.Float).SetFloat64(BigK1 * SmallK1))
	}
	k := UnpackK(prevK)

	calcSmallK := func() *big.Float {
		dPrev := new(big.Float).SetInt(CalcWork(prevTarget))
		dCurrent := new(big.Float).SetInt(CalcWork(currentTarget))

		lamdaF := new(big.Float).SetFloat64(LambdaPow2)

		// k == 1
		if k.Cmp(new(big.Float).SetFloat64(1)) > 0 {
			return new(big.Float).SetFloat64(1)
		}

		// (Lambda^-2) * k^(3/2) // TODO: fix pow of k
		lK := new(big.Float).Mul(lamdaF, k)

		// D(i-1) / D(i)
		dd := new(big.Float).Quo(dPrev, dCurrent)

		// [ D(i-1) / D(i) ] < [ (Lambda^2) * k^(3/2) ]
		if dd.Cmp(lK) < 0 {
			return k
		}

		// k * (Lambda^-2)
		lamdaMin2F := new(big.Float).SetFloat64(LambdaPowMin2)
		return new(big.Float).Mul(k, lamdaMin2F)
	}

	nextSmallK := calcSmallK()
	return PackK(new(big.Float).Mul(nextSmallK, k))
}

var (
	kValMask   = new(big.Int).Exp(big.NewInt(10), big.NewInt(19), nil)
	kPrecision = new(big.Float).SetInt(kValMask)

	oneLsh64 = new(big.Int).Lsh(bigOne, 64)

	jCoefficient = new(big.Float).Quo(
		new(big.Float).SetInt64(1),
		new(big.Float).SetInt(oneLsh64),
	)
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
	// (Di * Ki) * jaxutil.SatoshiPerJAXCoin
	d := CalcWork(bits)
	k1 := UnpackK(k)
	dFloat := new(big.Float).SetInt(d)
	reward, _ := new(big.Float).Mul(dFloat, k1).Float64()
	return reward
}

// CalcShardBlockSubsidy returns reward for shard block.
// - height is block height;
// - shards is a number of shards that were mined by a miner at the time;
// - bits is current target;
// - k is inflation-fix-coefficient.
func CalcShardBlockSubsidy(shards, bits, k uint32) int64 {
	// ((Di * Ki) / n)  * jaxutil.SatoshiPerJAXCoin
	d := CalcWork(bits)
	k1 := UnpackK(k)

	if shards == 0 {
		shards = 1
	}

	dRat := new(big.Float).SetInt64(d.Int64())
	shardsN := new(big.Float).SetInt64(int64(shards))

	// (Di * Ki)
	dk := new(big.Float).Mul(dRat, k1)
	// ((Di * Ki) / n)
	reward, _ := new(big.Float).Quo(dk, shardsN).Float64()
	if reward == 0 {
		return 0
	}

	return int64(reward * 1_0000)
}
