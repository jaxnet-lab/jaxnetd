// Copyright (c) 2015-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaindata

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/wire"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// txoFlags is a bitmask defining additional information and state for a
// transaction output in a utxo view.
type txoFlags uint8

const (
	// tfCoinBase indicates that a txout was contained in a coinbase tx.
	tfCoinBase txoFlags = 1 << iota

	// tfSpent indicates that a txout is spent.
	tfSpent

	// tfModified indicates that a txout has been modified since it was
	// loaded.
	tfModified
)

// UtxoEntry houses details about an individual transaction output in a utxo
// view such as whether or not it was contained in a coinbase tx, the height of
// the block that contains the tx, whether or not it is spent, its public key
// script, and how much it pays.
type UtxoEntry struct {
	// NOTE: Additions, deletions, or modifications to the order of the
	// definitions in this struct should not be changed without considering
	// how it affects alignment on 64-bit platforms.  The current order is
	// specifically crafted to result in minimal padding.  There will be a
	// lot of these in memory, so a few extra bytes of padding adds up.

	amount      int64
	pkScript    []byte // The public key script for the output.
	blockHeight int32  // Height of block containing tx.

	// packedFlags contains additional info about output such as whether it
	// is a coinbase, whether it is spent, and whether it has been modified
	// since it was loaded.  This approach is used in order to reduce memory
	// usage since there will be a lot of these in memory.
	packedFlags txoFlags
}

// isModified returns whether or not the output has been modified since it was
// loaded.
func (entry *UtxoEntry) isModified() bool {
	return entry.packedFlags&tfModified == tfModified
}

// IsCoinBase returns whether or not the output was contained in a coinbase
// transaction.
func (entry *UtxoEntry) IsCoinBase() bool {
	return entry.packedFlags&tfCoinBase == tfCoinBase
}

// BlockHeight returns the height of the block containing the output.
func (entry *UtxoEntry) BlockHeight() int32 {
	return entry.blockHeight
}

// IsSpent returns whether or not the output has been spent based upon the
// current state of the unspent transaction output view it was obtained from.
func (entry *UtxoEntry) IsSpent() bool {
	return entry.packedFlags&tfSpent == tfSpent
}

// Spend marks the output as spent.  Spending an output that is already spent
// has no effect.
func (entry *UtxoEntry) Spend() {
	// Nothing to do if the output is already spent.
	if entry.IsSpent() {
		return
	}

	// Mark the output as spent and modified.
	entry.packedFlags |= tfSpent | tfModified
}

// Amount returns the amount of the output.
func (entry *UtxoEntry) Amount() int64 {
	return entry.amount
}

// PkScript returns the public key script for the output.
func (entry *UtxoEntry) PkScript() []byte {
	return entry.pkScript
}

// Clone returns a shallow copy of the utxo entry.
func (entry *UtxoEntry) Clone() *UtxoEntry {
	if entry == nil {
		return nil
	}

	return &UtxoEntry{
		amount:      entry.amount,
		pkScript:    entry.pkScript,
		blockHeight: entry.blockHeight,
		packedFlags: entry.packedFlags,
	}
}

// UtxoViewpoint represents a view into the set of unspent transaction outputs
// from a specific point of view in the chain.  For example, it could be for
// the end of the main chain, some point in the history of the main chain, or
// down a side chain.
//
// The unspent outputs are needed by other transactions for things such as
// script validation and double spend prevention.
type UtxoViewpoint struct {
	entries  map[wire.OutPoint]*UtxoEntry
	bestHash chainhash.Hash

	eadAddresses map[string]*wire.EADAddresses
}

// BestHash returns the Hash of the best block in the chain the view currently
// respresents.
func (view *UtxoViewpoint) BestHash() *chainhash.Hash {
	return &view.bestHash
}

// SetBestHash sets the Hash of the best block in the chain the view currently
// respresents.
func (view *UtxoViewpoint) SetBestHash(hash *chainhash.Hash) {
	view.bestHash = *hash
}

