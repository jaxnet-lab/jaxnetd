package pow

import (
	"math/big"
	"testing"
)

func TestHashSortingLastBits(t *testing.T) {
	tests := []struct {
		powHash *big.Int
		chainID uint32
	}{
		{
			powHash: new(big.Int).SetUint64(0),
			chainID: 0,
		},
		{
			powHash: new(big.Int).SetUint64(1),
			chainID: 1,
		},
		{
			powHash: new(big.Int).SetUint64(8),
			chainID: 8,
		},
		{
			powHash: new(big.Int).SetUint64(4095), // 2^12-1
			chainID: 4095,
		},
		{
			powHash: new(big.Int).SetUint64(4096), // 2^12
			chainID: 0,
		},
		{
			powHash: new(big.Int).SetUint64(4097), // 2^12+1
			chainID: 1,
		},
		{
			powHash: new(big.Int).SetUint64(65535), // 2^16-1
			chainID: 4095,
		},
		{
			powHash: new(big.Int).SetUint64(65536), // 2^16
			chainID: 0,
		},
	}

	for _, test := range tests {
		got := HashSortingLastBits(test.powHash, 4096)

		if got != test.chainID {
			t.Errorf("HashSortingLastBits: got: (%d); want: (%d)", got, test.chainID)
			return
		}
	}
}

func TestValidateHashSortingRule(t *testing.T) {
	tests := []struct {
		powHash string
		chainID uint32
	}{
		{
			powHash: "00000000150288a5bfe367741d20e02167f24cbc14844cd069ee24e0a241a000",
			chainID: 0,
		},
		{
			powHash: "00000000150288a5bfe367741d20e02167f24cbc14844cd069ee24e0a241a001",
			chainID: 1,
		},
		{
			powHash: "00000000150288a5bfe367741d20e02167f24cbc14844cd069ee24e0a241a008",
			chainID: 8,
		},
		{
			powHash: "00000000150288a5bfe367741d20e02167f24cbc14844cd069ee24e0a2411fff",
			chainID: 4095,
		},
		{
			powHash: "00000000150288a5bfe367741d20e02167f24cbc14844cd069ee24e0a2411000",
			chainID: 0,
		},
		{
			powHash: "00000000150288a5bfe367741d20e02167f24cbc14844cd069ee24e0a2411001",
			chainID: 1,
		},
	}

	for _, test := range tests {
		powHash, _ := new(big.Int).SetString(test.powHash, 16)
		ok := ValidateHashSortingRule(powHash, 4096, test.chainID)

		if ok != true {
			t.Errorf("ValidateHashSortingRule: got: (%t); want: (%t)", ok, true)
			return
		}
	}
}
