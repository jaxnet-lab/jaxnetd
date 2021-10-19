package main

import (
	"fmt"
	"math"
	"math/big"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
)

func main() {
	// mainnetPowBits := chaincfg.MainNetParams.PowParams.PowLimitBits
	// testnetPowBits := chaincfg.TestNetParams.PowParams.PowLimitBits
	// fastnetPowBits := chaincfg.FastNetParams.PowParams.PowLimitBits
	//
	// fmt.Printf("mainnet: bits=%08x, target=%064x 2^%d  %d \n",
	// 	mainnetPowBits, pow.CompactToBig(mainnetPowBits), PowerOfTwo(CalcWork(mainnetPowBits).Uint64()), CalcWork(mainnetPowBits).Uint64())
	//
	// fmt.Printf("testnet: bits=%08x, target=%064x 2^%d  %d \n",
	// 	testnetPowBits, pow.CompactToBig(testnetPowBits), PowerOfTwo(CalcWork(testnetPowBits).Uint64()), CalcWork(testnetPowBits).Uint64())
	//
	// fmt.Printf("fastnet: bits=%08x, target=%064x 2^%d  %d \n",
	// 	fastnetPowBits, pow.CompactToBig(fastnetPowBits), PowerOfTwo(CalcWork(fastnetPowBits).Uint64()), CalcWork(fastnetPowBits).Uint64())

	pcHashRate := 893_000 // hash per sec
	hcShard := uint64(pcHashRate * 375 / 10)
	hcBeacon := pcHashRate * 60 * 10

	shardPow := PowerOfTwo(hcShard)           // 2^24
	beaconPow := PowerOfTwo(uint64(hcBeacon)) // 2^28

	fmt.Println(hcBeacon, beaconPow, hcShard, shardPow)
	printTargetInfo("new_beacon", targetByPowOfTwo(uint(beaconPow-12)))
	printTargetInfo("new_beacon", 0x1f01FFF0)

	printTargetInfo("new_shard", targetByPowOfTwo(uint(shardPow-12)))
	printTargetInfo("new_shard", 0x1f1FF000)

}

func printTargetInfo(msg string, bits uint32) {
	fmt.Printf("%s: bits %08x, target %064x, 2^%d,  work %d \n", msg,
		bits, pow.CompactToBig(bits), PowerOfTwo(pow.CalcWork(bits).Uint64()), pow.CalcWork(bits).Uint64())
}

func targetByPowOfTwo(powOfTwo uint) uint32 {
	oneLsh := new(big.Int).Lsh(bigOne, powOfTwo)
	target := new(big.Int).Div(oneLsh256, oneLsh)
	return pow.BigToCompact(target)
}

var (
	bigOne    = new(big.Int).SetInt64(1)
	oneLsh256 = new(big.Int).Lsh(bigOne, 256)
)

func PowerOfTwo(n uint64) uint64 {
	// Return the number if it's already a power of 2.
	if n&(n-1) == 0 {
		return n
	}

	// Figure out and return the next power of two.
	exponent := uint64(math.Log2(float64(n)))
	return exponent // 2^exponent
}

func mesureHashRate() {
	block := chaincfg.MainNetParams.GenesisBlock().Copy()
	//
	timer := time.NewTimer(time.Minute)
	count := 0

	for {
		select {
		case <-timer.C:
			fmt.Printf("Hash speed: %v kilohashes/s", (count/60)/1000)
			break

		default:
			hash := block.BlockHash()
			_ = hash
			count += 2
		}
	}
}