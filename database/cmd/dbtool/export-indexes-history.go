package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"

	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"golang.org/x/sync/errgroup"
)

type histScanner struct {
	addresses AddressUniqueIndex
	hashes    HashUniqueIndex
	scripts   UTXOIndex

	addressTxBus   chan row
	txOperationBus chan row
	utxoBus        chan row
	inputsBus      chan row
}

func (histScanner) New(shardID uint32) histScanner {
	return histScanner{
		addresses:      NewAddressIndex(shardID),
		hashes:         NewHashIndex(shardID),
		scripts:        NewUTXOIndex(),
		addressTxBus:   make(chan row),
		txOperationBus: make(chan row),
		utxoBus:        make(chan row),
		inputsBus:      make(chan row),
	}
}

func (scanner *histScanner) runWriters(writerCtx context.Context, eGroup *errgroup.Group, shardID uint32) {
	eGroup.Go(func() error { return scanner.addresses.Write(writerCtx) })
	eGroup.Go(func() error { return scanner.hashes.Write(writerCtx) })
	eGroup.Go(func() error {
		blocksFile, err := NewCSVStorage(filepath.Join(cfg.DataDir, getChainDir(shardID), "archive"+"addresses_txs.csv"))
		if err != nil {
			return err
		}
		return blocksFile.WriteData(writerCtx, scanner.addressTxBus)
	})
	eGroup.Go(func() error {
		blocksFile, err := NewCSVStorage(filepath.Join(cfg.DataDir, getChainDir(shardID), "archive"+"tx_ops.csv"))
		if err != nil {
			return err
		}
		return blocksFile.WriteData(writerCtx, scanner.txOperationBus)
	})
	eGroup.Go(func() error {
		blocksFile, err := NewCSVStorage(filepath.Join(cfg.DataDir, getChainDir(shardID), "archive"+"utxo.csv"))
		if err != nil {
			return err
		}
		return blocksFile.WriteData(writerCtx, scanner.utxoBus)
	})
	eGroup.Go(func() error {
		blocksFile, err := NewCSVStorage(filepath.Join(cfg.DataDir, getChainDir(shardID), "archive"+"inputs.csv"))
		if err != nil {
			return err
		}
		return blocksFile.WriteData(writerCtx, scanner.inputsBus)
	})
}

func historyScanner(offset int, limit *int, shardID uint32) error {
	ctx, cancelScanning := context.WithCancel(context.Background())
	writerCtx, cancelWrite := context.WithCancel(context.Background())

	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, os.Interrupt)
	go func() {
		<-interruptChannel
		cancelScanning()
	}()
	scanner := histScanner{}.New(shardID)
	eGroup := errgroup.Group{}

	scanner.runWriters(writerCtx, &eGroup, shardID)

	eGroup.Go(func() error {
		return scanner.scan(ctx, cancelWrite, offset, limit, shardID)
	})

	return eGroup.Wait()
}

func (scanner *histScanner) scan(ctx context.Context, cancel context.CancelFunc, offset int, limit *int, shardID uint32) error {
	// Load the block database.
	var dbShard database.DB
	dbBeacon, err := loadBlockDB(chainctx.NewBeaconChain(activeNetParams))
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
		log.Errorf("Error creating blockchain %s, aborting", err)
		return err
	}

	log.Infof("Start scanning...")
	best := blockChain.BestSnapshot()
	log.Infof("Best State: height=%d hash=%s time=%s", best.Height, best.Hash, best.MedianTime)

	flushInputs := false
	flushOuts := false

	end := best.Height
	if limit != nil {
		end = int32(*limit)
	}

	for height := int32(offset); height <= end; height++ {
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

		timestamp := blk.MsgBlock().Header.Timestamp().UTC().Format(time.RFC3339)
		txCount := len(txs)
		for txID, tx := range txs {
			fmt.Printf("\r\033[0K-> Process Block	hash=%s	time=%s	height=%d/%d	tx=%d/%d ",
				blk.Hash(), timestamp,
				blk.Height(), end, txID, txCount,
			)
			// write to tx_hashes.csv
			txHashID := scanner.hashes.Add(tx.Hash().String())

			for outID, out := range tx.MsgTx().TxOut {
				select {
				case <-ctx.Done():
					log.Info("\nStop scanning.")
					cancel()
					return nil
				default:
				}

				scanner.scripts.Add(txCount, outID, out.PkScript, out.Value)

				class, addrr, _, err := txscript.ExtractPkScriptAddrs(out.PkScript, activeNetParams)
				if err != nil || class == txscript.NonStandardTy {
					continue
				}
				for _, adr := range addrr {
					addressID := scanner.addresses.Add(adr.EncodeAddress())

					scanner.utxoBus <- row{
						flush: flushOuts,
						data: UTXO{
							Address:  addressID,
							TxHash:   txHashID,
							OutID:    outID,
							Amount:   out.Value,
							PkScript: hex.EncodeToString(out.PkScript),
						},
					}

					scanner.addressTxBus <- row{
						flush: flushOuts,
						data: AddressTx{
							Address:   addressID,
							TxHash:    txHashID,
							OutID:     outID,
							Direction: true,
						},
					}

					scanner.txOperationBus <- row{
						flush: flushOuts,
						data: TxOperation{
							TxHash:      tx.Hash().String(),
							TxIndex:     txID,
							Address:     adr.EncodeAddress(),
							Amount:      out.Value,
							BlockNumber: int(blk.Height()),
							IsInput:     false,
							PkScript:    hex.EncodeToString(out.PkScript),
							DateTime:    timestamp,
						},
					}
					flushOuts = false
				}
			}

			if chaindata.IsCoinBase(tx) {
				continue
			}

			for inID, in := range tx.MsgTx().TxIn {
				select {
				case <-ctx.Done():
					log.Info("\nStop scanning.")
					cancel()
					return nil
				default:
				}

				parentHashID := scanner.hashes.Add(in.PreviousOutPoint.Hash.String())
				pkScript, value := scanner.scripts.Get(parentHashID, int(in.PreviousOutPoint.Index))
				class, addrr, _, err := txscript.ExtractPkScriptAddrs(pkScript, activeNetParams)
				if err != nil || class == txscript.NonStandardTy {
					continue
				}

				scanner.inputsBus <- row{
					flush: flushInputs,
					data: ShortInput{
						TxHash:      txHashID,
						InputTxHash: parentHashID,
						InputTxIDx:  int(in.PreviousOutPoint.Index),
					},
				}

				for _, adr := range addrr {
					addressID := scanner.addresses.Add(adr.EncodeAddress())

					scanner.addressTxBus <- row{
						flush: flushInputs,
						data: AddressTx{
							Address:   addressID,
							TxHash:    txHashID,
							OutID:     inID,
							Direction: true,
						},
					}

					scanner.txOperationBus <- row{
						flush: flushInputs,
						data: TxOperation{
							TxHash:      tx.Hash().String(),
							TxIndex:     txID,
							Address:     adr.EncodeAddress(),
							Amount:      value,
							BlockNumber: int(blk.Height()),
							IsInput:     false,
							PkScript:    hex.EncodeToString(pkScript),
							DateTime:    timestamp,
						},
					}
					flushInputs = false
				}
			}
		}

		flushInputs = true
		flushOuts = true
	}

	cancel()
	log.Info("\nFinish scanning.")
	return nil
}
