// Copyright (c) 2014-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"

	"gitlab.com/jaxnet/core/shard.core.git/rpcclient"
)

func main() {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         "116.202.107.209:8334",
		User:         "somerpc",
		Pass:         "somerpc",
		HTTPPostMode: true,  // Bitcoin core only supports HTTP POST mode
		DisableTLS:   false, // Bitcoin core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown()

	// Get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Block count: %d", blockCount)

	balance, err := client.GetBalance("mijhw2WHeqgimoTqoKMWSCRVs8XFXxk9qx")
	fmt.Println(balance, err)
	info, err := client.GetInfo()
	fmt.Printf("%+v\n", info)
	v, err := client.Version()
	fmt.Printf("%+v\n", v)

	bi, err := client.GetBlockChainInfo()
	fmt.Printf("%+v\n", bi)
}
