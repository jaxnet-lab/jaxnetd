package wire

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	"gitlab.com/jaxnet/jaxnetd/node/encoder"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

type EADAddresses struct {
	ID          uint64
	OwnerPubKey []byte
	IPs         []EADAddress
}

func (msg *EADAddresses) AddAddress(ip net.IP, port uint16, expiresAt int64, shardID uint32,
	hash *chainhash.Hash, ind int) *EADAddresses {

	var ipExist bool
	for i, p := range msg.IPs {
		if p.IP.Equal(ip) && p.Shard == shardID {
			p := p
			p.TxHash = hash
			p.TxOutIndex = ind
			p.Port = port
			p.ExpiresAt = time.Unix(expiresAt, 0)
			msg.IPs[i] = p

			ipExist = true
			break
		}
	}

	if !ipExist {
		msg.IPs = append(msg.IPs, EADAddress{
			IP:         ip,
			Port:       port,
			ExpiresAt:  time.Unix(expiresAt, 0),
			Shard:      shardID,
			TxHash:     hash,
			TxOutIndex: ind,
		})
	}

	return msg
}

func (msg *EADAddress) HasShard(shards ...uint32) (allPresent bool, hasOneOf bool) {
	var matchCount int
	for _, shard := range shards {
		if shard == msg.Shard {
			matchCount += 1
		}
	}

	return matchCount == len(shards), matchCount > 0
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
	// Shard shows what shards the agent works with.
	Shard      uint32
	TxHash     *chainhash.Hash
	TxOutIndex int
}

// FilterOut returns true if the address has no shards left in which it works.
func (msg *EADAddress) FilterOut(ip net.IP, shardID uint32) (*EADAddress, bool) {
	if !msg.IP.Equal(ip) {
		return msg, false
	}

	clone := *msg
	if msg.Shard != shardID {
		clone.Shard = msg.Shard
	}

	return &clone, msg.Shard == shardID
}

func (msg *EADAddress) Command() string {
	return CmdEadIp
}

func (msg *EADAddress) MaxPayloadLength(uint32) uint32 {
	return 16 + 4 + 8
}

func (msg *EADAddress) BtcDecode(r io.Reader, pver uint32, _ encoder.MessageEncoding) error {
	var ip [16]byte

	err := encoder.ReadElements(r, (*encoder.Uint32Time)(&msg.ExpiresAt), &ip)
	if err != nil {
		return err
	}

	id, err := encoder.ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	msg.Shard = uint32(id)

	// Sigh. Bitcoin protocol mixes little and big endian.
	port, err := encoder.BinarySerializer.Uint16(r, bigEndian)
	if err != nil {
		return err
	}
	rawHash, err := encoder.ReadVarBytes(r, pver, chainhash.HashSize, "TxHash")
	if err != nil {
		return err
	}

	msg.TxHash, err = chainhash.NewHash(rawHash)
	if err != nil {
		return err
	}

	txOutId, err := encoder.ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	*msg = EADAddress{
		ExpiresAt:  msg.ExpiresAt,
		IP:         ip[:],
		Port:       port,
		Shard:      msg.Shard,
		TxHash:     msg.TxHash,
		TxOutIndex: int(txOutId),
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

	err = encoder.WriteVarInt(w, uint64(msg.Shard))
	if err != nil {
		return err
	}

	// Sigh.  Bitcoin protocol mixes little and big endian.
	err = binary.Write(w, bigEndian, msg.Port)
	if err != nil {
		return err
	}
	err = encoder.WriteVarBytes(w, pver, msg.TxHash.CloneBytes())
	if err != nil {
		return err
	}

	err = encoder.WriteVarInt(w, uint64(msg.TxOutIndex))
	if err != nil {
		return err
	}

	return nil
}
