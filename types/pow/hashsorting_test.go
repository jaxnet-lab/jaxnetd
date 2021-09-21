package pow

import (
	"math/big"
	"testing"
)

func TestHashSortingLastBits(t *testing.T) {
	tests := []struct {
		bigInt *big.Int
		bits   uint32
		ok     bool
	}{
		{
			bigInt: new(big.Int).SetUint64(0),
			bits:   0,
			ok:     true,
		},
		{
			bigInt: new(big.Int).SetUint64(8),
			bits:   8,
			ok:     true,
		},
		{
			bigInt: new(big.Int).SetUint64(4095), // 2^12-1
			bits:   4095,
			ok:     true,
		},
		// out of range, return 0, false
		{
			bigInt: new(big.Int).SetUint64(65535), // 2^16-1
			bits:   0,
			ok:     false,
		},
	}

	for _, test := range tests {
		got, ok := HashSortingLastBits(test.bigInt)
		if ok != test.ok {
			t.Errorf("HashSortingLastBits got not ok, want: (%d)", test.bits)
			return
		}

		if got != test.bits {
			t.Errorf("HashSortingLastBits: got: (%d); want: (%d)", got, test.bits)
			return
		}

	}
}

func TestHashSortingFirstBits(t *testing.T) {
	var shardOne = "0010000000000000000000000000000000000000000000000000000000000000"

	bigshardOne, ok := new(big.Int).SetString(shardOne, 16)
	if !ok {
		t.Error("HashSortingFirstBits failed to init test value bigShardID")
		return
	}

	var shardTen = "00a0000000000000000000000000000000000000000000000000000000000000"

	bigshardTen, ok := new(big.Int).SetString(shardTen, 16)
	if !ok {
		t.Error("HashSortingFirstBits failed to init test value bigShardID")
		return
	}

	tests := []struct {
		bigInt *big.Int
		bits   uint32
		ok     bool
	}{
		{
			bigInt: new(big.Int).SetUint64(0),
			bits:   0,
			ok:     false,
		},
		{
			bigInt: new(big.Int).SetUint64(4095), // 2^12-1
			bits:   0,
			ok:     false,
		},
		{
			bigInt: bigshardOne,
			bits:   1,
			ok:     true,
		},
		{
			bigInt: bigshardTen,
			bits:   10,
			ok:     true,
		},
	}

	for _, test := range tests {
		got, ok := HashSortingFirstBits(test.bigInt)
		if ok != test.ok {
			t.Errorf("HashSortingFirstBits got not ok, want: (%d)", test.bits)
			return
		}

		if got != test.bits {
			t.Errorf("HashSortingFirstBits: got: (%d); want: (%d)", got, test.bits)
			return
		}

	}
}
