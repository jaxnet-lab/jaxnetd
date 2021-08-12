// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"gitlab.com/jaxnet/jaxnetd/node/encoder"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

// TestBlockHeader tests the BlockHeader API.
func TestBlockHeader(t *testing.T) {
	nonce64, err := encoder.RandomUint64()
	if err != nil {
		t.Errorf("RandomUint64: Error generating nonce: %v", err)
	}
	nonce := uint32(nonce64)

	hash := mainNetGenesisHash
	merkleHash := mainNetGenesisMerkleRoot
	bits := uint32(0x1d00ffff)
	bh := NewBeaconBlockHeader(1, hash, merkleHash, hash, time.Now(), bits, nonce)

	// Ensure we get the same data back out.
	if bh.PrevBlock() != hash {
		t.Errorf("NewShardBlockHeader: wrong prev hash - got %v, want %v",
			spew.Sprint(bh.PrevBlock), spew.Sprint(hash))
	}
	if bh.MerkleRoot() != merkleHash {
		t.Errorf("NewShardBlockHeader: wrong merkle root - got %v, want %v",
			spew.Sprint(bh.MerkleRoot), spew.Sprint(merkleHash))
	}
	if bh.Bits() != bits {
		t.Errorf("NewShardBlockHeader: wrong bits - got %v, want %v",
			bh.Bits(), bits)
	}
	if bh.Nonce() != nonce {
		t.Errorf("NewShardBlockHeader: wrong nonce - got %v, want %v",
			bh.Nonce(), nonce)
	}
}

