// Copyright (c) 2014-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	"gitlab.com/jaxnet/core/shard.core/btcec"
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/network/rpcclient"
	"gitlab.com/jaxnet/core/shard.core/node/chain"
	"gitlab.com/jaxnet/core/shard.core/txscript"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type Transaction struct {
	TxId               string `json:"txid"`
	SourceAddress      string `json:"source_address"`
	DestinationAddress string `json:"destination_address"`
	Amount             int64  `json:"amount"`
	UnsignedTx         string `json:"unsignedtx"`
	SignedTx           string `json:"signedtx"`
}

func CreateTransaction(destination string, amount int64, txHash string, netParams *chaincfg.Params) (*wire.MsgTx, Transaction, error) {
	var transaction Transaction

	pkBytes, err := hex.DecodeString("679887f4c91a1f509fd0ddd8365d3377675027982c16b5605fd2e9c198981ad1")
	if err != nil {
		return nil, Transaction{}, err
	}
	privKey, pb := btcec.PrivKeyFromBytes(btcec.S256(), pkBytes)

	addresspubkey, _ := btcutil.NewAddressPubKey(pb.SerializeUncompressed(), netParams)

	sourceUtxoHash, _ := chainhash.NewHashFromStr(txHash)

	destinationAddress, err := btcutil.DecodeAddress(destination, &chaincfg.MainNetParams)
	sourceAddress, err := btcutil.DecodeAddress(addresspubkey.EncodeAddress(), netParams)
	if err != nil {
		return nil, Transaction{}, err
	}
	destinationPkScript, _ := txscript.PayToAddrScript(destinationAddress)
	sourcePkScript, _ := txscript.PayToAddrScript(sourceAddress)
	redeemTx := wire.NewMsgTx(wire.TxVersion)
	prevOut := wire.NewOutPoint(sourceUtxoHash, 0)
	redeemTxIn := wire.NewTxIn(prevOut, nil, nil)
	redeemTx.AddTxIn(redeemTxIn)
	redeemTxOut := wire.NewTxOut(amount, destinationPkScript)
	redeemTx.AddTxOut(redeemTxOut)
	sigScript, err := txscript.SignatureScript(redeemTx, 0, sourcePkScript, txscript.SigHashAll, privKey, false)
	if err != nil {
		return nil, Transaction{}, err
	}
	redeemTx.TxIn[0].SignatureScript = sigScript
	flags := txscript.StandardVerifyFlags
	vm, err := txscript.NewEngine(sourcePkScript, redeemTx, 0, flags, nil, nil, amount)
	if err != nil {
		return nil, Transaction{}, err
	}
	if err := vm.Execute(); err != nil {
		return nil, Transaction{}, err
	}
	var unsignedTx bytes.Buffer
	var signedTx bytes.Buffer
	redeemTx.Serialize(&signedTx)

	transaction.UnsignedTx = hex.EncodeToString(unsignedTx.Bytes())
	transaction.Amount = amount
	transaction.SignedTx = hex.EncodeToString(signedTx.Bytes())
	transaction.SourceAddress = sourceAddress.EncodeAddress()
	transaction.DestinationAddress = destinationAddress.EncodeAddress()
	return redeemTx, transaction, nil
}

