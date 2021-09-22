package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
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

func (histScanner) New() histScanner {
	return histScanner{
		addresses:      NewAddressIndex(),
		hashes:         NewHashIndex(),
		scripts:        NewUTXOIndex(),
		addressTxBus:   make(chan row),
		txOperationBus: make(chan row),
		utxoBus:        make(chan row),
		inputsBus:      make(chan row),
	}
}

func (scanner *histScanner) runWriters(eGroup *errgroup.Group, writerCtx context.Context) {
	eGroup.Go(func() error { return scanner.addresses.Write(writerCtx) })
	eGroup.Go(func() error { return scanner.hashes.Write(writerCtx) })
	eGroup.Go(func() error {
		blocksFile, err := NewCSVStorage("archive/addresses_txs.csv")
		if err != nil {
			return err
		}
		return blocksFile.WriteData(writerCtx, scanner.addressTxBus)
	})
	eGroup.Go(func() error {
		blocksFile, err := NewCSVStorage("archive/tx_ops.csv")
		if err != nil {
			return err
		}
		return blocksFile.WriteData(writerCtx, scanner.txOperationBus)
	})
	eGroup.Go(func() error {
		blocksFile, err := NewCSVStorage("archive/utxo.csv")
		if err != nil {
			return err
		}
		return blocksFile.WriteData(writerCtx, scanner.utxoBus)
	})
	eGroup.Go(func() error {
		blocksFile, err := NewCSVStorage("archive/inputs.csv")
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
		select {
		case <-interruptChannel:
			cancelScanning()
		}
	}()
	scanner := histScanner{}.New()
	eGroup := errgroup.Group{}

	scanner.runWriters(&eGroup, writerCtx)

	eGroup.Go(func() error {
		return scanner.scan(ctx, cancelWrite, offset, limit, shardID)
	})

	return eGroup.Wait()
}

func (scanner *histScanner) scan(ctx context.Context, cancel context.CancelFunc, offset int, limit *int, shardID uint32) error {
	// Load the block database.
	db, err := loadBlockDB(relevantChain(shardID)) // todo: add shardID as arg
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
		for txId, tx := range txs {
			fmt.Printf("\r\033[0K-> Process Block	hash=%s	time=%s	height=%d/%d	tx=%d/%d ",
				blk.Hash(), timestamp,
				blk.Height(), end, txId, txCount,
			)
			// write to tx_hashes.csv
			txHashID := scanner.hashes.Add(tx.Hash().String())

			for outId, out := range tx.MsgTx().TxOut {
				select {
				case <-ctx.Done():
					log.Info("\nStop scanning.")
					cancel()
					return nil
				default:
				}

				scanner.scripts.Add(txCount, outId, out.PkScript, out.Value)

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
							OutID:    outId,
							Amount:   out.Value,
							PkScript: hex.EncodeToString(out.PkScript),
						},
					}

					scanner.addressTxBus <- row{
						flush: flushOuts,
						data: AddressTx{
							Address:   addressID,
							TxHash:    txHashID,
							OutID:     outId,
							Direction: true,
						},
					}

					scanner.txOperationBus <- row{
						flush: flushOuts,
						data: TxOperation{
							TxHash:      tx.Hash().String(),
							TxIndex:     txId,
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

			for inId, in := range tx.MsgTx().TxIn {
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
							OutID:     inId,
							Direction: true,
						},
					}

					scanner.txOperationBus <- row{
						flush: flushInputs,
						data: TxOperation{
							TxHash:      tx.Hash().String(),
							TxIndex:     txId,
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
