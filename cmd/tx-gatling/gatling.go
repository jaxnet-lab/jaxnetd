package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmanager"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmanager/models"
	"gopkg.in/yaml.v3"
)

type Destination struct {
	Address string `yaml:"address"`
	Amount  int64  `yaml:"amount"`
}

type Config struct {
	Net             string            `yaml:"net"`
	NodeRPC         txmanager.NodeRPC `yaml:"node_rpc"`
	SyncData        bool              `yaml:"sync_data"`
	DataFile        string            `yaml:"data_file"`
	SyncStartHeight int64             `yaml:"sync_start_height"`
	SenderSecret    string            `yaml:"sender_secret"`
	SenderAddress   string            `yaml:"sender_address"`
	Destinations    []Destination     `yaml:"destinations"`
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
	fmt.Println("FATAL:", msg+" ->", err)
	os.Exit(1)
	return
}

func main() {
	// config := parseConfiguration()
	config := Config{
		Net: "testnet",
		NodeRPC: txmanager.NodeRPC{
			Host: "127.0.0.1:18334",
			User: "somerpc",
			Pass: "somerpc",
		},
		// SyncData:        true,
		DataFile:        "miner_utxo.csv",
		SyncStartHeight: 14694,
		SenderSecret:    "9e3f8b9b5e3698fa5a8b4b927eecec88fc36a91c14963d89e41747d1acf89dfb",
		SenderAddress:   "mg6co2k1wBug21jvCVbm8YoFfSmDQxMwZW",
		Destinations: []Destination{
			{Address: "n2aB8oW7piSBTeZ8xZhknssnAddV4RVbiM", Amount: 125_0000_0000},
			{Address: "n43KSoTrhQRoCBpAM8b7B6eWqB7PT2QpHG", Amount: 75_0000_0000},
			{Address: "mpi7Qb6uRREWEzgFTryijxY1AomaJU8gkt", Amount: 100_0000_0000},
		},
	}

	// runner.UpNode()

	client, err := txmanager.NewTxMan(txmanager.ManagerCfg{
		Net:        config.Net,
		RPC:        config.NodeRPC,
		PrivateKey: config.SenderSecret,
	})
	if err != nil {
		exitError("unable to init client", err)
		return
	}

	if config.SyncData {
		rows, err := client.CollectUTXO(config.SenderAddress)
		if err != nil {
			exitError("unable to collect UTXO", err)
			return
		}
		err = storage.NewCSVStorage(config.DataFile).SaveRows(rows)
		if err != nil {
			exitError("unable to save UTXO", err)
			return
		}
		return
	}

	var sentTxs []models.Transaction
	for _, dest := range config.Destinations {
		tx, err := client.NewTx(dest.Address, dest.Amount,
			txmanager.UTXOFromCSV(config.DataFile))
		if err != nil {
			exitError("unable to send tx", err)
			return
		}
		sentTxs = append(sentTxs, tx)
		fmt.Printf("Sent new tx: %s ", tx.TxHash)
	}

	for _, tx := range sentTxs {
		hash, _ := chainhash.NewHashFromStr(tx.TxHash)
		txResult, err := client.RPC.GetTransaction(hash)
		if err != nil {
			exitError("unable to get tx", err)
			return
		}

		fmt.Printf("Tx Result: %+v", txResult)
	}

}
