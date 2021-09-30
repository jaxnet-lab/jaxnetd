/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package pow

import (
	"math/big"
	"testing"
)

func TestGetDifficulty(t *testing.T) {
	d := CalcWork(0x1d0ffff0)
	println(d.String())
	d = CalcWork(0x1e0dffff)
	println(d.String())
	d = CalcWork(0x170ED0EB)

	println(d.String())
	println(new(big.Int).Div(d, new(big.Int).Lsh(bigOne, 32)).String())
	// 81595492539307259101866
	// 18997641161758.95
	// 18997931047181

	// println(new(big.Rat).Mul(d, k1).FloatString(4))
	// reward, err := strconv.ParseFloat(new(big.Rat).Mul(d, k1).FloatString(4), 64)
	// println(err)
	// println(reward)
	// println(int64(reward * 1000))
}
