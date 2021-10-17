// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"gitlab.com/jaxnet/jaxnetd/btcec"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/jaxutil/txmodels"
	"gitlab.com/jaxnet/jaxnetd/jaxutil/txutils"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

func main() {
	app := &App{}
	cliApp := &cli.App{
		Name:     "jax-tx-tools",
		Usage:    "routine transactions",
		Flags:    app.InitFlags(),
		Before:   app.InitCfg,
		Commands: app.getCommands(),
		Action:   app.defaultAction,
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
			Action: app.sendTxCmd,
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
			Name:  "new-htlc-address",
			Usage: "creates new HTLC(hybrid-time-lock-contract) address",
			Flags: []cli.Flag{
				standardFlags[flagLockTime],
				standardFlags[flagAddress],
			},
			Action: app.NewHTLCAddress,
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
			Usage: "decodes hex-encoded data",
			Subcommands: cli.Commands{
				{
					Name:  "block",
					Usage: "decode hex encoded block",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "block",
							Aliases:  []string{"b"},
							Usage:    "hex-encoded body of block",
							Required: true,
						},
						&cli.StringFlag{
							Name:  "shard",
							Usage: "is this shard block or beacon",
						},
					},
					Action: func(c *cli.Context) error {
						script := c.String("block")
						shard := c.Bool("shard")
						var block = wire.EmptyBeaconBlock()
						if shard {
							block = wire.EmptyShardBlock()
						}

						decodedHex, err := hex.DecodeString(script)
						if err != nil {
							return cli.NewExitError(err, 1)
						}
						err = block.Deserialize(bytes.NewBuffer(decodedHex))
						if err != nil {
							return cli.NewExitError(err, 1)
						}
						spew.Dump(block)
						return nil
					},
				},
				{
					Name:  "tx",
					Usage: "decode hex encoded transaction body",
					Flags: []cli.Flag{
						standardFlags[flagTxBody],
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
						standardFlags[flagRedeemScript],
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
		{
			Name:   "gen-kp",
			Usage:  "generate new key pair and addresses",
			Action: app.genKp,
		},
	}
}

type App struct {
	config Config
	txutils.Operator
	shardID uint32
}

func (app *App) InitFlags() []cli.Flag {
	return []cli.Flag{
		standardFlags[flagConfig],
		standardFlags[flagDataFile],
		standardFlags[flagSecretKey],
		standardFlags[flagShard],
		standardFlags[flagRunFromConfig],
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
	if secret != "" {
		app.config.SenderSecret = secret
	}

	shardID := c.Uint64(flagShard)

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

func (app *App) defaultAction(c *cli.Context) error {
	runCmd := c.Bool(flagRunFromConfig)
	if !runCmd {
		fmt.Println("No command provided")
		return nil
	}

	if app.config.Cmd.SendTxs != nil {
		return app.sendTxCmd(nil)
	}

	return nil
}

func (app *App) SyncUTXOFlags() []cli.Flag {
	return []cli.Flag{
		standardFlags[flagAddress],
		standardFlags[flagOffset],
		standardFlags[flagShards],
		standardFlags[flagSplitFiles],
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
			err = txmodels.NewCSVStorage(fmt.Sprintf("chain-%d-%s", u, dataFile)).SaveRows(rows)
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
		err = txmodels.NewCSVStorage(dataFile).SaveRows(allRows)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to save UTXO"), 1)
		}
	}

	return nil
}

func (app *App) sendTxCmd(*cli.Context) error {
	if app.config.Cmd.SendTxs == nil {
		return cli.NewExitError("invalid configuration - cmd.send_tx is nil", 1)
	}
	key, err := txutils.NewKeyData(app.config.Cmd.SendTxs.SenderSecret, app.config.NetParams())
	if err != nil {
		return err
	}

	sentTxs := make(map[string]uint32, len(app.config.Cmd.SendTxs.Destinations))
	for _, dest := range app.config.Cmd.SendTxs.Destinations {
		fmt.Println("")
		fmt.Printf("Try to send %d satoshi to %s ", dest.Amount, dest.Address)
		txHash, err := sendTx(app.TxMan, key, dest.ShardID, dest.Address, dest.Amount, 0)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to creat and publish tx"), 1)
		}
		sentTxs[txHash] = dest.ShardID
	}

	fmt.Println("")

	time.Sleep(5 * time.Second)

	for txHash, shardID := range sentTxs {
		err := txutils.WaitForTx(app.TxMan.RPC().ForShard(shardID), shardID, txHash, 0)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to get tx"), 1)
		}
	}

	return nil
}

