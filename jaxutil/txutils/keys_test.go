package txutils

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/jaxnetd/btcec"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

func TestNewKeyData(t *testing.T) {
	key, err := btcec.NewPrivateKey(btcec.S256())
	assert.NoError(t, err)

	pk := (*btcec.PublicKey)(&key.PublicKey).SerializeUncompressed()
	simNetAddress, err := jaxutil.NewAddressPubKeyHash(jaxutil.Hash160(pk), &chaincfg.SimNetParams)
	assert.NoError(t, err)

	private := hex.EncodeToString(key.Serialize())
	println(private)
	keyData, err := NewKeyData(private, &chaincfg.SimNetParams)
	assert.NoError(t, err)
	assert.Equal(t, keyData.Address.EncodeAddress(), simNetAddress.EncodeAddress())

	println(fmt.Sprintf("%x", key.Serialize()))

	keyData, err = NewKeyData(fmt.Sprintf("%x", key.Serialize()), &chaincfg.SimNetParams)
	assert.NoError(t, err)
	assert.Equal(t, keyData.Address.EncodeAddress(), simNetAddress.EncodeAddress())

	MinerSk := ""
	AliceSk := ""
	BobSk := ""
	EvaSk := ""

	for _, sk := range []string{MinerSk, AliceSk, BobSk, EvaSk} {
		kd, _ := NewKeyData(sk, &chaincfg.TestNet3Params)
		println(kd.AddressPubKey.String())
	}
}
