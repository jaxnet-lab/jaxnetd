// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"time"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
)

// processOrphans determines if there are any orphans which depend on the passed
// block hash (they are no longer orphans if true) and potentially accepts them.
// It repeats the process for the newly accepted blocks (to detect further
// orphans which may no longer be orphans) until there are no more.
//
// The flags do not modify the behavior of this function directly, however they
// are needed to pass along to maybeAcceptBlock.
//
// This function MUST be called with the chain state lock held (for writes).
func (b *BlockChain) processOrphans(root chainhash.Hash, flags chaindata.BehaviorFlags) error {
	// Start with processing at least the passed hash.  Leave a little room
	// for additional orphan blocks that need to be processed without
	// needing to grow the array in the common case.

	processMMRRoots := make([]*chainhash.Hash, 0, 10)
	processMMRRoots = append(processMMRRoots, &root)
	for len(processMMRRoots) > 0 {
		// Pop the first hash to process from the slice.

		processMMRRoot := processMMRRoots[0]
		processMMRRoots[0] = nil // Prevent GC leak.
		processMMRRoots = processMMRRoots[1:]

		// Look up all orphans that are parented by the block we just
		// accepted.  This will typically only be one, but it could
		// be multiple if multiple blocks are mined and broadcast
		// around the same time.  The one with the most proof of work
		// will eventually win out.  An indexing for loop is
		// intentionally used over a range here as range does not
		// reevaluate the slice on each iteration nor does it adjust the
		// index for the modified slice.
		for i := 0; i < len(b.blocksDB.orphanIndex.mmrRootsOrphans[*processMMRRoot]); i++ {
			orphan := b.blocksDB.orphanIndex.mmrRootsOrphans[*processMMRRoot][i]
			if orphan == nil {
				log.Warn().Str("chain", b.chain.Name()).
					Msgf("Found a nil entry at index %d in the orphan dependency list for block with mmr root %s", i, processMMRRoot)
				continue
			}

			// Remove the orphan from the orphan pool.
			b.blocksDB.removeOrphanBlock(orphan)
			i--

			prevBlock, _, err := b.blocksDB.getBlockParent(orphan.block.PrevMMRRoot())
			if err != nil {
				return err
			}

			prevHash := prevBlock.GetHash()
			bph := orphan.block.MsgBlock().Header.PrevBlockHash()
			if !prevHash.IsEqual(&bph) {
				str := fmt.Sprintf("hash(%d,%s) of parent resolved by mmr(%s) not match with hash(%d,%s) from header",
					prevBlock.Height(), prevHash, orphan.block.PrevMMRRoot(), orphan.block.Height(), bph)
				return chaindata.NewRuleError(chaindata.ErrInvalidAncestorBlock, str)
			}

			// Potentially accept the block into the block chain.
			_, err = b.maybeAcceptBlock(orphan.block, prevBlock, flags)
			if err != nil {
				return err
			}

			// Add this block to the list of blocks to process so
			// any orphan blocks that depend on this block are
			// handled too.
			processMMRRoots = append(processMMRRoots, &orphan.actualMMR)
		}
	}
	return nil
}

