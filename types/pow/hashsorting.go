package pow

import (
	"math/big"
)

// ValidateHashSortingRule checks if chain ID equals chainIDCount or equals remainder of modulo operation with chain ID as dividend and
// chainIDCount as divisor.
func ValidateHashSortingRule(headerTarget *big.Int, chainIDCount, chainID uint32) bool {

	lastBits := HashSortingLastBits(headerTarget, chainIDCount)

	if lastBits == chainID {
			return true
		}
		return false
}

func HashSortingLastBits(diff *big.Int, chainIDCount uint32) uint32 {
	lastBitsMask := new(big.Int).SetUint64(uint64(chainIDCount - 1))

	res := diff.And(diff, lastBitsMask)
	bits := res.Bits()
	if len(bits) > 0 {
		if uint32(bits[0]) < chainIDCount {
			return uint32(bits[0])
		} else {
			return uint32(bits[0]) % chainIDCount
		}
	}

	return 0
}

