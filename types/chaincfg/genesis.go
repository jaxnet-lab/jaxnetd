/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaincfg

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

var (
	genesisBlock      *wire.MsgBlock
	genesisHash       *chainhash.Hash
	genesisMerkleRoot *chainhash.Hash

	shardsGenesisTx         *wire.MsgTx
	shardsGenesisMerkleRoot *chainhash.Hash
	shardGenesisBlocks      = map[uint32]wire.MsgBlock{}
	shardGenesisHash        = map[uint32]chainhash.Hash{}
)

func cleanState() {
	genesisBlock = nil
	genesisHash = nil
	genesisMerkleRoot = nil
	shardsGenesisTx = nil
	shardsGenesisMerkleRoot = nil
	shardGenesisBlocks = map[uint32]wire.MsgBlock{}
	shardGenesisHash = map[uint32]chainhash.Hash{}
}

func GenesisMerkleRoot() chainhash.Hash {
	if genesisMerkleRoot != nil {
		return *genesisMerkleRoot
	}

	GenesisCoinbaseTx()

	genesisMerkleRoot = new(chainhash.Hash)
	*genesisMerkleRoot = genesisCoinbaseTx.TxHash()
	return *genesisMerkleRoot
}

func BeaconGenesisHash(name types.JaxNet) *chainhash.Hash {
	if genesisHash != nil {
		return genesisHash
	}

	BeaconGenesisBlock(name)
	return genesisHash
}

func BeaconGenesisBlock(name types.JaxNet) *wire.MsgBlock {
	if genesisBlock != nil {
		return genesisBlock
	}
	fmt.Println(name.String())

	var opts GenesisBlockOpts
	switch name {
	case types.TestNet:
		opts = GenesisBlockOpts{
			Version:    1,
			PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
			MerkleRoot: GenesisMerkleRoot(),      // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
			Timestamp:  time.Unix(1633687865, 0), // Fri  8 Oct 10:11:52 UTC 2021
			Bits:       0x1d0ffff0,               // 487587824 [0000000ffff00000000000000000000000000000000000000000000000000000]
			Nonce:      0x18aea41a,               // 414098458
		}

	case types.FastTestNet:
		opts = GenesisBlockOpts{
			Version:    1,
			PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
			MerkleRoot: GenesisMerkleRoot(),      // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
			Timestamp:  time.Unix(1633687865, 0), // Fri  8 Oct 10:11:52 UTC 2021
			Bits:       0x1e0fffff,               // 486604799 [00000000ffff0000000000000000000000000000000000000000000000000000]
			Nonce:      0x18aea41a,
		}
	case types.SimNet:
		opts = GenesisBlockOpts{
			Version:    1,
			PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
			MerkleRoot: GenesisMerkleRoot(),      // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
			Timestamp:  time.Unix(1633687865, 0), // Fri  8 Oct 10:11:52 UTC 2021
			Bits:       0x207fffff,               // 545259519 [7fffff0000000000000000000000000000000000000000000000000000000000]
			Nonce:      2,
		}
	default:
		opts = GenesisBlockOpts{
			Version:    1,
			PrevBlock:  chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
			MerkleRoot: GenesisMerkleRoot(),      // 4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b
			Timestamp:  time.Unix(1633687865, 0), // Fri  8 Oct 10:11:52 UTC 2021
			Bits:       0x1d0ffff0,               // 487587824 [0000000ffff00000000000000000000000000000000000000000000000000000]
			Nonce:      0x7c2bac1d,               // 2083236893
		}
	}

	GenesisCoinbaseTx()

	genesisBlock = &wire.MsgBlock{
		Header: wire.NewBeaconBlockHeader(
			wire.NewBVersion(opts.Version),
			opts.PrevBlock,
			opts.MerkleRoot,
			chainhash.Hash{},
			opts.Timestamp,
			opts.Bits,
			opts.Nonce,
		),
		Transactions: []*wire.MsgTx{genesisCoinbaseTx},
	}
	genesisBlock.Header.SetK(pow.PackK(pow.K1))
	genesisBlock.Header.SetVoteK(pow.PackK(pow.K1))
	genesisHash = new(chainhash.Hash)
	*genesisHash = genesisBlock.BlockHash()

	return genesisBlock

}

func ShardGenesisHash(shardID uint32) *chainhash.Hash {
	hash := shardGenesisHash[shardID]
	return &hash
}

func ShardGenesisBlock(shardID uint32) *wire.MsgBlock {
	shardBlock := shardGenesisBlocks[shardID]
	return &shardBlock
}

func SetShardGenesisBlock(shardID uint32, beaconBlock *wire.MsgBlock) *wire.MsgBlock {
	shardBlock, ok := shardGenesisBlocks[shardID]
	if ok {
		return &shardBlock
	}

	ShardGenesisCoinbaseTx()

	coinbaseAux := wire.CoinbaseAux{}.FromBlock(beaconBlock)
	shardBlock = wire.MsgBlock{
		ShardBlock: true,
		Header: wire.NewShardBlockHeader(
			chainhash.Hash{},
			*shardsGenesisMerkleRoot,
			ShardPoWBits,
			*beaconBlock.Header.BeaconHeader(),
			coinbaseAux,
		),
		Transactions: []*wire.MsgTx{shardsGenesisTx},
	}

	shardGenesisBlocks[shardID] = shardBlock
	shardGenesisHash[shardID] = shardBlock.BlockHash()

	return &shardBlock
}

