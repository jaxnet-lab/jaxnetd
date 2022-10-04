/*
 * Copyright (c) 2022 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaindata

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/jaxutil/bch"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

var (
	beaconHashStub    = chainhash.HashH([]byte("only_for_documentation_and_tests"))
	mainnetAddress, _ = jaxutil.DecodeAddress("1JAXmGDsiE2CyK31dYZsMamM18pPebRDAk", &chaincfg.MainNetParams)
	txHashes          = func() []chainhash.Hash {
		txs := []string{
			"ee19cc5e240456d27087eaaf1770b637852f777efc54847e773be171a44fbbbf", // coinbase
			"4a192844a39917bb6110362ae2ac4c823745c8356a05bce8b6c267a2f087be4e",
			"5b3a3089a05d17bda104f117727fa22a109c6e25b5773d7e0794f00ee3902df1",
			"8c26d14a2f0765741c9de103995e5e00c67064340333e48d8be16f3b141bb5ad",
			"862577320ef43a2bcc508302d48d8d067e7cf982afef2307a9c84c86e31f8c7e",
		}
		txh := make([]chainhash.Hash, len(txs))
		for i, tx := range txs {
			txh[i] = *newHashFromStr(tx)
		}

		return txh
	}()
)

func getBTCAux(t *testing.T, address jaxutil.Address, beacon *chainhash.Hash, burnReward bool) *wire.BTCBlockAux {
	var (
		// btc testnet 2140915
		prevHashStr   = "000000000000004d44ac8dacfad95e61283f1ea0864f0d8088c1318d9c672787"
		merkleRootStr = "0eed2dccdadf9b0f472e476e03cbccd4b973b7362cc49832eae159829cf11822"
		// btc testnet 2140916
		hashStr          = "0000000000000051c48b99694a65a54ee5bcc36881d241bd1913c2d68869c3ee"
		version   int32  = 0x20000000
		bits      uint32 = 0x1a00ac63
		nonce     uint32 = 0x235ed3a0
		timestamp int64  = 1644811503 // 2022-02-14 06:05:03 GMT +2

	)

	btcHeader := wire.NewBTCBlockHeader(
		version,
		newHashFromStr(prevHashStr),
		newHashFromStr(merkleRootStr),
		bits,
		nonce,
	)
	btcHeader.Timestamp = time.Unix(timestamp, 0)
	assert.Equal(t, hashStr, btcHeader.BlockHash().String(), "block hash doesn't match")

	var (
		nextHeight           int32 = 2148321
		fee                  int64 = 8_3000   // satoshi
		bitcoinTestnetReward int64 = 488_2812 // satoshi

	)

	newCoinbaseTx, err := CreateBitcoinCoinbaseTx(
		bitcoinTestnetReward,
		fee,
		nextHeight,
		address,
		beacon.CloneBytes(),
		burnReward,
	)
	assert.NoError(t, err)

	btcHeader.CoinbaseAux.Tx = *newCoinbaseTx.MsgTx()
	updateProofAndRoot(btcHeader)
	return btcHeader
}

func updateProofAndRoot(aux *wire.BTCBlockAux) {
	txHashes[0] = aux.CoinbaseAux.Tx.TxHash()
	aux.MerkleRoot = chainhash.MerkleTreeRoot(txHashes)
	aux.CoinbaseAux.TxMerkleProof = chainhash.BuildCoinbaseMerkleTreeProof(txHashes)
}

func TestValidateBTCCoinbase(t *testing.T) {
	breakCoinbaseProof := func(aux *wire.BTCBlockAux) *wire.BTCBlockAux {
		aux.CoinbaseAux.TxMerkleProof = []chainhash.Hash{aux.CoinbaseAux.Tx.TxHash()}
		return aux
	}

	getTypeATx := func() *wire.BTCBlockAux {
		aux := getBTCAux(t, mainnetAddress, &beaconHashStub, false)
		aux.CoinbaseAux.Tx.TxOut = aux.CoinbaseAux.Tx.TxOut[1:]
		updateProofAndRoot(aux)
		return aux
	}

	tests := []struct {
		name             string
		aux              *wire.BTCBlockAux
		wantRewardBurned bool
		wantErr          bool
	}{
		{
			name: "invalid coinbase aux", wantRewardBurned: false, wantErr: true,
			aux: breakCoinbaseProof(getBTCAux(t, mainnetAddress, &beaconHashStub, false)),
		},
		{
			name: "TYPE_A valid ", wantRewardBurned: false, wantErr: false,
			aux: getTypeATx(),
		},
		{
			name: "TYPE_A with 4 outs ", wantRewardBurned: false, wantErr: false,
			aux: func() *wire.BTCBlockAux {
				aux := getBTCAux(t, mainnetAddress, &beaconHashStub, false)
				aux.CoinbaseAux.Tx.TxOut = aux.CoinbaseAux.Tx.TxOut[1:]
				aux.CoinbaseAux.Tx.AddTxOut(aux.CoinbaseAux.Tx.TxOut[1])
				updateProofAndRoot(aux)
				return aux
			}(),
		},
		{
			name: "TYPE_B valid ", wantRewardBurned: false, wantErr: false,
			aux: getBTCAux(t, mainnetAddress, &beaconHashStub, false),
		},
		{
			name: "TYPE_C valid ", wantRewardBurned: true, wantErr: false,
			aux: getBTCAux(t, mainnetAddress, &beaconHashStub, true),
		},
		{
			name: "TYPE_B invalid block reward ", wantRewardBurned: false, wantErr: true,
			aux: func() *wire.BTCBlockAux {
				aux := getBTCAux(t, mainnetAddress, &beaconHashStub, false)
				aux.CoinbaseAux.Tx.TxOut[1].Value = 6_3000_0000
				updateProofAndRoot(aux)
				return aux
			}(),
		},
		{
			name: "TYPE_B small block reward and huge fee ", wantRewardBurned: false, wantErr: true,
			aux: func() *wire.BTCBlockAux {
				aux := getBTCAux(t, mainnetAddress, &beaconHashStub, false)
				aux.CoinbaseAux.Tx.TxOut[2].Value = 6_3000_0000
				updateProofAndRoot(aux)
				return aux
			}(),
		},
		{
			name: "TYPE_B valid block reward and huge fee", wantRewardBurned: false, wantErr: false,
			aux: func() *wire.BTCBlockAux {
				aux := getBTCAux(t, mainnetAddress, &beaconHashStub, false)
				aux.CoinbaseAux.Tx.TxOut[1].Value = 6_2500_0000
				aux.CoinbaseAux.Tx.TxOut[2].Value = 6_3000_0000
				updateProofAndRoot(aux)
				return aux
			}(),
		},
		{
			name: "TYPE_B valid witness output", wantRewardBurned: false, wantErr: false,
			aux: func() *wire.BTCBlockAux {
				aux := getBTCAux(t, mainnetAddress, &beaconHashStub, false)
				script, _ := txscript.NullDataScript(beaconHashStub.CloneBytes())
				aux.CoinbaseAux.Tx.AddTxOut(wire.NewTxOut(0, script))
				updateProofAndRoot(aux)
				return aux
			}(),
		},
		{
			name: "TYPE_B invalid witness output", wantRewardBurned: false, wantErr: true,
			aux: func() *wire.BTCBlockAux {
				aux := getBTCAux(t, mainnetAddress, &beaconHashStub, false)
				script, _ := txscript.NullDataScript(beaconHashStub.CloneBytes())
				aux.CoinbaseAux.Tx.AddTxOut(wire.NewTxOut(60_0000, script))
				updateProofAndRoot(aux)
				return aux
			}(),
		},
		{
			name: "TYPE_B invalid witness output", wantRewardBurned: false, wantErr: true,
			aux: func() *wire.BTCBlockAux {
				aux := getBTCAux(t, mainnetAddress, &beaconHashStub, false)
				script, _ := txscript.PayToAddrScript(mainnetAddress)
				aux.CoinbaseAux.Tx.AddTxOut(wire.NewTxOut(0, script))
				updateProofAndRoot(aux)
				return aux
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRewardBurned, err := ValidateBTCCoinbase(tt.aux)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBTCCoinbase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotRewardBurned != tt.wantRewardBurned {
				t.Errorf("ValidateBTCCoinbase() gotRewardBurned = %v, want %v", gotRewardBurned, tt.wantRewardBurned)
			}
		})
	}
}

func Test_validateCoinbaseAux(t *testing.T) {
	type args struct {
		merkleRoot chainhash.Hash
		aux        *wire.CoinbaseAux
	}
	validAux := getBTCAux(t, mainnetAddress, &beaconHashStub, false)

	invalidProof := getBTCAux(t, mainnetAddress, &beaconHashStub, false)
	invalidProof.CoinbaseAux.TxMerkleProof = []chainhash.Hash{invalidProof.CoinbaseAux.Tx.TxHash()}

	invalidCoinbase := getBTCAux(t, mainnetAddress, &beaconHashStub, false)
	invalidCoinbase.CoinbaseAux.Tx.TxOut[0].Value = 4424242

	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "valid proof",
			args:    args{merkleRoot: validAux.MerkleRoot, aux: &validAux.CoinbaseAux},
			wantErr: assert.NoError,
		},
		{
			name:    "invalid merkle root",
			args:    args{merkleRoot: beaconHashStub, aux: &validAux.CoinbaseAux},
			wantErr: assert.Error,
		},
		{
			name:    "invalid proof",
			args:    args{merkleRoot: invalidProof.MerkleRoot, aux: &invalidProof.CoinbaseAux},
			wantErr: assert.Error,
		},
		{
			name:    "invalid coinbaseHash",
			args:    args{merkleRoot: invalidCoinbase.MerkleRoot, aux: &invalidCoinbase.CoinbaseAux},
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateCoinbaseAux(tt.args.merkleRoot, tt.args.aux), fmt.Sprintf("validateCoinbaseAux(%v, %v)", tt.args.merkleRoot, tt.args.aux))
		})
	}
}

func Test_checkBtcVanityAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{address: "16pqjKFhkg5GvkzkBA9AyndJFPWBcCMREp", want: false},
		{address: "1JAXmGDsiE2CyK31dYZsMamM18pPebRDAk", want: true},
		{address: "1JAXNETJAXNETJAXNETJAXNETJAXW3bkUN", want: true},
	}
	for _, tt := range tests {
		address, err := jaxutil.DecodeAddress(tt.address, &chaincfg.MainNetParams)
		assert.NoError(t, err)

		script, err := txscript.PayToAddrScript(address)
		assert.NoError(t, err)

		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, checkBtcVanityAddress(script), "checkBtcVanityAddress(%v)", script)
		})
	}

	tests = []struct {
		name    string
		address string
		want    bool
	}{
		{address: "bitcoincash:qzewdwpudm5zpjkv282n3xdzzgwa8pd3ks5kglv0c6", want: false},
		{address: "bitcoincash:qqjaxnetjaxnetjaxnetjaxnetjaxnetju326ted65", want: true},
	}
	for _, tt := range tests {
		address, err := bch.DecodeBCHAddress(tt.address, &chaincfg.MainNetParams)
		assert.NoError(t, err)
		script, err := bch.PayToAddrScript(address)
		assert.NoError(t, err)

		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, bch.JaxVanityPrefix(script), "checkBtcVanityAddress(%v)", script)
		})
	}
}

func Test_validateProofOfInclusion(t *testing.T) {
	hash1 := chainhash.HashH([]byte("beaconExclusiveHash"))
	validScript, _ := BTCCoinbaseScript(34, []byte{0xCA, 0xFE, 0xFF, 0xAA}, hash1.CloneBytes())
	invalidScript, _ := BTCCoinbaseScript(34, []byte{0xCA, 0xFE, 0xFF, 0xAA}, []byte{0xCA, 0xFE, 0xFF, 0xAA})

	hash2, _ := chainhash.NewHash(hexToBytes("f2f5965f0915b3357e5393543b0b624e9a8169e480ea741f8d0037d1332dc8e6"))
	validScript2 := hexToBytes("038a7e0b1362696e616e63652f3839346700010234cc4cbf066a61786e657420f2f5965f0915b3357e5393543b0b624e9a8169e480ea741f8d0037d1332dc8e6066a61786e6574000094e19e520000")

	hash3, _ := chainhash.NewHash(hexToBytes("463ffd0895e326678eb32e793e7edbf7b8e7faa22c3dee24bb3b257f231ee565"))
	validScript3 := hexToBytes("038a7e0b1362696e616e63652f3839346700010136969299066a61786e657420463ffd0895e326678eb32e793e7edbf7b8e7faa22c3dee24bb3b257f231ee565066a61786e65740000952fdb4b7100")

	tests := []struct {
		name                string
		beaconExclusiveHash chainhash.Hash
		signatureScript     []byte
		want                bool
	}{
		{beaconExclusiveHash: hash1, signatureScript: validScript, want: true},
		{beaconExclusiveHash: *hash2, signatureScript: validScript, want: false},
		{beaconExclusiveHash: hash1, signatureScript: invalidScript, want: false},

		{beaconExclusiveHash: *hash2, signatureScript: validScript2, want: true},
		{beaconExclusiveHash: *hash3, signatureScript: validScript3, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want,
				validateProofOfInclusion(tt.beaconExclusiveHash, tt.signatureScript),
				"validateProofOfInclusion(%v, %v)", tt.beaconExclusiveHash, tt.signatureScript)
		})
	}
}