// TestBlockHeaderWire tests the BlockHeader wire encode and decode for various
// protocol versions.
// func TestBlockHeaderWire(t *testing.T) {
//	nonce := uint32(123123) // 0x1e0f3
//	pver := uint32(70001)
//
//	// baseBlockHdr is used in the various tests as a baseline BlockHeader.
//	bits := uint32(0x1d00ffff)
//	baseBlockHdr := shard.NewShardBlockHeader(1,
//		mainNetGenesisHash, mainNetGenesisMerkleRoot, chainhash.Hash{},
//		time.Unix(0x495fab29, 0), bits, nonce,
//	)
//
//	// baseBlockHdrEncoded is the wire encoded bytes of baseBlockHdr.
//	baseBlockHdrEncoded := []byte{
//		0x01, 0x00, 0x00, 0x00, // Version 1
//		0x6f, 0xe2, 0x8c, 0x0a, 0xb6, 0xf1, 0xb3, 0x72,
//		0xc1, 0xa6, 0xa2, 0x46, 0xae, 0x63, 0xf7, 0x4f,
//		0x93, 0x1e, 0x83, 0x65, 0xe1, 0x5a, 0x08, 0x9c,
//		0x68, 0xd6, 0x19, 0x00, 0x00, 0x00, 0x00, 0x00, // PrevBlock
//		0x3b, 0xa3, 0xed, 0xfd, 0x7a, 0x7b, 0x12, 0xb2,
//		0x7a, 0xc7, 0x2c, 0x3e, 0x67, 0x76, 0x8f, 0x61,
//		0x7f, 0xc8, 0x1b, 0xc3, 0x88, 0x8a, 0x51, 0x32,
//		0x3a, 0x9f, 0xb8, 0xaa, 0x4b, 0x1e, 0x5e, 0x4a, // MerkleRoot
//		0x29, 0xab, 0x5f, 0x49, // Timestamp
//		0xff, 0xff, 0x00, 0x1d, // Bits
//		0xf3, 0xe0, 0x01, 0x00, // Nonce
//	}
//
//	tests := []struct {
//		in   Chain.BlockHeader // Data to encode
//		out  Chain.BlockHeader // Expected decoded data
//		buf  []byte            // Wire encoding
//		pver uint32            // Protocol version for wire encoding
//		enc  MessageEncoding   // Message encoding variant to use
//	}{
//		// Latest protocol version.
//		{
//			baseBlockHdr,
//			baseBlockHdr,
//			baseBlockHdrEncoded,
//			ProtocolVersion,
//			BaseEncoding,
//		},
//
//		// Protocol version BIP0035Version.
//		{
//			baseBlockHdr,
//			baseBlockHdr,
//			baseBlockHdrEncoded,
//			BIP0035Version,
//			BaseEncoding,
//		},
//
//		// Protocol version BIP0031Version.
//		{
//			baseBlockHdr,
//			baseBlockHdr,
//			baseBlockHdrEncoded,
//			BIP0031Version,
//			BaseEncoding,
//		},
//
//		// Protocol version NetAddressTimeVersion.
//		{
//			baseBlockHdr,
//			baseBlockHdr,
//			baseBlockHdrEncoded,
//			NetAddressTimeVersion,
//			BaseEncoding,
//		},
//
//		// Protocol version MultipleAddressVersion.
//		{
//			baseBlockHdr,
//			baseBlockHdr,
//			baseBlockHdrEncoded,
//			MultipleAddressVersion,
//			BaseEncoding,
//		},
//	}
//
//	t.Logf("Running %d tests", len(tests))
//	for i, test := range tests {
//		// Encode to wire format.
//		var buf bytes.Buffer
//		err := writeBlockHeader(&buf, test.pver, test.in)
//		if err != nil {
//			t.Errorf("writeBlockHeader #%d error %v", i, err)
//			continue
//		}
//		if !bytes.Equal(buf.Bytes(), test.buf) {
//			t.Errorf("writeBlockHeader #%d\n got: %s want: %s", i,
//				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
//			continue
//		}
//
//		buf.Reset()
//		err = test.in.BtcEncode(&buf, pver, 0)
//		if err != nil {
//			t.Errorf("BtcEncode #%d error %v", i, err)
//			continue
//		}
//		if !bytes.Equal(buf.Bytes(), test.buf) {
//			t.Errorf("BtcEncode #%d\n got: %s want: %s", i,
//				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
//			continue
//		}
//
//		// Decode the block header from wire format.
//		var bh BlockHeader
//		rbuf := bytes.NewReader(test.buf)
//		err = readBlockHeader(rbuf, test.pver, &bh)
//		if err != nil {
//			t.Errorf("readBlockHeader #%d error %v", i, err)
//			continue
//		}
//		if !reflect.DeepEqual(&bh, test.out) {
//			t.Errorf("readBlockHeader #%d\n got: %s want: %s", i,
//				spew.Sdump(&bh), spew.Sdump(test.out))
//			continue
//		}
//
//		rbuf = bytes.NewReader(test.buf)
//		err = bh.BtcDecode(rbuf, pver, test.enc)
//		if err != nil {
//			t.Errorf("BtcDecode #%d error %v", i, err)
//			continue
//		}
//		if !reflect.DeepEqual(&bh, test.out) {
//			t.Errorf("BtcDecode #%d\n got: %s want: %s", i,
//				spew.Sdump(&bh), spew.Sdump(test.out))
//			continue
//		}
//	}
// }