func (app *App) NewMultiSigTxFlags() []cli.Flag {
	return []cli.Flag{
		standardFlags[flagFirstPubKey],
		standardFlags[flagSecondPubKey],
		standardFlags[flagAmount],
		standardFlags[flagSendTx],
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

	repo := txmodels.NewCSVStorage(app.config.DataFile)
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
		_, err = app.TxMan.RPC().ForShard(app.shardID).SendRawTransaction(tx.RawTX)
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

func (app *App) NewHTLCAddress(c *cli.Context) error {
	address := c.String(flagAddress)
	lockTime := c.Int64(flagLockTime)
	jAddress, err := jaxutil.DecodeAddress(address, app.TxMan.NetParams)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	htlcAddress, err := txscript.HTLCScriptAddress(jAddress, int32(lockTime), app.TxMan.NetParams)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Printf("Craft new HTLC Address\nAddress: %s\n", htlcAddress)
	return nil
}

func (app *App) AddSignatureToTxFlags() []cli.Flag {
	return []cli.Flag{
		standardFlags[flagTxBody],
		standardFlags[flagRedeemScript],
		standardFlags[flagSendTx],
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
		_, err = app.TxMan.RPC().ForShard(app.shardID).SendRawTransaction(tx.RawTX)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
		}
		fmt.Printf("Tx Sent: %s\n", tx.TxHash)

	}

	return nil
}

func (app *App) SpendUTXOFlags() []cli.Flag {
	return []cli.Flag{
		standardFlags[flagTxHash],
		standardFlags[flagAddress],
		standardFlags[flagOutIn],
		standardFlags[flagAmount],
		standardFlags[flagAmount],
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
		_, err = app.TxMan.RPC().ForShard(app.shardID).SendRawTransaction(tx.RawTX)
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "tx not sent"), 1)
		}
		fmt.Printf("Tx Sent: %s\n", tx.TxHash)
	}

	return nil
}

func (*App) genKp(*cli.Context) error {
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		fmt.Printf("failed to make privKey for  %v", err)
		return cli.NewExitError("failed to generate kp", 1)
	}

	nets := []chaincfg.Params{
		chaincfg.MainNetParams,
		chaincfg.TestNetParams,
	}

	pubKeys := map[string][]byte{
		"uncompressed": (*btcec.PublicKey)(&key.PublicKey).SerializeUncompressed(),
		"compressed":   (*btcec.PublicKey)(&key.PublicKey).SerializeCompressed(),
		// "hybrid":       (*btcec.PublicKey)(&key.PublicKey).SerializeHybrid(),
	}

	fmt.Printf("PrivateKey: %x\n", key.Serialize())

	for _, net := range nets {
		fmt.Println("\n" + net.Name + ":")

		wif, err := jaxutil.NewWIF(key, &net, true)
		if err != nil {
			println("[error] " + err.Error())
			return cli.NewExitError("failed to generate wif", 1)
		}

		fmt.Printf("   WIF: %s\n", wif.String())

		for pubKeyTy, pk := range pubKeys {
			fmt.Println()
			fmt.Println("   PubKey [" + pubKeyTy + "]:")

			addressPubKey, err := jaxutil.NewAddressPubKey(pk, &net)
			if err != nil {
				println("[error] " + err.Error())
				return cli.NewExitError("failed to generate kp", 1)
			}

			fmt.Printf("   AddressPubKey: %s\n", addressPubKey.String())
			fmt.Printf("   AddressPubKeyHash: %s\n", addressPubKey.EncodeAddress())
		}
	}

	return nil
}

func sendTx(txMan *txutils.TxMan, senderKP *txutils.KeyData, shardID uint32, destination string, amount int64, timeLock uint32) (string, error) {
	senderAddress := senderKP.Address.EncodeAddress()
	senderUTXOIndex := txmodels.NewUTXORepo("", senderAddress)

	var err error
	err = senderUTXOIndex.ReadIndex()
	if err != nil {
		return "", errors.Wrap(err, "unable to open UTXO index")
	}

	err = senderUTXOIndex.CollectFromRPC(txMan.RPC(), shardID, map[string]bool{senderAddress: true})
	if err != nil {
		return "", errors.Wrap(err, "unable to collect UTXO")
	}

	tx, err := txMan.WithKeys(senderKP).ForShard(shardID).
		NewTx(destination, amount, &senderUTXOIndex)
	if err != nil {
		return "", errors.Wrap(err, "unable to create new tx")
	}
	if tx == nil || tx.RawTX == nil {
		return "", errors.New("tx empty")
	}
	_, err = txMan.RPC().ForShard(shardID).SendRawTransaction(tx.RawTX)
	if err != nil {
		return "", errors.Wrap(err, "unable to publish new tx")
	}
	err = senderUTXOIndex.SaveIndex()
	if err != nil {
		return "", errors.Wrap(err, "unable to save UTXO index")
	}
	fmt.Printf("Sent tx %s at shard %d\n", tx.TxHash, shardID)
	return tx.TxHash, nil
}
