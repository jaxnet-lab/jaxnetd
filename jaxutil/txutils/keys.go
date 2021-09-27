// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txutils

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/btcec"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/jaxutil/txmodels"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
)

type KeyData struct {
	PrivateKey    *btcec.PrivateKey
	Address       jaxutil.Address
	AddressPubKey *jaxutil.AddressPubKey
}

func GenerateKey(networkCfg *chaincfg.Params) (*KeyData, error) {
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return nil, errors.Wrap(err, "failed to make privKey")
	}

	pk := (*btcec.PublicKey)(&key.PublicKey).SerializeUncompressed()
	addressPubKey, err := jaxutil.NewAddressPubKey(pk, networkCfg)
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
	addressPubKey, err := jaxutil.NewAddressPubKey(publicKey.SerializeUncompressed(), networkCfg)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse address pub key")
	}

	return &KeyData{
		PrivateKey:    privateKey,
		AddressPubKey: addressPubKey,
		Address:       addressPubKey,
	}, nil
}

func (kd *KeyData) GetKey(address jaxutil.Address) (*btcec.PrivateKey, bool, error) {
	if address.EncodeAddress() == kd.Address.EncodeAddress() {
		return kd.PrivateKey, false, nil
	}

	return nil, false, errors.New("nope")
}

type MultiSigAddress struct {
	Address            string `json:"address"`
	RedeemScript       string `json:"redeemScript"`
	SignaturesRequired int    `json:"signaturesRequired"`
	RawRedeemScript    []byte `json:"-"`
}

func MakeMultiSigScript(keys []string, nRequired int, net *chaincfg.Params) (*MultiSigAddress, error) {
	keysesPrecious := make([]*jaxutil.AddressPubKey, len(keys))

	for i, pubKey := range keys {
		// try to parse as pubkey address
		rawPK, err := hex.DecodeString(pubKey)
		if err != nil {
			return nil, err
		}

		addr, err := jaxutil.NewAddressPubKey(rawPK, net)
		if err != nil {
			return nil, err
		}

		keysesPrecious[i] = addr
	}

	script, err := txscript.MultiSigScript(keysesPrecious, nRequired, true)
	if err != nil {
		return nil, err
	}
	address, err := jaxutil.NewAddressScriptHash(script, net)
	if err != nil {
		// above is a valid script, shouldn't happen.
		return nil, err
	}

	return &MultiSigAddress{
		Address:            address.EncodeAddress(),
		RedeemScript:       hex.EncodeToString(script),
		SignaturesRequired: nRequired,
		RawRedeemScript:    script,
	}, nil
}

type MultiSigLockAddress struct {
	Address               string `json:"address"`
	RefundPublicKey       string `json:"refundPublicKey"`
	RefundDefferingPeriod int32  `json:"refundDefferingPeriod"`
	RedeemScript          string `json:"redeemScript"`
	SignaturesRequired    int    `json:"signaturesRequired"`
	RawRedeemScript       []byte `json:"-"`
}

func MakeMultiSigLockAddress(keys []string, nRequired int, refundPublicKey string,
	refundDefferingPeriod int32, net *chaincfg.Params) (*MultiSigLockAddress, error) {
	refungAddress, err := AddressPubKeyFromString(refundPublicKey, net)
	if err != nil {
		return nil, err
	}

	keysesPrecious := make([]*jaxutil.AddressPubKey, len(keys))

	for i, pubKey := range keys {
		keysesPrecious[i], err = AddressPubKeyFromString(pubKey, net)
		if err != nil {
			return nil, err
		}

	}

	script, err := txscript.MultiSigLockScript(keysesPrecious, nRequired, refungAddress, refundDefferingPeriod, true)
	if err != nil {
		return nil, err
	}
	address, err := jaxutil.NewAddressScriptHash(script, net)
	if err != nil {
		// above is a valid script, shouldn't happen.
		return nil, err
	}

	return &MultiSigLockAddress{
		Address:               address.EncodeAddress(),
		RefundPublicKey:       refundPublicKey,
		RefundDefferingPeriod: refundDefferingPeriod,
		RedeemScript:          hex.EncodeToString(script),
		SignaturesRequired:    nRequired,
		RawRedeemScript:       script,
	}, nil
}

func SetRedeemScript(utxo txmodels.UTXO, redeemScript string, net *chaincfg.Params) (txmodels.UTXO, error) {
	rawScript, err := hex.DecodeString(redeemScript)
	if err != nil {
		return utxo, errors.Wrap(err, "unable to decode hex script")
	}
	script, err := DecodeScript(rawScript, net)
	if err != nil {
		return utxo, errors.Wrap(err, "unable to parse script")
	}
	utxo.PKScript = redeemScript
	utxo.ScriptType = script.Type
	return utxo, nil
}

func DecodeScript(script []byte, net *chaincfg.Params) (*jaxjson.DecodeScriptResult, error) {
	// The disassembled string will contain [error] inline if the script
	// doesn't fully parse, so ignore the error here.
	asm, _ := txscript.DisasmString(script)

	// Get information about the script.
	// Ignore the error here since an error means the script couldn't parse
	// and there is no additional information about it anyways.
	scriptClass, address, reqSigns, _ := txscript.ExtractPkScriptAddrs(script, net)
	addresses := make([]string, len(address))
	for i, addr := range address {
		addresses[i] = addr.EncodeAddress()
	}

	// Convert the script itself to a pay-to-script-hash address.
	p2sh, err := jaxutil.NewAddressScriptHash(script, net)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert script to pay-to-script-hash")
	}

	// Generate and return the reply.
	reply := &jaxjson.DecodeScriptResult{
		Asm:       asm,
		ReqSigs:   int32(reqSigns),
		Type:      scriptClass.String(),
		Addresses: addresses,
	}
	if scriptClass != txscript.ScriptHashTy {
		reply.P2sh = p2sh.EncodeAddress()
	}

	return reply, nil
}

func AddressPubKeyFromString(pubKey string, net *chaincfg.Params) (*jaxutil.AddressPubKey, error) {
	// try to parse as pubkey address
	rawPK, err := hex.DecodeString(pubKey)
	if err != nil {
		return nil, err
	}

	return jaxutil.NewAddressPubKey(rawPK, net)
}
