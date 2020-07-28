package blockntfns

import (
	"fmt"
	"gitlab.com/jaxnet/core/shard.core.git/wire/chain/shard"
)

// BlockNtfn is an interface that coalesces all the different types of block
// notifications.
type BlockNtfn interface {
	// header returns the header of the block for which this notification is
	// for.
	Header() shard.header

	// Height returns the height of the block for which this notification is
	// for.
	Height() uint32

	// ChainTip returns the header of the new tip of the chain after
	// processing the block being connected/disconnected.
	ChainTip() shard.header
}

// Connected is a block notification that gets dispatched to clients when the
// filter header of a new block has been found that extends the current chain.
type Connected struct {
	header shard.header
	height uint32
}

// A compile-time check to ensure Connected satisfies the BlockNtfn interface.
var _ BlockNtfn = (*Connected)(nil)

// NewBlockConnected creates a new Connected notification for the given block.
func NewBlockConnected(header shard.header, height uint32) *Connected {
	return &Connected{header: header, height: height}
}

// header returns the header of the block extending the chain.
func (n *Connected) Header() shard.header {
	return n.header
}

// Height returns the height of the block extending the chain.
func (n *Connected) Height() uint32 {
	return n.height
}

// ChainTip returns the header of the new tip of the chain after processing the
// block being connected.
func (n *Connected) ChainTip() shard.header {
	return n.header
}

// String returns the string representation of a Connected notification.
func (n *Connected) String() string {
	return fmt.Sprintf("block connected (height=%d, hash=%v)", n.height,
		n.header.BlockHash())
}

// Disconnected if a notification that gets dispatched to clients when a reorg
// has been detected at the tip of the chain.
type Disconnected struct {
	headerDisconnected shard.header
	heightDisconnected uint32
	chainTip           shard.header
}

// A compile-time check to ensure Disconnected satisfies the BlockNtfn
// interface.
var _ BlockNtfn = (*Disconnected)(nil)

// NewBlockDisconnected creates a Disconnected notification for the given block.
func NewBlockDisconnected(headerDisconnected shard.header,
	heightDisconnected uint32, chainTip shard.header) *Disconnected {

	return &Disconnected{
		headerDisconnected: headerDisconnected,
		heightDisconnected: heightDisconnected,
		chainTip:           chainTip,
	}
}

// header returns the header of the block being disconnected.
func (n *Disconnected) Header() shard.header {
	return n.headerDisconnected
}

// Height returns the height of the block being disconnected.
func (n *Disconnected) Height() uint32 {
	return n.heightDisconnected
}

// ChainTip returns the header of the new tip of the chain after processing the
// block being disconnected.
func (n *Disconnected) ChainTip() shard.header {
	return n.chainTip
}

// String returns the string representation of a Disconnected notification.
func (n *Disconnected) String() string {
	return fmt.Sprintf("block disconnected (height=%d, hash=%v)",
		n.heightDisconnected, n.headerDisconnected.BlockHash())
}
