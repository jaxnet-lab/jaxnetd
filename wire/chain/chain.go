package chain

var DefaultChain IChain

type IChain interface {
	//DecodeHeader(r io.Reader) (BlockHeader, error)
	//EncodeHeader(w io.Writer, h BlockHeader) error
	//Serialize(h BlockHeader, w io.Writer) error
	//Deserialize(r io.Reader) (BlockHeader, error)
	NewHeader() BlockHeader
	//BlockOne()
	//GenesisHash() chainhash.Hash
}

func SetChain(defaultChain IChain) {
	DefaultChain = defaultChain
}
