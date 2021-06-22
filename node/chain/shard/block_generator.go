/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package shard

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"time"

	mmtree "gitlab.com/jaxnet/core/merged-mining-tree"
	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/mmr"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type BeaconBlockProvider interface {
	BlockTemplate(useCoinbaseValue bool, burnReward int) (mining.BlockTemplate, error)
	ShardCount() (uint32, error)
}

type BlockGenerator struct {
	beacon                 BeaconBlockProvider
	shardsMergedMiningTree mmr.IShardsMergedMiningTree
}

func NewChainBlockGenerator(beacon BeaconBlockProvider, db database.DB, genesis chainhash.Hash) *BlockGenerator {
	return &BlockGenerator{
		beacon:                 beacon,
		shardsMergedMiningTree: mmr.MergedMiningTree(mmr.Storage(db), genesis.CloneBytes()),
	}
}

func (c *BlockGenerator) NewBlockHeader(_ wire.BVersion, prevHash, merkleRootHash chainhash.Hash,
	timestamp time.Time, bits, nonce uint32, burnReward int) (wire.BlockHeader, error) {
	header, cAux, err := c.generateBeaconHeader(nonce, burnReward)
	if err != nil {
		return nil, err
	}

	return wire.NewShardBlockHeader(prevHash, merkleRootHash, timestamp, bits, *header, cAux), nil
}

func (c *BlockGenerator) ValidateBlockHeader(header wire.BlockHeader) error {
	lastKnownShardsAmount, err := c.beacon.ShardCount()
	if err != nil {
		// An error will occur if it is impossible
		// to get the last block from the chain state.
		return fmt.Errorf("can't fetch last beacon block: %w", err)
	}

	beaconHeader := header.BeaconHeader()
	shardHeader := header.(*wire.ShardHeader)

	treeValidationShouldBeSkipped := false
	mmNumber := shardHeader.MergeMiningNumber()
	if mmNumber%2 == 0 {
		if header.BeaconHeader().Shards() == mmNumber {
			if mmNumber <= lastKnownShardsAmount {
				treeValidationShouldBeSkipped = true
			}
		}
	}

	if !treeValidationShouldBeSkipped {
		hashes, coding, codingBitsLen := beaconHeader.MergedMiningTreeCodingProof()

		var (
			providedRoot   = shardHeader.MergeMiningRoot()
			validationRoot mmtree.BinHash
		)
		copy(validationRoot[:], providedRoot[:])

		tree := mmtree.NewSparseMerkleTree(lastKnownShardsAmount)
		err = tree.Validate(codingBitsLen, coding, hashes, mmNumber, validationRoot)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *BlockGenerator) ValidateCoinbaseTx(block *wire.MsgBlock, height int32) error {
	aux := block.Header.BeaconHeader().BTCAux()
	if len(aux.Tx.TxOut) != 3 {
		return errors.New("invalid format of btc aux coinbase tx: less than 3 out")
	}

	shardHeader := block.Header.(*wire.ShardHeader)
	if len(shardHeader.CoinbaseAux.Tx.TxOut) != 3 {
		return errors.New("invalid format of beacon aux coinbase tx: less than 3 out")
	}

	if len(block.Transactions[0].TxOut) != 3 {
		return errors.New("invalid format of shard coinbase tx: less than 3 out")
	}

	jaxNetLink, _ := txscript.NullDataScript([]byte(types.JaxNetLink))
	jaxBurn, _ := txscript.NullDataScript([]byte(types.JaxBurnAddr))

	btcCoinbaseTx := aux.CoinbaseAux.Tx

	btcJaxNetLinkOut := bytes.Equal(btcCoinbaseTx.TxOut[0].PkScript, jaxNetLink) &&
		btcCoinbaseTx.TxOut[0].Value == 0
	if !btcJaxNetLinkOut {
		return errors.New("invalid format of btc aux coinbase tx: first out must be zero and have JaxNetLink")
	}

	btcBurnReward := bytes.Equal(btcCoinbaseTx.TxOut[1].PkScript, jaxBurn)
	// properReward := beaconCoinbaseTx.TxOut[1].Value == calcBlockSubsidy(height)

	beaconCoinbaseTx := block.Header.(*wire.ShardHeader).CoinbaseAux.Tx

	beaconJaxNetLinkOut := bytes.Equal(beaconCoinbaseTx.TxOut[0].PkScript, jaxNetLink) &&
		beaconCoinbaseTx.TxOut[0].Value == 0
	if !beaconJaxNetLinkOut {
		return errors.New("invalid format of beacon coinbase tx: first out must be zero and have JaxNetLink")
	}

	beaconJaxBurnReward := bytes.Equal(beaconCoinbaseTx.TxOut[1].PkScript, jaxBurn)
	if !btcBurnReward && !beaconJaxBurnReward {
		return errors.New("invalid format of beacon coinbase tx: BTC not burned, Jax reward prohibited")
	}

	shardCoinbaseTx := block.Transactions[0]

	shardJaxNetLinkOut := bytes.Equal(shardCoinbaseTx.TxOut[0].PkScript, jaxNetLink) &&
		shardCoinbaseTx.TxOut[0].Value == 0
	if !shardJaxNetLinkOut {
		return errors.New("invalid format of shard coinbase tx: first out must be zero and have JaxNetLink")
	}

	shardJaxBurnReward := bytes.Equal(shardCoinbaseTx.TxOut[1].PkScript, jaxBurn)
	if !btcBurnReward && !beaconJaxBurnReward && !shardJaxBurnReward {
		return errors.New("invalid format of shard coinbase tx: BTC  or JaxNet not burned, Jax reward prohibited")
	}

	return nil
}
func (c *BlockGenerator) AcceptBlock(blockHeader wire.BlockHeader) error {
	h := blockHeader.BlockHash()
	_, err := c.shardsMergedMiningTree.Append(big.NewInt(0), h.CloneBytes())
	return err
}

func (c *BlockGenerator) CalcBlockSubsidy(height int32, header wire.BlockHeader) int64 {
	// Equivalent to: baseSubsidy / 2^(height/subsidyHalvingInterval)
	return 50 >> uint(height/210000)
}

func (c *BlockGenerator) generateBeaconHeader(nonce uint32, burnReward int) (*wire.BeaconHeader, wire.CoinbaseAux, error) {
	blockTemplate, err := c.beacon.BlockTemplate(false, burnReward)
	if err != nil {
		return nil, wire.CoinbaseAux{}, err
	}

	coinbaseAux := wire.CoinbaseAux{
		Tx:       *blockTemplate.Block.Transactions[0].Copy(),
		TxMerkle: make([]chainhash.Hash, len(blockTemplate.Block.Transactions)),
	}

	for i, tx := range blockTemplate.Block.Transactions {
		coinbaseAux.TxMerkle[i] = tx.TxHash()
	}

	beaconHeader := blockTemplate.Block.Header.BeaconHeader()
	beaconHeader.SetNonce(nonce)

	return beaconHeader, coinbaseAux, nil
}
