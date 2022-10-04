/*
 * Copyright (c) 2022 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package blockchain

import (
	"time"

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
		tip := blocksDB.bestChain.Tip()
		log.Info().Str("hash", tip.GetHash().String()).
			Str("mmt_root", tip.ActualMMRRoot().String()).
			Int32("height", tip.Height()).
			Time("block_time", time.Unix(tip.Timestamp(), 0)).
			Msg("Best Chain Tip")
		return nil
	}

	log.Error().Err(err).Msgf("fast load failed, trying to load with full rescan")
	blocksDB = newBlocksStorage(db, db.Chain().Params())
	_, err = tryToLoadAndRepairState(db, &blocksDB)

	tip := blocksDB.bestChain.Tip()
	log.Info().Str("hash", tip.GetHash().String()).
		Str("mmt_root", tip.ActualMMRRoot().String()).
		Int32("height", tip.Height()).
		Time("block_time", time.Unix(tip.Timestamp(), 0)).
		Msg("Best Chain Tip")
	return err
}

func tryToLoadAndRepairState(db database.DB, blocksDB *rBlockStorage) (*chaindata.BestState, error) {
	bestState, err := initChainState(db, blocksDB, true)
	if err != nil {
		return nil, err
	}

	err = db.Update(func(dbTx database.Tx) error {
		serialIDs := make([]int64, 0, len(blocksDB.bestChain.nodes))

		for i, node := range blocksDB.bestChain.nodes {
			if node.Height()%1000 == 0 {
				log.Info().Str("chain", db.Chain().Name()).
					Int32("processed_blocks_count", node.Height()).
					Msgf("Refill state")
			}
			serialIDs = append(serialIDs, blocksDB.bestChain.nodes[i].SerialID())
			err := chaindata.RepoTx(dbTx).PutMMRRoot(
				blocksDB.bestChain.nodes[i].ActualMMRRoot(),
				blocksDB.bestChain.nodes[i].GetHash())
			if err != nil {
				return err
			}

		}
		err := chaindata.RepoTx(dbTx).PutSerialIDsList(serialIDs)
		if err != nil {
			return err
		}

		tip := blocksDB.bestChain.Tip()
		bestBlock, err := chaindata.RepoTx(dbTx).FetchBlockByHash(tip.GetHash())
		if err != nil {
			return err
		}

		numTxns := uint64(len(bestBlock.Transactions()))
		blockSize := uint64(bestBlock.MsgBlock().SerializeSize())
		blockWeight := uint64(chaindata.GetBlockWeight(bestBlock))

		curTotalTxns := bestState.TotalTxns // TODO: fix me

		state := chaindata.NewBestState(tip,
			blocksDB.bestChain.mmrTree.CurrentRoot(),
			blockSize,
			blockWeight,
			blocksDB.bestChain.mmrTree.CurrenWeight(),
			numTxns,
			curTotalTxns,
			tip.CalcPastMedianTime(),
			bestState.LastSerialID,
		)
		return chaindata.RepoTx(dbTx).PutBestState(state, tip.WorkSum())
	})
	return bestState, err
}
