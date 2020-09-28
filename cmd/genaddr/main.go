package main

import (
	"fmt"
	"os"

	"gitlab.com/jaxnet/core/shard.core.git/btcec"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chaincore"
)

func main() {
	genKeys()
}

func genKeys() {
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		fmt.Printf("failed to make privKey for  %v", err)
	}

	// pk := (*btcec.PublicKey)(&key.PublicKey).SerializeCompressed()
	pk := (*btcec.PublicKey)(&key.PublicKey).SerializeUncompressed()
	addressPubKey, err := btcutil.NewAddressPubKey(pk, &chaincore.SimNetParams)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	simNetAddress, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pk), &chaincore.SimNetParams)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	mainNetAddress, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pk), &chaincore.MainNetParams)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	testNetAddress, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pk), &chaincore.TestNet3Params)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	fmt.Printf("PrivateKey:\t%x\n", key.Serialize())
	fmt.Printf("AddressPubKey:\t%x\n", addressPubKey.String())
	fmt.Printf("SimNet :\t%s\n", simNetAddress.EncodeAddress())
	fmt.Printf("TestNet:\t%s\n", testNetAddress.EncodeAddress())
	fmt.Printf("MainNet:\t%s\n", mainNetAddress.EncodeAddress())
}
