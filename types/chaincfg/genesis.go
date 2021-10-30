/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaincfg

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"sync"
	"time"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type genesisDataState struct {
	genesisBlock      *wire.MsgBlock
	genesisHash       *chainhash.Hash
	genesisMerkleRoot *chainhash.Hash
	genesisTx         *wire.MsgTx

	shardsGenesisTx         map[uint32]wire.MsgTx
	shardsGenesisMerkleRoot map[uint32]chainhash.Hash
	shardGenesisBlocks      map[uint32]wire.MsgBlock
	shardGenesisHash        map[uint32]chainhash.Hash
}

func newGenesisDataState() *genesisDataState {
	return &genesisDataState{
		genesisBlock:            nil,
		genesisHash:             nil,
		genesisMerkleRoot:       nil,
		shardsGenesisTx:         map[uint32]wire.MsgTx{},
		shardsGenesisMerkleRoot: map[uint32]chainhash.Hash{},
		shardGenesisBlocks:      map[uint32]wire.MsgBlock{},
		shardGenesisHash:        map[uint32]chainhash.Hash{},
	}
}

var (
	stateLock      sync.RWMutex
	genesisStorage = map[wire.JaxNet]*genesisDataState{
		wire.MainNet:     newGenesisDataState(),
		wire.TestNet:     newGenesisDataState(),
		wire.FastTestNet: newGenesisDataState(),
		wire.SimNet:      newGenesisDataState(),
	}
)

func cleanState() {
	stateLock.Lock()
	for net := range genesisStorage {
		genesisStorage[net] = newGenesisDataState()
	}
	stateLock.Unlock()
}

func genesisMerkleRoot(name wire.JaxNet) chainhash.Hash {
	state := genesisStorage[name]

	if state.genesisMerkleRoot != nil {
		return *state.genesisMerkleRoot
	}

	genesisCoinbaseTx(name)

	state.genesisMerkleRoot = new(chainhash.Hash)
	txHash := state.genesisTx.TxHash()
	state.genesisMerkleRoot = &txHash
	return txHash
}

func beaconGenesisHash(name wire.JaxNet) *chainhash.Hash {
	state := genesisStorage[name]

	if state.genesisHash != nil {
		return state.genesisHash
	}

	beaconGenesisBlock(name)
	return state.genesisHash
}

func beaconGenesisBlock(name wire.JaxNet) *wire.MsgBlock {
	stateLock.Lock()
	defer stateLock.Unlock()
	state := genesisStorage[name]

	if state.genesisBlock != nil {
		return state.genesisBlock
	}

	var opts GenesisBlockOpts
	switch name {
	case wire.TestNet:
		opts = GenesisBlockOpts{
			Version:   1,
			PrevBlock: chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
			Timestamp: time.Unix(1633687865, 0), // Fri  8 Oct 10:11:52 UTC 2021
			Bits:      testNetPowLimitBitsBeacon,
			Nonce:     0x18aea41a, // 414098458
		}

	case wire.FastTestNet:
		opts = GenesisBlockOpts{
			Version:   1,
			PrevBlock: chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
			Timestamp: time.Unix(1633687865, 0), // Fri  8 Oct 10:11:52 UTC 2021
			Bits:      fastnetBeaconPoWBits,
			Nonce:     0x18aea41a,
		}
	case wire.SimNet:
		opts = GenesisBlockOpts{
			Version:   1,
			PrevBlock: chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
			Timestamp: time.Unix(1633687865, 0), // Fri  8 Oct 10:11:52 UTC 2021
			Bits:      0x207fffff,               // 545259519 [7fffff0000000000000000000000000000000000000000000000000000000000]
			Nonce:     2,
		}
	default:
		opts = GenesisBlockOpts{
			Version:   1,
			PrevBlock: chainhash.Hash{},          // 0000000000000000000000000000000000000000000000000000000000000000
			Timestamp: time.Unix(1633687865, 0),  // Fri  8 Oct 10:11:52 UTC 2021
			Bits:      mainNetPowLimitBitsBeacon, // 487587824 [0000000ffff00000000000000000000000000000000000000000000000000000]
			Nonce:     0x7c2bac1d,                // 2083236893
		}
	}

	genesisCoinbaseTx(name)

	state.genesisBlock = &wire.MsgBlock{
		Header: wire.NewBeaconBlockHeader(
			wire.NewBVersion(opts.Version),
			0,
			opts.PrevBlock,
			opts.PrevBlock,
			genesisMerkleRoot(name),
			chainhash.Hash{},
			opts.Timestamp,
			opts.Bits,
			1,
			opts.Nonce,
		),
		Transactions: []*wire.MsgTx{state.genesisTx},
	}
	state.genesisBlock.Header.SetK(pow.PackK(pow.K1))
	state.genesisBlock.Header.SetVoteK(pow.PackK(pow.K1))
	state.genesisHash = new(chainhash.Hash)
	*state.genesisHash = state.genesisBlock.BlockHash()

	return state.genesisBlock

}

