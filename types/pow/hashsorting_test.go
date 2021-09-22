package pow

import (
	"math/big"
	"testing"
)

func TestHashSortingLastBits(t *testing.T) {
	tests := []struct {
		bigInt *big.Int
		bits   uint32
	}{
		{
			bigInt: new(big.Int).SetUint64(0),
			bits:   0,
		},
		{
			bigInt: new(big.Int).SetUint64(8),
			bits:   8,
		},
		{
			bigInt: new(big.Int).SetUint64(4095), // 2^12-1
			bits:   4095,
		},
		{
			bigInt: new(big.Int).SetUint64(4096), // 2^12
			bits:   0,
		},
		{
			bigInt: new(big.Int).SetUint64(4097), // 2^12+1
			bits:   1,
		},
		{
			bigInt: new(big.Int).SetUint64(65535), // 2^16-1
			bits:   4095,
		},
		{
			bigInt: new(big.Int).SetUint64(65536), // 2^16
			bits:   0,
		},
	}

	for _, test := range tests {
		got := HashSortingLastBits(test.bigInt, 4096)

		if got != test.bits {
			t.Errorf("HashSortingLastBits: got: (%d); want: (%d)", got, test.bits)
			return
		}
	}
}
