package chaindata

import (
	"fmt"
	"math/big"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/types/pow"
)

func Test_validateVoteK(t *testing.T) {
	bigOne := big.NewInt(1)

	{
		oneLsh60 := new(big.Int).Lsh(bigOne, 60)
		K1 := new(big.Float).Quo(
			new(big.Float).SetFloat64(1),
			new(big.Float).SetInt(oneLsh60),
		)

		// 8.67361738e-19
		fmt.Println(K1.String())

		k2p60 := pow.PackK(K1)
		fmt.Printf("%0x\n", k2p60)

		uK2p60 := pow.UnpackK(k2p60)
		fmt.Println(uK2p60.String())

		err := validateVoteK(k2p60, k2p60)
		fmt.Println(err)

		oneLsh124 := new(big.Int).Lsh(bigOne, 100)
		Kx := new(big.Float).Quo(
			new(big.Float).SetFloat64(1),
			new(big.Float).SetInt(oneLsh124),
		)
		fmt.Println(Kx.String())

		k2p100 := pow.PackK(Kx)
		fmt.Printf("%0x\n", k2p100)

		uK2p100 := pow.UnpackK(k2p100)
		fmt.Println(uK2p100.String())

		err = validateVoteK(k2p100, k2p100)
		fmt.Println(err)

		oneLsh124 = new(big.Int).Lsh(bigOne, 124)
		Kx = new(big.Float).Quo(
			new(big.Float).SetFloat64(1),
			new(big.Float).SetInt(oneLsh124),
		)
		fmt.Println(Kx.String())

		k2p124 := pow.PackK(Kx)
		fmt.Printf("%0x\n", k2p124)

		uK2p124 := pow.UnpackK(k2p124)
		fmt.Println(uK2p124.String())

		err = validateVoteK(k2p124, k2p124)
		fmt.Println(err)

		err = validateVoteK(k2p60, k2p100)
		fmt.Println(err)

		err = validateVoteK(k2p100, k2p60)
		fmt.Println(err)

		err = validateVoteK(k2p60, k2p124)
		fmt.Println(err)

		err = validateVoteK(k2p100, k2p124)
		fmt.Println(err)
	}
	//
	{
		type args struct {
			prevK *big.Float
			delta float64
			err   bool
		}

		oneLsh100 := new(big.Int).Lsh(bigOne, 100)
		kDiv2 := new(big.Float).Quo(
			new(big.Float).SetFloat64(1),
			new(big.Float).SetInt(oneLsh100),
		)

		tests := []args{
			{prevK: pow.K1, delta: 1, err: false},

			{prevK: pow.K1, delta: 1.001, err: true},
			{prevK: pow.K1, delta: 1.005, err: true},
			{prevK: pow.K1, delta: 1.01, err: true},
			{prevK: pow.K1, delta: 1.012, err: true},
			{prevK: pow.K1, delta: 1.02, err: true},

			{prevK: pow.K1, delta: 0.99, err: false},
			{prevK: pow.K1, delta: 0.98, err: false},
			{prevK: pow.K1, delta: 0.975, err: false},
			{prevK: pow.K1, delta: 0.97, err: false},
			{prevK: pow.K1, delta: 0.96, err: true},
			//
			{prevK: kDiv2, delta: 1, err: false},

			{prevK: kDiv2, delta: 1.001, err: false},
			{prevK: kDiv2, delta: 1.005, err: false},
			{prevK: kDiv2, delta: 1.01, err: false},
			{prevK: kDiv2, delta: 1.015, err: true},
			{prevK: kDiv2, delta: 1.02, err: true},

			{prevK: kDiv2, delta: 0.99, err: false},
			{prevK: kDiv2, delta: 0.98, err: false},
			{prevK: kDiv2, delta: 0.975, err: false},
			{prevK: kDiv2, delta: 0.97, err: false},
			{prevK: kDiv2, delta: 0.96, err: true},
		}

		for _, tt := range tests {
			K1 := pow.PackK(tt.prevK)
			newK := new(big.Float).Mul(
				tt.prevK,
				new(big.Float).SetFloat64(tt.delta),
			)
			pNewK := pow.PackK(newK)
			err := validateVoteK(K1, pNewK)
			if (err != nil) != tt.err {
				t.Errorf("K1, pNewK -->%v | %v\n", tt.delta, err)
			}

		}
	}
}
