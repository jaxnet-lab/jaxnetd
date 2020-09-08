package chaincfg_test

import (
	"bytes"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"reflect"
	"strings"
	"testing"
)

// Define some of the required parameters for a user-registered
// network.  This is necessary to test the registration of and
// lookup of encoding magics from the network.
var mockNetParams = chaincfg.Params{
	Name:             "mocknet",
	Net:              1<<32 - 1,
	PubKeyHashAddrID: 0x9f,
	ScriptHashAddrID: 0xf9,
	Bech32HRPSegwit:  "tc",
	HDPrivateKeyID:   [4]byte{0x01, 0x02, 0x03, 0x04},
	HDPublicKeyID:    [4]byte{0x05, 0x06, 0x07, 0x08},
}

func TestRegister(t *testing.T) {
	type registerTest struct {
		name   string
		params *chaincfg.Params
		err    error
	}
	type magicTest struct {
		magic byte
		valid bool
	}
	type prefixTest struct {
		prefix string
		valid  bool
	}
	type hdTest struct {
		priv []byte
		want []byte
		err  error
	}

	tests := []struct {
		name           string
		register       []registerTest
		p2pkhMagics    []magicTest
		p2shMagics     []magicTest
		segwitPrefixes []prefixTest
		hdMagics       []hdTest
	}{
		{
			name: "default networks",
			register: []registerTest{
				{
					name:   "duplicate mainnet",
					params: &chaincfg.MainNetParams,
					err:    chaincfg.ErrDuplicateNet,
				},
				{
					name:   "duplicate regtest",
					params: &chaincfg.RegressionNetParams,
					err:    chaincfg.ErrDuplicateNet,
				},
				{
					name:   "duplicate testnet3",
					params: &chaincfg.TestNet3Params,
					err:    chaincfg.ErrDuplicateNet,
				},
				{
					name:   "duplicate simnet",
					params: &chaincfg.SimNetParams,
					err:    chaincfg.ErrDuplicateNet,
				},
			},
			p2pkhMagics: []magicTest{
				{
					magic: chaincfg.MainNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.TestNet3Params.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.RegressionNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.SimNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: mockNetParams.PubKeyHashAddrID,
					valid: false,
				},
				{
					magic: 0xFF,
					valid: false,
				},
			},
			p2shMagics: []magicTest{
				{
					magic: chaincfg.MainNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.TestNet3Params.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.RegressionNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.SimNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: mockNetParams.ScriptHashAddrID,
					valid: false,
				},
				{
					magic: 0xFF,
					valid: false,
				},
			},
			segwitPrefixes: []prefixTest{
				{
					prefix: chaincfg.MainNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chaincfg.TestNet3Params.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chaincfg.RegressionNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chaincfg.SimNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: strings.ToUpper(chaincfg.MainNetParams.Bech32HRPSegwit + "1"),
					valid:  true,
				},
				{
					prefix: mockNetParams.Bech32HRPSegwit + "1",
					valid:  false,
				},
				{
					prefix: "abc1",
					valid:  false,
				},
				{
					prefix: "1",
					valid:  false,
				},
				{
					prefix: chaincfg.MainNetParams.Bech32HRPSegwit,
					valid:  false,
				},
			},
			hdMagics: []hdTest{
				{
					priv: chaincfg.MainNetParams.HDPrivateKeyID[:],
					want: chaincfg.MainNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chaincfg.TestNet3Params.HDPrivateKeyID[:],
					want: chaincfg.TestNet3Params.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chaincfg.RegressionNetParams.HDPrivateKeyID[:],
					want: chaincfg.RegressionNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chaincfg.SimNetParams.HDPrivateKeyID[:],
					want: chaincfg.SimNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: mockNetParams.HDPrivateKeyID[:],
					err:  chaincfg.ErrUnknownHDKeyID,
				},
				{
					priv: []byte{0xff, 0xff, 0xff, 0xff},
					err:  chaincfg.ErrUnknownHDKeyID,
				},
				{
					priv: []byte{0xff},
					err:  chaincfg.ErrUnknownHDKeyID,
				},
			},
		},
		{
			name: "register mocknet",
			register: []registerTest{
				{
					name:   "mocknet",
					params: &mockNetParams,
					err:    nil,
				},
			},
			p2pkhMagics: []magicTest{
				{
					magic: chaincfg.MainNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.TestNet3Params.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.RegressionNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.SimNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: mockNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: 0xFF,
					valid: false,
				},
			},
			p2shMagics: []magicTest{
				{
					magic: chaincfg.MainNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.TestNet3Params.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.RegressionNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.SimNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: mockNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: 0xFF,
					valid: false,
				},
			},
			segwitPrefixes: []prefixTest{
				{
					prefix: chaincfg.MainNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chaincfg.TestNet3Params.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chaincfg.RegressionNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chaincfg.SimNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: strings.ToUpper(chaincfg.MainNetParams.Bech32HRPSegwit + "1"),
					valid:  true,
				},
				{
					prefix: mockNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: "abc1",
					valid:  false,
				},
				{
					prefix: "1",
					valid:  false,
				},
				{
					prefix: chaincfg.MainNetParams.Bech32HRPSegwit,
					valid:  false,
				},
			},
			hdMagics: []hdTest{
				{
					priv: mockNetParams.HDPrivateKeyID[:],
					want: mockNetParams.HDPublicKeyID[:],
					err:  nil,
				},
			},
		},
		{
			name: "more duplicates",
			register: []registerTest{
				{
					name:   "duplicate mainnet",
					params: &chaincfg.MainNetParams,
					err:    chaincfg.ErrDuplicateNet,
				},
				{
					name:   "duplicate regtest",
					params: &chaincfg.RegressionNetParams,
					err:    chaincfg.ErrDuplicateNet,
				},
				{
					name:   "duplicate testnet3",
					params: &chaincfg.TestNet3Params,
					err:    chaincfg.ErrDuplicateNet,
				},
				{
					name:   "duplicate simnet",
					params: &chaincfg.SimNetParams,
					err:    chaincfg.ErrDuplicateNet,
				},
				{
					name:   "duplicate mocknet",
					params: &mockNetParams,
					err:    chaincfg.ErrDuplicateNet,
				},
			},
			p2pkhMagics: []magicTest{
				{
					magic: chaincfg.MainNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.TestNet3Params.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.RegressionNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.SimNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: mockNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: 0xFF,
					valid: false,
				},
			},
			p2shMagics: []magicTest{
				{
					magic: chaincfg.MainNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.TestNet3Params.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.RegressionNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chaincfg.SimNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: mockNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: 0xFF,
					valid: false,
				},
			},
			segwitPrefixes: []prefixTest{
				{
					prefix: chaincfg.MainNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chaincfg.TestNet3Params.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chaincfg.RegressionNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chaincfg.SimNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: strings.ToUpper(chaincfg.MainNetParams.Bech32HRPSegwit + "1"),
					valid:  true,
				},
				{
					prefix: mockNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: "abc1",
					valid:  false,
				},
				{
					prefix: "1",
					valid:  false,
				},
				{
					prefix: chaincfg.MainNetParams.Bech32HRPSegwit,
					valid:  false,
				},
			},
			hdMagics: []hdTest{
				{
					priv: chaincfg.MainNetParams.HDPrivateKeyID[:],
					want: chaincfg.MainNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chaincfg.TestNet3Params.HDPrivateKeyID[:],
					want: chaincfg.TestNet3Params.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chaincfg.RegressionNetParams.HDPrivateKeyID[:],
					want: chaincfg.RegressionNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chaincfg.SimNetParams.HDPrivateKeyID[:],
					want: chaincfg.SimNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: mockNetParams.HDPrivateKeyID[:],
					want: mockNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: []byte{0xff, 0xff, 0xff, 0xff},
					err:  chaincfg.ErrUnknownHDKeyID,
				},
				{
					priv: []byte{0xff},
					err:  chaincfg.ErrUnknownHDKeyID,
				},
			},
		},
	}

	for _, test := range tests {
		for _, regTest := range test.register {
			err := chaincfg.Register(regTest.params)
			if err != regTest.err {
				t.Errorf("%s:%s: Registered network with unexpected error: got %v expected %v",
					test.name, regTest.name, err, regTest.err)
			}
		}
		for i, magTest := range test.p2pkhMagics {
			valid := chaincfg.IsPubKeyHashAddrID(magTest.magic)
			if valid != magTest.valid {
				t.Errorf("%s: P2PKH magic %d valid mismatch: got %v expected %v",
					test.name, i, valid, magTest.valid)
			}
		}
		for i, magTest := range test.p2shMagics {
			valid := chaincfg.IsScriptHashAddrID(magTest.magic)
			if valid != magTest.valid {
				t.Errorf("%s: P2SH magic %d valid mismatch: got %v expected %v",
					test.name, i, valid, magTest.valid)
			}
		}
		for i, prxTest := range test.segwitPrefixes {
			valid := chaincfg.IsBech32SegwitPrefix(prxTest.prefix)
			if valid != prxTest.valid {
				t.Errorf("%s: segwit prefix %s (%d) valid mismatch: got %v expected %v",
					test.name, prxTest.prefix, i, valid, prxTest.valid)
			}
		}
		for i, magTest := range test.hdMagics {
			pubKey, err := chaincfg.HDPrivateKeyToPublicKeyID(magTest.priv[:])
			if !reflect.DeepEqual(err, magTest.err) {
				t.Errorf("%s: HD magic %d mismatched error: got %v expected %v ",
					test.name, i, err, magTest.err)
				continue
			}
			if magTest.err == nil && !bytes.Equal(pubKey, magTest.want[:]) {
				t.Errorf("%s: HD magic %d private and public mismatch: got %v expected %v ",
					test.name, i, pubKey, magTest.want[:])
			}
		}
	}
}