// LookupEntry returns information about a given transaction output according to
// the current state of the view.  It will return nil if the passed output does
// not exist in the view or is otherwise not available such as when it has been
// disconnected during a reorg.
func (view *UtxoViewpoint) LookupEntry(outpoint wire.OutPoint) *UtxoEntry {
	return view.entries[outpoint]
}

// addTxOut adds the specified output to the view if it is not provably
// unspendable.  When the view already has an entry for the output, it will be
// marked unspent.  All fields will be updated for existing entries since it's
// possible it has changed during a reorg.
func (view *UtxoViewpoint) addTxOut(outpoint wire.OutPoint, txOut *wire.TxOut, isCoinBase bool, blockHeight int32) {
	// Don't add provably unspendable outputs.
	if txscript.IsUnspendable(txOut.PkScript) {
		return
	}

	// Update existing entries.  All fields are updated because it's
	// possible (although extremely unlikely) that the existing entry is
	// being replaced by a different transaction with the same Hash.  This
	// is allowed so long as the previous transaction is fully spent.
	entry := view.LookupEntry(outpoint)
	if entry == nil {
		entry = new(UtxoEntry)
		view.entries[outpoint] = entry
	}

	entry.amount = txOut.Value
	entry.pkScript = txOut.PkScript
	entry.blockHeight = blockHeight
	entry.packedFlags = tfModified
	if isCoinBase {
		entry.packedFlags |= tfCoinBase
	}
}

// AddTxOut adds the specified output of the passed transaction to the view if
// it exists and is not provably unspendable.  When the view already has an
// entry for the output, it will be marked unspent.  All fields will be updated
// for existing entries since it's possible it has changed during a reorg.
func (view *UtxoViewpoint) AddTxOut(tx *jaxutil.Tx, txOutIdx uint32, blockHeight int32) {
	// Can't add an output for an out of bounds index.
	if txOutIdx >= uint32(len(tx.MsgTx().TxOut)) {
		return
	}

	// Update existing entries.  All fields are updated because it's
	// possible (although extremely unlikely) that the existing entry is
	// being replaced by a different transaction with the same Hash.  This
	// is allowed so long as the previous transaction is fully spent.
	prevOut := wire.OutPoint{Hash: *tx.Hash(), Index: txOutIdx}
	txOut := tx.MsgTx().TxOut[txOutIdx]
	view.addTxOut(prevOut, txOut, IsCoinBase(tx), blockHeight)
}

// AddTxOuts adds all outputs in the passed transaction which are not provably
// unspendable to the view.  When the view already has entries for any of the
// outputs, they are simply marked unspent.  All fields will be updated for
// existing entries since it's possible it has changed during a reorg.
func (view *UtxoViewpoint) AddTxOuts(tx *jaxutil.Tx, blockHeight int32) {
	// Loop all of the transaction outputs and add those which are not
	// provably unspendable.
	isCoinBase := IsCoinBase(tx)
	prevOut := wire.OutPoint{Hash: *tx.Hash()}
	for txOutIdx, txOut := range tx.MsgTx().TxOut {
		// Update existing entries.  All fields are updated because it's
		// possible (although extremely unlikely) that the existing
		// entry is being replaced by a different transaction with the
		// same Hash.  This is allowed so long as the previous
		// transaction is fully spent.
		prevOut.Index = uint32(txOutIdx)
		view.addTxOut(prevOut, txOut, isCoinBase, blockHeight)
	}
}

