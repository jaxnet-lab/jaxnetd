package main

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txutils"
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
				Action: app.SyncUTXOCmd,
				Flags:  app.SyncUTXOFlags(),
			},

			{
				Name:   "send-tx",
				Usage:  "send transactions with values from config file",
				Action: app.SendTxCmd,
			},
			// {
			// 	Name:   "multisig-tx",
			// 	Usage:  "creates new 2of2 multi sig transaction",
			// 	Flags:  app.NewMultiSigTxFlags(),
			// 	Action: app.NewMultiSigTxCmd,
			// },
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
	txutils.Operator
}

func (app *App) InitFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   "./config.yaml",
			Usage:   "path to configuration",
		},
		&cli.StringFlag{
			Name:    "data-file",
			Aliases: []string{"f"},
			Value:   "",
			EnvVars: []string{"TX_DATA_FILE"},
			Usage:   "path to CSV input/output, will override value from config file",
		},
		&cli.StringFlag{
			Name:    "secret-key",
			Aliases: []string{"k"},
			Value:   "",
			EnvVars: []string{"TX_SECRET_KEY"},
			Usage:   "secret key for signing actions, will override value from config file",
		},
	}
}
func (app *App) InitCfg(c *cli.Context) error {
	var err error
	app.config, err = parseConfig(c.String("config"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	dataFile := c.String("data-file")
	if dataFile != "" {
		app.config.DataFile = dataFile
	}

	secret := c.String("secret-key")
	if dataFile != "" {
		app.config.SenderSecret = secret
	}

	app.Operator, err = txutils.NewOperator(txutils.ManagerCfg{
		Net:        app.config.Net,
		RPC:        app.config.NodeRPC,
		PrivateKey: app.config.SenderSecret,
	})
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to init TxMan"), 1)
	}
	return nil
}

func (app *App) SyncUTXOFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "address",
			Aliases: []string{"a"},
			Value:   "",
			EnvVars: []string{"TX_ADDRESS"},
			Usage:   "filter UTXO by address",
		},
		&cli.Int64Flag{
			Name:    "offset",
			Aliases: []string{"o"},
			EnvVars: []string{"TX_BLOCK_OFFSET"},
			Usage:   "offset for starting block height",
		},
	}
}
func (app *App) SyncUTXOCmd(c *cli.Context) error {
	offset := c.Int64("offset")
	address := c.String("address")
	dataFile := c.String("data-file")

	fmt.Printf("Start collecting...")

	rows, lastBlock, err := app.TxMan.CollectUTXO(address, offset)
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to collect UTXO"), 1)
	}

	fmt.Printf("\nFound %d UTXOs for <%s> in blocks[%d, %d]\n", len(rows), address, offset, lastBlock)

	err = storage.NewCSVStorage(dataFile).SaveRows(rows)
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to save UTXO"), 1)
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
		fmt.Printf("Try to send %d satoshi to %s ", dest.Amount, dest.Address)
		tx, err := app.TxMan.NewTx(dest.Address, dest.Amount, provider)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to send tx"), 1)
		}

		fmt.Printf("Sent Tx\nHash: %s\nBody: %s\n", tx.TxHash, tx.SignedTx)

		_, err = app.TxMan.RPC.SendRawTransaction(tx.RawTX, true)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
		}

		sentTxs = append(sentTxs, tx)
	}

	time.Sleep(2 * time.Second)

	for _, tx := range sentTxs {
		hash, _ := chainhash.NewHashFromStr(tx.TxHash)
		txResult, err := app.TxMan.RPC.GetRawTransaction(hash)
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

func (app *App) NewMultiSigTxFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "first-pk",
			Aliases:  []string{"f"},
			Usage:    "hex-encoded public key of first recipient",
			EnvVars:  []string{"TX_FIRST_PK"},
			Required: true,
		},
		&cli.StringFlag{
			Name:     "second-pk",
			Aliases:  []string{"s"},
			EnvVars:  []string{"TX_SECOND_PK"},
			Usage:    "hex-encoded public key of second recipient",
			Required: true,
		},
		&cli.Int64Flag{
			Name:     "amount",
			Aliases:  []string{"a"},
			Value:    0,
			Usage:    "amount of new tx",
			Required: true,
		},
		&cli.BoolFlag{
			Name:    "send-tx",
			Aliases: []string{"t"},
			Usage:   "craft and send transaction if set",
		},
	}
}

