package blockchain

import (
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/database"
	"gitlab.com/jaxnet/core/shard.core/node/chaindata"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

// FetchUtxoView loads unspent transaction outputs for the inputs referenced by
// the passed transaction from the point of view of the end of the main chain.
// It also attempts to fetch the utxos for the outputs of the transaction itself
// so the returned view can be examined for duplicate transactions.
//
// This function is safe for concurrent access however the returned view is NOT.
func (b *BlockChain) FetchUtxoView(tx *btcutil.Tx) (*chaindata.UtxoViewpoint, error) {
	// Create a set of needed outputs based on those referenced by the
	// inputs of the passed transaction and the outputs of the transaction
	// itself.
	neededSet := make(map[wire.OutPoint]struct{})
	prevOut := wire.OutPoint{Hash: *tx.Hash()}
	for txOutIdx := range tx.MsgTx().TxOut {
		prevOut.Index = uint32(txOutIdx)
		neededSet[prevOut] = struct{}{}
	}
	if !chaindata.IsCoinBase(tx) { // todo: here will be new special case
		for _, txIn := range tx.MsgTx().TxIn {
			neededSet[txIn.PreviousOutPoint] = struct{}{}
		}
	}

	// Request the utxos from the point of view of the end of the main
	// chain.
	view := chaindata.NewUtxoViewpoint()
	b.chainLock.RLock()
	err := view.FetchUtxosMain(b.db, neededSet)
	b.chainLock.RUnlock()
	return view, err
}

// FetchUtxoEntry loads and returns the requested unspent transaction output
// from the point of view of the end of the main chain.
//
// NOTE: Requesting an output for which there is no data will NOT return an
// error.  Instead both the entry and the error will be nil.  This is done to
// allow pruning of spent transaction outputs.  In practice this means the
// caller must check if the returned entry is nil before invoking methods on it.
//
// This function is safe for concurrent access however the returned entry (if
// any) is NOT.
func (b *BlockChain) FetchUtxoEntry(outpoint wire.OutPoint) (*chaindata.UtxoEntry, error) {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()

	var entry *chaindata.UtxoEntry
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		entry, err = chaindata.DBFetchUtxoEntry(dbTx, outpoint)
		return err
	})
	if err != nil {
		return nil, err
	}

	return entry, nil
}
