package chaindata

import (
	"math/big"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type Repo interface {
	PutVersion(key []byte, version uint32) error
	FetchOrCreateVersion(key []byte, defaultVersion uint32) (uint32, error)

	FetchAllBlocksHashBySerialID(serialID int64, onlyOrphan bool) ([]SerialIDBlockMeta, error)
	FetchBlockHashBySerialID(serialID int64) (*chainhash.Hash, int64, error)
	FetchBlockSerialID(hash *chainhash.Hash) (int64, int64, error)
	PutBlockHashToSerialID(hash chainhash.Hash, serialID int64) error
	PutHashToSerialIDWithPrev(hash chainhash.Hash, serialID, prevSerialID int64) error
	PutSerialIDsList(serialIDs []int64) error
	GetBestChainSerialIDs() ([]BestChainBlockRecord, error)

	StoreBlock(block *jaxutil.Block) error
	HasBlock(hash *chainhash.Hash) (bool, error)
	HasBlocks(hashes []chainhash.Hash) ([]bool, error)
	FetchBlockHeader(hash *chainhash.Hash) ([]byte, error)
	FetchBlockHeaders(hashes []chainhash.Hash) ([][]byte, error)
	FetchBlock(hash *chainhash.Hash) ([]byte, error)
	FetchBlocks(hashes []chainhash.Hash) ([][]byte, error)
	FetchBlockRegion(region *database.BlockRegion) ([]byte, error)
	FetchBlockRegions(regions []database.BlockRegion) ([][]byte, error)

	FetchSpendJournalEntry(block *jaxutil.Block) ([]SpentTxOut, error)
	PutSpendJournalEntry(blockHash *chainhash.Hash, stxos []SpentTxOut) error
	RemoveSpendJournalEntry(blockHash *chainhash.Hash) error
	FetchUtxoEntryByHash(hash *chainhash.Hash) (*UtxoEntry, error)
	FetchUtxoEntry(outpoint wire.OutPoint) (*UtxoEntry, error)
	FetchUtxoEntries(limit int) (map[wire.OutPoint]*UtxoEntry, error)
	PutUtxoView(view *UtxoViewpoint) error

	PutBlockIndex(hash *chainhash.Hash, height int32) error
	RemoveBlockIndex(hash *chainhash.Hash, height int32) error
	FetchHeightByHash(hash *chainhash.Hash) (int32, error)
	PutBestState(snapshot *BestState, workSum *big.Int) error
	FetchBlockByNode(node blocknodes.IBlockNode) (*jaxutil.Block, error)
	FetchBlockByHash(hash chainhash.Hash) (*jaxutil.Block, error)
	StoreBlockNode(node blocknodes.IBlockNode) error
	DBStoreBlock(block *jaxutil.Block) error

	PutMMRRoot(mmrRoot, blockHash chainhash.Hash) error
	GetBlocksMMRRoots() (map[chainhash.Hash]chainhash.Hash, error)
	GetMMRRootForBlock(blockHash *chainhash.Hash) (chainhash.Hash, error)

	PutEADAddresses(updateSet map[string]*wire.EADAddresses) error
	FetchAllEADAddresses() (map[string]*wire.EADAddresses, error)
	FetchEADAddresses(ownerPK string) (*wire.EADAddresses, error)

	StoreShardGenesisInfo(shardID uint32, blockHeight int32, blockHash *chainhash.Hash, blockSerialID int64) error
	StoreLastShardInfo(shardID uint32) error
	GetShardGenesisInfo() (map[uint32]ShardInfo, uint32)
}
