package shard

import "gitlab.com/jaxnet/core/shard.core.git/wire/chain"

//
type shardChain struct {
}

//
func Chain() chain.IChain {
	return &shardChain{}
}

func (c *shardChain) NewHeader() chain.BlockHeader {
	return &Header{}
}

//
//func (c *shardChain) BlockOne() {
//
//}
//
//func (c *shardChain) GenesisHash() chainhash.Hash {
//	return [32]byte{}
//}

//func (c *shardChain) DecodeHeader(r io.Reader) (chain.BlockHeader, error) {
//	h := &Header{}
//	err := ReadBlockHeader(r, h)
//	return h, err
//}
//
//func (c *shardChain) EncodeHeader(w io.Writer, h chain.BlockHeader) error {
//	//h := &Header{}
//	header := h.(*Header)
//	return WriteBlockHeader(w,0,  header)
//}
