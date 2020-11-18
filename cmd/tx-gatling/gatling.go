// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txutils"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
)

func main() {
	app := &App{}
	cliApp := &cli.App{
		Name:     "tx-gatling",
		Usage:    "routine transactions",
		Flags:    app.InitFlags(),
		Before:   app.InitCfg,
		Commands: app.getCommands(),
		// Action: app.DefaultAction,
	}

	err := cliApp.Run(os.Args)
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
}

func (app *App) getCommands() cli.Commands {
	return []*cli.Command{
		{
			Name:   "sync",
			Usage:  "fetch UTXO data to CSV file",
			Action: app.SyncUTXOCmd,
			Flags:  app.SyncUTXOFlags(),
		},

		{
			Name:   "send-tx",
			Usage:  "send transactions with values from config file",
			Action: app.SendTxCmd,
		},
		{
			Name:   "multisig-tx",
			Usage:  "creates new 2of2 multi sig transaction",
			Flags:  app.NewMultiSigTxFlags(),
			Action: app.NewMultiSigTxCmd,
		},
		{
			Name:   "multisig-address",
			Usage:  "creates new 2of2 multi sig address and redeem script",
			Flags:  app.NewMultiSigTxFlags(),
			Action: app.NewMultiSigAddressCmd,
		},
		{
			Name:   "add-signature",
			Usage:  "adds signature to transaction with the multiSig inputs",
			Flags:  app.AddSignatureToTxFlags(),
			Action: app.AddSignatureToTxCmd,
		},

		{
			Name:   "spend-utxo",
			Usage:  "create new transaction with single UTXO as input",
			Flags:  app.SpendUTXOFlags(),
			Action: app.SpendUTXOCmd,
		},
		{
			Name:  "decode",
			Usage: "fetch UTXO data to CSV file",
			Subcommands: cli.Commands{
				{
					Name:  "tx",
					Usage: "decode hex encoded transaction body",
					Flags: []cli.Flag{
						getFlags()[flagTxBody],
					},
					Action: func(c *cli.Context) error {
						txBody := c.String(flagTxBody)

						tx, err := txutils.DecodeTx(txBody)
						if err != nil {
							return cli.NewExitError(err, 1)
						}

						jsonTx := txutils.TxToJson(tx, app.config.NetParams())
						data, _ := json.MarshalIndent(jsonTx, "", "  ")
						println(string(data))
						return nil
					},
				},
				{
					Name:  "script",
					Usage: "decode hex encoded redeem script",
					Flags: []cli.Flag{
						getFlags()[flagRedeemScript],
					},
					Action: func(c *cli.Context) error {
						script := c.String(flagRedeemScript)

						decodedScript, err := hex.DecodeString(script)
						if err != nil {
							return cli.NewExitError(err, 1)
						}

						result, err := txutils.DecodeScript(decodedScript, app.config.NetParams())
						if err != nil {
							return cli.NewExitError(err, 1)
						}

						data, _ := json.MarshalIndent(result, "", "  ")
						println(string(data))
						return nil
					},
				},
			},
		},
	}
}

type App struct {
	config Config
	txutils.Operator
	shardID uint32
}

func (app *App) InitFlags() []cli.Flag {
	flags := getFlags()
	return []cli.Flag{
		flags[flagConfig],
		flags[flagDataFile],
		flags[flagSecretKey],
		flags[flagShard],
	}
}

func (app *App) InitCfg(c *cli.Context) error {
	var err error
	app.config, err = parseConfig(c.String(flagConfig))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	dataFile := c.String(flagDataFile)
	if dataFile != "" {
		app.config.DataFile = dataFile
	}

	secret := c.String(flagSecretKey)
	if dataFile != "" {
		app.config.SenderSecret = secret
	}

	shardID := c.Uint64(flagShard)
	if dataFile != "" {
		app.config.SenderSecret = secret
	}

	// todo cleanup
	app.shardID = uint32(shardID)
	app.config.ShardID = app.shardID
	app.Operator, err = txutils.NewOperator(txutils.ManagerCfg{
		Net:        app.config.Net,
		ShardID:    uint32(shardID),
		RPC:        app.config.NodeRPC,
		PrivateKey: app.config.SenderSecret,
	})
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to init TxMan"), 1)
	}
	return nil
}