// ProcessBlock is the main workhorse for handling insertion of new blocks into
// the blockchain.  It includes functionality such as rejecting duplicate
// blocks, ensuring blocks follow all rules, orphan handling, and insertion into
// the blockchain along with best chain selection and reorganization.
//
// When no errors occurred during processing, the first return value indicates
// whether or not the block is on the main chain and the second indicates
// whether or not the block is an orphan.
//
// This function is safe for concurrent access.
func (b *BlockChain) ProcessBlock(block *jaxutil.Block, blockActualMMR chainhash.Hash, flags chaindata.BehaviorFlags) (bool, bool, error) {
	b.chainLock.Lock()
	defer b.chainLock.Unlock()

	fastAdd := flags&chaindata.BFFastAdd == chaindata.BFFastAdd

	blockHash := block.Hash()
	log.Trace().Str("chain", b.chain.Name()).Msgf("Processing block %v", blockHash)

	// The block must not already exist in the main chain or side chains.
	exists, err := b.blocksDB.blockExists(b.db, blockHash)
	if err != nil {
		return false, false, err
	}
	if exists {
		str := fmt.Sprintf("already have block %v", blockHash)
		return false, false, chaindata.NewRuleError(chaindata.ErrDuplicateBlock, str)
	}

	// The block must not already exist as an orphan.
	_, orphan := b.blocksDB.orphanIndex.orphans[*blockHash]
	if orphan {
		str := fmt.Sprintf("already have block (orphan) %v", blockHash)
		return false, false, chaindata.NewRuleError(chaindata.ErrDuplicateBlock, str)
	}

	// Perform preliminary sanity checks on the block and its transactions.
	err = chaindata.CheckBlockSanityWF(block, b.chain.Params(), b.TimeSource, flags)
	if err != nil {
		return false, false, err
	}

	// Find the previous checkpoint and perform some additional checks based
	// on the checkpoint.  This provides a few nice properties such as
	// preventing old side chain blocks before the last checkpoint,
	// rejecting easy to mine, but otherwise bogus, blocks that could be
	// used to eat memory, and ensuring expected (versus claimed) proof of
	// work requirements since the previous checkpoint are met.
	blockHeader := block.MsgBlock().Header
	checkpointNode, err := b.findPreviousCheckpoint()
	if err != nil {
		return false, false, err
	}
	if checkpointNode != nil {
		// Ensure the block timestamp is after the checkpoint timestamp.
		checkpointTime := time.Unix(checkpointNode.Timestamp(), 0)
		if blockHeader.Timestamp().Before(checkpointTime) {
			str := fmt.Sprintf("block %v has timestamp %v before last checkpoint timestamp %v",
				blockHash, blockHeader.Timestamp(), checkpointTime)
			return false, false, chaindata.NewRuleError(chaindata.ErrCheckpointTimeTooOld, str)
		}

		if !fastAdd {
			// Even though the checks prior to now have already ensured the
			// proof of work exceeds the claimed amount, the claimed amount
			// is a field in the block header which could be forged.  This
			// check ensures the proof of work is at least the minimum
			// expected based on elapsed time since the last checkpoint and
			// maximum adjustment allowed by the retarget rules.
			duration := blockHeader.Timestamp().Sub(checkpointTime)

			requiredTarget := pow.CompactToBig(b.calcEasiestDifficulty(checkpointNode.Bits(), duration))
			currentTarget := pow.CompactToBig(blockHeader.Bits())

			if currentTarget.Cmp(requiredTarget) > 0 {
				str := fmt.Sprintf("block target difficulty of %064x is too low when compared to the previous "+
					"checkpoint", currentTarget)
				return false, false, chaindata.NewRuleError(chaindata.ErrDifficultyTooLow, str)
			}
		}
	}

	prevBlock, prevExists, err := b.blocksDB.getBlockParent(block.PrevMMRRoot())
	if prevExists && err != nil {
		return false, false, err
	}

	if !prevExists {
		log.Info().Str("chain", b.chain.Name()).
			Msgf("Adding orphan block (%d,%v) with prev_mmr_root %v", block.Height(), blockHash, block.PrevMMRRoot())
		b.blocksDB.addOrphanBlock(block, blockActualMMR)
		return false, true, nil
	}

	prevHash := prevBlock.GetHash()
	bph := block.MsgBlock().Header.PrevBlockHash()
	if !prevHash.IsEqual(&bph) {
		str := fmt.Sprintf("hash(%s) of parent resolved by mmr(%s) not match with hash(%s) from header",
			prevHash, block.PrevMMRRoot(), bph)
		return false, false, chaindata.NewRuleError(chaindata.ErrInvalidAncestorBlock, str)
	}

	// The block has passed all context independent checks and appears sane
	// enough to potentially accept it into the block chain.
	isMainChain, err := b.maybeAcceptBlock(block, prevBlock, flags)
	if err != nil {
		return false, false, err
	}

	actualMMRRoot, found := b.blocksDB.getMMRRootForHash(*blockHash)
	if !found {
		// how it's possible?
		actualMMRRoot = blockActualMMR
	}

	// Handle orphan blocks.
	// Accept any orphan blocks that depend on this block (they are
	// no longer orphans) and repeat for those accepted blocks until
	// there are no more.
	err = b.processOrphans(actualMMRRoot, flags)
	if err != nil {
		return false, false, err
	}

	log.Info().Str("chain", b.chain.Name()).Msgf("Accepted block (%d,%v)", block.Height(), blockHash)

	return isMainChain, false, nil
}
