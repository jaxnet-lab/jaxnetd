package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/chain"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"golang.org/x/sync/errgroup"
)

func rawScanner(offset int) error {
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
		return scan(ctx, cancelWrite, offset, blocksChan, inputsChan, outputsChan)
	})

	return eGroup.Wait()
}

func scan(ctx context.Context, cancel context.CancelFunc, offset int, blocksChan, inputsChan, outputsChan chan row) error {
	// Load the block database.
	db, err := loadBlockDB(chain.BeaconChain) // todo: add shardID as arg
	if err != nil {
		return err
	}

	defer db.Close()

	interrupt := make(chan struct{})
	var checkpoints []chaincfg.Checkpoint
	var indexManager blockchain.IndexManager

	blockChain, err := blockchain.New(&blockchain.Config{
		DB:           db,
		Interrupt:    interrupt,
		ChainParams:  activeNetParams,
		Checkpoints:  checkpoints,
		IndexManager: indexManager,
		TimeSource:   chaindata.NewMedianTime(),
		SigCache:     txscript.NewSigCache(100000),
		HashCache:    txscript.NewHashCache(100000),
	})
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