// GenesisCoinbaseTx is the coinbase transaction for the genesis blocks for
// the main network, regression test network, and test network (version 3).
func GenesisCoinbaseTx() wire.MsgTx {
	if genesisCoinbaseTx != nil {
		return *genesisCoinbaseTx
	}
	genesisCoinbaseTx = new(wire.MsgTx)

	rawTx, err := hex.DecodeString(genesisTxHex)
	if err != nil {
		panic("invalid genesis tx hex-data")
	}
	err = genesisCoinbaseTx.Deserialize(bytes.NewBuffer(rawTx))
	if err != nil {
		panic("invalid genesis tx data")
	}

	return *genesisCoinbaseTx
}

func ShardGenesisCoinbaseTx() wire.MsgTx {
	if shardsGenesisTx != nil {
		return *shardsGenesisTx
	}
	shardsGenesisTx = new(wire.MsgTx)

	rawTx, err := hex.DecodeString(shardsGenesisTxHex)
	if err != nil {
		panic("invalid genesis tx hex-data")
	}
	err = shardsGenesisTx.Deserialize(bytes.NewBuffer(rawTx))
	if err != nil {
		panic("invalid genesis tx data")
	}

	shardsGenesisMerkleRoot = new(chainhash.Hash)
	*shardsGenesisMerkleRoot = shardsGenesisTx.TxHash()
	return *shardsGenesisTx
}

// GenesisCoinbaseTx is the coinbase transaction for the genesis blocks for
// the main network, regression test network, and test network (version 3).
var genesisCoinbaseTx = &wire.MsgTx{
	Version: 1,
	TxIn: []*wire.TxIn{
		{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{
				0x04, 0xff, 0xff, 0x00, 0x1d, 0x01, 0x04, 0x45, /* |.......E| */
				0x54, 0x68, 0x65, 0x20, 0x54, 0x69, 0x6d, 0x65, /* |The Time| */
				0x73, 0x20, 0x30, 0x33, 0x2f, 0x4a, 0x61, 0x6e, /* |s 03/Jan| */
				0x2f, 0x32, 0x30, 0x30, 0x39, 0x20, 0x43, 0x68, /* |/2009 Ch| */
				0x61, 0x6e, 0x63, 0x65, 0x6c, 0x6c, 0x6f, 0x72, /* |ancellor| */
				0x20, 0x6f, 0x6e, 0x20, 0x62, 0x72, 0x69, 0x6e, /* | on brin| */
				0x6b, 0x20, 0x6f, 0x66, 0x20, 0x73, 0x65, 0x63, /* |k of sec|*/
				0x6f, 0x6e, 0x64, 0x20, 0x62, 0x61, 0x69, 0x6c, /* |ond bail| */
				0x6f, 0x75, 0x74, 0x20, 0x66, 0x6f, 0x72, 0x20, /* |out for |*/
				0x62, 0x61, 0x6e, 0x6b, 0x73, /* |banks| */
				// 3be9e384f5472ff2ca389c36309c657a39312c6fe89976902834ee2ec45a36f5
				0xf5, 0x36, 0x5a, 0xc4, 0x2e, 0xee, 0x34, 0x28,
				0x90, 0x76, 0x99, 0xe8, 0x6f, 0x2c, 0x31, 0x39,
				0x7a, 0x65, 0x9c, 0x30, 0x36, 0x9c, 0x38, 0xca,
				0xf2, 0x2f, 0x47, 0xf5, 0x84, 0xe3, 0xe9, 0x3b,
				// a715b1dba7700fd7d6976782ab75ec2a25e1c226749e849bea0d1fc0858ceb04
				0x04, 0xeb, 0x8c, 0x85, 0xc0, 0x1f, 0x0d, 0xea,
				0x9b, 0x84, 0x9e, 0x74, 0x26, 0xc2, 0xe1, 0x25,
				0x2a, 0xec, 0x75, 0xab, 0x82, 0x67, 0x97, 0xd6,
				0xd7, 0x0f, 0x70, 0xa7, 0xdb, 0xb1, 0x15, 0xa7,
			},
			Sequence: 0xffffffff,
		},
	},
	TxOut: []*wire.TxOut{
		{
			Value: 0x12a05f200,
			PkScript: []byte{
				0x41, 0x04, 0x67, 0x8a, 0xfd, 0xb0, 0xfe, 0x55, /* |A.g....U| */
				0x48, 0x27, 0x19, 0x67, 0xf1, 0xa6, 0x71, 0x30, /* |H'.g..q0| */
				0xb7, 0x10, 0x5c, 0xd6, 0xa8, 0x28, 0xe0, 0x39, /* |..\..(.9| */
				0x09, 0xa6, 0x79, 0x62, 0xe0, 0xea, 0x1f, 0x61, /* |..yb...a| */
				0xde, 0xb6, 0x49, 0xf6, 0xbc, 0x3f, 0x4c, 0xef, /* |..I..?L.| */
				0x38, 0xc4, 0xf3, 0x55, 0x04, 0xe5, 0x1e, 0xc1, /* |8..U....| */
				0x12, 0xde, 0x5c, 0x38, 0x4d, 0xf7, 0xba, 0x0b, /* |..\8M...| */
				0x8d, 0x57, 0x8a, 0x4c, 0x70, 0x2b, 0x6b, 0xf1, /* |.W.Lp+k.| */
				0x1d, 0x5f, 0xac, /* |._.| */
			},
		},
	},
	LockTime: 0,
}

const (
	// todo
	genesisTxHex       = ""
	shardsGenesisTxHex = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff170005ffffffff00000e2f503253482f6a61786e6574642fffffffff03000000000000000014bc473af4c71c45d5aa3278adc99701ded3740a54000000000000000014bc473af4c71c45d5aa3278adc99701ded3740a5400000000000000001976a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac00000000"
)
