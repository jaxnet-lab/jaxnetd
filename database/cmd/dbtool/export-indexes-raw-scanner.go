package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node"
	"gitlab.com/jaxnet/jaxnetd/node/chain"
	"gitlab.com/jaxnet/jaxnetd/node/chain/beacon"
	"gitlab.com/jaxnet/jaxnetd/node/chain/shard"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"golang.org/x/sync/errgroup"
)

func rawScanner(offset int, shardID uint32) error {
	ctx, cancelScanning := context.WithCancel(context.Background())
	writerCtx, cancelWrite := context.WithCancel(context.Background())

	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, os.Interrupt)
	go func() {
		select {
		case <-interruptChannel:
			cancelScanning()
		}
	}()

	eGroup := errgroup.Group{}

	blocksChan := make(chan row)
	inputsChan := make(chan row)
	outputsChan := make(chan row)

	eGroup.Go(func() error {
		blocksFile, err := NewCSVStorage("blocks.csv")
		if err != nil {
			return err
		}
		return blocksFile.WriteData(writerCtx, blocksChan)
	})

	eGroup.Go(func() error {
		inputsFile, err := NewCSVStorage("inputs.csv")
		if err != nil {
			return err
		}
		return inputsFile.WriteData(writerCtx, inputsChan)
	})

	eGroup.Go(func() error {
		outputsFile, err := NewCSVStorage("outputs.csv")
		if err != nil {
			return err
		}
		return outputsFile.WriteData(writerCtx, outputsChan)
	})

	eGroup.Go(func() error {
		return scan(ctx, cancelWrite, offset, blocksChan, inputsChan, outputsChan, shardID)
	})

	return eGroup.Wait()
}

func scan(ctx context.Context, cancel context.CancelFunc, offset int, blocksChan, inputsChan, outputsChan chan row, shardID uint32) error {
	// Load the block database.
	var dbShard database.DB
	dbBeacon, err := loadBlockDB(beacon.Chain(activeNetParams))
	if err != nil {
		return err
	}
	defer dbBeacon.Close()

	// we need dbShard only if we are dealing with shards. In order to properly close the db connection, we need to create the connection in this function
	// if we pass it as nil to prepareBlockchain, the case will be handled safely thanks to early exit from function based on shardID
	if shardID != 0 {
		dbShard, err = loadBlockDB(relevantChain(shardID))
		if err != nil {
			return err
		}
		defer dbShard.Close()
	}

	blockChain, err := prepareBlockchain(filepath.Join(cfg.DataDir, "shards.json"), shardID, dbBeacon, dbShard)
	if err != nil {
		return err
	}

	log.Infof("Start scanning...")
	best := blockChain.BestSnapshot()
	log.Infof("Best State: height=%d hash=%s time=%s", best.Height, best.Hash, best.MedianTime)

	flushBlocks := false
	flushInputs := false
	flushOuts := false

	for height := int32(offset); height <= best.Height; height++ {
		select {
		case <-ctx.Done():
			log.Info("Stop scanning.")
			cancel()
			return nil
		default:
		}

		blk, err := blockChain.BlockByHeight(height)
		if err != nil {
			return err
		}
		txs := blk.Transactions()
		result := Block{
			BlockHash:   blk.Hash().String(),
			BlockHeight: int64(blk.Height()),
			TxCount:     len(txs),
			Weight:      blk.MsgBlock().SerializeSize(),
			Bits:        blk.MsgBlock().Header.Bits(),
			Timestamp:   blk.MsgBlock().Header.Timestamp().UTC().Format(time.RFC3339),
		}

		for txId, tx := range txs {
			fmt.Printf("\r\033[0K-> Process Block	hash=%s	time=%s	height=%d/%d	tx=%d/%d ",
				blk.Hash(), blk.MsgBlock().Header.Timestamp().UTC().Format(time.RFC3339),
				blk.Height(), best.Height, txId, result.TxCount,
			)

			result.InCount += len(tx.MsgTx().TxIn)
			result.OutCount += len(tx.MsgTx().TxOut)
			txId = txId
			coinbase := chaindata.IsCoinBase(tx)

			for outId, out := range tx.MsgTx().TxOut {
				select {
				case <-ctx.Done():
					log.Info("\nStop scanning.")
					cancel()
					return nil
				default:
				}

				class, addrr, _, _ := txscript.ExtractPkScriptAddrs(out.PkScript, activeNetParams)
				addresses := make([]string, 0, len(addrr))
				for _, adr := range addrr {
					addresses = append(addresses, adr.EncodeAddress())
				}

				op := Output{
					PkScript:     hex.EncodeToString(out.PkScript),
					PkScriptType: class.String(),
					Addresses:    "[" + strings.Join(addresses, ";") + "]",
					OutID:        outId,
					Amount:       out.Value,
					TxHash:       tx.Hash().String(),
					TxIndex:      txId,
					Coinbase:     coinbase,
					BlockHash:    blk.Hash().String(),
					BlockHeight:  int64(blk.Height()),
				}
				outputsChan <- row{flush: flushOuts, data: op}
				flushOuts = false
			}

			if chaindata.IsCoinBase(tx) {
				continue
			}

			for inId, in := range tx.MsgTx().TxIn {
				select {
				case <-ctx.Done():
					log.Info("\nStop scanning.")
					cancel()
					return nil
				default:
				}

				op := Input{
					SignatureScript: hex.EncodeToString(in.SignatureScript),
					InID:            inId,
					TxHash:          tx.Hash().String(),
					TxIndex:         txId,
					OriginTxHash:    in.PreviousOutPoint.Hash.String(),
					OriginIdx:       int(in.PreviousOutPoint.Index),
					BlockHash:       blk.Hash().String(),
					BlockHeight:     int64(blk.Height()),
				}
				inputsChan <- row{flush: flushInputs, data: op}
				flushInputs = false
			}
		}

		blocksChan <- row{flush: flushBlocks, data: result}

		flushBlocks = true
		flushInputs = true
		flushOuts = true
	}

	cancel()
	log.Info("\nFinish scanning.")
	return nil
}

