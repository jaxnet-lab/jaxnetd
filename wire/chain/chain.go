package chain

import (
	"io"
)

var DefaultChain IChain

type IChain interface {
	//DecodeHeader(r io.Reader) (BlockHeader, error)
	//EncodeHeader(w io.Writer, h BlockHeader) error
	//Serialize(h BlockHeader, w io.Writer) error
	//Deserialize(r io.Reader) (BlockHeader, error)
	Read(r io.Reader) (BlockHeader, error)
	Write(w io.Writer, h BlockHeader) error
	NewHeader() BlockHeader
	//BlockOne()
	//GenesisHash() chainhash.Hash
}

func SetChain(defaultChain IChain) {
	DefaultChain = defaultChain
}

func NewHeader() BlockHeader {
	return DefaultChain.NewHeader()
}

// readBlockHeader reads a bitcoin block Header from r.  See Deserialize for
// decoding block headers stored to disk, such as in a database, as opposed to
// decoding from the wire.
func ReadBlockHeader(r io.Reader) (res BlockHeader, err error) {
	return DefaultChain.Read(r)
}

// writeBlockHeader writes a bitcoin block Header to w.  See Serialize for
// encoding block headers to be stored to disk, such as in a database, as
// opposed to encoding for the wire.
func WriteBlockHeader(w io.Writer, bh BlockHeader) error {
	return DefaultChain.Write(w, bh)
}
