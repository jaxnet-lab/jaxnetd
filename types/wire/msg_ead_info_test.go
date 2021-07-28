package wire

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/jaxnetd/btcec"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

func TestEADAddress_BtcEncode(t *testing.T) {
	hash, _ := chainhash.NewHashFromStr("3368d7faaaa40086cf7c442a107db9959f340cb67e28b14d759a37fd945592d4")
	val := EADAddress{
		IP:         net.IPv4(127, 0, 2, 42),
		Port:       28023,
		ExpiresAt:  time.Now(),
		Shard:      43,
		TxOutIndex: 1,
		TxHash:     hash,
	}

	buf := bytes.NewBuffer(nil)
	err := val.BtcEncode(buf, ProtocolVersion, BaseEncoding)
	if err != nil {
		t.Errorf("unable to encode: %v", err)
		return
	}

	data := buf.Bytes()
	newVal := new(EADAddress)
	dataBuf := bytes.NewBuffer(data)

	err = newVal.BtcDecode(dataBuf, ProtocolVersion, BaseEncoding)
	if err != nil {
		t.Errorf("unable to decode: %v", err)
		return
	}

	if !newVal.IP.Equal(val.IP) {
		t.Error("ip mismatch")
		return
	}

	if newVal.Port != val.Port {
		t.Error("Port mismatch")
		return
	}
	if newVal.TxOutIndex != val.TxOutIndex {
		t.Error("TxOutIndex mismatch")
		return
	}
	if newVal.TxHash.String() != hash.String() {
		t.Error("TxHash mismatch")
		return
	}
	if newVal.ExpiresAt.Unix() != val.ExpiresAt.Unix() {
		t.Error("ExpiresAt mismatch")
		return
	}

}
func TestEADAddresses_BtcEncode(t *testing.T) {
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Errorf("failed to make privKey: %v", err)
		return
	}
	pk := (*btcec.PublicKey)(&key.PublicKey).
		SerializeCompressed()
	hash, _ := chainhash.NewHashFromStr("3368d7faaaa40086cf7c442a107db9959f340cb67e28b14d759a37fd945592d4")

	val := EADAddresses{
		ID:          123,
		OwnerPubKey: pk,
		Addresses: []EADAddress{{
			IP:         net.IPv4(127, 0, 2, 42),
			Port:       28023,
			ExpiresAt:  time.Now(),
			TxHash:     hash,
			TxOutIndex: 3,
			Shard:      12,
		}},
	}

	buf := bytes.NewBuffer(nil)
	err = val.BtcEncode(buf, ProtocolVersion, BaseEncoding)
	if err != nil {
		t.Errorf("unable to encode: %v", err)
		return
	}

	data := buf.Bytes()
	newVal := new(EADAddresses)
	dataBuf := bytes.NewBuffer(data)

	err = newVal.BtcDecode(dataBuf, ProtocolVersion, BaseEncoding)
	if err != nil {
		t.Errorf("unable to decode: %v", err)
		return
	}

	if newVal.ID != val.ID {
		t.Error("ID mismatch")
		return
	}

	if len(newVal.Addresses) != len(val.Addresses) {
		t.Error("len(val.Addresses) mismatch")
		return
	}
}

func TestEADAddresses_AddAddress(t *testing.T) {
	set := EADAddresses{
		ID:          42,
		OwnerPubKey: []byte("test_3333"),
		Addresses:   []EADAddress{},
	}

	assert.Equal(t, 0, len(set.Addresses))

	ips := []net.IP{
		0: net.IPv4(14, 12, 33, 21),
		1: net.IPv4(14, 12, 33, 22),
		2: net.IPv4(14, 12, 33, 23),
		3: nil,
		4: nil,
	}

	urls := []string{
		0: "",
		1: "",
		2: "",
		3: "test_url_1",
		4: "test_url_2",
	}

	set.AddAddress(ips[0], urls[0], 0, 32, 1, &chainhash.ZeroHash, 0)
	assert.Equal(t, 1, len(set.Addresses))

	set.AddAddress(ips[0], urls[0], 0, 32, 1, &chainhash.ZeroHash, 1)
	assert.Equal(t, 1, len(set.Addresses))

	set.AddAddress(ips[1], urls[1], 0, 32, 1, &chainhash.ZeroHash, 0)
	set.AddAddress(ips[2], urls[2], 0, 32, 1, &chainhash.ZeroHash, 0)
	set.AddAddress(ips[3], urls[3], 0, 32, 1, &chainhash.ZeroHash, 0)
	set.AddAddress(ips[4], urls[4], 0, 32, 1, &chainhash.ZeroHash, 0)

	assert.Equal(t, 5, len(set.Addresses))

	set.AddAddress(ips[3], urls[3], 0, 32, 1, &chainhash.ZeroHash, 55)
	set.AddAddress(ips[4], urls[4], 0, 32, 1, &chainhash.ZeroHash, 12)

	assert.Equal(t, 5, len(set.Addresses))

	set.Addresses = append(set.Addresses, EADAddress{
		IP:         ips[4],
		Port:       21,
		URL:        urls[4],
		ExpiresAt:  time.Time{},
		Shard:      1,
		TxHash:     &chainhash.ZeroHash,
		TxOutIndex: 21,
	})

	set.Addresses = append(set.Addresses, EADAddress{
		IP:         ips[4],
		Port:       211,
		URL:        urls[4],
		ExpiresAt:  time.Time{},
		Shard:      1,
		TxHash:     &chainhash.ZeroHash,
		TxOutIndex: 22,
	})

	set.Addresses = append(set.Addresses, EADAddress{
		IP:         ips[4],
		Port:       211,
		URL:        urls[4],
		ExpiresAt:  time.Time{},
		Shard:      1,
		TxHash:     &chainhash.ZeroHash,
		TxOutIndex: 23,
	})
	assert.Equal(t, 8, len(set.Addresses))

	set.AddAddress(ips[4], urls[4], 0, 32, 1, &chainhash.ZeroHash, 12)

	assert.Equal(t, 5, len(set.Addresses))

}
