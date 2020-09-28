package chain

import "gitlab.com/jaxnet/core/shard.core.git/shards/chain/chaincore"

//var DefaultChain IChain

type IChain interface {
	//Read(r io.Reader) (BlockHeader, error)
	//Write(w io.Writer, h BlockHeader) error
	IsBeacon() bool
	ShardID() int32
	NewHeader() BlockHeader
	//NewBlockHeader(version int32, prevHash, merkleRootHash chainhash.Hash,
	//	mmr chainhash.Hash, timestamp time.Time, bits uint32, nonce uint32) BlockHeader
	NewNode(blockHeader BlockHeader, parent IBlockNode) IBlockNode
	MaxBlockHeaderPayload() int
	BlockHeaderOverhead() int
	Params() *chaincore.Params
	GenesisBlock() interface{}
	//BlockOne()
	//GenesisHash() chainhash.Hash
}
