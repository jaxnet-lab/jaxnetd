/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package txutils

import (
	"gitlab.com/jaxnet/core/shard.core/btcec"
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core/network/rpcclient"
	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type TxClient interface {
	SetKey(key *KeyData)
	WithKeys(key *KeyData) *TxMan
	ForShard(shardID uint32) *TxMan
	RPC() *rpcclient.Client
	CollectUTXO(address string, offset int64) (txmodels.UTXORows, int64, error)
	CollectUTXOs(opts UTXOCollectorOpts) (map[uint32]txmodels.UTXORows, int64, error)
	CollectUTXOIndex(shardID uint32, offset int64,
		filter map[string]bool, index *txmodels.UTXOIndex) (*txmodels.UTXOIndex, int64, error)
	NetworkFee() (int64, error)
	NewTx(destination string, amount int64, utxoPrv UTXOProvider,
		redeemScripts ...string) (*txmodels.Transaction, error)
	NewSwapTx(spendingMap map[string]txmodels.UTXO, postVerify bool,
		redeemScripts ...string) (*txmodels.SwapTransaction, error)
	DraftToSignedTx(data txmodels.DraftTx, postVerify bool) (*wire.MsgTx, error)
	AddSignatureToSwapTx(msgTx *wire.MsgTx, shards []uint32,
		redeemScripts ...string) (*wire.MsgTx, error)
	AddSignatureToTx(msgTx *wire.MsgTx, redeemScripts ...string) (*wire.MsgTx, error)
	SignUTXOForTx(msgTx *wire.MsgTx, utxo txmodels.ShortUTXO, inIndex int, postVerify bool) ([]byte, error)
	NewMultiSig2of2Address(firstPubKey, second string) (*MultiSigAddress, error)
	DecodeScript(script []byte) (*btcjson.DecodeScriptResult, error)
}

type KeyStoreProvider interface {
	GetKey(btcutil.Address) (*btcec.PrivateKey, bool, error)
	AddressPubKey() btcutil.Address
	AddressPubKeyHash() btcutil.Address
	Address() btcutil.Address
	AddressString() string
}

type InMemoryKeystore struct {
	kd KeyData
}

func NewInMemoryKeystore(secret string, net chaincfg.NetName) (KeyStoreProvider, error) {
	kd, err := NewKeyData(secret, net.Params())
	return &InMemoryKeystore{kd: *kd}, err
}

func (*InMemoryKeystore) FromKeyData(data *KeyData) KeyStoreProvider {
	return &InMemoryKeystore{kd: *data}
}

func (kp *InMemoryKeystore) GetKey(address btcutil.Address) (*btcec.PrivateKey, bool, error) {
	return kp.kd.GetKey(address)
}

func (kp *InMemoryKeystore) AddressPubKeyHash() btcutil.Address {
	return kp.kd.AddressPubKey.AddressPubKeyHash()
}

func (kp *InMemoryKeystore) AddressPubKey() btcutil.Address {
	return kp.kd.AddressPubKey
}

func (kp *InMemoryKeystore) Address() btcutil.Address {
	return kp.kd.Address
}

func (kp *InMemoryKeystore) AddressString() string {
	return kp.kd.Address.EncodeAddress()
}

// DEPRECATED
func (kp *InMemoryKeystore) KeyData() *KeyData {
	return &kp.kd
}

type NewUTXOProvider interface {
	// RedeemScript(address string) (script string)

	// SelectForAmount returns a list of txmodels.UTXO which satisfies the passed amount.
	SelectForAmount(amount int64, shardID uint32, addresses ...string) (txmodels.UTXORows, error)
	// GetForAmount returns a single txmodels.UTXO with Value GreaterOrEq passed amount.
	GetForAmount(amount int64, shardID uint32, addresses ...string) (*txmodels.UTXO, error)
}

type NewTxClient interface {
	SetKey(keystore KeyStoreProvider)
	WithKeys(keystore KeyStoreProvider) NewTxClient

	SetShard(shardID uint32)
	ForShard(shardID uint32) NewTxClient

	RPC() *rpcclient.Client
	NetworkFee() (int64, error)
	DecodeScript(script []byte) (*btcjson.DecodeScriptResult, error)
	NewMultiSig2of2Address(firstPubKey, second string) (*MultiSigAddress, error)

	CollectUTXO(address string, offset int64) (txmodels.UTXORows, int64, error)
	CollectUTXOs(opts UTXOCollectorOpts) (map[uint32]txmodels.UTXORows, int64, error)
	CollectUTXOIndex(shardID uint32, offset int64,
		filter map[string]bool, index *txmodels.UTXOIndex) (*txmodels.UTXOIndex, int64, error)

	NewTx(tx TxBuilder) (*txmodels.Transaction, error)

	AddSignatureToSwapTx(msgTx *wire.MsgTx, shards []uint32,
		redeemScripts ...string) (*wire.MsgTx, error)
	AddSignatureToTx(msgTx *wire.MsgTx, redeemScripts ...string) (*wire.MsgTx, error)

	SignUTXOForTx(msgTx *wire.MsgTx, utxo txmodels.ShortUTXO, inIndex int, postVerify bool) ([]byte, error)
}
