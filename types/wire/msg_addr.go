// Copyright (c) 2013-2015 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"fmt"
	"io"
	"net"
)

// MaxAddrPerMsg is the maximum number of addresses that can be in a single
// bitcoin addr message (MsgAddr).
const MaxAddrPerMsg = 1000

// MsgAddr implements the Message interface and represents a bitcoin
// addr message.  It is used to provide a list of known active peers on the
// network.  An active server is considered one that has transmitted a message
// within the last 3 hours.  Nodes which have not transmitted in that time
// frame should be forgotten.  Each message is limited to a maximum number of
// addresses, which is currently 1000.  As a result, multiple messages must
// be used to relay the full list.
//
// Use the AddAddress function to build up the list of known addresses when
// sending an addr message to another server.
type MsgAddr struct {
	AddrList []*NetAddress
	Shards   []*ShardAddress
}

// AddAddress adds a known active server to the message.
func (msg *MsgAddr) AddAddress(na *NetAddress) error {
	if len(msg.AddrList)+1 > MaxAddrPerMsg {
		str := fmt.Sprintf("too many addresses in message [max %v]",
			MaxAddrPerMsg)
		return Error("MsgAddr.AddAddress", str)
	}

	msg.AddrList = append(msg.AddrList, na)
	return nil
}

// AddShardAddress adds a known active server to the message.
func (msg *MsgAddr) AddShardAddress(shardID uint32, na *NetAddress) error {
	if len(msg.AddrList)+1 > MaxAddrPerMsg {
		str := fmt.Sprintf("too many addresses in message [max %v]",
			MaxAddrPerMsg)
		return Error("MsgAddr.AddAddress", str)
	}

	msg.Shards = append(msg.Shards, &ShardAddress{
		ShardID: shardID,
		Address: na,
	})
	return nil
}

// AddAddresses adds multiple known active peers to the message.
func (msg *MsgAddr) AddAddresses(netAddrs ...*NetAddress) error {
	for _, na := range netAddrs {
		err := msg.AddAddress(na)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddAddresses adds multiple known active peers to the message.
func (msg *MsgAddr) AddShards(netAddrs ...*ShardAddress) error {
	msg.Shards = append(msg.Shards, netAddrs...)
	return nil
}

// ClearAddresses removes all addresses from the message.
func (msg *MsgAddr) ClearAddresses() {
	msg.AddrList = []*NetAddress{}
	msg.Shards = []*ShardAddress{}
}

// BtcDecode decodes r using the bitcoin protocol encoding into the receiver.
// This is part of the Message interface implementation.
func (msg *MsgAddr) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	// Limit to max addresses per message.
	if count > MaxAddrPerMsg {
		str := fmt.Sprintf("too many addresses for message "+
			"[count %v, max %v]", count, MaxAddrPerMsg)
		return Error("MsgAddr.BtcDecode", str)
	}

	addrList := make([]NetAddress, count)
	msg.AddrList = make([]*NetAddress, 0, count)
	for i := uint64(0); i < count; i++ {
		na := &addrList[i]
		err := readNetAddress(r, na, true)
		if err != nil {
			return err
		}
		if err = msg.AddAddress(na); err != nil {
			return err
		}
	}

	count, err = ReadVarInt(r, pver)
	if err != nil {
		return err
	}

	shards := make([]*ShardAddress, count)
	for i := uint64(0); i < count; i++ {
		na := shards[i]
		id, err := ReadVarInt(r, pver)
		if err != nil {
			return err
		}
		na.ShardID = uint32(id)
		err = readNetAddress(r, na.Address, true)
		if err != nil {
			return err
		}
	}
	return msg.AddShards(shards...)
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *MsgAddr) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	// Protocol versions before MultipleAddressVersion only allowed 1 address
	// per message.
	count := len(msg.AddrList)
	// if pver < MultipleAddressVersion && count > 1 {
	// 	str := fmt.Sprintf("too many addresses for message of "+
	// 		"protocol version %v [count %v, max 1]", pver, count)
	// 	return Error("MsgAddr.BtcEncode", str)
	//
	// }
	if count > MaxAddrPerMsg {
		str := fmt.Sprintf("too many addresses for message "+
			"[count %v, max %v]", count, MaxAddrPerMsg)
		return Error("MsgAddr.BtcEncode", str)
	}

	err := WriteVarInt(w, uint64(count))
	if err != nil {
		return err
	}

	for _, na := range msg.AddrList {
		err = writeNetAddress(w, na, true)
		if err != nil {
			return err
		}
	}

	shardsCount := len(msg.Shards)
	err = WriteVarInt(w, uint64(shardsCount))
	if err != nil {
		return err
	}

	for _, na := range msg.Shards {
		err := WriteVarInt(w, uint64(na.ShardID))
		if err != nil {
			return err
		}

		err = writeNetAddress(w, na.Address, true)
		if err != nil {
			return err
		}
	}

	return nil
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgAddr) Command() string {
	return CmdAddr
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgAddr) MaxPayloadLength(pver uint32) uint32 {
	// if pver < MultipleAddressVersion {
	// 	// Num addresses (varInt) + a single net addresses.
	// 	return encoder.MaxVarIntPayload + maxNetAddressPayload(pver)
	// }

	// Num addresses (varInt) + max allowed addresses.
	return MaxVarIntPayload + (MaxAddrPerMsg * maxNetAddressPayload())
}

// NewMsgAddr returns a new bitcoin addr message that conforms to the
// Message interface.  See MsgAddr for details.
func NewMsgAddr() *MsgAddr {
	return &MsgAddr{
		AddrList: make([]*NetAddress, 0, MaxAddrPerMsg),
	}
}

type MsgPortRedirect struct {
	ShardID uint32

	// Port the server is using.
	Port uint32

	// IP address of the server.
	IP net.IP
}

func (m *MsgPortRedirect) BtcDecode(r io.Reader, _ uint32, _ MessageEncoding) error {
	var ip [16]byte

	err := ReadElements(r, &m.ShardID, &m.Port, &ip)
	if err != nil {
		return err
	}

	m.IP = ip[:]
	return nil
}

func (m *MsgPortRedirect) BtcEncode(w io.Writer, _ uint32, _ MessageEncoding) error {
	// Ensure to always write 16 bytes even if the ip is nil.
	var ip [16]byte
	if m.IP != nil {
		copy(ip[:], m.IP.To16())
	}

	return WriteElements(w, m.ShardID, m.Port, ip)
}

func (m *MsgPortRedirect) Command() string {
	return CmdPortRedirect
}

func (m *MsgPortRedirect) MaxPayloadLength(uint32) uint32 {
	// shardID 4 bytes + port 4 bytes + ip 16 bytes.
	return 4 + 4 + 16
}

func NewMsgPortRedirect(shardID, port uint32, ip net.IP) *MsgPortRedirect {
	return &MsgPortRedirect{
		Port:    port,
		ShardID: shardID,
		IP:      ip,
	}
}
