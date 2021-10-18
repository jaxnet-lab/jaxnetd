package main

import (
	"fmt"
	"math"

	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
)

func main() {
	mainnetPowBits := chaincfg.MainNetParams.PowParams.PowLimitBits
	testnetPowBits := chaincfg.TestNetParams.PowParams.PowLimitBits
	fastnetPowBits := chaincfg.FastNetParams.PowParams.PowLimitBits

	fmt.Printf("mainnet: bits=%08x, target=%064x 2^%d  %d \n",
		mainnetPowBits, pow.CompactToBig(mainnetPowBits), PowerOfTwo(pow.CalcWork(mainnetPowBits).Uint64()), pow.CalcWork(mainnetPowBits).Uint64())

	fmt.Printf("testnet: bits=%08x, target=%064x 2^%d  %d \n",
		testnetPowBits, pow.CompactToBig(testnetPowBits), PowerOfTwo(pow.CalcWork(testnetPowBits).Uint64()), pow.CalcWork(testnetPowBits).Uint64())

	fmt.Printf("fastnet: bits=%08x, target=%064x 2^%d  %d \n",
		fastnetPowBits, pow.CompactToBig(fastnetPowBits), PowerOfTwo(pow.CalcWork(fastnetPowBits).Uint64()), pow.CalcWork(fastnetPowBits).Uint64())

	// block := chaincfg.MainNetParams.GenesisBlock().Copy()

	// timer := time.NewTimer(time.Minute)
	// count := 0
	//
	// for {
	// 	select {
	// 	case <-timer.C:
	// 		fmt.Printf("Hash speed: %v kilohashes/s", (count/60)/1000)
	// 		break
	//
	// 	default:
	// 		hash := block.BlockHash()
	// 		_ = hash
	// 		count += 2
	// 	}
	// }

	hcShard := uint64(893_000 * 37.5)
	hcBeacon := 893_000 * 60 * 10

	shardPow := PowerOfTwo(uint64(hcShard))   // 2^24
	beaconPow := PowerOfTwo(uint64(hcBeacon)) // 2^28

	fmt.Println(hcBeacon, beaconPow, hcShard, shardPow)
}

func PowerOfTwo(n uint64) uint64 {
	// Return the number if it's already a power of 2.
	if n&(n-1) == 0 {
		return n
	}

	// Figure out and return the next power of two.
	exponent := uint64(math.Log2(float64(n)))
	return exponent // 2^exponent
}
