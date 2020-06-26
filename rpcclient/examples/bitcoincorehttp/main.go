// Copyright (c) 2014-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"log"

	"gitlab.com/jaxnet/core/shard.core.git/rpcclient"
)

func main() {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         "0.0.0.0:8332",
		User:         "somerpc",
		Pass:         "somerpc",
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer client.Shutdown()

	// Get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Printf("Block count: %d", blockCount)

	//secret, err := btcec.NewPrivateKey(btcec.S256())
	//if err != nil {
	//	return
	//}
	//wif, err :=  btcutil.NewWIF(secret, &chaincfg.JaxNetParams, false)
	//fmt.Println()
	//fmt.Println(string(wif.SerializePubKey()))
	//
	//
	//addr, err := btcutil.NewAddressPubKey(wif.PrivKey.PubKey().SerializeCompressed(), &chaincfg.JaxNetParams)
	//fmt.Println(addr.String(), addr.AddressPubKeyHash())
	//

	//btcutil.

	//params := &chaincfg.JaxNetParams
	//params.Name = "jaxnet"
	//params.Net = wire.BitcoinNet(0x12121212)
	//params.PubKeyHashAddrID = byte(0x6F)
	//params.PrivateKeyID = byte(0x80)
	//

	//fmt.Println(params.Net, activeNetParams.Net)
	//fmt.Println(params.PubKeyHashAddrID, activeNetParams.PubKeyHashAddrID)
	//fmt.Println(params.PrivateKeyID, activeNetParams.PrivateKeyID)
	//1CzWYLMRUt2dUnvirRtS68vQjgVjYDxsZh
	jaxAddres, err := btcutil.DecodeAddress("15VS71vPzPMxY5fXwJUVkqyHge94g4Uub4", &chaincfg.JaxNetParams)

	//fmt.Println("ImportAddress")
	//if err := client.ImportAddress("1CzWYLMRUt2dUnvirRtS68vQjgVjYDxsZh"); err != nil{
	//	fmt.Println(err)
	//}

	fmt.Println("Unlock wallet ")
	if err := client.WalletPassphrase("1R23x73d4", 100); err != nil {
		fmt.Println("Cant create wallet", err)
		return
	}

	//if err:= client.ImportPrivKey(wif); err != nil{
	//	fmt.Println("Can't import KPK", err)
	//}

	//if err := client.SetAccount(jaxAddres, "JaxMain"); err != nil {
	//	fmt.Println("Error SetAccount", err)
	//}
	//if err := client.CreateNewAccount("JaxMiner"); err != nil {
	//	fmt.Println(err)
	//}

	accs, err := client.ListAccounts()
	fmt.Println("Accounts", accs)

	err = client.SetTxFee(0)
	fmt.Println("SetTxFee", err)

	//ok, err := client.SendFrom("default", jaxAddres, 0)
	//fmt.Println("Move trx ", ok, err)

	aaddr, err := client.GetAccountAddress("JaxMiner")
	fmt.Println("JaxMiner address: ", aaddr, err)

	//fmt.Println(toAddress, err)
	aa, err := client.GetReceivedByAddress(jaxAddres)
	fmt.Println("GetReceivedByAddress", aa, jaxAddres.String(), err)

	bl, err := client.GetBalance("JaxMiner")
	fmt.Println("GetBalance", bl, err)

	//
	account, err := client.ListAccounts()
	fmt.Println("accounts", account, err)
	//fmt.Println("Create wallet")
	//err = client.CreateEncryptedWallet("1R23x73d4")
	//if err != nil {
	//	fmt.Println("Failed to Create wallet", err)
	//	return
	//}
	//
	//

	defer func() {
		if err := client.WalletLock(); err != nil {
			fmt.Println("Can't lock account")
			return
		}
	}()

	//acc, err := client.GetAccount(toAddress)
	//fmt.Println("Account ", acc, err)
	//fmt.Println("Create account")
	//if err = client.CreateNewAccount("MainAccount"); err != nil {
	//	fmt.Println("FAiled to create account ", err)
	//}

	//accAddress, err := client.GetAccountAddress("MainAccount")
	//fmt.Println("Acc address ", accAddress.String(), err)

	balance, err := client.GetBalance("MainAccount")
	fmt.Println("balance", balance, err)

	//info, err := client.GetInfo()
	//fmt.Printf("%+v\n", info)
	v, err := client.Version()
	fmt.Printf("%+v\n", v)

	bi, err := client.GetBlockChainInfo()
	fmt.Printf("%+v\n", bi)
}
