package txutils

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/core/shard.core.git/btcec"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
)

func TestNewKeyData(t *testing.T) {
	key, err := btcec.NewPrivateKey(btcec.S256())
	assert.NoError(t, err)

	pk := (*btcec.PublicKey)(&key.PublicKey).SerializeUncompressed()
	simNetAddress, err := btcutil.NewAddressPubKeyHash(btcutil.Hash160(pk), &chain.SimNetParams)
	assert.NoError(t, err)

	private := hex.EncodeToString(key.Serialize())
	println(private)
	keyData, err := NewKeyData(private, &chain.SimNetParams)
	assert.NoError(t, err)
	assert.Equal(t, keyData.Address.EncodeAddress(), simNetAddress.EncodeAddress())

	println(fmt.Sprintf("%x", key.Serialize()))

	keyData, err = NewKeyData(fmt.Sprintf("%x", key.Serialize()), &chain.SimNetParams)
	assert.NoError(t, err)
	assert.Equal(t, keyData.Address.EncodeAddress(), simNetAddress.EncodeAddress())

	MinerSk := "3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b"
	AliceSk := "6443fb332e1cbfe456674aacf2be1327b6f9fc9c782061ee04ca35e17608d651"
	BobSk := "6bb4b4a9d5512c84f14bd38248dafb80c2424ae50a0495be8e4f657d734f1bd4"
	EvaSk := "bdfb934f403bd6c3f74730f9690f6fc22863388f473860eb001a1e7f02261b79"

	for _, sk := range []string{MinerSk, AliceSk, BobSk, EvaSk} {
		kd, _ := NewKeyData(sk, &chain.TestNet3Params)
		println(kd.AddressPubKey.String())
	}
}
