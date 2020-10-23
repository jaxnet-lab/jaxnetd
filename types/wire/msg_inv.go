// Copyright (c) 2013-2015 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"gitlab.com/jaxnet/core/shard.core/node/encoder"
	"gitlab.com/jaxnet/core/shard.core/types"
)

// defaultInvListAlloc is the default size used for the backing array for an
// inventory list.  The array will dynamically grow as needed, but this
// figure is intended to provide enough space for the max number of inventory
// vectors in a *typical* inventory message without needing to grow the backing
// array multiple times.  Technically, the list can grow to MaxInvPerMsg, but
// rather than using that large figure, this figure more accurately reflects the
// typical case.
const defaultInvListAlloc = 1000

// MsgInv implements the Message interface and represents a bitcoin inv message.
// It is used to advertise a server's known data such as blocks and transactions
// through inventory vectors.  It may be sent unsolicited to inform other peers
// of the data or in response to a getblocks message (MsgGetBlocks).  Each
// message is limited to a maximum number of inventory vectors, which is
// currently 50,000.
//
// Use the AddInvVect function to build up the list of inventory vectors when
// sending an inv message to another server.
type MsgInv struct {
	InvList []*types.InvVect
}

// AddInvVect adds an inventory vector to the message.
func (msg *MsgInv) AddInvVect(iv *types.InvVect) error {
	if len(msg.InvList)+1 > types.MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [max %v]",
			types.MaxInvPerMsg)
		return messageError("MsgInv.AddInvVect", str)
	}

	msg.InvList = append(msg.InvList, iv)
	return nil
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgInv) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
	count, err := encoder.ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// Limit to max inventory vectors per message.
	if count > types.MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [%v]", count)
		return messageError("MsgInv.BtcDecode", str)
	}

	// Create a contiguous slice of inventory vectors to deserialize into in
	// order to reduce the number of allocations.
	invList := make([]types.InvVect, count)
	msg.InvList = make([]*types.InvVect, 0, count)
	for i := uint64(0); i < count; i++ {
		iv := &invList[i]
		err := encoder.ReadInvVect(r, iv)
		if err != nil {
			return err
		}
		msg.AddInvVect(iv)
	}

	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgInv) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
	// Limit to max inventory vectors per message.
	count := len(msg.InvList)
	if count > types.MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [%v]", count)
		return messageError("MsgInv.BtcEncode", str)
	}

	err := encoder.WriteVarInt(w, uint64(count))
	if err != nil {
		return err
	}

	for _, iv := range msg.InvList {
		err := encoder.WriteInvVect(w, iv)
		if err != nil {
			return err
		}
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgInv) Command() string {
	return CmdInv
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgInv) MaxPayloadLength(pver uint32) uint32 {
	// Num inventory vectors (varInt) + max allowed inventory vectors.
	return encoder.MaxVarIntPayload + (types.MaxInvPerMsg * types.MaxInvVectPayload)
}

// NewMsgInv returns a new bitcoin inv message that conforms to the Message
// interface.  See MsgInv for details.
func NewMsgInv() *MsgInv {
	return &MsgInv{
		InvList: make([]*types.InvVect, 0, defaultInvListAlloc),
	}
}

// NewMsgInvSizeHint returns a new bitcoin inv message that conforms to the
// Message interface.  See MsgInv for details.  This function differs from
// NewMsgInv in that it allows a default allocation size for the backing array
// which houses the inventory vector list.  This allows callers who know in
// advance how large the inventory list will grow to avoid the overhead of
// growing the internal backing array several times when appending large amounts
// of inventory vectors with AddInvVect.  Note that the specified hint is just
// that - a hint that is used for the default allocation size.  Adding more
// (or less) inventory vectors will still work properly.  The size hint is
// limited to MaxInvPerMsg.
func NewMsgInvSizeHint(sizeHint uint) *MsgInv {
	// Limit the specified hint to the maximum allow per message.
	if sizeHint > types.MaxInvPerMsg {
		sizeHint = types.MaxInvPerMsg
	}

	return &MsgInv{
		InvList: make([]*types.InvVect, 0, sizeHint),
	}
}