func (app *App) SyncUTXOFlags() []cli.Flag {
	flags := getFlags()

	return []cli.Flag{
		flags[flagAddress],
		flags[flagOffset],
		flags[flagShards],
		flags[flagSplitFiles],
	}
}

func (app *App) SyncUTXOCmd(c *cli.Context) error {
	offset := c.Int64(flagOffset)
	address := c.String(flagAddress)
	dataFile := c.String(flagDataFile)
	shards := c.Int64Slice(flagShards)
	splitFiles := c.Bool(flagSplitFiles)

	fmt.Println("Start collecting...")

	opts := txutils.UTXOCollectorOpts{
		Offset:          offset,
		FilterAddresses: []string{address},
	}
	if len(shards) > 0 && shards[0] == -1 {
		opts.AllChains = true
	} else {
		for _, shard := range shards {
			opts.Shards = append(opts.Shards, uint32(shard))
		}
	}

	set, lastBlock, err := app.TxMan.CollectUTXOs(opts)
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to collect UTXO"), 1)
	}

	if splitFiles {
		for u, rows := range set {
			err = storage.NewCSVStorage(fmt.Sprintf("chain-%d-%s", u, dataFile)).SaveRows(rows)
			if err != nil {
				return cli.NewExitError(errors.Wrap(err, "unable to save UTXO"), 1)
			}
			fmt.Printf("\nFound %d UTXOs for <%s> in blocks[%d, %d]\n", len(rows), address, offset, lastBlock)
		}
	} else {
		var allRows txmodels.UTXORows
		for _, rows := range set {
			allRows = append(allRows, rows...)
		}

		fmt.Printf("\nFound %d UTXOs for <%s> in blocks[%d, %d]\n", len(allRows), address, offset, lastBlock)
		err = storage.NewCSVStorage(dataFile).SaveRows(allRows)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to save UTXO"), 1)
		}
	}

	return nil
}

func (app *App) SendTxCmd(*cli.Context) error {
	repo := storage.NewCSVStorage(app.config.DataFile)
	utxo, err := repo.FetchData()
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to fetch UTXO"), 1)
	}

	provider := txutils.UTXOFromRows(utxo)
	var sentTxs []txmodels.Transaction
	for _, dest := range app.config.Destinations {
		fmt.Println("")

		fmt.Printf("Try to send %d satoshi to %s ", dest.Amount, dest.Address)
		tx, err := app.TxMan.NewTx(dest.Address, dest.Amount, provider)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to send tx"), 1)
		}

		fmt.Printf("Sent Tx\nHash: %s\nBody: %s\n", tx.TxHash, tx.SignedTx)

		_, err = app.TxMan.RPC().ForShard(app.shardID).SendRawTransaction(tx.RawTX, true)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
		}

		sentTxs = append(sentTxs, *tx)
	}

	fmt.Println("")

	time.Sleep(2 * time.Second)

	for _, tx := range sentTxs {
		hash, _ := chainhash.NewHashFromStr(tx.TxHash)
		txResult, err := app.TxMan.RPC().ForShard(app.shardID).GetTxOut(hash, 0, true)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to get tx"), 1)
		}

		confirmations := int64(-1)
		if txResult != nil {
			confirmations = txResult.Confirmations
		}

		fmt.Printf("Tx Result: %s | %d |\n", tx.TxHash, confirmations)
	}

	if err = repo.SaveRows(utxo); err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to save updated UTXO"), 1)
	}

	return nil
}

