package txmanager

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/core/shard.core.git/btcec"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
)

func TestNewKeyData(t *testing.T) {
	key, err := btcec.NewPrivateKey(btcec.S256())
	assert.NoError(t, err)

	pk := (*btcec.PublicKey)(&key.PublicKey).SerializeUncompressed()
	simNetAddress, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pk), &chaincfg.SimNetParams)
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
}
