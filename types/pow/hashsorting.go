package pow

import (
	"math/big"
)

// ValidateHashSortingRule checks if chain ID equals to chainIDCount or equals to
// remainder of modulo operation with chain ID as dividend and chainIDCount as divisor.
func ValidateHashSortingRule(powHash *big.Int, chainIDCount, chainID uint32) bool {
	return HashSortingLastBits(powHash, chainIDCount) == chainID
}

func HashSortingLastBits(powHash *big.Int, chainIDCount uint32) uint32 {
	lastBitsMask := new(big.Int).SetUint64(uint64(chainIDCount - 1))

	res := powHash.And(powHash, lastBitsMask)
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
