package main

import (
	"encoding/hex"
	"fmt"
	"log"
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
		Action: app.DefaultAction,
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

		fmt.Printf("Sent Tx\nHash: %s\nBody: %s\n", tx.TxHash, tx.SignedTx)

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
	sendTX := false

	cfg := txmanager.ManagerCfg{
		Net: app.config.Net,
		RPC: app.config.NodeRPC,
	}

	// create new instance of tx manager,

	manager, err := txmanager.NewTxMan(cfg)
	if err != nil {
		log.Fatal("unable to init manager:", err)
	}

	// aliceSecret is hex-encoded private key
	aliceSecret := "330c18e1c5548ec726fd6286f11ea29e7f15a13e9d4c62205ea6230dcb064769"
	aliceKP, err := txmanager.NewKeyData(aliceSecret, cfg.NetParams())
	if err != nil {
		log.Fatal("unable to create aliceKP:", err)
	}

	// bobSecret is hex-encoded private key
	bobSecret := "6e38943e714b5b3ead017ab003939366e798b07dff5f9e12de913d7388aa572c"

	bobKP, err := txmanager.NewKeyData(bobSecret, cfg.NetParams())
	if err != nil {
		log.Fatal("unable to create bobKP:", err)
	}

	if sendTX {
		utxo, err := manager.CollectUTXO(aliceKP.AddressPubKey.EncodeAddress(), 0)
		if err != nil {
			log.Fatal("unable to collect utxo:", err)
		}

		manager.SetKey(aliceKP)
		sentTx, err := manager.NewMultiSig2of2Tx(
			aliceKP.AddressPubKey,
			bobKP.AddressPubKey,
			360*txmanager.OneCoin,
			txmanager.UTXOFromRows(utxo))
		if err != nil {
			log.Fatal("unable to collect utxo:", err)
		}

		_, err = manager.RPC.SendRawTransaction(sentTx.RawTX, true)
		if err != nil {
			log.Fatal("tx not sent:", err)
		}
		fmt.Printf("Sent Tx\nHash: %s\nBody: %s\n", sentTx.TxHash, sentTx.SignedTx)
	}
	for _, testTx := range []string{tx2} {
		txWitMultisig, _ := txmanager.DecodeTx(testTx)
		manager.SetKey(aliceKP)
		utxo := models.UTXO{
			TxHash:   txWitMultisig.TxHash().String(),
			OutIndex: 0,
			Value:    txWitMultisig.TxOut[0].Value,
			Used:     false,
			PKScript: hex.EncodeToString(txWitMultisig.TxOut[0].PkScript),
		}

		tx, err := manager.NewTx(bobKP.AddressPubKey.EncodeAddress(), 360*txmanager.OneCoin, txmanager.SingleUTXO(utxo))
		if err != nil {
			log.Println("new tx error:", err.Error())
			continue
		}

		updTx1, err := decodeAndSignTx(manager, tx.SignedTx)
		if err != nil {
			log.Println("decodeAndSignTx error:", err.Error())
			continue
		}
		fmt.Printf("Sign Tx\nBody: %s\n", updTx1)
	}

	return nil
}

func decodeAndSignTx(manager *txmanager.TxMan, tx string) (string, error) {
	msgTx, err := txmanager.DecodeTx(tx)
	if err != nil {
		return "", errors.Wrap(err, "unable to decode tx")
	}

	msgTx, err = manager.AddSignatureToTx(msgTx)
	if err != nil {
		return "", errors.Wrap(err, "unable add signature")
	}

	return txmanager.EncodeTx(msgTx), nil
}