func (app *App) NewMultiSigTxFlags() []cli.Flag {
	flags := getFlags()
	return []cli.Flag{
		flags[flagFirstPubKey],
		flags[flagSecondPubKey],
		flags[flagAmount],
		flags[flagSendTx],
	}
}

func (app *App) NewMultiSigTxCmd(c *cli.Context) error {
	firstRecipient := c.String(flagFirstPubKey)
	secondRecipient := c.String(flagSecondPubKey)
	amount := c.Int64(flagAmount)
	send := c.Bool(flagSendTx)

	mAddr, err := app.TxMan.NewMultiSig2of2Address(firstRecipient, secondRecipient)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("Craft new mustisig Address\nAddress: %s\nRedeemScript: %s\n", mAddr.Address, mAddr.RedeemScript)

	signer, err := txutils.NewKeyData(app.config.SenderSecret, app.config.NetParams())
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	repo := storage.NewCSVStorage(app.config.DataFile)
	utxo, err := repo.FetchData()
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to fetch UTXO"), 1)
	}

	tx, err := app.TxMan.WithKeys(signer).NewTx(mAddr.Address, amount, txutils.UTXOFromRows(utxo))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("Craft new Tx\nHash: %s\nBody: %s\n", tx.TxHash, tx.SignedTx)

	if send {
		_, err = app.TxMan.RPC().ForShard(app.shardID).SendRawTransaction(tx.RawTX, true)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
		}
		fmt.Printf("Tx Sent: %s\n", tx.TxHash)
	}

	return nil
}

func (app *App) NewMultiSigAddressCmd(c *cli.Context) error {
	firstRecipient := c.String(flagFirstPubKey)
	secondRecipient := c.String(flagSecondPubKey)
	mAddr, err := app.TxMan.NewMultiSig2of2Address(firstRecipient, secondRecipient)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("Craft new mustisig Address\nAddress: %s\nRedeemScript: %s\n", mAddr.Address, mAddr.RedeemScript)
	return nil
}

func (app *App) AddSignatureToTxFlags() []cli.Flag {
	flags := getFlags()
	return []cli.Flag{
		flags[flagTxBody],
		flags[flagRedeemScript],
		flags[flagSendTx],
	}
}

func (app *App) AddSignatureToTxCmd(c *cli.Context) error {
	txBody := c.String(flagTxBody)
	script := c.String(flagRedeemScript)
	send := c.Bool(flagSendTx)

	signer, err := txutils.NewKeyData(app.config.SenderSecret, app.config.NetParams())
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	tx, err := app.AddSignatureToTx(*signer, txBody, script)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("Add signature to Tx\nHash: %s\nBody: %s\n", tx.TxHash, tx.SignedTx)

	if send {
		_, err = app.TxMan.RPC().ForShard(app.shardID).SendRawTransaction(tx.RawTX, true)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
		}
		fmt.Printf("Tx Sent: %s\n", tx.TxHash)

	}

	return nil
}

func (app *App) SpendUTXOFlags() []cli.Flag {
	flags := getFlags()
	return []cli.Flag{
		flags[flagTxHash],
		flags[flagAddress],
		flags[flagOutIn],
		flags[flagAmount],
		flags[flagAmount],
	}
}

func (app *App) SpendUTXOCmd(c *cli.Context) error {
	txHash := c.String(flagTxHash)
	destination := c.String(flagAddress)
	outIndex := c.Uint64(flagOutIn)
	amount := c.Int64(flagAmount)
	send := c.Bool(flagAmount)

	signer, err := txutils.NewKeyData(app.config.SenderSecret, app.config.NetParams())
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	tx, err := app.SpendUTXO(*signer, txHash, uint32(outIndex), destination, amount)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("Craft new Tx\nHash: %s\nBody: %s\n", tx.TxHash, tx.SignedTx)

	if send {
		_, err = app.TxMan.RPC().ForShard(app.shardID).SendRawTransaction(tx.RawTX, true)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
		}
		fmt.Printf("Tx Sent: %s\n", tx.TxHash)
	}

	return nil
}