func main() {
	ch := chain.BeaconChain
	trx, transaction, err := CreateTransaction("1KKKK6N21XKo48zWKuQKXdvSsCf95ibHFa", 1000, "ca0b8007c6a5751f6fdabc0c9341b75940914c14172c57a91338bbbba1e95f3d", ch.Params())
	if err != nil {
		return
	}
	data, _ := json.Marshal(transaction)
	fmt.Println(string(data))

	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         "0.0.0.0:8334",
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

	// trx, err := CreateTrx("1KKKK6N21XKo48zWKuQKXdvSsCf95ibHFa", 1, "ca0b8007c6a5751f6fdabc0c9341b75940914c14172c57a91338bbbba1e95f3d")

	// data2, _ := hex.DecodeString(transaction.SignedTx)
	// client.SendRawTransaction()
	// trx, err := client.DecodeRawTransaction(data2)
	// fmt.Println(trx, err)
	//
	h, err := client.SendRawTransaction(trx, true)
	fmt.Println("SendRawTransaction ", h, err)

	// // Get the current block count.
	// blockCount, err := client.GetBlockCount()
	// if err != nil {
	//	log.Fatal(err)
	//	return
	// }
	// log.Printf("Block count: %d", blockCount)

	// secret, err := btcec.NewPrivateKey(btcec.S256())
	// if err != nil {
	//	return
	// }
	// wif, err := btcutil.NewWIF(secret, &chaincfg.JaxNetParams, true)
	// fmt.Println()
	// fmt.Println(string(wif.SerializePubKey()))

	//
	//
	// addr, err := btcutil.NewAddressPubKey(wif.PrivKey.PubKey().SerializeCompressed(), &chaincfg.JaxNetParams)
	// fmt.Println(addr.String(), addr.AddressPubKeyHash())
	//

	// btcutil.

	// params := &chaincfg.JaxNetParams
	// params.Name = "jaxnet"
	// params.Net = types.BitcoinNet(0x12121212)
	// params.PubKeyHashAddrID = byte(0x6F)
	// params.PrivateKeyID = byte(0x80)
	//

	// fmt.Println(params.Net, activeNetParams.Net)
	// fmt.Println(params.PubKeyHashAddrID, activeNetParams.PubKeyHashAddrID)
	// fmt.Println(params.PrivateKeyID, activeNetParams.PrivateKeyID)
	// 1CzWYLMRUt2dUnvirRtS68vQjgVjYDxsZh
	// jaxAddres, err := btcutil.DecodeAddress("15VS71vPzPMxY5fXwJUVkqyHge94g4Uub4", &chaincfg.JaxNetParams)
	//
	// //fmt.Println("ImportAddress")
	// //if err := client.ImportAddress("1CzWYLMRUt2dUnvirRtS68vQjgVjYDxsZh"); err != nil{
	// //	fmt.Println(err)
	// //}
	//
	// fmt.Println("Unlock wallet ")
	// if err := client.WalletPassphrase("1R23x73d4", 100); err != nil {
	//	fmt.Println("Cant create wallet", err)
	//	return
	// }
	//
	// acc, err := client.GetAccount(jaxAddres)
	// fmt.Println("Get account: ", acc, err)
	// //if err!= nil{
	// //	fmt.Println(err)
	// //}
	//
	// //if err:= client.ImportPrivKey(wif); err != nil{
	// //	fmt.Println("Can't import KPK", err)
	// //}
	//
	// //if err := client.SetAccount(jaxAddres, "JaxMain"); err != nil {
	// //	fmt.Println("Error SetAccount", err)
	// //}
	// //if err := client.CreateNewAccount("JaxMiner"); err != nil {
	// //	fmt.Println(err)
	// //}
	//
	// //if err := client.ImportPrivKey(wif); err != nil {
	// //	fmt.Println("Import key err", err)
	// //}
	//
	// accs, err := client.ListAccounts()
	// fmt.Println("Accounts", accs)
	//
	// err = client.SetTxFee(0)
	// fmt.Println("SetTxFee", err)
	//
	// //ok, err := client.SendFrom("default", jaxAddres, 0)
	// //fmt.Println("Move trx ", ok, err)
	//
	// aaddr, err := client.GetAccountAddress("JaxMiner")
	// fmt.Println("JaxMiner address: ", aaddr, err)
	//
	// //def, err := client.GetAccountAddress("default")
	// //fmt.Println("default address: ", def, err)
	// //
	// //imp, err := client.GetAccountAddress("imported")
	// //fmt.Println("imported address: ", imp, err)
	//
	// //fmt.Println(toAddress, err)
	// aa, err := client.GetReceivedByAddress(jaxAddres)
	// fmt.Println("GetReceivedByAddress", aa, jaxAddres.String(), err)
	//
	// bl, err := client.GetBalance("JaxMiner")
	// fmt.Println("GetBalance", bl, err)
	//
	// toAddr, _ := client.GetAccountAddress("MainAccount")
	//
	// amount, _ := btcutil.NewAmount(0.0001)
	// h, err := client.SendFrom("JaxMiner", toAddr, amount)
	// fmt.Println("Send result: ", h, err)
	// //
	// //account, err := client.ListAccounts()
	// //fmt.Println("accounts", account, err)
	// //fmt.Println("Create wallet")
	// //err = client.CreateEncryptedWallet("1R23x73d4")
	// //if err != nil {
	// //	fmt.Println("Failed to Create wallet", err)
	// //	return
	// //}
	// //
	// //
	//
	// defer func() {
	//	if err := client.WalletLock(); err != nil {
	//		fmt.Println("Can't lock account")
	//		return
	//	}
	// }()
	//
	// //acc, err := client.GetAccount(toAddress)
	// //fmt.Println("Account ", acc, err)
	// //fmt.Println("Create account")
	// //if err = client.CreateNewAccount("MainAccount"); err != nil {
	// //	fmt.Println("FAiled to create account ", err)
	// //}
	//
	// //accAddress, err := client.GetAccountAddress("MainAccount")
	// //fmt.Println("Acc address ", accAddress.String(), err)
	//
	// balance, err := client.GetBalance("MainAccount")
	// fmt.Println("balance", balance, err)
	//
	// //info, err := client.GetInfo()
	// //fmt.Printf("%+v\n", info)
	// v, err := client.Version()
	// fmt.Printf("%+v\n", v)
	//
	// bi, err := client.GetBlockChainInfo()
	// fmt.Printf("%+v\n", bi)
}