// func (app *App) NewMultiSigTxCmd(c *cli.Context) error {
// 	firstRecipient := c.String("first-pk")
// 	secondRecipient := c.String("second-pk")
// 	amount := c.Int64("amount")
// 	send := c.Bool("send-tx")
// 	signer, err := txutils.NewKeyData(app.config.SenderSecret, app.config.NetParams())
// 	if err != nil {
// 		return cli.NewExitError(err, 1)
// 	}
//
// 	tx, err := app.NewMultiSigTx(*signer, app.config.DataFile, firstRecipient, secondRecipient, amount)
// 	if err != nil {
// 		return cli.NewExitError(err, 1)
// 	}
//
// 	fmt.Printf("Craft new Tx\nHash: %s\nBody: %s\n", tx.TxHash, tx.SignedTx)
//
// 	if send {
// 		_, err = app.TxMan.RPC.SendRawTransaction(tx.RawTX, true)
// 		if err != nil {
// 			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
// 		}
// 		fmt.Printf("Tx Sent: %s\n", tx.TxHash)
// 	}
//
// 	return nil
// }

func (app *App) NewMultiSigAddressCmd(c *cli.Context) error {
	firstRecipient := c.String("first-pk")
	secondRecipient := c.String("second-pk")
	mAddr, err := app.TxMan.NewMultiSig2of2Address(firstRecipient, secondRecipient)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("Craft new mustisig Address\nAddress: %s\nRedeemScript: %s\n", mAddr.Address, mAddr.RedeemScript)
	return nil
}

func (app *App) AddSignatureToTxFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "tx-body",
			Aliases:  []string{"b"},
			Usage:    "hex-encoded body of transaction",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "redeem-script",
			Aliases:  []string{"s"},
			Usage:    "hex-encoded redeem script of tx input",
			Required: true,
		},
		&cli.BoolFlag{
			Name:    "send-tx",
			Aliases: []string{"t"},
			Usage:   "craft and send transaction if set",
		},
	}
}
func (app *App) AddSignatureToTxCmd(c *cli.Context) error {
	txBody := c.String("tx-body")
	script := c.String("redeem-script")
	send := c.Bool("send-tx")

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
		_, err = app.TxMan.RPC.SendRawTransaction(tx.RawTX, true)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
		}
		fmt.Printf("Tx Sent: %s\n", tx.TxHash)

	}

	return nil
}

func (app *App) SpendUTXOFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "tx-hash",
			Aliases:  []string{"x"},
			Usage:    "hash of transaction with source UTXO",
			Required: true,
		},
		&cli.Uint64Flag{
			Name:     "out-index",
			Aliases:  []string{"i"},
			Usage:    "index of source UTXO",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "address",
			Aliases:  []string{"a"},
			Value:    "",
			Usage:    "destination address of new tx",
			Required: true,
		},
		&cli.Int64Flag{
			Name:     "amount",
			Aliases:  []string{"a"},
			Value:    0,
			Usage:    "amount of new tx",
			Required: true,
		},
		&cli.BoolFlag{
			Name:    "send-tx",
			Aliases: []string{"t"},
			Usage:   "craft and send transaction if set",
		},
	}
}

func (app *App) SpendUTXOCmd(c *cli.Context) error {
	txHash := c.String("tx-hash")
	destination := c.String("address")
	outIndex := c.Uint64("out-index")
	amount := c.Int64("amount")

	send := c.Bool("send-tx")

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
		_, err = app.TxMan.RPC.SendRawTransaction(tx.RawTX, true)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
		}
		fmt.Printf("Tx Sent: %s\n", tx.TxHash)
	}

	return nil
}
