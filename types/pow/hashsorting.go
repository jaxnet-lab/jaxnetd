package pow

import (
	"math/big"
)

// ValidateHashSortingRule checks if chain ID equals to chainIDCount or equals to
// remainder of modulo operation with chain ID as dividend and chainIDCount as divisor.
func ValidateHashSortingRule(powHash *big.Int, hashSortingSlotNumber, chainID uint32) bool {
	return HashSortingLastBits(powHash, hashSortingSlotNumber) == chainID%hashSortingSlotNumber
}

func HashSortingLastBits(powHash *big.Int, hashSortingSlotNumber uint32) uint32 {
	lastBitsMask := new(big.Int).SetUint64(uint64(hashSortingSlotNumber - 1))

	res := powHash.And(powHash, lastBitsMask)
	bits := res.Bits()
	if len(bits) > 0 {
		if uint32(bits[0]) < hashSortingSlotNumber {
			return uint32(bits[0])
		}
		return uint32(bits[0]) % hashSortingSlotNumber
	}

	return 0
}
