package chain_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/jaxnet/core/shard.core.git/shards/chain"
)

// Define some of the required parameters for a user-registered
// network.  This is necessary to test the registration of and
// lookup of encoding magics from the network.
var mockNetParams = chain.Params{
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
		params *chain.Params
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
					params: &chain.MainNetParams,
					err:    chain.ErrDuplicateNet,
				},
				{
					name:   "duplicate regtest",
					params: &chain.RegressionNetParams,
					err:    chain.ErrDuplicateNet,
				},
				{
					name:   "duplicate testnet",
					params: &chain.TestNet3Params,
					err:    chain.ErrDuplicateNet,
				},
				{
					name:   "duplicate simnet",
					params: &chain.SimNetParams,
					err:    chain.ErrDuplicateNet,
				},
			},
			p2pkhMagics: []magicTest{
				{
					magic: chain.MainNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chain.TestNet3Params.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chain.RegressionNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chain.SimNetParams.PubKeyHashAddrID,
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
					magic: chain.MainNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chain.TestNet3Params.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chain.RegressionNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chain.SimNetParams.ScriptHashAddrID,
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
					prefix: chain.MainNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chain.TestNet3Params.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chain.RegressionNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chain.SimNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: strings.ToUpper(chain.MainNetParams.Bech32HRPSegwit + "1"),
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
					prefix: chain.MainNetParams.Bech32HRPSegwit,
					valid:  false,
				},
			},
			hdMagics: []hdTest{
				{
					priv: chain.MainNetParams.HDPrivateKeyID[:],
					want: chain.MainNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chain.TestNet3Params.HDPrivateKeyID[:],
					want: chain.TestNet3Params.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chain.RegressionNetParams.HDPrivateKeyID[:],
					want: chain.RegressionNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chain.SimNetParams.HDPrivateKeyID[:],
					want: chain.SimNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: mockNetParams.HDPrivateKeyID[:],
					err:  chain.ErrUnknownHDKeyID,
				},
				{
					priv: []byte{0xff, 0xff, 0xff, 0xff},
					err:  chain.ErrUnknownHDKeyID,
				},
				{
					priv: []byte{0xff},
					err:  chain.ErrUnknownHDKeyID,
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
					magic: chain.MainNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chain.TestNet3Params.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chain.RegressionNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chain.SimNetParams.PubKeyHashAddrID,
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
					magic: chain.MainNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chain.TestNet3Params.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chain.RegressionNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chain.SimNetParams.ScriptHashAddrID,
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
					prefix: chain.MainNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chain.TestNet3Params.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chain.RegressionNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chain.SimNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: strings.ToUpper(chain.MainNetParams.Bech32HRPSegwit + "1"),
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
					prefix: chain.MainNetParams.Bech32HRPSegwit,
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
					params: &chain.MainNetParams,
					err:    chain.ErrDuplicateNet,
				},
				{
					name:   "duplicate regtest",
					params: &chain.RegressionNetParams,
					err:    chain.ErrDuplicateNet,
				},
				{
					name:   "duplicate testnet",
					params: &chain.TestNet3Params,
					err:    chain.ErrDuplicateNet,
				},
				{
					name:   "duplicate simnet",
					params: &chain.SimNetParams,
					err:    chain.ErrDuplicateNet,
				},
				{
					name:   "duplicate mocknet",
					params: &mockNetParams,
					err:    chain.ErrDuplicateNet,
				},
			},
			p2pkhMagics: []magicTest{
				{
					magic: chain.MainNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chain.TestNet3Params.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chain.RegressionNetParams.PubKeyHashAddrID,
					valid: true,
				},
				{
					magic: chain.SimNetParams.PubKeyHashAddrID,
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
					magic: chain.MainNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chain.TestNet3Params.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chain.RegressionNetParams.ScriptHashAddrID,
					valid: true,
				},
				{
					magic: chain.SimNetParams.ScriptHashAddrID,
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
					prefix: chain.MainNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chain.TestNet3Params.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chain.RegressionNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: chain.SimNetParams.Bech32HRPSegwit + "1",
					valid:  true,
				},
				{
					prefix: strings.ToUpper(chain.MainNetParams.Bech32HRPSegwit + "1"),
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
					prefix: chain.MainNetParams.Bech32HRPSegwit,
					valid:  false,
				},
			},
			hdMagics: []hdTest{
				{
					priv: chain.MainNetParams.HDPrivateKeyID[:],
					want: chain.MainNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chain.TestNet3Params.HDPrivateKeyID[:],
					want: chain.TestNet3Params.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chain.RegressionNetParams.HDPrivateKeyID[:],
					want: chain.RegressionNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: chain.SimNetParams.HDPrivateKeyID[:],
					want: chain.SimNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: mockNetParams.HDPrivateKeyID[:],
					want: mockNetParams.HDPublicKeyID[:],
					err:  nil,
				},
				{
					priv: []byte{0xff, 0xff, 0xff, 0xff},
					err:  chain.ErrUnknownHDKeyID,
				},
				{
					priv: []byte{0xff},
					err:  chain.ErrUnknownHDKeyID,
				},
			},
		},
	}

	for _, test := range tests {
		for _, regTest := range test.register {
			err := chain.Register(regTest.params)
			if err != regTest.err {
				t.Errorf("%s:%s: Registered network with unexpected error: got %v expected %v",
					test.name, regTest.name, err, regTest.err)
			}
		}
		for i, magTest := range test.p2pkhMagics {
			valid := chain.IsPubKeyHashAddrID(magTest.magic)
			if valid != magTest.valid {
				t.Errorf("%s: P2PKH magic %d valid mismatch: got %v expected %v",
					test.name, i, valid, magTest.valid)
			}
		}
		for i, magTest := range test.p2shMagics {
			valid := chain.IsScriptHashAddrID(magTest.magic)
			if valid != magTest.valid {
				t.Errorf("%s: P2SH magic %d valid mismatch: got %v expected %v",
					test.name, i, valid, magTest.valid)
			}
		}
		for i, prxTest := range test.segwitPrefixes {
			valid := chain.IsBech32SegwitPrefix(prxTest.prefix)
			if valid != prxTest.valid {
				t.Errorf("%s: segwit prefix %s (%d) valid mismatch: got %v expected %v",
					test.name, prxTest.prefix, i, valid, prxTest.valid)
			}
		}
		for i, magTest := range test.hdMagics {
			pubKey, err := chain.HDPrivateKeyToPublicKeyID(magTest.priv[:])
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