// ConnectTransaction updates the view by adding all new utxos created by the
// passed transaction and marking all utxos that the transactions spend as
// spent.  In addition, when the 'stxos' argument is not nil, it will be updated
// to append an entry for each spent txout.  An error will be returned if the
// view does not contain the required utxos.
func (view *UtxoViewpoint) ConnectTransaction(tx *jaxutil.Tx, blockHeight int32, stxos *[]SpentTxOut) error {
	// Coinbase transactions don't have any inputs to spend.
	if IsCoinBase(tx) {
		// Add the transaction's outputs as available utxos.
		view.AddTxOuts(tx, blockHeight)
		return nil
	}

	if tx.MsgTx().SwapTx() {
		return view.connectSwapTransaction(tx, blockHeight, stxos)
	}

	if tx.MsgTx().Version == wire.TxVerEADAction {
		err := view.connectEADTransaction(tx)
		if err != nil {
			return err
		}
	}

	// Spend the referenced utxos by marking them spent in the view and,
	// if a slice was provided for the spent txout details, append an entry
	// to it.
	for _, txIn := range tx.MsgTx().TxIn {
		// Ensure the referenced utxo exists in the view.  This should
		// never happen unless there is a bug is introduced in the code.
		entry := view.entries[txIn.PreviousOutPoint]
		if entry == nil {
			return AssertError(fmt.Sprintf("view missing input %v",
				txIn.PreviousOutPoint))
		}

		// Only create the stxo details if requested.
		if stxos != nil {
			// Populate the stxo details using the utxo entry.
			var stxo = SpentTxOut{
				Amount:     entry.Amount(),
				PkScript:   entry.PkScript(),
				Height:     entry.BlockHeight(),
				IsCoinBase: entry.IsCoinBase(),
			}
			*stxos = append(*stxos, stxo)
		}

		// Mark the entry as spent.  This is not done until after the
		// relevant details have been accessed since spending it might
		// clear the fields from memory in the future.
		entry.Spend()
	}

	// Add the transaction's outputs as available utxos.
	view.AddTxOuts(tx, blockHeight)

	err := view.disconnectEADAddresses(stxos)
	if err != nil {
		return err
	}

	return nil
}

// connectSwapTransaction updates the view by adding all new utxos created by the
// passed transaction and marking all utxos that the transactions spend as
// spent. This function handles special case for the wire.TxMarkShardSwap transactions.
// wire.TxMarkShardSwap transaction is a special tx for atomic swap between chains.
// It can contain only TWO or FOUR inputs and TWO or FOUR outputs.
// TxIn and TxOut are strictly associated with each other by index.
// One pair corresponds to the current chain. The second is for another, unknown chain.
//
// | # | --- []TxIn ----- | --- | --- []TxOut ----- | # |
// | - | ---------------- | --- | ----------------- | - |
// | 0 | TxIn_0 ∈ Shard_X | --> | TxOut_0 ∈ Shard_X | 0 |
// | 1 | TxIn_1 ∈ Shard_X | --> | TxOut_1 ∈ Shard_X | 1 |
// | 2 | TxIn_2 ∈ Shard_Y | --> | TxOut_2 ∈ Shard_Y | 2 |
// | 3 | TxIn_3 ∈ Shard_Y | --> | TxOut_3 ∈ Shard_Y | 3 |
//
// The order is not deterministic.
func (view *UtxoViewpoint) connectSwapTransaction(tx *jaxutil.Tx, blockHeight int32, stxos *[]SpentTxOut) error {
	err := ValidateSwapTxStructure(tx.MsgTx(), -1)
	if err != nil {
		return err
	}

	inputs := tx.MsgTx().TxIn
	outputs := tx.MsgTx().TxOut

	// Spend the referenced utxos by marking them spent in the view and,
	// if a slice was provided for the spent txout details, append an entry
	// to it.
	for txInIdx, txIn := range inputs {

		// Check that the specified utxo exists in the view.
		// If UTXO does not exist, it means the UTXO is from a different chain.
		entry, ok := view.entries[txIn.PreviousOutPoint]
		if !ok || entry == nil {
			continue
		}

		// Only create the stxo details if requested.
		if stxos != nil {
			// Populate the stxo details using the utxo entry.
			var stxo = SpentTxOut{
				Amount:     entry.Amount(),
				PkScript:   entry.PkScript(),
				Height:     entry.BlockHeight(),
				IsCoinBase: entry.IsCoinBase(),
			}
			*stxos = append(*stxos, stxo)
		}

		// Mark the entry as spent.  This is not done until after the
		// relevant details have been accessed since spending it might
		// clear the fields from memory in the future.
		entry.Spend()

		prevOut := wire.OutPoint{Hash: *tx.Hash()}
		// Update existing entries.  All fields are updated because it's
		// possible (although extremely unlikely) that the existing
		// entry is being replaced by a different transaction with the
		// same Hash.  This is allowed so long as the previous
		// transaction is fully spent.
		prevOut.Index = uint32(txInIdx)
		txOut := outputs[txInIdx]
		view.addTxOut(prevOut, txOut, false, blockHeight)
	}
	return nil
}