// TestBlockHeaderSerialize tests BlockHeader serialize and deserialize.
func TestBeaconBlockHeaderSerialize(t *testing.T) {
	nonce := uint32(123123) // 0x1e0f3

	// baseBlockHdr is used in the various tests as a baseline BlockHeader.
	bits := uint32(0x1d00ffff)
	baseBlockHdr := &BeaconHeader{
		version:         1,
		prevBlock:       mainNetGenesisHash,
		merkleRoot:      mainNetGenesisMerkleRoot,
		mergeMiningRoot: chainhash.Hash{},
		timestamp:       time.Unix(0x495fab29, 0), // 2009-01-03 12:15:05 -0600 CST
		bits:            bits,
		shards:          0,
		treeEncoding:    nil,
		btcAux: BTCBlockAux{
			Version:    0,
			PrevBlock:  chainhash.Hash{},
			MerkleRoot: chainhash.Hash{},
			Timestamp:  time.Time{},
			Bits:       0,
			Nonce:      nonce,
			CoinbaseAux: CoinbaseAux{
				Tx:       MsgTx{},
				TxMerkle: nil,
			},
		},
	}

	// baseBlockHdrEncoded is the wire encoded bytes of baseBlockHdr.
	baseBlockHdrEncoded := []byte{
		0x01, 0x00, 0x00, 0x00, // Version 1

		0x6f, 0xe2, 0x8c, 0x0a, 0xb6, 0xf1, 0xb3, 0x72,
		0xc1, 0xa6, 0xa2, 0x46, 0xae, 0x63, 0xf7, 0x4f,
		0x93, 0x1e, 0x83, 0x65, 0xe1, 0x5a, 0x08, 0x9c,
		0x68, 0xd6, 0x19, 0x00, 0x00, 0x00, 0x00, 0x00, // PrevBlock

		0x3b, 0xa3, 0xed, 0xfd, 0x7a, 0x7b, 0x12, 0xb2,
		0x7a, 0xc7, 0x2c, 0x3e, 0x67, 0x76, 0x8f, 0x61,
		0x7f, 0xc8, 0x1b, 0xc3, 0x88, 0x8a, 0x51, 0x32,
		0x3a, 0x9f, 0xb8, 0xaa, 0x4b, 0x1e, 0x5e, 0x4a, // MerkleRoot

		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // MerkleRoot

		0x29, 0xab, 0x5f, 0x49, // Timestamp
		0xff, 0xff, 0x00, 0x1d, // Bits
		0xf3, 0xe0, 0x01, 0x00, // Nonce
		0x00, 0x00, 0x00, 0x00, // shards
	}

	tests := []struct {
		in  BeaconHeader // Data to encode
		out BeaconHeader // Expected decoded data
		buf []byte       // Serialized data
	}{
		{
			*baseBlockHdr,
			*baseBlockHdr,
			baseBlockHdrEncoded,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Serialize the block header.
		var buf bytes.Buffer
		err := test.in.Serialize(&buf)
		if err != nil {
			t.Errorf("Serialize #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("Serialize #%d\n got: %s want: %s", i,
				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
			continue
		}

		// Deserialize the block header.
		var bh = BeaconHeader{}
		rbuf := bytes.NewReader(test.buf)
		err = bh.Read(rbuf)
		if err != nil {
			t.Errorf("Deserialize #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(bh, test.out) {
			t.Errorf("Deserialize #%d\n got: %s want: %s", i,
				spew.Sdump(&bh), spew.Sdump(test.out))
			continue
		}
	}
}

// TestBlockHeaderSerialize tests BlockHeader serialize and deserialize.
func TestShardBlockHeaderSerialize(t *testing.T) {
	nonce := uint32(123123) // 0x1e0f3

	// baseBlockHdr is used in the various tests as a baseline BlockHeader.
	bits := uint32(0x1d00ffff)
	baseBlockHdr := ShardHeader{
		prevBlock:         mainNetGenesisHash,
		merkleRoot:        mainNetGenesisMerkleRoot,
		timestamp:         time.Unix(0x495fab29, 0), // 2009-01-03 12:15:05 -0600 CST
		bits:              bits,
		mergeMiningNumber: 0,
		bCHeader: BeaconHeader{
			version:         1,
			prevBlock:       mainNetGenesisHash,
			merkleRoot:      mainNetGenesisMerkleRoot,
			mergeMiningRoot: chainhash.Hash{},
			timestamp:       time.Unix(0x495fab29, 0), // 2009-01-03 12:15:05 -0600 CST
			bits:            bits,
			shards:          0,
			treeEncoding:    nil,
			btcAux: BTCBlockAux{
				Version:    0,
				PrevBlock:  chainhash.Hash{},
				MerkleRoot: chainhash.Hash{},
				Timestamp:  time.Time{},
				Bits:       0,
				Nonce:      nonce,
				CoinbaseAux: CoinbaseAux{
					Tx:       MsgTx{},
					TxMerkle: nil,
				},
			},
		},
	}

	// baseBlockHdrEncoded is the wire encoded bytes of baseBlockHdr.
	baseBlockHdrEncoded := []byte{
		0x6f, 0xe2, 0x8c, 0x0a, 0xb6, 0xf1, 0xb3, 0x72,
		0xc1, 0xa6, 0xa2, 0x46, 0xae, 0x63, 0xf7, 0x4f,
		0x93, 0x1e, 0x83, 0x65, 0xe1, 0x5a, 0x08, 0x9c,
		0x68, 0xd6, 0x19, 0x00, 0x00, 0x00, 0x00, 0x00, // PrevBlock

		0x3b, 0xa3, 0xed, 0xfd, 0x7a, 0x7b, 0x12, 0xb2,
		0x7a, 0xc7, 0x2c, 0x3e, 0x67, 0x76, 0x8f, 0x61,
		0x7f, 0xc8, 0x1b, 0xc3, 0x88, 0x8a, 0x51, 0x32,
		0x3a, 0x9f, 0xb8, 0xaa, 0x4b, 0x1e, 0x5e, 0x4a, // MerkleRoot

		0x29, 0xab, 0x5f, 0x49, // Timestamp
		0xff, 0xff, 0x00, 0x1d, // Bits
		0x00, 0x00, 0x00, 0x00, // mergeMiningNumber

		// bCHeader
		0x01, 0x00, 0x00, 0x00, // Version 1
		0x6f, 0xe2, 0x8c, 0x0a, 0xb6, 0xf1, 0xb3, 0x72,
		0xc1, 0xa6, 0xa2, 0x46, 0xae, 0x63, 0xf7, 0x4f,
		0x93, 0x1e, 0x83, 0x65, 0xe1, 0x5a, 0x08, 0x9c,
		0x68, 0xd6, 0x19, 0x00, 0x00, 0x00, 0x00, 0x00, // PrevBlock

		0x3b, 0xa3, 0xed, 0xfd, 0x7a, 0x7b, 0x12, 0xb2,
		0x7a, 0xc7, 0x2c, 0x3e, 0x67, 0x76, 0x8f, 0x61,
		0x7f, 0xc8, 0x1b, 0xc3, 0x88, 0x8a, 0x51, 0x32,
		0x3a, 0x9f, 0xb8, 0xaa, 0x4b, 0x1e, 0x5e, 0x4a, // MerkleRoot

		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // MerkleRoot

		0x29, 0xab, 0x5f, 0x49, // Timestamp
		0xff, 0xff, 0x00, 0x1d, // Bits
		0xf3, 0xe0, 0x01, 0x00, // Nonce
		0x00, 0x00, 0x00, 0x00,
	}

	tests := []struct {
		in  ShardHeader // Data to encode
		out ShardHeader // Expected decoded data
		buf []byte      // Serialized data
	}{
		{
			baseBlockHdr,
			baseBlockHdr,
			baseBlockHdrEncoded,
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Serialize the block header.
		var buf bytes.Buffer
		err := test.in.Serialize(&buf)
		if err != nil {
			t.Errorf("Serialize #%d error %v", i, err)
			continue
		}
		if !bytes.Equal(buf.Bytes(), test.buf) {
			t.Errorf("Serialize #%d\n got: %s want: %s", i,
				spew.Sdump(buf.Bytes()), spew.Sdump(test.buf))
			continue
		}

		// Deserialize the block header.
		var bh = ShardHeader{}
		rbuf := bytes.NewReader(test.buf)
		err = bh.Read(rbuf)
		if err != nil {
			t.Errorf("Deserialize #%d error %v", i, err)
			continue
		}
		if !reflect.DeepEqual(bh, test.out) {
			t.Errorf("Deserialize #%d\n got: %s want: %s", i,
				spew.Sdump(&bh), spew.Sdump(test.out))
			continue
		}
	}
}