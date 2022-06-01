// Copyright (c) 2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"io"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// MsgGetCFHeaders is a message similar to MsgGetHeaders, but for committed
// filter headers. It allows to set the FilterType field to get headers in the
// Chain of basic (0x00) or extended (0x01) headers.
type MsgGetCFHeaders struct {
	FilterType  FilterType
	StartHeight uint32
	StopHash    chainhash.Hash
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgGetCFHeaders) BtcDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	err := ReadElement(r, &msg.FilterType)
	if err != nil {
		return err
	}

	err = ReadElement(r, &msg.StartHeight)
	if err != nil {
		return err
	}

	return ReadElement(r, &msg.StopHash)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgGetCFHeaders) BtcEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	err := WriteElement(w, msg.FilterType)
	if err != nil {
		return err
	}

	err = WriteElement(w, &msg.StartHeight)
	if err != nil {
		return err
	}

	return WriteElement(w, &msg.StopHash)
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgGetCFHeaders) Command() string {
	return CmdGetCFHeaders
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgGetCFHeaders) MaxPayloadLength(pver uint32) uint32 {
	// Filter type + uint32 + block hash
	return 1 + 4 + chainhash.HashSize
}

// NewMsgGetCFHeaders returns a new bitcoin getcfheader message that conforms to
// the Message interface using the passed parameters and defaults for the
// remaining fields.
func NewMsgGetCFHeaders(filterType FilterType, startHeight uint32,
	stopHash *chainhash.Hash,
) *MsgGetCFHeaders {
	return &MsgGetCFHeaders{
		FilterType:  filterType,
		StartHeight: startHeight,
		StopHash:    *stopHash,
	}
}