func (view *UtxoViewpoint) EADAddressesSet() map[string]*wire.EADAddresses {
	return view.eadAddresses
}

func (view *UtxoViewpoint) connectEADTransaction(tx *jaxutil.Tx) error {
	outputs := tx.MsgTx().TxOut
	for outInd, out := range outputs {
		class := txscript.GetScriptClass(out.PkScript)
		if class != txscript.EADAddressTy {
			continue
		}

		scriptData, err := txscript.EADAddressScriptData(out.PkScript)
		if err != nil {
			return err
		}

		ownerKey := string(scriptData.RawKey)
		addr, ok := view.eadAddresses[ownerKey]
		if !ok {
			b := make([]byte, 8)
			_, _ = rand.Read(b)

			addr = &wire.EADAddresses{
				ID:          binary.LittleEndian.Uint64(b),
				OwnerPubKey: []byte(ownerKey),
				IPs:         []wire.EADAddress{},
			}
		}

		view.eadAddresses[ownerKey] = addr.AddAddress(
			scriptData.IP,
			scriptData.URL,
			uint16(scriptData.Port),
			scriptData.ExpirationDate,
			scriptData.ShardID,
			tx.Hash(),
			outInd,
		)
	}

	return nil
}

func (view *UtxoViewpoint) disconnectEADAddresses(stxos *[]SpentTxOut) error {
	if stxos == nil {
		return nil
	}

	for _, input := range *stxos {
		class := txscript.GetScriptClass(input.PkScript)
		if class != txscript.EADAddressTy {
			continue
		}

		if err := view.removeEAD(input.PkScript); err != nil {
			return err
		}
	}

	return nil
}

func (view *UtxoViewpoint) removeEAD(pkScript []byte) error {
	scriptData, err := txscript.EADAddressScriptData(pkScript)
	if err != nil {
		return err
	}
	ownerKey := string(scriptData.RawKey)
	address, ok := view.eadAddresses[ownerKey]
	if !ok || address == nil || len(address.IPs) < 1 {
		return nil
	}

	filtered := make([]wire.EADAddress, 0, len(address.IPs)-1)
	for _, p := range view.eadAddresses[ownerKey].IPs {
		addr, removed := p.FilterOut(scriptData.IP, scriptData.ShardID)
		if removed {
			continue
		}

		filtered = append(filtered, *addr)
	}
	if len(filtered) == 0 {
		view.eadAddresses[ownerKey] = nil
		return nil
	}
	address.IPs = filtered
	view.eadAddresses[ownerKey] = address
	return nil
}

// ConnectTransactions updates the view by adding all new utxos created by all
// of the transactions in the passed block, marking all utxos the transactions
// spend as spent, and setting the best Hash for the view to the passed block.
// In addition, when the 'stxos' argument is not nil, it will be updated to
// append an entry for each spent txout.
func (view *UtxoViewpoint) ConnectTransactions(block *jaxutil.Block, stxos *[]SpentTxOut) error {
	for _, tx := range block.Transactions() {
		err := view.ConnectTransaction(tx, block.Height(), stxos)
		if err != nil {
			return err
		}
	}

	// Update the best Hash for view to include this block since all of its
	// transactions have been connected.
	view.SetBestHash(block.Hash())
	return nil
}

