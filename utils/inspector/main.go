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

	"gitlab.com/jaxnet/core/shard.core/network/rpcclient"
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
	connCfg := &rpcclient.ConnConfig{
		// shardID: 42,
		Params: "fastnet",
		Pass:   "somerpc",
		User:   "somerpc",
		Host:   "127.0.0.1:18333",
		// Host: "159.69.117.159:12333",
		// Host: "198.199.125.197:18333",
		// Host: "116.202.107.209:22333",
		// User: "jaxnetrpc",
		// Pass: "AUL6VBjoQnhP3bfFzl",
		HTTPPostMode: true,
		DisableTLS:   true,
	}

	rpcClient, err := rpcclient.New(connCfg, nil)
	interruptOnError(err)
	rpcClient = rpcClient

	resp, err := rpcClient.ListShards()
	interruptOnError(err)

	for i := int64(0); ; i++ {
		block, err := rpcClient.ForBeacon().GetBeaconBlockBySerialNumber(i)
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Printf("Beacon Block: Hash=%s Height=%d SerialID=%d PrevSerialID=%d \n",
			"", block.Height, block.SerialID, block.PrevSerialID)
	}

	for _, info := range resp.Shards {
		fmt.Println("Shard #", info.ID)
		for i := int64(0); ; i++ {
			block, err := rpcClient.ForShard(info.ID).GetShardBlockBySerialNumber(i)
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Printf("Shard %d Block: Hash=%s Height=%d SerialID=%d PrevSerialID=%d \n",
				info.ID, "", block.Height, block.SerialID, block.PrevSerialID)
		}
	}
}
