/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaindata

import (
	"fmt"
	"strconv"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/jaxutil/base58"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

func Test_calcBlockSubsidy(t *testing.T) {
	tests := []struct {
		epoch  int
		height int32
		reward int64
	}{
		{epoch: 1, height: 1, reward: 340},
		{epoch: 1, height: 10, reward: 340},
		{epoch: 1, height: 100, reward: 340},
		{epoch: 1, height: 3072, reward: 340},
		{epoch: 1, height: 3073, reward: 330},
		{epoch: 1, height: 9216, reward: 330},
		{epoch: 1, height: 9217, reward: 320},
		{epoch: 1, height: 24677, reward: 300},
		{epoch: 1, height: 49100, reward: 260},
		{epoch: 1, height: 49152, reward: 260},
		{epoch: 2, height: 49153, reward: 260},
		{epoch: 2, height: 90000, reward: 225},
		{epoch: 2, height: 98304, reward: 220},
		{epoch: 3, height: 98305, reward: 220},
		{epoch: 3, height: 100000, reward: 220},
		{epoch: 3, height: 110000, reward: 190},
		{epoch: 3, height: 120000, reward: 160},
		{epoch: 3, height: 130000, reward: 145},
		{epoch: 3, height: 135000, reward: 130},
		{epoch: 3, height: 140000, reward: 115},
		{epoch: 3, height: 145000, reward: 100},
		{epoch: 4, height: 150000, reward: 100},
		{epoch: 4, height: 160000, reward: 90},
		{epoch: 4, height: 170000, reward: 80},
		{epoch: 5, height: 200000, reward: 55},
		{epoch: 5, height: 230000, reward: 35},
		{epoch: 5, height: 240000, reward: 25},
		{epoch: 6, height: 250000, reward: 20},
		{epoch: 6, height: 10000000, reward: 20},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if got := calcBlockSubsidy(tt.height); got != tt.reward*chaincfg.SatoshiPerBitcoin {
				t.Errorf("calcBlockSubsidy(%d) = %v, want %v", tt.height, got/chaincfg.SatoshiPerBitcoin, tt.reward)
			}
		})
	}

	f1, _ := txscript.NullDataScript([]byte(" jax.network "))
	f2, _ := txscript.NullDataScript([]byte("     JAX     "))

	fmt.Println(base58.CheckEncode(f1, chaincfg.MainNetParams.PubKeyHashAddrID))
	fmt.Println(base58.CheckEncode(f2, chaincfg.MainNetParams.PubKeyHashAddrID))
}