// fetchEntryByHash attempts to find any available utxo for the given Hash by
// searching the entire set of possible outputs for the given Hash.  It checks
// the view first and then falls back to the database if needed.
func (view *UtxoViewpoint) fetchEntryByHash(db database.DB, hash *chainhash.Hash) (*UtxoEntry, error) {
	// First attempt to find a utxo with the provided TxHash in the view.
	prevOut := wire.OutPoint{Hash: *hash}
	for idx := uint32(0); idx < MaxOutputsPerBlock; idx++ {
		prevOut.Index = idx
		entry := view.LookupEntry(prevOut)
		if entry != nil {
			return entry, nil
		}
	}

	// Check the database since it doesn't exist in the view.  This will
	// often by the case since only specifically referenced utxos are loaded
	// into the view.
	var entry *UtxoEntry
	err := db.View(func(dbTx database.Tx) error {
		var err error
		entry, err = dbFetchUtxoEntryByHash(dbTx, hash)
		return err
	})
	return entry, err
}

// CountSpentOutputs returns the number of utxos the passed block spends.
func CountSpentOutputs(block *jaxutil.Block) int {
	// Exclude the coinbase transaction since it can't spend anything.
	var numSpent int
	for _, tx := range block.Transactions()[1:] {
		if tx.MsgTx().SwapTx() {
			numSpent = numSpent + (len(tx.MsgTx().TxIn) / 2)
		} else {
			numSpent += len(tx.MsgTx().TxIn)
		}
	}
	return numSpent
}

