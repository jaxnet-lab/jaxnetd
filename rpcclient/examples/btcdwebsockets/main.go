// Copyright (c) 2014-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
	"log"
	"time"

	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/rpcclient"
)

func main() {
	// Only override the handlers for notifications you care about.
	// Also note most of these handlers will only be called if you register
	// for notifications.  See the documentation of the rpcclient
	// NotificationHandlers type for more details about each handler.
	ntfnHandlers := rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header chain.BlockHeader, txns []*btcutil.Tx) {
			log.Printf("Block connected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp())
		},
		OnFilteredBlockDisconnected: func(height int32, header chain.BlockHeader) {
			log.Printf("Block disconnected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp())
		},
	}

	// Connect to local btcd RPC server using websockets.
	//btcdHomeDir := btcutil.AppDataDir("btcd", false)
	//certs, err := ioutil.ReadFile(filepath.Join(btcdHomeDir, "rpc.cert"))
	//if err != nil {
	//	log.Fatal(err)
	//}
	connCfg := &rpcclient.ConnConfig{
		Endpoint:   "ws",
		Host:       "0.0.0.0:8334",
		User:       "somerpc",
		Pass:       "somerpc",
		DisableTLS: true, // Bitcoin core does not provide TLS by default
		//Certificates: certs,
	}
	client, err := rpcclient.New(connCfg, &ntfnHandlers)
	if err != nil {
		log.Fatal(err)
	}

	// Register for block connect and disconnect notifications.
	if err := client.NotifyBlocks(); err != nil {
		log.Fatal(err)
	}
	log.Println("NotifyBlocks: Registration Complete")

	// Get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Block count: %d", blockCount)

	// For this example gracefully shutdown the client after 10 seconds.
	// Ordinarily when to shutdown the client is highly application
	// specific.
	log.Println("Client shutdown in 10 seconds...")
	time.AfterFunc(time.Second*10, func() {
		log.Println("Client shutting down...")
		client.Shutdown()
		log.Println("Client shutdown complete.")
	})

	v, err := client.Version()
	fmt.Println(v, err)

	balance, err := client.GetBalance("mijhw2WHeqgimoTqoKMWSCRVs8XFXxk9qx")
	fmt.Println(balance, err)

	//client.EstimateSmartFee(1, )

	// Wait until the client either shuts down gracefully (or the user
	// terminates the process with Ctrl+C).
	client.WaitForShutdown()
}
