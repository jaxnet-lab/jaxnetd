// Copyright (c) 2013-2015 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"

	"gitlab.com/jaxnet/jaxnetd/node/encoder"
	"gitlab.com/jaxnet/jaxnetd/types"
)

// MsgGetData implements the Message interface and represents a bitcoin
// getdata message.  It is used to request data such as blocks and transactions
// from another server.  It should be used in response to the inv (MsgInv) message
// to request the actual data referenced by each inventory vector the receiving
// server doesn't already have.  Each message is limited to a maximum number of
// inventory vectors, which is currently 50,000.  As a result, multiple messages
// must be used to request larger amounts of data.
//
// Use the AddInvVect function to build up the list of inventory vectors when
// sending a getdata message to another server.
type MsgGetData struct {
	InvList []*types.InvVect
}

// AddInvVect adds an inventory vector to the message.
func (msg *MsgGetData) AddInvVect(iv *types.InvVect) error {
	if len(msg.InvList)+1 > types.MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [max %v]",
			types.MaxInvPerMsg)
		return messageError("MsgGetData.AddInvVect", str)
	}

	msg.InvList = append(msg.InvList, iv)
	return nil
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgGetData) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
	count, err := encoder.ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// Limit to max inventory vectors per message.
	if count > types.MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [%v]", count)
		return messageError("MsgGetData.BtcDecode", str)
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
func (msg *MsgGetData) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
	// Limit to max inventory vectors per message.
	count := len(msg.InvList)
	if count > types.MaxInvPerMsg {
		str := fmt.Sprintf("too many invvect in message [%v]", count)
		return messageError("MsgGetData.BtcEncode", str)
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
func (msg *MsgGetData) Command() string {
	return CmdGetData
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgGetData) MaxPayloadLength(pver uint32) uint32 {
	// Num inventory vectors (varInt) + max allowed inventory vectors.
	return encoder.MaxVarIntPayload + (types.MaxInvPerMsg * types.MaxInvVectPayload)
}

// NewMsgGetData returns a new bitcoin getdata message that conforms to the
// Message interface.  See MsgGetData for details.
func NewMsgGetData() *MsgGetData {
	return &MsgGetData{
		InvList: make([]*types.InvVect, 0, defaultInvListAlloc),
	}
}

// NewMsgGetDataSizeHint returns a new bitcoin getdata message that conforms to
// the Message interface.  See MsgGetData for details.  This function differs
// from NewMsgGetData in that it allows a default allocation size for the
// backing array which houses the inventory vector list.  This allows callers
// who know in advance how large the inventory list will grow to avoid the
// overhead of growing the internal backing array several times when appending
// large amounts of inventory vectors with AddInvVect.  Note that the specified
// hint is just that - a hint that is used for the default allocation size.
// Adding more (or less) inventory vectors will still work properly.  The size
// hint is limited to MaxInvPerMsg.
func NewMsgGetDataSizeHint(sizeHint uint) *MsgGetData {
	// Limit the specified hint to the maximum allow per message.
	if sizeHint > types.MaxInvPerMsg {
		sizeHint = types.MaxInvPerMsg
	}

	return &MsgGetData{
		InvList: make([]*types.InvVect, 0, sizeHint),
	}
}
