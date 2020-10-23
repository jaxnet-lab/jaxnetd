// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package main

import "github.com/urfave/cli/v2"

const (
	flagAddress      = "address"
	flagAmount       = "amount"
	flagConfig       = "config"
	flagDataFile     = "data-file"
	flagFirstPubKey  = "first-pk"
	flagSecondPubKey = "second-pk"
	flagOffset       = "offset"
	flagOutIn        = "out-index"
	flagRedeemScript = "redeem-script"
	flagSecretKey    = "secret-key"
	flagSendTx       = "send-tx"
	flagTxHash       = "tx-hash"
	flagTxBody       = "tx-body"
)

func getFlags() map[string]cli.Flag {
	return map[string]cli.Flag{
		flagConfig: &cli.StringFlag{
			Name:    flagConfig,
			Aliases: []string{"c"},
			Value:   "./config.yaml",
			Usage:   "path to configuration",
		},
		flagDataFile: &cli.StringFlag{
			Name:    flagDataFile,
			Aliases: []string{"f"},
			EnvVars: []string{"TX_DATA_FILE"},
			Usage:   "path to CSV input/output, will override value from config file",
		},
		flagSecretKey: &cli.StringFlag{
			Name:    flagSecretKey,
			Aliases: []string{"k"},
			Value:   "",
			EnvVars: []string{"TX_SECRET_KEY"},
			Usage:   "secret key for signing actions, will override value from config file",
		},
		flagTxHash: &cli.StringFlag{
			Name:     flagTxHash,
			Aliases:  []string{"x"},
			Usage:    "hash of transaction with source UTXO",
			Required: true,
		},
		flagOutIn: &cli.Uint64Flag{
			Name:     flagOutIn,
			Aliases:  []string{"i"},
			Usage:    "index of source UTXO",
			Required: true,
		},
		flagAddress: &cli.StringFlag{
			Name:    flagAddress,
			Aliases: []string{"a"},
			Value:   "",
			Usage:   "address string",
		},
		flagAmount: &cli.Int64Flag{
			Name:     flagAmount,
			Aliases:  []string{"a"},
			Value:    0,
			Usage:    "amount in satoshi",
			Required: true,
		},
		flagSendTx: &cli.BoolFlag{
			Name:    flagSendTx,
			Aliases: []string{"t"},
			Usage:   "craft and send transaction if set",
		},
		flagTxBody: &cli.StringFlag{
			Name:     flagTxBody,
			Aliases:  []string{"b"},
			Usage:    "hex-encoded body of transaction",
			Required: true,
		},
		flagRedeemScript: &cli.StringFlag{
			Name:     flagRedeemScript,
			Aliases:  []string{"s"},
			Usage:    "hex-encoded redeem script of tx input",
			Required: true,
		},
		flagFirstPubKey: &cli.StringFlag{
			Name:     flagFirstPubKey,
			Aliases:  []string{"f"},
			Usage:    "hex-encoded public key of first recipient",
			EnvVars:  []string{"TX_FIRST_PK"},
			Required: true,
		},
		flagSecondPubKey: &cli.StringFlag{
			Name:     flagSecondPubKey,
			Aliases:  []string{"s"},
			EnvVars:  []string{"TX_SECOND_PK"},
			Usage:    "hex-encoded public key of second recipient",
			Required: true,
		},
		flagOffset: &cli.Int64Flag{
			Name:    flagOffset,
			Aliases: []string{"o"},
			EnvVars: []string{"TX_BLOCK_OFFSET"},
			Usage:   "offset for block height",
		},
	}
}
