// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// MaxBlockLocatorsPerMsg is the maximum number of block locator hashes allowed
// per message.
const MaxBlockLocatorsPerMsg = 500

// const MaxBlockLocatorsPerMsg = 5 // TODO: ROLLBACK

// MsgGetBlocks implements the Message interface and represents a bitcoin
// getblocks message.  It is used to request a list of blocks starting after the
// last known hash in the slice of block locator hashes.  The list is returned
// via an inv message (MsgInv) and is limited by a specific hash to stop at or
// the maximum number of blocks per message, which is currently 500.
//
// Set the HashStop field to the hash at which to stop and use
// AddBlockLocatorMeta to build up the list of block locator hashes.
//
// The algorithm for building the block locator hashes should be to add the
// hashes in reverse order until you reach the genesis block.  In order to keep
// the list of locator hashes to a reasonable number of entries, first add the
// most recent 10 block hashes, then double the step each loop iteration to
// exponentially decrease the number of hashes the further away from head and
// closer to the genesis block you get.
type MsgGetBlocks struct {
	ProtocolVersion   uint32
	BlockLocatorMetas []*BlockLocatorMeta
	HashStop          chainhash.Hash
}

// AddBlockLocatorMeta adds a new block locator hash to the message.
func (msg *MsgGetBlocks) AddBlockLocatorMeta(hash *BlockLocatorMeta) error {
	if len(msg.BlockLocatorMetas)+1 > MaxBlockLocatorsPerMsg {
		str := fmt.Sprintf("too many block locator metas for message [max %v]", MaxBlockLocatorsPerMsg)
		return Error("MsgGetBlocks.AddBlockLocatorMeta", str)
	}

	msg.BlockLocatorMetas = append(msg.BlockLocatorMetas, hash)
	return nil
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgGetBlocks) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	err := ReadElement(r, &msg.ProtocolVersion)
	if err != nil {
		return err
	}

	// Read num block locator hashes and limit to max.
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	if count > MaxBlockLocatorsPerMsg {
		str := fmt.Sprintf("too many block locator hashes for message "+
			"[count %v, max %v]", count, MaxBlockLocatorsPerMsg)
		return Error("MsgGetBlocks.BtcDecode", str)
	}

	// Create a contiguous slice of hashes to deserialize into in order to
	// reduce the number of allocations.
	msg.BlockLocatorMetas = make([]*BlockLocatorMeta, 0, count)
	for i := uint64(0); i < count; i++ {
		val := &BlockLocatorMeta{}
		err := val.Deserialize(r)
		if err != nil {
			return err
		}
		_ = msg.AddBlockLocatorMeta(val)
	}

	return ReadElement(r, &msg.HashStop)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetBlocks) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	count := len(msg.BlockLocatorMetas)
	if count > MaxBlockLocatorsPerMsg {
		str := fmt.Sprintf("too many block locator metas for message "+
			"[count %v, max %v]", count, MaxBlockLocatorsPerMsg)
		return Error("MsgGetBlocks.BtcEncode", str)
	}

	err := WriteElement(w, msg.ProtocolVersion)
	if err != nil {
		return err
	}

	err = WriteVarInt(w, uint64(count))
	if err != nil {
		return err
	}

	for _, meta := range msg.BlockLocatorMetas {
		if err = meta.Serialize(w); err != nil {
			return err
		}
	}

	return WriteElement(w, &msg.HashStop)
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgGetBlocks) Command() string {
	return CmdGetBlocks
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgGetBlocks) MaxPayloadLength(pver uint32) uint32 {
	// Protocol version 4 bytes + num hashes (varInt) + max block locator
	// hashes + hash stop.
	return 4 + MaxVarIntPayload + (MaxBlockLocatorsPerMsg * chainhash.HashSize) + chainhash.HashSize
	// return 4 + MaxVarIntPayload + (MaxBlockLocatorsPerMsg * (chainhash.HashSize*2 + 8 + 4)) + chainhash.HashSize
}

// NewMsgGetBlocks returns a new bitcoin getblocks message that conforms to the
// Message interface using the passed parameters and defaults for the remaining
// fields.
func NewMsgGetBlocks(hashStop *chainhash.Hash) *MsgGetBlocks {
	return &MsgGetBlocks{
		ProtocolVersion:   ProtocolVersion,
		BlockLocatorMetas: make([]*BlockLocatorMeta, 0, MaxBlockLocatorsPerMsg),
		HashStop:          *hashStop,
	}
}

type BlockLocatorMeta struct {
	Hash chainhash.Hash
	// todo: think if this really required
	// PrevMMRRoot chainhash.Hash
	// Weight      uint64
	// Height      int32
}

func (msg *BlockLocatorMeta) String() string {
	return fmt.Sprintf("(hash=%s)", msg.Hash)
	// return fmt.Sprintf("(hash=%s, prev_mmr_root=%s, chainWeight=%d, height=%d)", msg.Hash, msg.PrevMMRRoot, msg.Weight, msg.Height)
}

func (msg *BlockLocatorMeta) SerializeSize() int {
	return chainhash.HashSize // *2 + 8 + 4
}

func (msg *BlockLocatorMeta) Serialize(w io.Writer) error {
	return WriteElements(w, &msg.Hash) // &msg.PrevMMRRoot, &msg.Weight, &msg.Height,
}

func (msg *BlockLocatorMeta) Deserialize(r io.Reader) error {
	return ReadElements(r, &msg.Hash) // &msg.PrevMMRRoot, &msg.Weight, &msg.Height,
}
