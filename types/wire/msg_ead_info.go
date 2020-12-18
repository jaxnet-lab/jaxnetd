package wire

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	"gitlab.com/jaxnet/core/shard.core/node/encoder"
)

type EADAddresses struct {
	ID          uint64
	OwnerPubKey []byte
	IPs         []EADAddress
}

func (msg *EADAddresses) Command() string {
	return CmdEadAddresses
}

func (msg *EADAddresses) MaxPayloadLength(uint32) uint32 {
	return MaxBlockPayload
}

func (msg *EADAddresses) BtcDecode(r io.Reader, pver uint32, enc encoder.MessageEncoding) error {
	alias, err := encoder.ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	msg.ID = alias

	msg.OwnerPubKey, err = encoder.ReadVarBytes(r, pver, 65*2, "OwnerPubKey")
	if err != nil {
		return err
	}
	count, err := encoder.ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	msg.IPs = make([]EADAddress, count)

	for i := range msg.IPs {
		err = msg.IPs[i].BtcDecode(r, pver, enc)
		if err != nil {
			return err
		}
	}

	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *EADAddresses) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
	err := encoder.WriteVarInt(w, msg.ID)
	if err != nil {
		return err
	}

	err = encoder.WriteVarBytes(w, pver, msg.OwnerPubKey)
	if err != nil {
		return err
	}

	// Protocol versions before MultipleAddressVersion only allowed 1 address
	// per message.
	count := len(msg.IPs)
	err = encoder.WriteVarInt(w, uint64(count))
	if err != nil {
		return err
	}

	for i := range msg.IPs {
		err = msg.IPs[i].BtcEncode(w, pver, enc)
		if err != nil {
			return err
		}
	}

	return nil
}

// EADAddress defines information about a Exchange Agent server on the network
// including the expiration time its IP address, and port.
type EADAddress struct {
	// IP address of the server.
	IP net.IP

	// Port the server is using.  This is encoded in big endian on the wire
	// which differs from most everything else.
	Port uint16

	// ExpiresAt Address expiration time.
	ExpiresAt time.Time
}

func (msg *EADAddress) Command() string {
	return CmdEadIp
}

func (msg *EADAddress) MaxPayloadLength(uint32) uint32 {
	return 16 + 4 + 8
}

func (msg *EADAddress) BtcDecode(r io.Reader, _ uint32, _ encoder.MessageEncoding) error {
	var ip [16]byte

	err := encoder.ReadElements(r, (*encoder.Uint32Time)(&msg.ExpiresAt), &ip)
	if err != nil {
		return err
	}

	// Sigh. Bitcoin protocol mixes little and big endian.
	port, err := encoder.BinarySerializer.Uint16(r, bigEndian)
	if err != nil {
		return err
	}

	*msg = EADAddress{
		ExpiresAt: msg.ExpiresAt,
		IP:        ip[:],
		Port:      port,
	}
	return nil
}

func (msg *EADAddress) BtcEncode(w io.Writer, pver uint32, enc encoder.MessageEncoding) error {
	// Ensure to always write 16 bytes even if the ip is nil.
	var ip [16]byte
	if msg.IP != nil {
		copy(ip[:], msg.IP.To16())
	}
	err := encoder.WriteElements(w, uint32(msg.ExpiresAt.Unix()), ip)
	if err != nil {
		return err
	}

	// Sigh.  Bitcoin protocol mixes little and big endian.
	return binary.Write(w, bigEndian, msg.Port)

}