// txs with multisig UTXO
var tx1 = `01000000071325fc88251c797445305e6005dd0bb20e250ecc7475a37979eddfa833ed1939000000008a473044022018f8a4018bbbb1e8496f6d348e769c40a5cc5d613718e83520c96d874656b69902205760f41d48574d21ab697910f517dca06d0cef8da97f882d9e6156249261cab3034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3ffffffff1234cc8ad94400689f37dd89495449cb830bb85463335b810136ca975e0c96d6000000008a4730440220659f68c84c6f9e3812102bd64e1421eac2d617b81989da95868df3778b5f18180220748b4be5d3948e6b172c62618de8b26b54d35171065db99c86af4e30f092e45a034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3ffffffff13eb922a1c76ea93ab3dbaf9e6ba68f36f0e932e3ecd06da8f4c08bcc218af12000000008a4730440220659f68c84c6f9e3812102bd64e1421eac2d617b81989da95868df3778b5f18180220748b4be5d3948e6b172c62618de8b26b54d35171065db99c86af4e30f092e45a034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3ffffffff0ca4bb31d3aa8f421f4b3c003f2f972a446126fbae5eccc2ec5f7ea89f28e270000000008a4730440220659f68c84c6f9e3812102bd64e1421eac2d617b81989da95868df3778b5f18180220748b4be5d3948e6b172c62618de8b26b54d35171065db99c86af4e30f092e45a034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3ffffffff477c3ad847fa690fe9530eb68e5cb908e3d8050074c3c797c0ff57bf647afc5e000000008a4730440220659f68c84c6f9e3812102bd64e1421eac2d617b81989da95868df3778b5f18180220748b4be5d3948e6b172c62618de8b26b54d35171065db99c86af4e30f092e45a034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3ffffffffe9d54571091599d3f2b7de031b3df7f5036ba2466fae074897bb9ad109a2aa50000000008a4730440220659f68c84c6f9e3812102bd64e1421eac2d617b81989da95868df3778b5f18180220748b4be5d3948e6b172c62618de8b26b54d35171065db99c86af4e30f092e45a034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3ffffffff8d23047ba41d246a86124cd40e7890943475e31d191d31d02c9825f5585d21ed000000008a4730440220659f68c84c6f9e3812102bd64e1421eac2d617b81989da95868df3778b5f18180220748b4be5d3948e6b172c62618de8b26b54d35171065db99c86af4e30f092e45a034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3ffffffff01000c58380900000017a914e9f73dbb5384ad037915ea47cac0847d67964ca28700000000`
var tx2 = `0100000004933d6f5be1549747539463946e0913c9acf857a4f066ae945bc374376fa4cbb2000000008a47304402203f46a7f2d83a6b14ecbae1445c19eb9e86502c24ee3548088ade6b35d3080aff0220134869b46b9d0873bc3943df4c80a3e8193130d56450eba45370ed7574cb0ba9034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3ffffffffc82f8bcde99c7bcd648f99c18362634ae0022cd381d4986f7945d508155e8b31000000008a4730440220659f68c84c6f9e3812102bd64e1421eac2d617b81989da95868df3778b5f18180220748b4be5d3948e6b172c62618de8b26b54d35171065db99c86af4e30f092e45a034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3ffffffffa6c8dcee0172af9aa5bb2e679fb536ed4835bb9fa296743019d755a291b0d09e000000008a4730440220659f68c84c6f9e3812102bd64e1421eac2d617b81989da95868df3778b5f18180220748b4be5d3948e6b172c62618de8b26b54d35171065db99c86af4e30f092e45a034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3fffffffffb6a6c9317fcd1b024486020867eeb3fcaf12eca4834ef251ccc3929a2a4de1c000000008a4730440220659f68c84c6f9e3812102bd64e1421eac2d617b81989da95868df3778b5f18180220748b4be5d3948e6b172c62618de8b26b54d35171065db99c86af4e30f092e45a034104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea3ffffffff010068c4610800000087524104a39fbfe1d05b006a70b40a39c22832d0c339ed8aeaad0ce4f8fda44acf08e22e7316b5b7fd13765e1e2e8188e322a6d818a5362734a8a82d308577fa7396dea341042f9babd2e8d634eda7036cb766391fd41850527e10cf6c4d085454fe983e3fd7608b69630f2c4ef51ae1e3db5f8ee9e0ced483161e6974ccce0293087c0a61a552ae00000000`
