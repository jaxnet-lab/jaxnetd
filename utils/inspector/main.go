/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/davecgh/go-spew/spew"
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
)

func interruptOnError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
func prettyPrint(val interface{}) {
	data, _ := json.MarshalIndent(val, "", "  ")
	fmt.Println(string(data))
}

func main() {

	// 64.225.64.8 alias=node-ams3 net=testnet
	// 128.199.31.83 alias=node-blr1 net=testnet
	// 161.35.220.11 alias=node-fra1 net=testnet
	// 138.68.152.77 alias=node-lon1 net=testnet
	// 159.89.89.207 alias=node-nyc1 net=testnet
	// 164.90.150.0 alias=node-sfo3 net=testnet
	// 128.199.64.36 alias=node-sgp1 net=testnet

	connCfg := &rpcclient.ConnConfig{
		// shardID: 42,
		Params: "fastnet",
		Pass:   "somerpc",
		User:   "somerpc",
		Host:   "127.0.0.1:18333",
		// Host:         "128.199.64.36:18333",
		// User:         "jaxnetrpc",
		// Pass:         "AUL6VBjoQnhP3bfFzl",
		HTTPPostMode: true,
		DisableTLS:   true,
	}

	rpcClient, err := rpcclient.New(connCfg, nil)
	interruptOnError(err)
	rpcClient = rpcClient

	resp, err := rpcClient.ListShards()
	interruptOnError(err)

	for i := int64(0); i < 10; i++ {
		block, err := rpcClient.ForBeacon().GetBeaconBlockBySerialNumber(i)
		if err != nil {
			fmt.Println(err)
			break
		}

		spew.Dump(block)
	}

	for _, info := range resp.Shards {
		fmt.Println("Shard #", info.ID)
		for i := int64(0); ; i++ {
			block, err := rpcClient.ForShard(info.ID).GetShardBlockBySerialNumber(i)
			if err != nil {
				fmt.Println(err)
				break
			}
			spew.Dump(block)
		}
	}
}
