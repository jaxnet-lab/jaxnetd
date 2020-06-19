package main

import (
	"log"
)

const (
	SERVER_HOST       = "You server host"
	SERVER_PORT       = 8444
	USER              = "user"
	PASSWD            = "passwd"
	USESSL            = false
	WALLET_PASSPHRASE = "WalletPassphrase"
)

func main() {
	bc, err := bitcoind.New(SERVER_HOST, SERVER_PORT, USER, PASSWD, USESSL)
	if err != nil {
		log.Fatalln(err)
	}

	//walletpassphrase
	err = bc.WalletPassphrase(WALLET_PASSPHRASE, 3600)
	log.Println(err)

	// backupwallet
	err = bc.BackupWallet("/tmp/wallet.dat")
	log.Println(err)

	// dumpprivkey
	privKey, err := bc.DumpPrivKey("1KU5DX7jKECLxh1nYhmQ7CahY7GMNMVLP3")
	log.Println(err, privKey)

}
