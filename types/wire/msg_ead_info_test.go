package wire

import (
	"bytes"
	"net"
	"testing"
	"time"

	"gitlab.com/jaxnet/core/shard.core/btcec"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
)

func TestEADAddress_BtcEncode(t *testing.T) {
	hash, _ := chainhash.NewHashFromStr("3368d7faaaa40086cf7c442a107db9959f340cb67e28b14d759a37fd945592d4")
	val := EADAddress{
		IP:         net.IPv4(127, 0, 2, 42),
		Port:       28023,
		ExpiresAt:  time.Now(),
		Shards:     []uint32{1, 2, 43},
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
		IPs: []EADAddress{{
			IP:         net.IPv4(127, 0, 2, 42),
			Port:       28023,
			ExpiresAt:  time.Now(),
			TxHash:     hash,
			TxOutIndex: 3,
			Shards:     []uint32{12, 11, 2},
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

	if len(newVal.IPs) != len(val.IPs) {
		t.Error("len(val.IPs) mismatch")
		return
	}
}
