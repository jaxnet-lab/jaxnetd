/*
 * Copyright (c) 2022 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package blockchain

import (
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
)

// VerifyStateSanity attempts to load and initialize the chain state from the
// database. When the db does not yet contain any chain state, both it and the
// chain state are initialized to the genesis block.
func VerifyStateSanity(db database.DB) error {
	blocksDB := newBlocksStorage(db, db.Chain().Params())
	_, err := initChainState(db, &blocksDB, false)
	if err == nil {
		return nil
	}

	log.Error().Err(err).Msgf("fast load failed, trying to load with full rescan")
	blocksDB = newBlocksStorage(db, db.Chain().Params())
	_, err = tryToLoadAndRepairState(db, &blocksDB)
	return err
}

func tryToLoadAndRepairState(db database.DB, blocksDB *rBlockStorage) (*chaindata.BestState, error) {
	bestState, _ := initChainState(db, blocksDB, true)
	err := db.Update(func(dbTx database.Tx) error {
		serialIDs := make([]int64, 0, len(blocksDB.bestChain.nodes))

		for i, node := range blocksDB.bestChain.nodes {
			if node.Height()%1000 == 0 {
				log.Info().Str("chain", db.Chain().Name()).
					Int32("processed_blocks_count", node.Height()).
					Msgf("Refill state")
			}
			serialIDs = append(serialIDs, blocksDB.bestChain.nodes[i].SerialID())
			err := chaindata.DBPutMMRRoot(dbTx,
				blocksDB.bestChain.nodes[i].ActualMMRRoot(),
				blocksDB.bestChain.nodes[i].GetHash())
			if err != nil {
				return err
			}

		}
		err := chaindata.DBPutSerialIDsList(dbTx, serialIDs)
		if err != nil {
			return err
		}

		return chaindata.DBPutBestState(dbTx, bestState, blocksDB.bestChain.Tip().WorkSum())
	})
	return bestState, err
}
