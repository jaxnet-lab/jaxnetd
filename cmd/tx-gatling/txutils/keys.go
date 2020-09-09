package txutils

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core.git/btcec"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
)

type KeyData struct {
	PrivateKey    *btcec.PrivateKey
	Address       btcutil.Address
	AddressPubKey *btcutil.AddressPubKey
}

func GenerateKey(networkCfg *chaincfg.Params) (*KeyData, error) {
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return nil, errors.Wrap(err, "failed to make privKey")
	}

	// pk := (*btcec.PublicKey)(&key.PublicKey).SerializeCompressed()
	pk := (*btcec.PublicKey)(&key.PublicKey).SerializeUncompressed()
	addressPubKey, err := btcutil.NewAddressPubKey(pk, &chaincfg.SimNetParams)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create address pub key")
	}

	return &KeyData{
		PrivateKey:    key,
		AddressPubKey: addressPubKey,
		Address:       addressPubKey,
	}, nil
}

func NewKeyData(privateKeyString string, networkCfg *chaincfg.Params) (*KeyData, error) {
	privateKeyBytes, err := hex.DecodeString(privateKeyString)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode private key from hex")
	}

	privateKey, publicKey := btcec.PrivKeyFromBytes(btcec.S256(), privateKeyBytes)
	// addressPubKey, err := btcutil.NewAddressPubKey(publicKey.SerializeCompressed(), networkCfg)
	addressPubKey, err := btcutil.NewAddressPubKey(publicKey.SerializeUncompressed(), networkCfg)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse address pub key")
	}

	return &KeyData{
		PrivateKey:    privateKey,
		AddressPubKey: addressPubKey,
		Address:       addressPubKey,
	}, nil
}

func (kd *KeyData) GetKey(address btcutil.Address) (*btcec.PrivateKey, bool, error) {
	if address.String() == kd.Address.EncodeAddress() {
		return kd.PrivateKey, false, nil
	}

	return nil, false, errors.New("nope")
}

type MultiSigAddress struct {
	Address         string `json:"address"`
	RedeemScript    string `json:"redeemScript"`
	RawRedeemScript []byte `json:"-"`
}

func MakeMultiSigScript(keys []string, nRequired int, net *chaincfg.Params) (*MultiSigAddress, error) {
	keysesPrecious := make([]*btcutil.AddressPubKey, len(keys))

	// The address list will made up either of addreseses (pubkey hash), for
	// which we need to look up the keys in wallet, straight pubkeys, or a
	// mixture of the two.
	for i, pubKey := range keys {
		// try to parse as pubkey address
		rawPK, err := hex.DecodeString(pubKey)
		// address, err := btcutil.DecodeAddress(pubKey, net)
		if err != nil {
			return nil, err
		}

		addr, err := btcutil.NewAddressPubKey(rawPK, net)
		if err != nil {
			return nil, err
		}

		keysesPrecious[i] = addr
	}

	script, err := txscript.MultiSigScript(keysesPrecious, nRequired)
	if err != nil {
		return nil, err
	}
	address, err := btcutil.NewAddressScriptHash(script, net)
	if err != nil {
		// above is a valid script, shouldn't happen.
		return nil, err
	}

	return &MultiSigAddress{
		Address:         address.EncodeAddress(),
		RedeemScript:    hex.EncodeToString(script),
		RawRedeemScript: script,
	}, nil
}