// DisconnectTransactions updates the view by removing all of the transactions
// created by the passed block, restoring all utxos the transactions spent by
// using the provided spent txo information, and setting the best Hash for the
// view to the block before the passed block.
func (view *UtxoViewpoint) DisconnectTransactions(db database.DB, block *jaxutil.Block, stxos []SpentTxOut) error {
	// Sanity check the correct number of stxos are provided.
	if len(stxos) != CountSpentOutputs(block) {
		return AssertError("DisconnectTransactions called with bad " +
			"spent transaction out information")
	}

	// Loop backwards through all transactions so everything is unspent in
	// reverse order.  This is necessary since transactions later in a block
	// can spend from previous ones.
	stxoIdx := len(stxos) - 1
	transactions := block.Transactions()
	for txIdx := len(transactions) - 1; txIdx > -1; txIdx-- {
		tx := transactions[txIdx]

		// All entries will need to potentially be marked as a coinbase.
		var packedFlags txoFlags
		isCoinBase := txIdx == 0
		if isCoinBase {
			packedFlags |= tfCoinBase
		}

		// Mark all of the spendable outputs originally created by the
		// transaction as spent.  It is instructive to note that while
		// the outputs aren't actually being spent here, rather they no
		// longer exist, since a pruned utxo set is used, there is no
		// practical difference between a utxo that does not exist and
		// one that has been spent.
		//
		// When the utxo does not already exist in the view, add an
		// entry for it and then mark it spent.  This is done because
		// the code relies on its existence in the view in order to
		// signal modifications have happened.
		txHash := tx.Hash()
		prevOut := wire.OutPoint{Hash: *txHash}
		for txOutIdx, txOut := range tx.MsgTx().TxOut {
			if txscript.IsUnspendable(txOut.PkScript) {
				continue
			}

			prevOut.Index = uint32(txOutIdx)
			entry := view.entries[prevOut]
			if entry == nil {
				entry = &UtxoEntry{
					amount:      txOut.Value,
					pkScript:    txOut.PkScript,
					blockHeight: block.Height(),
					packedFlags: packedFlags,
				}

				view.entries[prevOut] = entry
			}

			entry.Spend()
		}

		// Loop backwards through all of the transaction inputs (except
		// for the coinbase which has no inputs) and unspend the
		// referenced txos.  This is necessary to match the order of the
		// spent txout entries.
		if isCoinBase {
			continue
		}
		for txInIdx := len(tx.MsgTx().TxIn) - 1; txInIdx > -1; txInIdx-- {
			// Ensure the spent txout index is decremented to stay
			// in sync with the transaction input.
			stxo := &stxos[stxoIdx]
			stxoIdx--

			// When there is not already an entry for the referenced
			// output in the view, it means it was previously spent,
			// so create a new utxo entry in order to resurrect it.
			originOut := &tx.MsgTx().TxIn[txInIdx].PreviousOutPoint
			entry := view.entries[*originOut]
			if entry == nil {
				entry = new(UtxoEntry)
				view.entries[*originOut] = entry
			}

			// The legacy v1 spend journal format only stored the
			// coinbase flag and height when the output was the last
			// unspent output of the transaction.  As a result, when
			// the information is missing, search for it by scanning
			// all possible outputs of the transaction since it must
			// be in one of them.
			//
			// It should be noted that this is quite inefficient,
			// but it realistically will almost never run since all
			// new entries include the information for all outputs
			// and thus the only way this will be hit is if a long
			// enough reorg happens such that a block with the old
			// spend data is being disconnected.  The probability of
			// that in practice is extremely low to begin with and
			// becomes vanishingly small the more new blocks are
			// connected.  In the case of a fresh database that has
			// only ever run with the new v2 format, this code path
			// will never run.
			if stxo.Height == 0 {
				utxo, err := view.fetchEntryByHash(db, txHash)
				if err != nil {
					return err
				}
				if utxo == nil {
					return AssertError(fmt.Sprintf("unable "+
						"to resurrect legacy stxo %v",
						*originOut))
				}

				stxo.Height = utxo.BlockHeight()
				stxo.IsCoinBase = utxo.IsCoinBase()
			}

			// Restore the utxo using the stxo data from the spend
			// journal and mark it as modified.
			entry.amount = stxo.Amount
			entry.pkScript = stxo.PkScript
			entry.blockHeight = stxo.Height
			entry.packedFlags = tfModified
			if stxo.IsCoinBase {
				entry.packedFlags |= tfCoinBase
			}
		}
	}

	// Update the best Hash for view to the previous block since all of the
	// transactions for the current block have been disconnected.
	h := block.MsgBlock().Header.PrevBlock()
	view.SetBestHash(&h)
	return nil
}

// RemoveEntry removes the given transaction output from the current state of
// the view.  It will have no effect if the passed output does not exist in the
// view.
func (view *UtxoViewpoint) RemoveEntry(outpoint wire.OutPoint) {
	delete(view.entries, outpoint)
}

// Entries returns the underlying map that stores of all the utxo entries.
func (view *UtxoViewpoint) Entries() map[wire.OutPoint]*UtxoEntry {
	return view.entries
}

// Commit prunes all entries marked modified that are now fully spent and marks
// all entries as unmodified.
func (view *UtxoViewpoint) Commit() {
	for outpoint, entry := range view.entries {
		if entry == nil || (entry.isModified() && entry.IsSpent()) {
			delete(view.entries, outpoint)
			continue
		}

		entry.packedFlags ^= tfModified
	}
}

// FetchUtxosMain fetches unspent transaction output data about the provided
// set of outpoints from the point of view of the end of the main chain at the
// time of the call.
//
// Upon completion of this function, the view will contain an entry for each
// requested outpoint.  Spent outputs, or those which otherwise don't exist,
// will result in a nil entry in the view.
func (view *UtxoViewpoint) FetchUtxosMain(db database.DB, outpoints map[wire.OutPoint]struct{}) error {
	// Nothing to do if there are no requested outputs.
	if len(outpoints) == 0 {
		return nil
	}

	// Load the requested set of unspent transaction outputs from the point
	// of view of the end of the main chain.
	//
	// NOTE: Missing entries are not considered an error here and instead
	// will result in nil entries in the view.  This is intentionally done
	// so other code can use the presence of an entry in the store as a way
	// to unnecessarily avoid attempting to reload it from the database.
	return db.View(func(dbTx database.Tx) error {
		for outpoint := range outpoints {
			entry, err := DBFetchUtxoEntry(dbTx, outpoint)
			if err != nil {
				return err
			}

			view.entries[outpoint] = entry
		}

		// todo: omit this step if shard
		var err error
		view.eadAddresses, err = DBFetchAllEADAddresses(dbTx)
		if err != nil {
			return err
		}

		return nil
	})
}

