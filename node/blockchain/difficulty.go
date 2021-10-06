// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"math/big"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/blocknode"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
)

// calcEasiestDifficulty calculates the easiest possible difficulty that a block
// can have given starting difficulty bits and a duration.  It is mainly used to
// verify that claimed proof of work by a block is sane as compared to a
// known good checkpoint.
func (b *BlockChain) calcEasiestDifficulty(bits uint32, duration time.Duration) uint32 {
	return calcEasiestDifficulty(b.chainParams.PowParams, b.retargetOpts, bits, duration)
}

func calcEasiestDifficulty(powParams chaincfg.PowParams, opts retargetOpts, bits uint32, duration time.Duration) uint32 {
	// Convert types used in the calculations below.
	durationVal := int64(duration / time.Second)
	adjustmentFactor := big.NewInt(powParams.RetargetAdjustmentFactor)

	// The test network rules allow minimum difficulty blocks after more
	// than twice the desired amount of time needed to generate a block has
	// elapsed.
	if powParams.ReduceMinDifficulty {
		reductionTime := int64(powParams.MinDiffReductionTime / time.Second)
		if durationVal > reductionTime {
			return powParams.PowLimitBits
		}
	}

	// Since easier difficulty equates to higher numbers, the easiest
	// difficulty for a given duration is the largest value possible given
	// the number of retargets for the duration and starting difficulty
	// multiplied by the max adjustment factor.
	newTarget := pow.CompactToBig(bits)
	for durationVal > 0 && newTarget.Cmp(powParams.PowLimit) < 0 {
		newTarget.Mul(newTarget, adjustmentFactor)
		durationVal -= opts.maxRetargetTimespan
	}

	// Limit new value to the proof of work limit.
	if newTarget.Cmp(powParams.PowLimit) > 0 {
		newTarget.Set(powParams.PowLimit)
	}

	return pow.BigToCompact(newTarget)
}

// findPrevTestNetDifficulty returns the difficulty of the previous block which
// did not have the special testnet minimum difficulty rule applied.
//
// This function MUST be called with the chain state lock held (for writes).
func findPrevTestNetDifficulty(startNode blocknode.IBlockNode, blocksPerRetarget int32, powLimitBits uint32) uint32 {
	// Search backwards through the chain for the last block without
	// the special rule applied.
	iterNode := startNode
	for iterNode != nil && iterNode.Height()%blocksPerRetarget != 0 &&
		iterNode.Bits() == powLimitBits {

		iterNode = iterNode.Parent()
	}

	// Return the found difficulty or the minimum difficulty if no
	// appropriate block was found.
	lastBits := powLimitBits
	if iterNode != nil {
		lastBits = iterNode.Bits()
	}
	return lastBits
}

func (b *BlockChain) calcNextK(lastNode blocknode.IBlockNode) uint32 {
	if lastNode == nil {
		return pow.PackK(pow.K1)
	}

	// Return the previous block's difficulty requirements if this block
	// is not at a difficulty retarget interval.
	if (lastNode.Height()+1)%pow.KBeaconEpochLen != 0 {
		return lastNode.K()
	}

	if pow.BeaconEpoch(lastNode.Height()+1) <= 2 {
		return pow.PackK(pow.K1)
	}

	ancestor := lastNode.RelativeAncestor(pow.KBeaconEpochLen)
	return ancestor.CalcMedianVoteK()
}

// CalcNextK calculates the required k coefficient
//
// This function is safe for concurrent access.
func (b *BlockChain) CalcNextK() uint32 {
	b.chainLock.Lock()
	difficulty := b.calcNextK(b.bestChain.Tip())
	b.chainLock.Unlock()
	return difficulty
}

// calcNextRequiredDifficulty calculates the required difficulty for the block
// after the passed previous block node based on the difficulty retarget rules.
// This function differs from the exported CalcNextRequiredDifficulty in that
// the exported version uses the current best chain as the previous block node
// while this function accepts any block node.
func (b *BlockChain) calcNextRequiredDifficulty(lastNode blocknode.IBlockNode, newBlockTime time.Time) (uint32, error) {
	return calcNextRequiredDifficulty(b.chainParams, b.retargetOpts, lastNode, newBlockTime)
}

// The following fields are calculated based upon the provided chain
// parameters.  They are also set when the instance is created and
// can't be changed afterwards, so there is no need to protect them with
// a separate mutex.
type retargetOpts struct {
	minRetargetTimespan int64 // target timespan / adjustment factor
	maxRetargetTimespan int64 // target timespan * adjustment factor
	blocksPerRetarget   int32 // target timespan / target time per block
}

