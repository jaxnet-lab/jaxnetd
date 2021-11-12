package wire

import (
	"io"
	"net"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

const (
	// MaxEADDomainLen is limit for the length of ead address host/URI
	MaxEADDomainLen = 256
)

type EADAddresses struct {
	ID          uint64
	OwnerPubKey []byte
	Addresses   []EADAddress // Address is unique combination of {IP|URL, SHARD_ID}
}

func (msg *EADAddresses) AddAddress(ip net.IP, url string, port uint16, expiresAt int64, shardID uint32,
	hash *chainhash.Hash, ind int) {

	addresses := make([]EADAddress, 0, len(msg.Addresses)+1)
	for _, p := range msg.Addresses {
		if !p.Eq(ip, url, shardID) {
			addresses = append(addresses, p)
		}
	}

	addresses = append(addresses, EADAddress{
		IP:         ip,
		URL:        url,
		Port:       port,
		ExpiresAt:  time.Unix(expiresAt, 0),
		Shard:      shardID,
		TxHash:     hash,
		TxOutIndex: ind,
	})

	msg.Addresses = addresses
}

func (msg *EADAddress) HasShard(shards ...uint32) (hasOneOf bool) {
	for _, shard := range shards {
		if shard == msg.Shard {
			return true
		}
	}
	return false
}

func (msg *EADAddresses) Command() string {
	return CmdEadAddresses
}

func (msg *EADAddresses) MaxPayloadLength(uint32) uint32 {
	return MaxBlockPayload
}

func (msg *EADAddresses) BtcDecode(r io.Reader, pver uint32, enc MessageEncoding) error {
	alias, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	msg.ID = alias

	msg.OwnerPubKey, err = ReadVarBytes(r, pver, 65*2, "OwnerPubKey")
	if err != nil {
		return err
	}
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return err
	}
	msg.Addresses = make([]EADAddress, count)

	for i := range msg.Addresses {
		err = msg.Addresses[i].BtcDecode(r, pver, enc)
		if err != nil {
			return err
		}
	}

	return nil
}

// BtcEncode encodes the receiver to w using the bitcoin protocol encoding.
// This is part of the Message interface implementation.
func (msg *EADAddresses) BtcEncode(w io.Writer, pver uint32, enc MessageEncoding) error {
	err := WriteVarInt(w, msg.ID)
	if err != nil {
		return err
	}

	err = WriteVarBytes(w, pver, msg.OwnerPubKey)
	if err != nil {
		return err
	}

	// Protocol versions before MultipleAddressVersion only allowed 1 address
	// per message.
	count := len(msg.Addresses)
	err = WriteVarInt(w, uint64(count))
	if err != nil {
		return err
	}

	for i := range msg.Addresses {
		err = msg.Addresses[i].BtcEncode(w, pver, enc)
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
	// Port the server is using.
	Port uint16
	// URL address of the server.
	URL string
	// ExpiresAt Address expiration time.
	ExpiresAt time.Time
	// Shard shows what shards the agent works with.
	Shard      uint32
	TxHash     *chainhash.Hash
	TxOutIndex int
}

// Eq returns true if the address match with passed params.
func (msg *EADAddress) Eq(ip net.IP, url string, shardID uint32) bool {
	ipEq := msg.IP == nil || msg.IP.Equal(ip)
	urlEq := msg.URL == "" || msg.URL == url
	shardEq := msg.Shard == shardID
	return ipEq && urlEq && shardEq
}

func (msg *EADAddress) Command() string {
	return CmdEadIP
}

func (msg *EADAddress) MaxPayloadLength(uint32) uint32 {
	return 16 + 4 + 8 // todo: fix this
}

func (msg *EADAddress) BtcDecode(r io.Reader, pver uint32, _ MessageEncoding) error {
	var port uint32
	var txOutIndex uint64
	var isURL bool
	msg.TxHash = new(chainhash.Hash)
	err := ReadElements(r,
		(*Uint32Time)(&msg.ExpiresAt),
		&msg.Shard,
		msg.TxHash,
		&port,
		&txOutIndex,
		&isURL,
	)
	if err != nil {
		return err
	}
	*msg = EADAddress{
		ExpiresAt:  msg.ExpiresAt,
		Port:       uint16(port),
		Shard:      msg.Shard,
		TxHash:     msg.TxHash,
		TxOutIndex: int(txOutIndex),
	}

	if !isURL {
		var ip [16]byte
		err := ReadElement(r, &ip)
		if err != nil {
			return err
		}
		msg.IP = ip[:]
	} else {
		domain, err := ReadVarBytes(r, pver, MaxEADDomainLen, "TxHash")
		if err != nil {
			return err
		}
		msg.URL = string(domain)
	}

	return nil
}

func (msg *EADAddress) BtcEncode(w io.Writer, pver uint32, _ MessageEncoding) error {
	err := WriteElements(w,
		uint32(msg.ExpiresAt.Unix()),
		msg.Shard,
		msg.TxHash,
		uint32(msg.Port),
		uint64(msg.TxOutIndex),
		msg.IP == nil, // isDomain
	)
	if err != nil {
		return err
	}

	// Ensure to always write 16 bytes even if the ip is nil.
	if msg.IP != nil {
		var ip [16]byte
		copy(ip[:], msg.IP.To16())
		err = WriteElement(w, ip)
		if err != nil {
			return err
		}
		return nil
	}

	err = WriteVarBytes(w, pver, []byte(msg.URL))
	if err != nil {
		return err
	}

	return nil
}