func prepareBlockchain(shardsJSONPath string, shardID uint32, dbBeacon, dbShard database.DB) (*blockchain.BlockChain, error) {
	if shardID == 0 {
		return createBlockchain(dbBeacon, beacon.Chain(activeNetParams))
	}

	idx, err := deserializeShardData(shardsJSONPath)
	if err != nil {
		return nil, err
	}

	beaconBlockChain, err := createBlockchain(dbBeacon, beacon.Chain(activeNetParams))
	if err != nil {
		return nil, errors.Wrap(err, "error creating beacon blockchain")
	}

	block, err := getGenesisBlock(shardID, idx, beaconBlockChain)
	if err != nil {
		return nil, errors.Wrap(err, "error getting genesis block from beacon chain")
	}

	shardBlockChain, err := createBlockchain(dbShard, shard.Chain(shardID, activeNetParams, block.MsgBlock().Header.Copy().BeaconHeader(), block.MsgBlock().Transactions[0]))
	if err != nil {
		return nil, errors.Wrap(err, "error creating shard blockchain")
	}

	return shardBlockChain, nil
}

func deserializeShardData(filePath string) (node.Index, error) {
	var idx node.Index

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return node.Index{}, errors.Wrapf(err, "error reading file from %s", filePath)
	}

	if err := json.Unmarshal(data, &idx); err != nil {
		return node.Index{}, errors.Wrap(err, "error deserisalizing data")
	}

	return idx, nil
}

func createBlockchain(db database.DB, chain chain.IChainCtx) (*blockchain.BlockChain, error) {
	interrupt := make(chan struct{})
	var checkpoints []chaincfg.Checkpoint
	var indexManager blockchain.IndexManager
	return blockchain.New(&blockchain.Config{
		DB:           db,
		Interrupt:    interrupt,
		ChainParams:  activeNetParams,
		Checkpoints:  checkpoints,
		IndexManager: indexManager,
		TimeSource:   chaindata.NewMedianTime(),
		SigCache:     txscript.NewSigCache(100000),
		HashCache:    txscript.NewHashCache(100000),
		ChainCtx:     chain,
	})
}

func getGenesisBlock(shardID uint32, idx node.Index, blockChain *blockchain.BlockChain) (*jaxutil.Block, error) {
	shardInfo, ok := idx.Shards[shardID]
	if !ok {
		return nil, errors.New("errror")
	}

	return blockChain.BlockByHeight(shardInfo.GenesisHeight)
}
