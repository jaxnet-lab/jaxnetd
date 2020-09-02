package main

import (
	"fmt"
	"os"

	"gitlab.com/jaxnet/core/shard.core.git/btcec"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
)

func main() {
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		fmt.Printf("failed to make privKey for  %v", err)
	}

	pk := (*btcec.PublicKey)(&key.PublicKey).SerializeUncompressed()

	simNetAddress, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pk), &chaincfg.SimNetParams)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	mainNetAddress, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pk), &chaincfg.MainNetParams)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	testNetAddress, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pk), &chaincfg.TestNet3Params)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	fmt.Printf("PrivateKey:\t%x\n", key.Serialize())
	fmt.Printf("SimNet :\t%s\n", simNetAddress.EncodeAddress())
	fmt.Printf("TestNet:\t%s\n", testNetAddress.EncodeAddress())
	fmt.Printf("MainNet:\t%s\n", mainNetAddress.EncodeAddress())
}
