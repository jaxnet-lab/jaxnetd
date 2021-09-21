package pow

import (
	"encoding/binary"
	"math/big"
)

// LastBits = 2^12-1
const LastBits = 4095

// firstBits = reversed 2^12 -1
const firstBits = "FFF0000000000000000000000000000000000000000000000000000000000000"

func ValidateHashSortingRule(headerTarget *big.Int, chainID uint32) bool {
	// TODO: now max chainID is 2 ^ 12 -1. Fix it.
	// If chain ID is less than 2 ^ 12 − 1 = 4095, then take last 12 bits of H and compare
	// them with shard ID. Else split shard ID into two parts.
	// Check whether first 12 bits of H match IDh. Last 12 bits of H should match IDt.
	// IDh = (int) ID / 2 ^ 12
	// IDt = ID % 2 ^ 12

	lastBits, ok := HashSortingLastBits(headerTarget)
	if !ok {
		return false
	}

	if chainID == 0 { // case for beacon chain
		return lastBits == 0
	}

	if chainID <= LastBits {
		if lastBits == (chainID) {
			return true
		}
		return false
	}

	firstBits, ok := HashSortingFirstBits(headerTarget)
	if !ok {
		return false
	}

	IDh := int(chainID) / (LastBits + 1)
	IDt := int(chainID) % (LastBits + 1)
	if uint32(IDh) == firstBits && uint32(IDt) == lastBits {
		return true
	}

	return false
}

func HashSortingLastBits(diff *big.Int) (uint32, bool) {
	lastBitsMask := new(big.Int).SetUint64(LastBits)

	if diff.Cmp(lastBitsMask) <= 0 {
		res := diff.And(diff, lastBitsMask)
		bits := res.Bits()
		if len(bits) > 0 {
			return uint32(bits[0]), true
		}
		// if input is zero then
		return 0, true
	}

	return 0, false
}

func HashSortingFirstBits(diff *big.Int) (uint32, bool) {
	lastBitsMask := new(big.Int).SetUint64(LastBits)
	if diff.Cmp(lastBitsMask) <= 0 {
		return 0, false
	}

	firstBitsMask, ok := new(big.Int).SetString(firstBits, 16)
	if !ok {
		return 0, false
	}

	res := diff.And(diff, firstBitsMask)
	res = res.Rsh(res, 4)
	buffer := make([]byte, 32, 32)
	bytes := res.FillBytes(buffer)
	x1 := binary.LittleEndian.Uint32(bytes[0:4])

	return x1 >> 8, true
}
