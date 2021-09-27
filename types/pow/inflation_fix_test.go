/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package pow

import (
	"fmt"
	"math/big"
	"strconv"
	"testing"
)

func TestGetDifficulty(t *testing.T) {
	oldTarget := CompactToBig(0x1d00ffff)
	fmt.Printf("(%064x) target %08x \n", CompactToBig(0x1d00ffff), 0x1d00ffff)

	for i := 1; i <= 32; i += 1 {
		newBits := BigToCompact(new(big.Int).Mul(oldTarget, big.NewInt(1<<i)))
		fmt.Printf("(%064x) target %08x %d/%d \n", CompactToBig(newBits), newBits, i, 1<<i)
	}

	comp := PackRat(new(big.Rat).SetFloat64(K1 * SupplementaryK1))
	k1 := UnpackRat(comp)

	d := GetDifficulty(0x1e00ffff)
	println(d.FloatString(64))
	println(new(big.Rat).Mul(d, k1).FloatString(4))
	reward, err := strconv.ParseFloat(new(big.Rat).Mul(d, k1).FloatString(4), 64)
	println(err)
	println(reward)
	println(int64(reward * 1000))
}
