/*
 * Copyright (c) 2022 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package main

import (
	"fmt"
	"log"
	"os"
	"sort"

	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"

	"github.com/BurntSushi/toml"
	"github.com/olekukonko/tablewriter"
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
)

type Config struct {
	Chain string       `toml:"chain"`
	Nodes []ConnParams `toml:"nodes"`
}

type ConnParams struct {
	Name     string `toml:"name"`
	Address  string `toml:"address"`
	User     string `toml:"user"`
	Password string `toml:"pass"`
}

func main() {
	cfg := new(Config)
	_, err := toml.DecodeFile("./config.toml", cfg)
	checkError(err)

	rows := make([][]string, 0, len(cfg.Nodes)*4)
	emptyRow := []string{
		"N/A",
		"N/A",
		"N/A",
		"N/A",
		"N/A",
	}
	for _, node := range cfg.Nodes {
		rpc, err := rpcclient.New(&rpcclient.ConnConfig{
			Params:       cfg.Chain,
			Host:         node.Address,
			User:         node.User,
			Pass:         node.Password,
			DisableTLS:   true,
			HTTPPostMode: true,
		}, nil)
		checkError(err)

		shardList, err := rpc.ListShards()
		if err != nil {
			rows = append(rows,
				append(
					[]string{fmt.Sprintf("%s(%s)", node.Name, node.Address)},
					emptyRow...),
			)
			continue
		}

		var chains = []uint32{0}
		for shardID := range shardList.Shards {
			chains = append(chains, shardID)
		}
		sort.Slice(chains, func(i, j int) bool { return chains[i] < chains[j] })

		for _, shardID := range chains {
			bestHash, bestHeight, err := rpc.ForShard(shardID).GetBestBlock()
			if err != nil {
				rows = append(rows,
					append(
						[]string{fmt.Sprintf("%s(%s)", node.Name, node.Address)},
						emptyRow...),
				)
				continue
			}

			lastSerialID, err := rpc.ForShard(shardID).GetLastSerialBlockNumber()
			if err != nil {
				rows = append(rows,
					append(
						[]string{fmt.Sprintf("%s(%s)", node.Name, node.Address)},
						emptyRow...),
				)
				continue
			}

			template, err := rpc.ForShard(shardID).GetBlockTemplate(&jaxjson.TemplateRequest{
				Mode:         "template",
				Capabilities: []string{"coinbasetxn"},
			})

			hasTemplate := err == nil && template != nil

			var nextBlockReward int64
			var nextHeight int64
			if template != nil && template.CoinbaseValue != nil {
				nextBlockReward = *template.CoinbaseValue
			}
			if template != nil {
				nextHeight = template.Height
			}

			rows = append(rows,
				[]string{
					fmt.Sprintf("%s(%s)", node.Name, node.Address),
					fmt.Sprintf("chain_%d", shardID),
					bestHash.String(),
					fmt.Sprintf("%d [%d]", bestHeight, lastSerialID),
					fmt.Sprintf("%v", hasTemplate),
					fmt.Sprintf("%d / %d", nextHeight, nextBlockReward),
				})

		}

	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Node",
		"Chain",
		"LastBlock",
		"Height",
		"Has Job",
		"Next Height/Reward"})
	table.SetAutoMergeCells(true)
	table.SetRowLine(true)
	table.AppendBulk(rows)
	table.Render()
}

func checkError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}
