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

	_, _ = fmt.Fprintf(out, "%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
		"chain",
		"height",
		"serial_id",
		"prev_serial_id",
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
		"serial_id",
		"hash",
		"raw_data",
	)

	best, err := app.TxMan.RPC().ForBeacon().GetLastSerialBlockNumber()
	if err != nil {
		return cli.NewExitError(errors.Wrap(err, "unable to GetBestBlock"), 1)
	}

	for serialID := int64(1); serialID < best; serialID++ {
		fmt.Printf("\r\033[0K Processing: chain %s block %d ...", "beacon", serialID)

		block, err := app.TxMan.RPC().ForBeacon().GetBeaconBlockBySerialNumber(serialID)
		if err != nil {
			return cli.NewExitError(errors.Wrapf(err, "unable to get block by serialID(%v)", serialID), 1)
		}

		_, _ = fmt.Fprintf(out, "%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
			"beacon",
			block.Block.Header.Height(),
			block.SerialID,
			block.PrevSerialID,
			block.Block.BlockHash().String(),
			block.Block.Header.PrevBlocksMMRRoot(),
			block.Block.Header.PoWHash(),
			fmt.Sprintf("%08x", block.Block.Header.Bits()),
			fmt.Sprintf("%064x", pow.CompactToBig(block.Block.Header.Bits())),
			block.Block.Header.BeaconHeader().Nonce(),
			block.Block.Header.Timestamp().Unix(),
		)

		buf := bytes.NewBuffer(nil)
		_ = block.Block.Header.Write(buf)

		_, _ = fmt.Fprintf(outFullData, "%v,%v,%v,%v\n",
			"beacon",
			serialID,
			block.Block.BlockHash().String(),
			hex.EncodeToString(buf.Bytes()),
		)

	}

	for shardID := uint32(1); shardID < uint32(len(res.Shards)); shardID++ {
		fmt.Println()
		best, err := app.TxMan.RPC().ForShard(shardID).GetLastSerialBlockNumber()
		if err != nil {
			return cli.NewExitError(errors.Wrap(err, "unable to GetBestBlock"), 1)
		}
		for serialID := int64(1); serialID < best; serialID++ {
			fmt.Printf("\r\033[0K Processing: chain %s block %d ...", "shard_"+strconv.Itoa(int(shardID)), serialID)

			block, err := app.TxMan.RPC().ForShard(shardID).GetShardBlockBySerialNumber(serialID)
			if err != nil {
				return cli.NewExitError(errors.Wrapf(err, "unable to get block by serialID(%v)", serialID), 1)
			}

			_, _ = fmt.Fprintf(out, "%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
				"shard_"+strconv.Itoa(int(shardID)),
				block.Block.Header.Height(),
				block.SerialID,
				block.PrevSerialID,
				block.Block.BlockHash().String(),
				block.Block.Header.PrevBlocksMMRRoot(),
				block.Block.Header.PoWHash(),
				fmt.Sprintf("%08x", block.Block.Header.Bits()),
				fmt.Sprintf("%064x", pow.CompactToBig(block.Block.Header.Bits())),
				block.Block.Header.BeaconHeader().Nonce(),
				block.Block.Header.Timestamp().Unix(),
			)

			buf := bytes.NewBuffer(nil)
			_ = block.Block.Header.Write(buf)

			_, _ = fmt.Fprintf(outFullData, "%v,%v,%v,%v\n",
				"shard_"+strconv.Itoa(int(shardID)),
				serialID,
				block.Block.BlockHash().String(),
				hex.EncodeToString(buf.Bytes()),
			)
		}
	}

	return nil
}

// nolint: gomnd
func openFile(path, name string) (*os.File, error) {
	if _, err := os.Stat(path + "/" + name); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, err
		} // Create file
	}

	return os.OpenFile(path+"/"+name, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o755)
}
