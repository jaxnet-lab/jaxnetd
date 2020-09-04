package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmanager"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmanager/models"
)

func main() {
	app := &App{}
	cliApp := &cli.App{
		Name:   "tx-gatling",
		Usage:  "routine transactions",
		Flags:  app.InitFlags(),
		Before: app.InitCfg,
		Commands: []*cli.Command{
			{
				Name:   "sync",
				Usage:  "fetch UTXO data to CSV file",
				Action: app.SyncUTXO,
				Flags:  app.SyncUTXOFlags(),
			},

			{
				Name:   "send-tx",
				Usage:  "send transactions with values from config file",
				Action: app.SendTx,
			},
		},
		// Action: app.DefaultAction,
	}

	err := cliApp.Run(os.Args)
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
}

type App struct {
	config Config
	client *txmanager.TxMan
}

func (app *App) InitFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "path to configuration",
			Value:   "./config.yaml",
		},
	}
}

func (app *App) InitCfg(c *cli.Context) error {
	var err error
	app.config, err = parseConfig(c.String("config"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	app.client, err = txmanager.NewTxMan(txmanager.ManagerCfg{
		Net:        app.config.Net,
		RPC:        app.config.NodeRPC,
		PrivateKey: app.config.SenderSecret,
	})
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to init client"), 1)
	}
	return nil
}

func (app *App) SyncUTXOFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "address",
			Aliases: []string{"a"},
			Usage:   "filter UTXO by address",
		},
		&cli.Int64Flag{
			Name:    "offset",
			Aliases: []string{"o"},
			Usage:   "offset for starting block height",
		},
		&cli.StringFlag{
			Name:    "data-file",
			Aliases: []string{"f"},
			Value:   "./utxo.csv",
			Usage:   "path for CSV output with results, will override value from config file",
		},
	}
}

func (app *App) SyncUTXO(c *cli.Context) error {
	offset := c.Int64("offset")
	address := c.String("address")
	dataFile := c.String("data-file")

	rows, err := app.client.CollectUTXO(address, offset)
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to collect UTXO"), 1)
	}

	err = storage.NewCSVStorage(dataFile).SaveRows(rows)
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to save UTXO"), 1)
	}

	return nil
}

func (app *App) SendTx(*cli.Context) error {
	repo := storage.NewCSVStorage(app.config.DataFile)
	utxo, err := repo.FetchData()
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to fetch UTXO"), 1)
	}

	provider := txmanager.UTXOFromRows(utxo)
	var sentTxs []models.Transaction
	for _, dest := range app.config.Destinations {
		fmt.Printf("Try to send %d satoshi to %s ", dest.Amount, dest.Address)
		tx, err := app.client.NewTx(dest.Address, dest.Amount, provider)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to send tx"), 1)
		}

		buf := bytes.NewBuffer([]byte{})
		_ = tx.RawTX.Serialize(buf)
		println("serialized tx")
		fmt.Printf("Sent Tx\nHash: %s\nBody: %s\n", tx.TxHash, hex.EncodeToString(buf.Bytes()))

		_, err = app.client.RPC.SendRawTransaction(tx.RawTX, true)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
		}

		sentTxs = append(sentTxs, tx)
	}

	time.Sleep(2 * time.Second)

	for _, tx := range sentTxs {
		hash, _ := chainhash.NewHashFromStr(tx.TxHash)
		txResult, err := app.client.RPC.GetRawTransaction(hash)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to get tx"), 1)
		}

		fmt.Printf("Tx Result: %s | %d \n", txResult.Hash().String(), txResult.Index())
	}

	if err = repo.SaveRows(utxo); err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to save updated UTXO"), 1)
	}

	return nil
}

func (app *App) DefaultAction(c *cli.Context) error {

	return nil
}
