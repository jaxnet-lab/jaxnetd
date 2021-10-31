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
			Bits:      mainNetPowLimitBitsBeacon, // 0x1a040000 [0000000000000400000000000000000000000000000000000000000000000000]
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
			pow.CalcWork(opts.Bits),
			opts.Nonce,
		),
		Transactions: []*wire.MsgTx{state.genesisTx},
	}
	state.genesisBlock.Header.BeaconHeader().SetBTCAux(getBtcAux())
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
			pow.CalcWork(bits),
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
	case wire.FastTestNet, wire.SimNet:
		txHex = fastNetGenesisTxHex
	default:
		txHex = mainnetGenesisTxHex
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

func getBtcAux() wire.BTCBlockAux {
	rawData, err := hex.DecodeString(btcAux)
	if err != nil {
		panic("invalid genesis btc aux hex-data")
	}
	aux := wire.BTCBlockAux{}
	err = aux.Deserialize(bytes.NewBuffer(rawData))
	if err != nil {
		panic("invalid genesis btc aux hex-data")
	}

	return aux
}

const (
	fastNetGenesisTxHex = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff44202bf70f9cbe34cf53add3ed8c96b115e64c911de96007040000000000000000001d4a61782e4e6574776f726b20656e7465727320746865207261636521200473686562ffffffff0300f2052a010000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00f2052a010000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00f2052a010000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00000000"
	shardsGenesisTxHex  = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff170005ffffffff00000e2f503253482f6a61786e6574642fffffffff03000000000000000014bc473af4c71c45d5aa3278adc99701ded3740a54000000000000000014bc473af4c71c45d5aa3278adc99701ded3740a5400000000000000001976a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac00000000"
	mainnetGenesisTxHex = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff4220a09979e5088192d4ece5f7c49c444845a20d7bcc77cd010000000000000000001d4a61782e4e6574776f726b20656e746572732074686520726163652120027368ffffffff11006c042b028000001976a91437c052ab059140534d374a22ebd987c98b1f936f88ac006c042b028000001976a9142c5f6d1901b883fa0c8d6b801e9852f61d45898888ac006c042b028000001976a9149133bffc55515814e9ec162a6d57d9da540e0f6e88ac006c042b028000001976a91461c0dd65c120ebb073f15083c2df442e5ce0800688ac0034e28f867701001976a91491acc017b2d1a4e39f89cbb4c1fc8df6fb7d3add88ac0034e28f867701001976a914b5e8d343dec3dcbbfc55d75e4bd2b6086a1e85b688ac0080f420e6b500001976a9141ad411e604711225347dc9d494a5f5daa45a11a488ac000408f8fc4e00001976a914663b8725e17017269d258fe221fd629c97a5e39b88ac0036bcfc141f00001976a914d3c2ab8fd0a18c9da7bf3b324bdea419aeb4682088ac0080f420e6b500001976a914b32783bac902e0232da52d87781119a6bdb9231388ac00f0d55c299f00001976a9148857c122b5eb1dd56d4c5aef89fd83519af1ba3e88ac00c4093f5a4d00001976a914f315c973f7d50285e59067057530cff363611b9188ac00c4093f5a4d00001976a914fedb7732fd605fe976e9a9a72faeadc934cbfad788ac0080c6a47e8d03001976a914dfd7bbc925f22c9cf9248baa4877a240d0eb61d488ac00f0d55c299f00001976a9144b75edd9b6dc4bae8ac04a47d7b93508f9cfbfbd88ac00c4093f5a4d00001976a914d0908774ef842ca8620dfe71f0c6695e78b666e188ac00c4093f5a4d00001976a91404e9f23f9eb72a6b23ea881d9a4706bb7db38f1788ac00000000"
	btcAux              = "04600020f76b6a887c89fdea44bd89a77006645ed6bb2701862b00000000000000000000e94c9281dc6310592352059d264aeb47e81dc0475717a6204a9c7f8099fd292bbcab7d6108040e17fb2a86e401000000010000000000000000000000000000000000000000000000000000000000000000ffffffff5c0391cb0a192f5669614254432f4d696e6564206279206f3273706e65772f2cfabe6d6d1052ee457e675a7e7b554f4b1b8a4cbb9ee852c7ccb323fded51065a8fde4506100000000000000010be75a00f1f305410e2b49d65c11c0c00ffffffff045c067125000000001976a914536ffa992491508dca0354e52f32a3a7a679a53a88ac00000000000000002b6a2952534b424c4f434b3a7067a246e981a8500e3cc22276a3d0e520bbfabe559d66eaadcdab2e003a22590000000000000000266a24b9e11b6dff982ec0c3c11f96cdc25f874e3f44b98a79f1f0dc6de65b8a479893bd9c3a380000000000000000266a24aa21a9ede8db996cdca0d8f8ddc8c1d54e318c8262c28c0110bd6483bbc23243d2d1ca66000000000c78883bde8a25eecf9aa7129bcdfb43eebbf647fc58cf74af306f2cbaa042f094b1e126b731d7ccfa626b41bafc6bd5c8ec4177435807781aa3fe2352dea399944eb94a03cd16f7c0fdf60431caca48fa13ca1db40a6de9d58334b47342fad05201229d37df1dfd998a7da8c6ce2881a377f92077e822ee48fb7ca23ee6e2f5a044692ffeb0156c29d1ecb92ddbb68b32562ea1bcebebeae8eb7ce385c174c4aba8b54a37e24cf494f73d86870b2606b0bb3cd5f0b5a20e566d2a1e28ed4ab6cd4e1b08258deb00142c2a0cd07c1cddb4ff9674585f4a7314671bdfa518bfceada8ba1c6ed99430936314837417c46b74bf3d8567e36512347668b75fc8986768e15300b77982e4b55b1c415f8bc56941dbf33d58db93bc406fe469a27ac66e10fead34a89f81e22560ff611468daf26eb82bb931afbf3606993e790d085f99edfa0ca2318b508e22858590f823ef231ecc25d1072264e2663d1b464011584876666865049313b3f07436e12c0ad636dc8d6541cf817fefd339ead7185493d0dc"
)
