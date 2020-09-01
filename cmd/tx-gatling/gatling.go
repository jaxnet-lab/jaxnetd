package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/manager"
	"gopkg.in/yaml.v3"
)

type Destination struct {
	Address string `yaml:"address"`
	Amount  int64  `yaml:"amount"`
}

type Config struct {
	Net           string          `yaml:"net"`
	NodeRPC       manager.NodeRPC `yaml:"node_rpc"`
	SyncData      bool            `yaml:"sync_data"`
	DataFile      string          `yaml:"data_file"`
	Append        bool            `yaml:"append"`
	SenderSecret  string          `yaml:"sender_secret"`
	SenderAddress string          `yaml:"sender_address"`
	Destinations  []Destination   `yaml:"destinations"`
}

func parseConfiguration() Config {
	path := flag.String("config", "./config.yaml", "path to configuration file")
	flag.Parse()

	rawFile, err := ioutil.ReadFile(*path)
	if err != nil {
		exitError("Unable to read configuration: ", err)
	}

	cfg := Config{}
	if err = yaml.Unmarshal(rawFile, &cfg); err != nil {
		exitError("Unable to decode configuration: ", err)
	}

	return cfg
}

func exitError(msg string, err error) {
	fmt.Println("FATAL: ", msg, err)
	os.Exit(1)
	return
}

func main() {

	// runner.UpNode()

	// config := parseConfiguration()
	config := Config{
		Net: "testnet",
		NodeRPC: manager.NodeRPC{
			Host: "127.0.0.1:18334",
			User: "somerpc",
			Pass: "somerpc",
		},
		// SyncData:      true,
		DataFile:      "miner_utxo_1.csv",
		Append:        false,
		SenderSecret:  "9e3f8b9b5e3698fa5a8b4b927eecec88fc36a91c14963d89e41747d1acf89dfb",
		SenderAddress: "mg6co2k1wBug21jvCVbm8YoFfSmDQxMwZW",
		Destinations: []Destination{
			{Address: "n2aB8oW7piSBTeZ8xZhknssnAddV4RVbiM", Amount: 125_0000_0000},
			{Address: "n43KSoTrhQRoCBpAM8b7B6eWqB7PT2QpHG", Amount: 75_0000_0000},
			{Address: "mpi7Qb6uRREWEzgFTryijxY1AomaJU8gkt", Amount: 100_0000_0000},
		},
	}

	client, err := manager.NewTxMan(manager.ClientCfg{
		Net:        config.Net,
		RPC:        config.NodeRPC,
		SyncData:   config.SyncData,
		DataFile:   config.DataFile,
		Append:     config.Append,
		PrivateKey: config.SenderSecret,
	})
	if err != nil {
		exitError("unable to init client", err)
		return
	}

	defer client.Shutdown()
	if config.SyncData {
		err = client.CollectUTXO(config.SenderAddress)
		if err != nil {
			exitError("unable to collect UTXO", err)
			return
		}
	}
	var sentTxs []manager.Transaction
	for _, dest := range config.Destinations {
		msgTx, tx, err := client.CreateTransaction(dest.Address, dest.Amount)
		if err != nil {
			exitError("unable to send tx", err)
			return
		}
		sentTxs = append(sentTxs, tx)
		hash := msgTx.TxHash()
		fmt.Printf("Sent new tx ( %s ): %+v", hash, tx)
	}

	for _, tx := range sentTxs {
		hash, _ := chainhash.NewHashFromStr(tx.TxId)
		txResult, err := client.GetTransaction(hash)
		if err != nil {
			exitError("unable to get tx", err)
			return
		}

		fmt.Printf("Tx Result: %+v", txResult)
	}

}
