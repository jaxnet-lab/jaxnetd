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
	k1 := new(big.Rat).SetFloat64(K1) // k1

	println(k1.FloatString(64))
	// 0.0000000000000035527136788005009293556213378906250000000000000000

	d := GetDifficulty(0x1d00ffff) // mainet
	println(d.FloatString(64))
	// 65537.0000152590218966964217593652246890974288548104066529335469596399

	reward := new(big.Rat).Mul(k1, d)
	println(reward.FloatString(64))
	// 0.0000000002328341964217593652246890974288548104066529335469596399

}
