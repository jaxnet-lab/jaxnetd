package wire

import (
	"bytes"
	"net"
	"testing"
	"time"

	"gitlab.com/jaxnet/core/shard.core/btcec"
)

func TestEADAddress_BtcEncode(t *testing.T) {
	val := EADAddress{
		IP:        net.IPv4(127, 0, 2, 42),
		Port:      28023,
		ExpiresAt: time.Now(),
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

	val := EADAddresses{
		ID:          123,
		OwnerPubKey: pk,
		IPs: []EADAddress{{
			IP:        net.IPv4(127, 0, 2, 42),
			Port:      28023,
			ExpiresAt: time.Now(),
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
