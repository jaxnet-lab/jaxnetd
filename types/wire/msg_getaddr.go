// Copyright (c) 2013-2015 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"fmt"
	"io"

	"gitlab.com/jaxnet/jaxnetd/node/encoder"
)

// MsgGetAddr implements the Message interface and represents a bitcoin
// getaddr message. It is used to request a list of known active peers on the
// network from a server to help identify potential nodes.  The list is returned
// via one or more addr messages (MsgAddr).
//
// This message has no payload.
type MsgGetAddr struct {
	ShardID uint32
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgGetAddr) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
	buf, ok := r.(*bytes.Buffer)
	if !ok {
		return fmt.Errorf("MsgGetAddr.BtcDecode reader is not a " +
			"*bytes.Buffer")
	}

	err := encoder.ReadElement(buf, &msg.ShardID)
	if err != nil {
		return err
	}
	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetAddr) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
	return encoder.WriteElement(w, msg.ShardID)
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgGetAddr) Command() string {
	return CmdGetAddr
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgGetAddr) MaxPayloadLength(pver uint32) uint32 {
	return 4
}

// NewMsgGetAddr returns a new bitcoin getaddr message that conforms to the
// Message interface.  See MsgGetAddr for details.
func NewMsgGetAddr() *MsgGetAddr {
	return &MsgGetAddr{}
}
