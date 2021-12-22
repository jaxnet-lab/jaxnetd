/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package jaxjson

// easyjson:json
type BTCBlockAux struct {
	Version             int32        `json:"version"`
	VersionHex          string       `json:"versionHex"`
	Hash                string       `json:"hash"`
	PreviousHash        string       `json:"previousblockhash"`
	MerkleRoot          string       `json:"merkleroot"`
	Time                int64        `json:"time"`
	Nonce               uint32       `json:"nonce"`
	Bits                string       `json:"bits"`
	Difficulty          float64      `json:"difficulty"`
	CoinbaseTx          *TxRawResult `json:"coinbaseTx,omitempty"`
	CoinbaseTxHex       string       `json:"coinbaseTxHex,omitempty"`
	CoinbaseMerkleProof []string     `json:"coinbaseMerkleProof"`
}

// easyjson:json
type BeaconBlockHeader struct {
	Version             int32  `json:"version"`
	VersionHex          string `json:"versionHex"`
	Height              int64  `json:"height"`
	Hash                string `json:"hash"`
	PreviousHash        string `json:"previousblockhash"`
	PoWHash             string `json:"powhash"`
	ExclusiveHash       string `json:"exclusiveHash"`
	PrevBlocksMMRRoot   string `json:"prevBlocksMmrRoot,omitempty"`
	ActualBlocksMMRRoot string `json:"actualBlocksMmrRoot,omitempty"`

	K          string  `json:"k"`
	VoteK      string  `json:"voteK"`
	MerkleRoot string  `json:"merkleroot"`
	Time       int64   `json:"time"`
	Nonce      uint32  `json:"nonce"`
	Shards     uint32  `json:"shards"`
	Bits       string  `json:"bits"`
	Difficulty float64 `json:"difficulty"`

	MergeMiningRoot      string   `json:"mergeMiningRoot,omitempty"`
	MergeMiningNumber    uint32   `json:"mergeMiningNumber,omitempty"`
	TreeEncoding         string   `json:"treeEncoding,omitempty"`
	MergeMiningProof     []string `json:"mergeMiningProof,omitempty"`
	TreeCodingLengthBits uint32   `json:"treeCodingLengthBits"`

	BTCAux BTCBlockAux `json:"btcAux,omitempty"`
}

// GetBeaconBlockHeaderVerboseResult models the data from the getBeaconBlockHeader command when
// the verbose flag is set.  When the verbose flag is not set, getBeaconBlockHeader
// returns a hex-encoded string.
// easyjson:json
type GetBeaconBlockHeaderVerboseResult struct {
	BeaconBlockHeader
	SerialID      int64  `json:"serialID"`
	PrevSerialID  int64  `json:"prevSerialID"`
	NextHash      string `json:"nextblockhash,omitempty"`
	Confirmations int64  `json:"confirmations"`
}

// GetBeaconBlockVerboseResult models the data from the getBeaconBlock command when the
// verbose flag is set to 1.  When the verbose flag is set to 0, getBeaconBlock returns a
// hex-encoded string. When the verbose flag is set to 1, getBeaconBlock returns an object
// whose tx field is an array of transaction hashes. When the verbose flag is set to 2,
// getBeaconBlock returns an object whose tx field is an array of raw transactions.
// Use GetBeaconBlockVerboseTxResult to unmarshal data received from passing verbose=2 to getBeaconBlock.
// easyjson:json
type GetBeaconBlockVerboseResult struct {
	BeaconBlockHeader
	SerialID      int64         `json:"serialID"`
	PrevSerialID  int64         `json:"prevSerialID"`
	NextHash      string        `json:"nextblockhash,omitempty"`
	Confirmations int64         `json:"confirmations"`
	StrippedSize  int32         `json:"strippedsize"`
	Size          int32         `json:"size"`
	Weight        int32         `json:"weight"`
	TxHashes      []string      `json:"txHashes,omitempty"`
	Tx            []TxRawResult `json:"tx,omitempty"`
}

// easyjson:json
type ShardBlockHeader struct {
	Version             int32    `json:"version"`
	VersionHex          string   `json:"versionHex"`
	Height              int64    `json:"height"`
	Hash                string   `json:"hash"`
	PreviousHash        string   `json:"previousblockhash"`
	PoWHash             string   `json:"powhash"`
	ExclusiveHash       string   `json:"exclusiveHash"`
	PrevBlocksMMRRoot   string   `json:"prevBlocksMmrRoot,omitempty"`
	ActualBlocksMMRRoot string   `json:"actualBlocksMmrRoot,omitempty"`
	MerkleRoot          string   `json:"merkleroot"`
	Bits                string   `json:"bits"`
	Difficulty          float64  `json:"difficulty"`
	ShardMerkleProof    []string `json:"shardMerkleProof"`

	BeaconAuxHeader           BeaconBlockHeader `json:"beaconAuxHeader"`
	BeaconCoinbaseTx          *TxRawResult      `json:"beaconCoinbaseTx,omitempty"`
	BeaconCoinbaseTxHex       string            `json:"beaconCoinbaseTxHex,omitempty"`
	BeaconCoinbaseMerkleProof []string          `json:"beaconCoinbaseMerkleProof"`
}

// GetShardBlockVerboseResult models the data from the getShardBlock command when the
// verbose flag is set to 1.  When the verbose flag is set to 0, getShardBlock returns a
// hex-encoded string. When the verbose flag is set to 1, getShardBlock returns an object
// whose tx field is an array of transaction hashes. When the verbose flag is set to 2,
// getShardBlock returns an object whose tx field is an array of raw transactions.
// Use GetShardBlockVerboseResult to unmarshal data received from passing verbose=2 to getShardBlock.
// easyjson:json
type GetShardBlockVerboseResult struct {
	ShardBlockHeader
	SerialID      int64         `json:"serialID"`
	PrevSerialID  int64         `json:"prevSerialID"`
	NextHash      string        `json:"nextblockhash,omitempty"`
	Confirmations int64         `json:"confirmations"`
	StrippedSize  int32         `json:"strippedsize"`
	Size          int32         `json:"size"`
	Weight        int32         `json:"weight"`
	TxHashes      []string      `json:"txHashes,omitempty"`
	Tx            []TxRawResult `json:"tx,omitempty"`
}

// GetShardBlockHeaderVerboseResult models the data from the getShardBlockHeader command when
// the verbose flag is set.  When the verbose flag is not set, getShardBlockHeader
// returns a hex-encoded string.
// easyjson:json
type GetShardBlockHeaderVerboseResult struct {
	ShardBlockHeader
	SerialID      int64  `json:"serialID"`
	PrevSerialID  int64  `json:"prevSerialID"`
	NextHash      string `json:"nextblockhash,omitempty"`
	Confirmations int64  `json:"confirmations"`
}
