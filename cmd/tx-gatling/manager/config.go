package manager

import (
	"encoding/hex"
	"errors"

	"gitlab.com/jaxnet/core/shard.core.git/btcec"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
)

type NodeRPC struct {
	Host string `yaml:"host"`
	User string `yaml:"user"`
	Pass string `yaml:"pass"`
}

type ClientCfg struct {
	Net        string  `yaml:"net"`
	RPC        NodeRPC `yaml:"rpc"`
	SyncData   bool    `yaml:"sync_data"`
	DataFile   string  `yaml:"data_file"`
	Append     bool    `yaml:"append"`
	PrivateKey string  `yaml:"private_key"`
}

func (cfg *ClientCfg) NetParams() *chaincfg.Params {
	switch cfg.Net {
	case "simnet":
		return &chaincfg.SimNetParams
	case "testnet":
		return &chaincfg.TestNet3Params
	case "mainnet":
		return &chaincfg.MainNetParams
	}

	return &chaincfg.Params{}
}

type KeyData struct {
	PrivateKeyString string
	PrivateKey       *btcec.PrivateKey
	PublicKey        *btcec.PublicKey
	AddressPubKey    *btcutil.AddressPubKey
	Address          btcutil.Address
}

func NewKeyData(privateKeyString string, networkCfg *chaincfg.Params) (*KeyData, error) {
	privateKeyBytes, err := hex.DecodeString(privateKeyString)
	if err != nil {
		return nil, err
	}

	privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), privateKeyBytes)
	senderAddressPubKey, err := btcutil.NewAddressPubKey(publicKey.SerializeUncompressed(), networkCfg)
	if err != nil {
		return nil, err
	}
	sourceAddress, err := btcutil.DecodeAddress(senderAddressPubKey.EncodeAddress(), networkCfg)

	return &KeyData{
		PrivateKeyString: privateKeyString,
		PrivateKey:       privateKey,
		PublicKey:        publicKey,
		AddressPubKey:    senderAddressPubKey,
		Address:          sourceAddress,
	}, nil
}

func (kd *KeyData) GetKey(address btcutil.Address) (*btcec.PrivateKey, bool, error) {
	if address == kd.Address {
		return kd.PrivateKey, false, nil
	}
	return nil, false, errors.New("nope")
}
