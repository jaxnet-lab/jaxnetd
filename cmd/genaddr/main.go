// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package main

import (
	"fmt"
	"os"

	"gitlab.com/jaxnet/jaxnetd/btcec"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

func main() {
	genKeys()
}

func genKeys() {
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		fmt.Printf("failed to make privKey for  %v", err)
	}

	wifMain, err := jaxutil.NewWIF(key, &chaincfg.MainNetParams, true)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}
	wifTest, err := jaxutil.NewWIF(key, &chaincfg.TestNet3Params, true)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}
	wifSim, err := jaxutil.NewWIF(key, &chaincfg.SimNetParams, true)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	// pk := (*btcec.PublicKey)(&key.PublicKey).SerializeCompressed()
	pk := (*btcec.PublicKey)(&key.PublicKey).SerializeUncompressed()
	addressPubKey, err := jaxutil.NewAddressPubKey(pk, &chaincfg.SimNetParams)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	simNetAddress, err := jaxutil.NewAddressPubKeyHash(jaxutil.Hash160(pk), &chaincfg.SimNetParams)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	mainNetAddress, err := jaxutil.NewAddressPubKeyHash(jaxutil.Hash160(pk), &chaincfg.MainNetParams)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	testNetAddress, err := jaxutil.NewAddressPubKeyHash(jaxutil.Hash160(pk), &chaincfg.TestNet3Params)
	if err != nil {
		println("[error] " + err.Error())
		os.Exit(1)
	}

	fmt.Printf("PrivateKey:\t%x\n", key.Serialize())
	fmt.Printf("Wallet Import Format:\n")
	fmt.Printf("WIF SimNet :\t%s\n", wifSim)
	fmt.Printf("WIF TestNet:\t%s\n", wifTest)
	fmt.Printf("WIF MainNet:\t%s\n", wifMain)

	fmt.Printf("AddressPubKey:\t%x\n", addressPubKey.String())
	fmt.Printf("SimNet :\t%s\n", simNetAddress.EncodeAddress())
	fmt.Printf("TestNet:\t%s\n", testNetAddress.EncodeAddress())
	fmt.Printf("MainNet:\t%s\n", mainNetAddress.EncodeAddress())
}
