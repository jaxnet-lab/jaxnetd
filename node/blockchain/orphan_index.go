/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package blockchain

import (
	"sync"
	"time"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

type orphanIndex struct {
	// These fields are related to handling of orphan blocks.  They are
	// protected by a combination of the chain lock and the orphan lock.
	orphanLock sync.RWMutex

	orphans          map[chainhash.Hash]*orphanBlock
	actualMMRToBlock map[chainhash.Hash]*orphanBlock

	// prevOrphans     map[chainhash.Hash][]*orphanBlock
	mmrRootsOrphans map[chainhash.Hash][]*orphanBlock

	oldestOrphan *orphanBlock
}

// IsKnownOrphan returns whether the passed hash is currently a known orphan.
// Keep in mind that only a limited number of orphans are held onto for a
// limited amount of time, so this function must not be used as an absolute
// way to test if a block is an orphan block.  A full block (as opposed to just
// its hash) must be passed to ProcessBlock for that purpose.  However, calling
// ProcessBlock with an orphan that already exists results in an error, so this
// function provides a mechanism for a caller to intelligently detect *recent*
// duplicate orphans and react accordingly.
//
// This function is safe for concurrent access.
func (b *orphanIndex) IsKnownOrphan(hash *chainhash.Hash) bool {
	// Protect concurrent access.  Using a read lock only so multiple
	// readers can query without blocking each other.
	b.orphanLock.RLock()
	_, exists := b.orphans[*hash]
	b.orphanLock.RUnlock()

	return exists
}

// GetOrphanRoot returns the head of the chain for the provided hash from the
// map of orphan blocks.
//
// This function is safe for concurrent access.
func (b *BlockChain) GetOrphanRoot(hash *chainhash.Hash) *chainhash.Hash {
	return b.blocksDB.GetOrphanMMRRoot(hash)
}

func (storage *rBlockStorage) GetOrphanMMRRoot(hash *chainhash.Hash) *chainhash.Hash {
	// Protect concurrent access.  Using a read lock only so multiple
	// readers can query without blocking each other.
	storage.orphanIndex.orphanLock.RLock()
	defer storage.orphanIndex.orphanLock.RUnlock()

	// Keep looping while the parent of each orphaned block is
	// known and is an orphan itself.
	orphanRoot := hash
	prevHash := hash
	for {
		orphan, exists := storage.orphanIndex.orphans[*prevHash]
		if !exists {
			break
		}

		orphanRoot = prevHash

		orphan, exists = storage.orphanIndex.actualMMRToBlock[orphan.block.PrevMMRRoot()]
		if exists {
			prevHash = orphan.block.Hash()
		} else {
			break
		}
	}

	return orphanRoot
}

// removeOrphanBlock removes the passed orphan block from the orphan pool and
// previous orphan index.
func (storage *rBlockStorage) removeOrphanBlock(orphan *orphanBlock) {
	// Protect concurrent access.
	storage.orphanIndex.orphanLock.Lock()
	defer storage.orphanIndex.orphanLock.Unlock()

	// Remove the orphan block from the orphan pool.
	orphanHash := orphan.block.Hash()
	delete(storage.orphanIndex.orphans, *orphanHash)

	// Remove the reference from the previous orphan index too.  An indexing
	// for loop is intentionally used over a range here as range does not
	// reevaluate the slice on each iteration nor does it adjust the index
	// for the modified slice.

	prevMMRRoot := orphan.block.PrevMMRRoot()

	orphans := storage.orphanIndex.mmrRootsOrphans[prevMMRRoot]
	for i := 0; i < len(orphans); i++ {
		hash := orphans[i].block.Hash()

		if hash.IsEqual(orphanHash) {
			copy(orphans[i:], orphans[i+1:])
			orphans[len(orphans)-1] = nil
			orphans = orphans[:len(orphans)-1]
			i--
		}
	}

	storage.orphanIndex.mmrRootsOrphans[prevMMRRoot] = orphans

	// Remove the map entry altogether if there are no longer any orphans
	// which depend on the parent hash.
	if len(storage.orphanIndex.mmrRootsOrphans[prevMMRRoot]) == 0 {
		delete(storage.orphanIndex.mmrRootsOrphans, prevMMRRoot)
	}
}

// addOrphanBlock adds the passed block (which is already determined to be
// an orphan prior calling this function) to the orphan pool.  It lazily cleans
// up any expired blocks so a separate cleanup poller doesn't need to be run.
// It also imposes a maximum limit on the number of outstanding orphan
// blocks and will remove the oldest received orphan block if the limit is
// exceeded.
func (storage *rBlockStorage) addOrphanBlock(block *jaxutil.Block, blockActualMMR chainhash.Hash) {
	// Remove expired orphan blocks.
	for _, oBlock := range storage.orphanIndex.orphans {
		if time.Now().After(oBlock.expiration) {
			storage.removeOrphanBlock(oBlock)
			continue
		}

		// Update the oldest orphan block pointer so it can be discarded
		// in case the orphan pool fills up.
		if storage.orphanIndex.oldestOrphan == nil || oBlock.expiration.Before(storage.orphanIndex.oldestOrphan.expiration) {
			storage.orphanIndex.oldestOrphan = oBlock
		}
	}

	// Limit orphan blocks to prevent memory exhaustion.
	if len(storage.orphanIndex.orphans)+1 > maxOrphanBlocks {
		// Remove the oldest orphan to make room for the new one.
		storage.removeOrphanBlock(storage.orphanIndex.oldestOrphan)
		storage.orphanIndex.oldestOrphan = nil
	}

	// Protect concurrent access.  This is intentionally done here instead
	// of near the top since removeOrphanBlock does its own locking and
	// the range iterator is not invalidated by removing map entries.
	storage.orphanIndex.orphanLock.Lock()
	defer storage.orphanIndex.orphanLock.Unlock()

	// Insert the block into the orphan map with an expiration time
	// 1 hour from now.
	expiration := time.Now().Add(time.Hour)
	oBlock := &orphanBlock{
		block:      block,
		actualMMR:  blockActualMMR,
		expiration: expiration,
	}
	storage.orphanIndex.orphans[*block.Hash()] = oBlock
	if !blockActualMMR.IsZero() {
		storage.orphanIndex.actualMMRToBlock[blockActualMMR] = oBlock
	}

	// Add to previous hash lookup index for faster dependency lookups.
	prevMMRRoot := block.MsgBlock().Header.PrevBlocksMMRRoot()
	storage.orphanIndex.mmrRootsOrphans[prevMMRRoot] = append(storage.orphanIndex.mmrRootsOrphans[prevMMRRoot], oBlock)
}
