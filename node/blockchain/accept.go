// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
)

// maybeAcceptBlock potentially accepts a block into the block chain and, if
// accepted, returns whether or not it is on the main chain.  It performs
// several validation checks which depend on its position within the block chain
// before adding it.  The block is expected to have already gone through
// ProcessBlock before calling this function with it.
//
// The flags are also passed to checkBlockContext and connectBestChain.  See
// their documentation for how the flags modify their behavior.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) maybeAcceptBlock(block *jaxutil.Block, prevNode blocknodes.IBlockNode, flags chaindata.BehaviorFlags) (bool, error) {
	// The height of this block is one more than the referenced previous
	// block.

	// Perform checks of the coinbase tx structure according to merge mining spec.
	err := b.blockGen.ValidateJaxAuxRules(block.MsgBlock(), block.Height())
	if err != nil {
		return false, chaindata.NewRuleError(chaindata.ErrInvalidJaxHeaderAux, err.Error())
	}

	// The block must pass all of the validation rules which depend on the
	// position of the block within the block chain.
	err = b.checkBlockContext(block, prevNode, flags)
	if err != nil {
		return false, err
	}

	// Insert the block into the database if it's not already there.  Even
	// though it is possible the block will ultimately fail to connect, it
	// has already passed all proof-of-work and validity tests which means
	// it would be prohibitively expensive for an attacker to fill up the
	// disk with a bunch of blocks that fail to connect.  This is necessary
	// since it allows block download to be decoupled from the much more
	// expensive connection logic.  It also has some other nice properties
	// such as making blocks that never become part of the main chain or
	// blocks that fail to connect available for further analysis.
	err = b.db.Update(func(dbTx database.Tx) error {
		return chaindata.RepoTx(dbTx).DBStoreBlock(block)
	})
	if err != nil {
		return false, err
	}

	// Incrementing lastSerialID, because block was stored.
	b.blocksDB.lastSerialID++

	// Create a new block node for the block and add it to the node index. Even
	// if the block ultimately gets connected to the main chain, it starts out
	// on a side chain.
	blockHeader := block.MsgBlock().Header
	newNode := b.chain.NewNode(blockHeader, prevNode, b.blocksDB.lastSerialID)

	newNode.SetStatus(blocknodes.StatusDataStored)

	b.blocksDB.index.AddNode(newNode)
	err = b.blocksDB.index.flushToDB()
	if err != nil {
		return false, err
	}

	err = b.db.Update(func(dbTx database.Tx) error {
		err := chaindata.RepoTx(dbTx).PutMMRRoot(b.blocksDB.index.MMRTreeRoot(), newNode.GetHash())
		if err != nil {
			return err
		}

		return chaindata.RepoTx(dbTx).PutHashToSerialIDWithPrev(newNode.GetHash(),
			newNode.SerialID(), newNode.Parent().SerialID())
	})

	if err != nil {
		return false, err
	}

	// Connect the passed block to the chain while respecting proper chain
	// selection according to the chain with the most proof of work.  This
	// also handles validation of the transaction scripts.
	isMainChain, err := b.connectBestChain(newNode, block, flags)
	if err != nil {
		return false, err
	}

	// Notify the caller that the new block was accepted into the block
	// chain.  The caller would typically want to react by relaying the
	// inventory to other peers.
	b.chainLock.Unlock()
	b.sendNotification(NTBlockAccepted, BlockNotification{Block: block, ActualMMRRoot: newNode.ActualMMRRoot()})
	b.chainLock.Lock()

	return isMainChain, nil
}
