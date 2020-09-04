package txmanager

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core.git/btcec"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
)

type KeyData struct {
	PrivateKey    *btcec.PrivateKey
	Address       btcutil.Address
	AddressPubKey *btcutil.AddressPubKey
}

func NewKeyData(privateKeyString string, networkCfg *chaincfg.Params) (*KeyData, error) {
	privateKeyBytes, err := hex.DecodeString(privateKeyString)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode private key from hex")
	}

	privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), privateKeyBytes)
	senderAddressPubKey, err := btcutil.NewAddressPubKey(publicKey.SerializeUncompressed(), networkCfg)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse address pub key")
	}

	return &KeyData{
		PrivateKey:    privateKey,
		AddressPubKey: senderAddressPubKey,
		Address:       senderAddressPubKey,
	}, nil
}

func (kd *KeyData) GetKey(address btcutil.Address) (*btcec.PrivateKey, bool, error) {
	if address.String() == kd.Address.EncodeAddress() {
		return kd.PrivateKey, false, nil
	}

	return nil, false, errors.New("nope")
}