func shardGenesisHash(name wire.JaxNet, shardID uint32) *chainhash.Hash {
	state := genesisStorage[name]

	hash := state.shardGenesisHash[shardID]
	return &hash
}

func shardGenesisBlock(name wire.JaxNet, shardID uint32) *wire.MsgBlock {
	stateLock.RLock()
	defer stateLock.RUnlock()
	state := genesisStorage[name]
	shardBlock := state.shardGenesisBlocks[shardID]
	return &shardBlock
}

func setShardGenesisBlock(name wire.JaxNet, shardID uint32, beaconBlock *wire.MsgBlock) *wire.MsgBlock {
	stateLock.Lock()
	defer stateLock.Unlock()

	state := genesisStorage[name]

	shardBlock, ok := state.shardGenesisBlocks[shardID]
	if ok {
		return &shardBlock
	}

	shardGenesisCoinbaseTx(name, shardID)

	var bits uint32
	switch name {
	case wire.TestNet:
		bits = testNetPowLimitBitsShard
	case wire.MainNet:
		bits = mainNetPowLimitBitsShard
	default:
		bits = fastnetShardPoWBits
	}

	gtx := state.shardsGenesisTx[shardID]
	coinbaseAux := wire.CoinbaseAux{}.FromBlock(beaconBlock, false)
	shardBlock = wire.MsgBlock{
		Header: wire.NewShardBlockHeader(
			0,
			chainhash.Hash{},
			chainhash.Hash{},
			state.shardsGenesisMerkleRoot[shardID],
			bits,
			1,
			*beaconBlock.Header.BeaconHeader(),
			coinbaseAux,
		),
		Transactions: []*wire.MsgTx{&gtx},
	}

	state.shardGenesisBlocks[shardID] = *shardBlock.Copy()
	state.shardGenesisHash[shardID] = shardBlock.BlockHash()

	return &shardBlock
}

// genesisCoinbaseTx is the coinbase transaction for the genesis blocks for
// the main network, regression test network, and test network (version 3).
func genesisCoinbaseTx(name wire.JaxNet) wire.MsgTx {
	state := genesisStorage[name]

	if state.genesisTx != nil {
		return *state.genesisTx
	}

	state.genesisTx = new(wire.MsgTx)

	txHex := ""
	switch name {
	case wire.FastTestNet:
		txHex = fastNetGenesisTxHex
	default:
		state.genesisTx = mainNetgenesisCoinbaseTx
		return *state.genesisTx
	}

	rawTx, err := hex.DecodeString(txHex)
	if err != nil {
		panic("invalid genesis tx hex-data")
	}
	err = state.genesisTx.Deserialize(bytes.NewBuffer(rawTx))
	if err != nil {
		panic("invalid genesis tx data")
	}

	return *state.genesisTx
}

func shardGenesisCoinbaseTx(name wire.JaxNet, shardID uint32) wire.MsgTx {
	state := genesisStorage[name]
	tx, ok := state.shardsGenesisTx[shardID]
	if ok {
		return tx
	}
	tx = wire.MsgTx{}

	rawTx, err := hex.DecodeString(shardsGenesisTxHex)
	if err != nil {
		panic("invalid genesis tx hex-data")
	}

	err = tx.Deserialize(bytes.NewBuffer(rawTx))
	if err != nil {
		panic("invalid genesis tx data")
	}

	sid := make([]byte, 4)
	binary.LittleEndian.PutUint32(sid, shardID)
	script := []byte{
		0x0b,
		0x73, 0x68, 0x61, 0x72, 0x64, 0x5f, 0x63,
		0x68, 0x61, 0x69, 0x6e, // ASCII: shard_chain

		0x04,
		sid[0], sid[1], sid[2], sid[3],

		0x11,
		0x2f, 0x67, 0x65, 0x6e, 0x65, 0x73, 0x69, 0x73,
		0x2f, 0x6a, 0x61, 0x78, 0x6e, 0x65, 0x74, 0x64,
		0x2f, // ASCII: /genesis/jaxnetd/
	}

	// todo put shardID into genesis tx script signature
	tx.TxIn[0].SignatureScript = script

	state.shardsGenesisMerkleRoot[shardID] = tx.TxHash()
	state.shardsGenesisTx[shardID] = tx
	return tx
}

// GenesisCoinbaseTx is the coinbase transaction for the genesis blocks for
// the main network, regression test network, and test network (version 3).
var mainNetgenesisCoinbaseTx = &wire.MsgTx{
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
	fastNetGenesisTxHex = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff44202bf70f9cbe34cf53add3ed8c96b115e64c911de96007040000000000000000001d4a61782e4e6574776f726b20656e7465727320746865207261636521200473686562ffffffff0300f2052a010000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00f2052a010000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00f2052a010000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00000000"
	shardsGenesisTxHex  = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff170005ffffffff00000e2f503253482f6a61786e6574642fffffffff03000000000000000014bc473af4c71c45d5aa3278adc99701ded3740a54000000000000000014bc473af4c71c45d5aa3278adc99701ded3740a5400000000000000001976a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac00000000"
)
