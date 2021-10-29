package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
)

func (app *App) SyncHeadersCmd(c *cli.Context) error {
	path := c.String(flagDataDir)

	res, err := app.TxMan.RPC().ListShards()
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to get list of available shards"), 1)
	}

	fmt.Printf("%d shards are available\n", len(res.Shards))

	out, err := openFile(path, "headers.csv")
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to open out file"), 1)
	}

	outFullData, err := openFile(path, "full_headers.csv")
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to open out file"), 1)
	}

	defer out.Close()
	defer outFullData.Close()

	_, _ = fmt.Fprintf(out, "%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
		"chain",
		"height",
		"hash",
		"blocks_mmr",
		"pow_hash",
		"bits",
		"target",
		"nonce",
		"time",
	)

	_, _ = fmt.Fprintf(outFullData, "%v,%v,%v,%v\n",
		"chain",
		"height",
		"hash",
		"raw_data",
	)

	_, best, err := app.TxMan.RPC().ForBeacon().GetBestBlock()
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to GetBestBlock"), 1)
	}

	for height := int64(1); height < int64(best); height++ {
		fmt.Printf("\r\033[0K Processing: chain %s block %d ...", "beacon", height)

		hash, err := app.TxMan.RPC().ForBeacon().GetBlockHash(height)
		if err != nil {
			return cli.NewExitError(errors.Wrapf(err, "unable to get block hash by height(%d)", height), 1)
		}
		header, err := app.TxMan.RPC().ForBeacon().GetBeaconBlockHeader(hash)
		if err != nil {
			return cli.NewExitError(errors.Wrapf(err, "unable to get block by hash(%s)", hash), 1)
		}

		_, _ = fmt.Fprintf(out, "%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
			"beacon",
			height,
			hash.String(),
			header.PrevBlocksMMRRoot(),
			header.PoWHash(),
			fmt.Sprintf("%08x", header.Bits()),
			fmt.Sprintf("%064x", pow.CompactToBig(header.Bits())),
			header.BeaconHeader().Nonce(),
			header.Timestamp().Unix(),
		)

		buf := bytes.NewBuffer(nil)
		_ = header.Write(buf)

		_, _ = fmt.Fprintf(outFullData, "%v,%v,%v,%v\n",
			"beacon",
			height,
			hash.String(),
			hex.EncodeToString(buf.Bytes()),
		)

	}

	for shardID := uint32(1); shardID < uint32(len(res.Shards)); shardID++ {
		fmt.Println()
		_, best, err := app.TxMan.RPC().ForShard(shardID).GetBestBlock()
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to GetBestBlock"), 1)
		}
		for height := int64(1); height < int64(best); height++ {
			fmt.Printf("\r\033[0K Processing: chain %s block %d ...", "shard_"+strconv.Itoa(int(shardID)), height)
			hash, err := app.TxMan.RPC().ForShard(shardID).GetBlockHash(height)
			if err != nil {
				return cli.NewExitError(errors.Wrapf(err, "unable to get block hash by height(%d)", height), 1)
			}
			header, err := app.TxMan.RPC().ForShard(shardID).GetShardBlockHeader(hash)
			if err != nil {
				return cli.NewExitError(errors.Wrapf(err, "unable to get block by hash(%s)", hash), 1)
			}

			_, _ = fmt.Fprintf(out, "%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
				"shard_"+strconv.Itoa(int(shardID)),
				height,
				hash.String(),
				header.PrevBlocksMMRRoot(),
				header.PoWHash(),
				fmt.Sprintf("%08x", header.Bits()),
				fmt.Sprintf("%064x", pow.CompactToBig(header.Bits())),
				header.BeaconHeader().Nonce(),
				header.Timestamp().Unix(),
			)

			buf := bytes.NewBuffer(nil)
			_ = header.Write(buf)

			_, _ = fmt.Fprintf(outFullData, "%v,%v,%v,%v\n",
				"shard_"+strconv.Itoa(int(shardID)),
				height,
				hash.String(),
				hex.EncodeToString(buf.Bytes()),
			)
		}
	}

	return nil
}

func openFile(path, name string) (*os.File, error) {
	if _, err := os.Stat(path + "/" + name); os.IsNotExist(err) {
		os.MkdirAll(path, 0700) // Create file
	}

	return os.OpenFile(path+"/"+name, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
}
