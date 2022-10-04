package blockchain

import (
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// FetchUtxoView loads unspent transaction outputs for the inputs referenced by
// the passed transaction from the point of view of the end of the main chain.
// It also attempts to fetch the utxos for the outputs of the transaction itself
// so the returned view can be examined for duplicate transactions.
//
// This function is safe for concurrent access however the returned view is NOT.
func (b *BlockChain) FetchUtxoView(tx *jaxutil.Tx) (*chaindata.UtxoViewpoint, error) {
	// Create a set of needed outputs based on those referenced by the
	// inputs of the passed transaction and the outputs of the transaction
	// itself.
	neededSet := make(map[wire.OutPoint]struct{})
	prevOut := wire.OutPoint{Hash: *tx.Hash()}
	for txOutIdx := range tx.MsgTx().TxOut {
		prevOut.Index = uint32(txOutIdx)
		neededSet[prevOut] = struct{}{}
	}
	if !chaindata.IsCoinBase(tx) {
		for _, txIn := range tx.MsgTx().TxIn {
			neededSet[txIn.PreviousOutPoint] = struct{}{}
		}
	}

	// Request the utxos from the point of view of the end of the main
	// chain.
	view := chaindata.NewUtxoViewpoint(b.chain.IsBeacon())
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
		entry, err = chaindata.RepoTx(dbTx).FetchUtxoEntry(outpoint)
		return err
	})
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (b *BlockChain) ListUtxoEntry(limit int) (map[wire.OutPoint]*chaindata.UtxoEntry, error) {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()
	var entries map[wire.OutPoint]*chaindata.UtxoEntry
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		entries, err = chaindata.RepoTx(dbTx).FetchUtxoEntries(limit)
		return err
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func (b *BlockChain) ListEADAddresses() (map[string]*wire.EADAddresses, error) {
	b.chainLock.RLock()
	defer b.chainLock.RUnlock()

	var entries map[string]*wire.EADAddresses
	err := b.db.View(func(dbTx database.Tx) error {
		var err error
		entries, err = chaindata.RepoTx(dbTx).FetchAllEADAddresses()
		return err
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}