// calcNextRequiredDifficulty calculates the required difficulty for the block
// after the passed previous block node based on the difficulty retarget rules.
// This function differs from the exported CalcNextRequiredDifficulty in that
// the exported version uses the current best chain as the previous block node
// while this function accepts any block node.
func calcNextRequiredDifficulty(chainParams *chaincfg.Params, opts retargetOpts, lastNode blocknode.IBlockNode, newBlockTime time.Time) (uint32, error) {
	// Genesis block.
	if lastNode == nil {
		return chainParams.PowParams.PowLimitBits, nil
	}

	// Return the previous block's difficulty requirements if this block
	// is not at a difficulty retarget interval.
	if (lastNode.Height()+1)%opts.blocksPerRetarget != 0 {
		// For networks that support it, allow special reduction of the
		// required difficulty once too much time has elapsed without
		// mining a block.
		if chainParams.PowParams.ReduceMinDifficulty {
			// Return minimum difficulty when more than the desired
			// amount of time has elapsed without mining a block.
			reductionTime := int64(chainParams.PowParams.MinDiffReductionTime / time.Second)
			allowMinTime := lastNode.Timestamp() + reductionTime
			if newBlockTime.Unix() > allowMinTime {
				return chainParams.PowParams.PowLimitBits, nil
			}

			// The block was mined within the desired timeframe, so
			// return the difficulty for the last block which did
			// not have the special minimum difficulty rule applied.
			return findPrevTestNetDifficulty(lastNode, opts.blocksPerRetarget, chainParams.PowParams.PowLimitBits), nil
		}

		// For the main network (or any unrecognized networks), simply
		// return the previous block's difficulty requirements.
		return lastNode.Bits(), nil
	}

	// Get the block node at the previous retarget (targetTimespan days worth of blocks).
	firstNode := lastNode.RelativeAncestor(opts.blocksPerRetarget - 5)
	if firstNode == nil {
		return 0, chaindata.AssertError("unable to obtain previous retarget block")
	}

	epochStartMedianTimestamp := firstNode.CalcPastMedianTimeForN(5).Unix()
	epochEndMedianTimestamp := lastNode.CalcPastMedianTimeForN(5).Unix()

	// Limit the amount of adjustment that can occur to the previous
	// difficulty.
	actualTimespan := epochEndMedianTimestamp - epochStartMedianTimestamp
	adjustedTimespan := actualTimespan

	if actualTimespan < opts.minRetargetTimespan {
		adjustedTimespan = opts.minRetargetTimespan
	} else if actualTimespan > opts.maxRetargetTimespan {
		adjustedTimespan = opts.maxRetargetTimespan
	}

	// Calculate new target difficulty as:
	//  currentDifficulty * (adjustedTimespan / targetTimespan)
	// The result uses integer division which means it will be slightly
	// rounded down.  Jaxnetd also uses integer division to calculate this
	// result.
	oldTarget := pow.CompactToBig(lastNode.Bits())
	newTarget := new(big.Int).Mul(oldTarget, big.NewInt(adjustedTimespan))
	targetTimeSpan := int64(chainParams.PowParams.TargetTimespan / time.Second)
	newTarget.Div(newTarget, big.NewInt(targetTimeSpan))

	// Limit new value to the proof of work limit.
	if newTarget.Cmp(chainParams.PowParams.PowLimit) > 0 {
		newTarget.Set(chainParams.PowParams.PowLimit)
	}

	// Log new target difficulty and return it.  The new target logging is
	// intentionally converting the bits back to a number instead of using
	// newTarget since conversion to the compact representation loses
	// precision.
	newTargetBits := pow.BigToCompact(newTarget)
	log.Debug().Msgf("Difficulty retarget at block height %d", lastNode.Height()+1)
	log.Debug().Msgf("Old target %08x (%064x)", lastNode.Bits(), oldTarget)
	log.Debug().Msgf("New target %08x (%064x)", newTargetBits, pow.CompactToBig(newTargetBits))
	log.Debug().Msgf("Actual timespan %v, adjusted timespan %v, target timespan %v",
		time.Duration(actualTimespan)*time.Second,
		time.Duration(adjustedTimespan)*time.Second,
		chainParams.PowParams.TargetTimespan)

	return newTargetBits, nil
}

// CalcNextRequiredDifficulty calculates the required difficulty for the block
// after the end of the current best chain based on the difficulty retarget
// rules.
//
// This function is safe for concurrent access.
func (b *BlockChain) CalcNextRequiredDifficulty(timestamp time.Time) (uint32, error) {
	b.chainLock.Lock()
	difficulty, err := b.calcNextRequiredDifficulty(b.bestChain.Tip(), timestamp)
	b.chainLock.Unlock()
	return difficulty, err
}
