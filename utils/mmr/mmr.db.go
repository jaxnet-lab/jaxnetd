package mmr

//type nodeData struct {
//	Weight *big.Int
//	Hash   Hash
//}

type IStore interface {
	GetNode(index uint64) (res *BlockData, ok bool)
	SetNode(index uint64, data *BlockData)
	GetBlock(index uint64) (res *BlockData, ok bool)
	SetBlock(index uint64, data *BlockData)
	Nodes() (res []uint64)
	Blocks() (res []uint64)
	Debug()
}