// FetchUtxos loads the unspent transaction outputs for the provided set of
// outputs into the view from the database as needed unless they already exist
// in the view in which case they are ignored.
func (view *UtxoViewpoint) FetchUtxos(db database.DB, outpoints map[wire.OutPoint]struct{}) error {
	// Nothing to do if there are no requested outputs.
	if len(outpoints) == 0 {
		return nil
	}

	// Filter entries that are already in the view.
	neededSet := make(map[wire.OutPoint]struct{})
	for outpoint := range outpoints {
		// Already loaded into the current view.
		if _, ok := view.entries[outpoint]; ok {
			continue
		}

		neededSet[outpoint] = struct{}{}
	}

	// Request the input utxos from the database.
	return view.FetchUtxosMain(db, neededSet)
}

// FetchInputUtxos loads the unspent transaction outputs for the inputs
// referenced by the transactions in the given block into the view from the
// database as needed.  In particular, referenced entries that are earlier in
// the block are added to the view and entries that are already in the view are
// not modified.
func (view *UtxoViewpoint) FetchInputUtxos(db database.DB, block *jaxutil.Block) error {
	// Build a map of in-flight transactions because some of the inputs in
	// this block could be referencing other transactions earlier in this
	// block which are not yet in the chain.
	txInFlight := map[chainhash.Hash]int{}
	transactions := block.Transactions()
	for i, tx := range transactions {
		txInFlight[*tx.Hash()] = i
	}

	// Loop through all of the transaction inputs (except for the coinbase
	// which has no inputs) collecting them into sets of what is needed and
	// what is already known (in-flight).
	neededSet := make(map[wire.OutPoint]struct{})
	for i, tx := range transactions[1:] {
		// switch tx.MsgTx().Version {
		// case wire.TxVerShardsSwap:
		// 	// todo: here fault tolerant fetch
		//
		// case wire.TxVerRegular,
		// 	wire.TxVerTimeLock:
		// 	fallthrough
		//
		// default:

		for _, txIn := range tx.MsgTx().TxIn {
			// It is acceptable for a transaction input to reference
			// the output of another transaction in this block only
			// if the referenced transaction comes before the
			// current one in this block.  Add the outputs of the
			// referenced transaction as available utxos when this
			// is the case.  Otherwise, the utxo details are still
			// needed.
			//
			// NOTE: The >= is correct here because i is one less
			// than the actual position of the transaction within
			// the block due to skipping the coinbase.
			originHash := &txIn.PreviousOutPoint.Hash
			if inFlightIndex, ok := txInFlight[*originHash]; ok &&
				i >= inFlightIndex {

				originTx := transactions[inFlightIndex]
				view.AddTxOuts(originTx, block.Height())
				continue
			}

			// Don't request entries that are already in the view
			// from the database.
			if _, ok := view.entries[txIn.PreviousOutPoint]; ok {
				continue
			}

			neededSet[txIn.PreviousOutPoint] = struct{}{}
		}
		// }

	}

	// Request the input utxos from the database.
	return view.FetchUtxosMain(db, neededSet)
}

// NewUtxoViewpoint returns a new empty unspent transaction output view.
func NewUtxoViewpoint() *UtxoViewpoint {
	return &UtxoViewpoint{
		entries:      make(map[wire.OutPoint]*UtxoEntry),
		eadAddresses: make(map[string]*wire.EADAddresses),
	}
}
