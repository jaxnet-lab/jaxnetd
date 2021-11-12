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
	"math/big"
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

	var aux wire.BTCBlockAux
	var opts GenesisBlockOpts
	switch name {
	case wire.TestNet:
		aux = getBtcAux(false)
		opts = GenesisBlockOpts{
			Version:   1,
			PrevBlock: chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
			Timestamp: time.Unix(1633687865, 0), // Fri  8 Oct 10:11:52 UTC 2021
			Bits:      testNetPowLimitBitsBeacon,
			Nonce:     0x18aea41a, // 414098458
		}

	case wire.FastTestNet:
		aux = getBtcAux(false)
		opts = GenesisBlockOpts{
			Version:   1,
			PrevBlock: chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
			Timestamp: time.Unix(1633687865, 0), // Fri  8 Oct 10:11:52 UTC 2021
			Bits:      fastnetBeaconPoWBits,
			Nonce:     0x18aea41a,
		}
	case wire.SimNet:
		aux = getBtcAux(false)
		opts = GenesisBlockOpts{
			Version:   1,
			PrevBlock: chainhash.Hash{},         // 0000000000000000000000000000000000000000000000000000000000000000
			Timestamp: time.Unix(1633687865, 0), // Fri  8 Oct 10:11:52 UTC 2021
			Bits:      0x207fffff,               // 545259519 [7fffff0000000000000000000000000000000000000000000000000000000000]
			Nonce:     2,
		}
	default:
		aux = getBtcAux(true)
		opts = GenesisBlockOpts{
			Version:   1,
			PrevBlock: chainhash.Hash{},          // 0000000000000000000000000000000000000000000000000000000000000000
			Timestamp: time.Unix(1635719400, 0),  // Sun Oct 31 2021 22:30:00 GMT+0000
			Bits:      mainNetPowLimitBitsBeacon, // 0x1a040000 [0000000000000400000000000000000000000000000000000000000000000000]
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
			new(big.Int).SetInt64(0),
			opts.Nonce,
		),
		Transactions: []*wire.MsgTx{state.genesisTx},
	}

	state.genesisBlock.Header.BeaconHeader().SetBTCAux(aux)
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
			new(big.Int).SetInt64(0),
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
func genesisCoinbaseTx(name wire.JaxNet) {
	state := genesisStorage[name]

	if state.genesisTx != nil {
		return
	}

	state.genesisTx = new(wire.MsgTx)

	var txHex string
	switch name {
	case wire.FastTestNet, wire.SimNet:
		txHex = fastNetGenesisTxHex
	case wire.TestNet:
		txHex = testnetGenesisTxHex
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

func getBtcAux(mainnet bool) wire.BTCBlockAux {
	if mainnet {
		rawData, err := hex.DecodeString(btcAux)
		if err != nil {
			panic("invalid genesis btc aux hex-data")
		}
		aux := wire.BTCBlockAux{}
		err = aux.Deserialize(bytes.NewBuffer(rawData))
		if err != nil {
			panic("invalid genesis btc aux hex-data")
		}
	}

	rawData, err := hex.DecodeString(btcTestNetAux)
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
	testnetGenesisTxHex = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff4220a09979e5088192d4ece5f7c49c444845a20d7bcc77cd010000000000000000001d4a61782e4e6574776f726b20656e746572732074686520726163652120027368ffffffff11006c042b028000001976a91437c052ab059140534d374a22ebd987c98b1f936f88ac006c042b028000001976a9142c5f6d1901b883fa0c8d6b801e9852f61d45898888ac006c042b028000001976a9149133bffc55515814e9ec162a6d57d9da540e0f6e88ac006c042b028000001976a91461c0dd65c120ebb073f15083c2df442e5ce0800688ac0034e28f867701001976a91491acc017b2d1a4e39f89cbb4c1fc8df6fb7d3add88ac0034e28f867701001976a914b5e8d343dec3dcbbfc55d75e4bd2b6086a1e85b688ac0080f420e6b500001976a9141ad411e604711225347dc9d494a5f5daa45a11a488ac000408f8fc4e00001976a914663b8725e17017269d258fe221fd629c97a5e39b88ac0036bcfc141f00001976a914d3c2ab8fd0a18c9da7bf3b324bdea419aeb4682088ac0080f420e6b500001976a914b32783bac902e0232da52d87781119a6bdb9231388ac00f0d55c299f00001976a9148857c122b5eb1dd56d4c5aef89fd83519af1ba3e88ac00c4093f5a4d00001976a914f315c973f7d50285e59067057530cff363611b9188ac00c4093f5a4d00001976a914fedb7732fd605fe976e9a9a72faeadc934cbfad788ac0080c6a47e8d03001976a914dfd7bbc925f22c9cf9248baa4877a240d0eb61d488ac00f0d55c299f00001976a9144b75edd9b6dc4bae8ac04a47d7b93508f9cfbfbd88ac00c4093f5a4d00001976a914d0908774ef842ca8620dfe71f0c6695e78b666e188ac00c4093f5a4d00001976a91404e9f23f9eb72a6b23ea881d9a4706bb7db38f1788ac00000000"
	btcTestNetAux       = "04600020f76b6a887c89fdea44bd89a77006645ed6bb2701862b00000000000000000000e94c9281dc6310592352059d264aeb47e81dc0475717a6204a9c7f8099fd292bbcab7d6108040e17fb2a86e401000000010000000000000000000000000000000000000000000000000000000000000000ffffffff5c0391cb0a192f5669614254432f4d696e6564206279206f3273706e65772f2cfabe6d6d1052ee457e675a7e7b554f4b1b8a4cbb9ee852c7ccb323fded51065a8fde4506100000000000000010be75a00f1f305410e2b49d65c11c0c00ffffffff045c067125000000001976a914536ffa992491508dca0354e52f32a3a7a679a53a88ac00000000000000002b6a2952534b424c4f434b3a7067a246e981a8500e3cc22276a3d0e520bbfabe559d66eaadcdab2e003a22590000000000000000266a24b9e11b6dff982ec0c3c11f96cdc25f874e3f44b98a79f1f0dc6de65b8a479893bd9c3a380000000000000000266a24aa21a9ede8db996cdca0d8f8ddc8c1d54e318c8262c28c0110bd6483bbc23243d2d1ca66000000000c78883bde8a25eecf9aa7129bcdfb43eebbf647fc58cf74af306f2cbaa042f094b1e126b731d7ccfa626b41bafc6bd5c8ec4177435807781aa3fe2352dea399944eb94a03cd16f7c0fdf60431caca48fa13ca1db40a6de9d58334b47342fad05201229d37df1dfd998a7da8c6ce2881a377f92077e822ee48fb7ca23ee6e2f5a044692ffeb0156c29d1ecb92ddbb68b32562ea1bcebebeae8eb7ce385c174c4aba8b54a37e24cf494f73d86870b2606b0bb3cd5f0b5a20e566d2a1e28ed4ab6cd4e1b08258deb00142c2a0cd07c1cddb4ff9674585f4a7314671bdfa518bfceada8ba1c6ed99430936314837417c46b74bf3d8567e36512347668b75fc8986768e15300b77982e4b55b1c415f8bc56941dbf33d58db93bc406fe469a27ac66e10fead34a89f81e22560ff611468daf26eb82bb931afbf3606993e790d085f99edfa0ca2318b508e22858590f823ef231ecc25d1072264e2663d1b464011584876666865049313b3f07436e12c0ad636dc8d6541cf817fefd339ead7185493d0dc"

	mainnetGenesisTxHex = "01000000010000000000000000000000000000000000000000000000000000000000000000fffffffffd67035361706572652041756465204a61784e65740a576520686f7065207468617420627920696e74726f647563696e672061207472756c7920646563656e7472616c697a656420656e657267792d7374616e64617264206d6f6e65746172792073797374656d2c20746865204a61784e65742070726f746f636f6c2077696c6c2068656c7020746f20666163696c697461746520746865207472616e73666f726d6174696f6e206f662068756d616e69747920696e746f206120747970652d3220636976696c697a6174696f6e20616e642070617665207468652077617920666f7220616e20696d70726f76656420736f63696574792074686174206368616d70696f6e7320736369656e746966696320616476616e63656d656e742c206a7573746963652c20616e74692d636f7272757074696f6e2c20756e6976657273616c20626173696320696e636f6d652c2061207472616e73616374696f6e2d74617820626173656420676f7665726e6d656e742c20616e64206f74686572732e200a49742773206f757220736f6c656d6e206c69666574696d6520726573706f6e736962696c69747920746f20646566656e64206f75722070726f746f636f6c2c20696e636c7564696e67206f7572206e6574776f726b20746f6b656e7320284a41584e4554202d204a584e2c204a4158292c20616e6420776f726b20746f67657468657220746f206272696e672061626f757420746865206368616e6765207765207769736820746f207365652e0a4f6e204f63746f6265722033312c20323032312c20746865203133746820616e6e6976657273617279206f662074686520426974636f696e2070617065722c20776520656e746572656420746865207261636520746f2074686520746f702e2e2e0a0a5370656369616c206372656469747320746f3a0a49757269692053687973686174736b79690a546172617320456d656c79616e656e6b6f0a4c75636173204c656765720a4d696b6520536865620a44696d61204368697a686576736b790a44722e20416264656c68616b696d2053656e68616a692048616669640a5361746f736869204e616b616d6f746f0a44722e20572e2053636f74742053746f726e657474610a44722e205374756172742048616265720a44722e2052616c706820432e204d65726b6c650a0a2d20614a61785072696d65e2809dffffffff11006c042b028000001976a91437c052ab059140534d374a22ebd987c98b1f936f88ac006c042b028000001976a9142c5f6d1901b883fa0c8d6b801e9852f61d45898888ac006c042b028000001976a9149133bffc55515814e9ec162a6d57d9da540e0f6e88ac006c042b028000001976a91461c0dd65c120ebb073f15083c2df442e5ce0800688ac0053ec89867701001976a91491acc017b2d1a4e39f89cbb4c1fc8df6fb7d3add88ac0053ec89867701001976a914b5e8d343dec3dcbbfc55d75e4bd2b6086a1e85b688ac0080f420e6b500001976a9141ad411e604711225347dc9d494a5f5daa45a11a488ac000408f8fc4e00001976a914663b8725e17017269d258fe221fd629c97a5e39b88ac0036bcfc141f00001976a914d3c2ab8fd0a18c9da7bf3b324bdea419aeb4682088ac0080f420e6b500001976a914b32783bac902e0232da52d87781119a6bdb9231388ac00f0d55c299f00001976a9148857c122b5eb1dd56d4c5aef89fd83519af1ba3e88ac00c4093f5a4d00001976a914f315c973f7d50285e59067057530cff363611b9188ac00c4093f5a4d00001976a914fedb7732fd605fe976e9a9a72faeadc934cbfad788ac0080c6a47e8d03001976a914dfd7bbc925f22c9cf9248baa4877a240d0eb61d488ac00f0d55c299f00001976a9144b75edd9b6dc4bae8ac04a47d7b93508f9cfbfbd88ac00c4093f5a4d00001976a914d0908774ef842ca8620dfe71f0c6695e78b666e188ac00c4093f5a4d00001976a91404e9f23f9eb72a6b23ea881d9a4706bb7db38f1788ac00000000"
	btcAux              = "0400ff3fdaf4f28af524e87cfac3b385652d3d55d10a4367354a02000000000000000000e5d2b2ef6652679458555a006ad90ff31f41cf60d56b59265c6ba79b5219d43b0a317f61cffe0c174c27a51602000000010000000000000000000000000000000000000000000000000000000000000000ffffffff4e0332cc0a04f48a88e14254432e636f6d2f6a69326575fabe6d6ddf37ecfc803e002749b6f392fb227da20e1958b969670408eeb35391977a60860200000076090e700b5cce173148000000000000ffffffff0348bd5f27000000001976a91474e878616bd5e5236ecb22667627eeecbff54b9f88ac0000000000000000266a24aa21a9ed8df2ed6145a6c4f688ac5fc2807a631b1eb0d78886d5d28b51796c49ba630fb40000000000000000266a24b9e11b6dd931f165d05796aed39be238dd47d1bc8e4b20afc49399317e7d89db083d9e76000000000ce3e432dbea4eba16f48884c7184715256a1c188a112089dfe0533ac30bdb979db6bddd68291111080eae0f47bcbd299f832df2177ccca32cd8d2f5ef347d06ced36579d10c03b6e6fb36a808e0bd5184f8e3b0e6b76cb950556e128649ec44c975cef92caa3bd4fdb473b1f9fc26638e01a5e78973ceff04637cf8ed901b58374c2b686914a41a3dc8959b428bbcf34188c6ccb5f4d98d20c1865121c2979db8b05a2f1b989d909246fcca3602c7429a3004d3a41c86a94dce8fbfe15b26fa9cde9fbf0f992b94b15afe2daa5c44b5813e20bc06721204194a77b7116d05a460e664daf8c018a14897e1c3f3c648c41e39fdbbcf6865c505148ca6105260f9302bbc3e7af74a176957706383a055689300c810cfc6a0dda19ccea668ca1bb8f01678d72cfca2f2bccbf6ff3fe17156e82ee2f79fb0225e768f6e0f6e707e5fea5010ecd9c9b4bb38ebf10c41d6e6d2acfc7ebf3ad39cf0f2ceb741f6615703e56a156219797c11af3e41866efe534df1204f1349b56a108a77fd7764cea9b71b"
)
