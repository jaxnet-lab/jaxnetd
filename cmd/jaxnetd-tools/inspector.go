package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
)

func (app *App) SyncPoWStatsCmd(c *cli.Context) error {
	res, err := app.TxMan.RPC().ListShards()
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to get list of available shards"), 1)
	}

	fmt.Printf("%d shards are available\n", len(res.Shards))

	out, err := os.OpenFile("./headers.csv", os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to open out file"), 1)
	}

	defer out.Close()
	_, _ = fmt.Fprintf(out, "%v,%v,%v,%v,%v,%v,%v,%v,%v",
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

	_, best, err := app.TxMan.RPC().ForBeacon().GetBestBlock()
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to GetBestBlock"), 1)
	}

	for height := int64(1); height < int64(best); height++ {
		hash, err := app.TxMan.RPC().ForBeacon().GetBlockHash(height)
		if err != nil {
			return cli.NewExitError(errors.Wrapf(err, "unable to get block hash by height(%d)", height), 1)
		}
		header, err := app.TxMan.RPC().ForBeacon().GetBeaconBlockHeader(hash)
		if err != nil {
			return cli.NewExitError(errors.Wrapf(err, "unable to get block by hash(%s)", hash), 1)
		}

		_, _ = fmt.Fprintf(out, "%v,%v,%v,%v,%v,%v,%v,%v,%v",
			"beacon",
			height,
			hash.String(),
			header.BlocksMerkleMountainRoot(),
			header.PoWHash(),
			fmt.Sprintf("%08x", header.Bits()),
			fmt.Sprintf("%064x", pow.CompactToBig(header.Bits())),
			header.BeaconHeader().Nonce(),
			header.Timestamp().Unix(),
		)
		header.PoWHash()
	}

	for shardID := uint32(1); shardID < uint32(len(res.Shards)); shardID++ {
		_, best, err := app.TxMan.RPC().ForShard(shardID).GetBestBlock()
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to GetBestBlock"), 1)
		}
		for height := int64(1); height < int64(best); height++ {
			hash, err := app.TxMan.RPC().ForShard(shardID).GetBlockHash(height)
			if err != nil {
				return cli.NewExitError(errors.Wrapf(err, "unable to get block hash by height(%d)", height), 1)
			}
			header, err := app.TxMan.RPC().ForShard(shardID).GetShardBlockHeader(hash)
			if err != nil {
				return cli.NewExitError(errors.Wrapf(err, "unable to get block by hash(%s)", hash), 1)
			}

			_, _ = fmt.Fprintf(out, "%v,%v,%v,%v,%v,%v,%v,%v,%v",
				"shard_"+strconv.Itoa(int(shardID)),
				height,
				hash.String(),
				header.BlocksMerkleMountainRoot(),
				header.PoWHash(),
				fmt.Sprintf("%08x", header.Bits()),
				fmt.Sprintf("%064x", pow.CompactToBig(header.Bits())),
				header.BeaconHeader().Nonce(),
				header.Timestamp().Unix(),
			)
			header.PoWHash()
		}
	}

	return nil
}
