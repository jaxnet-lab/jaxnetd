package main

import (
	"fmt"
	"gitlab.com/jaxnet/core/shard.core.git/btcec"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
)

func main() {
	data, key := GenerateKeyAddress(chain.DefaultChain.Params())
	fmt.Println("res", data, " err: ", key)
}

func GenerateKeyAddress(params *chaincfg.Params) ([]byte, string) {
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		fmt.Printf("failed to make privKey for  %v", err)
	}

	pk := (*btcec.PublicKey)(&key.PublicKey).
		SerializeUncompressed()
	address, err := btcutil.NewAddressPubKeyHash(
		btcutil.Hash160(pk), params)
	keyBytes := key.Serialize()
	//keyHex := hex.EncodeToString(keyBytes)

	fmt.Printf("PrivateKey: %x \n", keyBytes)
	fmt.Printf("Address: %q\n", address.EncodeAddress())

	return keyBytes, address.EncodeAddress()
}

//
//import (
//	"log"
//)
//
//const (
//	SERVER_HOST       = "You server host"
//	SERVER_PORT       = 8444
//	USER              = "user"
//	PASSWD            = "passwd"
//	USESSL            = false
//	WALLET_PASSPHRASE = "WalletPassphrase"
//)
//
//func main() {
//	bc, err := bitcoind.New(SERVER_HOST, SERVER_PORT, USER, PASSWD, USESSL)
//	if err != nil {
//		log.Fatalln(err)
//	}
//
//	//walletpassphrase
//	err = bc.WalletPassphrase(WALLET_PASSPHRASE, 3600)
//	log.Println(err)
//
//	// backupwallet
//	err = bc.BackupWallet("/tmp/wallet.dat")
//	log.Println(err)
//
//	// dumpprivkey
//	privKey, err := bc.DumpPrivKey("1KU5DX7jKECLxh1nYhmQ7CahY7GMNMVLP3")
//	log.Println(err, privKey)
//
//}
