// Copyright (c) 2014-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package jaxjson

import (
	"encoding/json"
	"time"
)

//go:generate easyjson --omit_empty chainsvrresults.go  chainsvrresults_blocks.go jsonrpc.go

// easyjson:json
type GetChainStatsResult struct {
	Time                   int64  `json:"time"`
	Txs                    int64  `json:"txcount"`
	Txrate                 string `json:"txrate"`
	WindowFinalBlockHash   string `json:"window_final_block_hash"`
	WindowFinalBlockHeight int64  `json:"window_final_block_height"`
	WindowBlockCount       int64  `json:"window_block_count"`
	WindowTxCount          int64  `json:"window_tx_count"`
	WindowInterval         int64  `json:"window_interval"`
}

// GetBlockStatsResult models the data from the getblockstats command.
// easyjson:json
type GetBlockStatsResult struct {
	AverageFee         int64   `json:"avgfee"`
	AverageFeeRate     int64   `json:"avgfeerate"`
	AverageTxSize      int64   `json:"avgtxsize"`
	FeeratePercentiles []int64 `json:"feerate_percentiles"`
	Hash               string  `json:"blockhash"`
	Height             int64   `json:"height"`
	SerialID           int64   `json:"serialID"`
	PrevSerialID       int64   `json:"prevSerialID"`
	Ins                int64   `json:"ins"`
	MaxFee             int64   `json:"maxfee"`
	MaxFeeRate         int64   `json:"maxfeerate"`
	MaxTxSize          int64   `json:"maxtxsize"`
	MedianFee          int64   `json:"medianfee"`
	MedianTime         int64   `json:"mediantime"`
	MedianTxSize       int64   `json:"mediantxsize"`
	MinFee             int64   `json:"minfee"`
	MinFeeRate         int64   `json:"minfeerate"`
	MinTxSize          int64   `json:"mintxsize"`
	Outs               int64   `json:"outs"`
	SegWitTotalSize    int64   `json:"swtotal_size"`
	SegWitTotalWeight  int64   `json:"swtotal_weight"`
	SegWitTxs          int64   `json:"swtxs"`
	Subsidy            int64   `json:"subsidy"`
	Time               int64   `json:"time"`
	TotalOut           int64   `json:"total_out"`
	TotalSize          int64   `json:"total_size"`
	TotalWeight        int64   `json:"total_weight"`
	Txs                int64   `json:"txs"`
	UTXOIncrease       int64   `json:"utxo_increase"`
	UTXOSizeIncrease   int64   `json:"utxo_size_inc"`
}

// CreateMultiSigResult models the data returned from the createmultisig
// command.
type CreateMultiSigResult struct {
	Address      string `json:"address"`
	RedeemScript string `json:"redeemScript"`
}

// DecodeScriptResult models the data returned from the decodescript command.
type DecodeScriptResult struct {
	Asm       string   `json:"asm"`
	ReqSigs   int32    `json:"reqSigs,omitempty"`
	Type      string   `json:"type"`
	Addresses []string `json:"addresses,omitempty"`
	P2sh      string   `json:"p2sh,omitempty"`
}

// GetAddedNodeInfoResultAddr models the data of the addresses portion of the
// getaddednodeinfo command.
type GetAddedNodeInfoResultAddr struct {
	Address   string `json:"address"`
	Connected string `json:"connected"`
}

// GetAddedNodeInfoResult models the data from the getaddednodeinfo command.
// easyjson:json
type GetAddedNodeInfoResult struct {
	AddedNode string                        `json:"addednode"`
	Connected *bool                         `json:"connected,omitempty"`
	Addresses *[]GetAddedNodeInfoResultAddr `json:"addresses,omitempty"`
}

// easyjson:json
type GetAddedNodeInfoResults []GetAddedNodeInfoResult

// SoftForkDescription describes the current state of a soft-fork which was
// deployed using a super-majority block signalling.
type SoftForkDescription struct {
	ID      string `json:"id"`
	Version uint32 `json:"version"`
	Reject  struct {
		Status bool `json:"status"`
	} `json:"reject"`
}

// Bip9SoftForkDescription describes the current state of a defined BIP0009
// version bits soft-fork.
type Bip9SoftForkDescription struct {
	Status     string `json:"status"`
	Bit        uint8  `json:"bit"`
	StartTime1 int64  `json:"startTime"`
	StartTime2 int64  `json:"start_time"`
	Timeout    int64  `json:"timeout"`
	Since      int32  `json:"since"`
}

// StartTime returns the starting time of the softfork as a Unix epoch.
func (d *Bip9SoftForkDescription) StartTime() int64 {
	if d.StartTime1 != 0 {
		return d.StartTime1
	}
	return d.StartTime2
}

// SoftForks describes the current softforks enabled by the backend. Softforks
// activated through BIP9 are grouped together separate from any other softforks
// with different activation types.
type SoftForks struct {
	SoftForks     []*SoftForkDescription              `json:"softforks"`
	Bip9SoftForks map[string]*Bip9SoftForkDescription `json:"bip9_softforks"`
}

// UnifiedSoftForks describes a softforks in a general manner, irrespective of
// its activation type. This was a format introduced by jaxnetd v0.19.0
type UnifiedSoftFork struct {
	Type                    string                   `json:"type"`
	BIP9SoftForkDescription *Bip9SoftForkDescription `json:"bip9"`
	Height                  int32                    `json:"height"`
	Active                  bool                     `json:"active"`
}

// UnifiedSoftForks describes the current softforks enabled the by the backend
// in a unified manner, i.e, softforks with different activation types are
// grouped together. This was a format introduced by jaxnetd v0.19.0
type UnifiedSoftForks struct {
	SoftForks map[string]*UnifiedSoftFork `json:"softforks"`
}

// GetBlockChainInfoResult models the data returned from the getblockchaininfo
// command.
type GetBlockChainInfoResult struct {
	Chain                string  `json:"chain"`
	Blocks               int32   `json:"blocks"`
	Headers              int32   `json:"headers"`
	BestBlockHash        string  `json:"bestblockhash"`
	Difficulty           float64 `json:"difficulty"`
	MedianTime           int64   `json:"mediantime"`
	VerificationProgress float64 `json:"verificationprogress,omitempty"`
	Pruned               bool    `json:"pruned"`
	PruneHeight          int32   `json:"pruneheight,omitempty"`
	ChainWork            string  `json:"chainwork,omitempty"`
	Shards               uint32  `json:"shards"`
	*SoftForks
	*UnifiedSoftForks
}

// GetBlockTemplateResultTx models the transactions field of the
// getblocktemplate command.
type GetBlockTemplateResultTx struct {
	Data    string  `json:"data"`
	Hash    string  `json:"hash"`
	Depends []int64 `json:"depends"`
	Fee     int64   `json:"fee"`
	SigOps  int64   `json:"sigops"`
	Weight  int64   `json:"weight"`
}

// GetBlockTemplateResultAux models the coinbaseaux field of the
// getblocktemplate command.
type GetBlockTemplateResultAux struct {
	Flags string `json:"flags"`
}

// GetBlockTemplateResult models the data returned from the getblocktemplate
// command.
// easyjson:json
type GetBlockTemplateResult struct {
	// Base fields from BIP 0022.  CoinbaseAux is optional.  One of
	// CoinbaseTxn or CoinbaseValue must be specified, but not both.
	Bits              string                     `json:"bits"`
	ChainWeight       string                     `json:"chainweight"`
	CurTime           int64                      `json:"curtime"`
	Height            int64                      `json:"height"`
	PreviousHash      string                     `json:"previousblockhash"` // DEPRECATED this field will be dropped, use the PrevBlocksMMRRoot
	PrevBlocksMMRRoot string                     `json:"prevblocksmmrroot"`
	SerialID          int64                      `json:"serialID"`
	PrevSerialID      int64                      `json:"prevSerialID"`
	SigOpLimit        int64                      `json:"sigoplimit,omitempty"`
	SizeLimit         int64                      `json:"sizelimit,omitempty"`
	WeightLimit       int64                      `json:"weightlimit,omitempty"`
	Transactions      []GetBlockTemplateResultTx `json:"transactions"`
	Version           int32                      `json:"version"`
	Shards            uint32                     `json:"shards"`
	CoinbaseAux       *GetBlockTemplateResultAux `json:"coinbaseaux,omitempty"`
	CoinbaseTxn       *GetBlockTemplateResultTx  `json:"coinbasetxn,omitempty"`
	CoinbaseValue     *int64                     `json:"coinbasevalue,omitempty"`
	WorkID            string                     `json:"workid,omitempty"`

	// Witness commitment defined in BIP 0141.
	DefaultWitnessCommitment string `json:"default_witness_commitment,omitempty"`

	// Optional long polling from BIP 0022.
	LongPollID  string `json:"longpollid,omitempty"`
	LongPollURI string `json:"longpolluri,omitempty"`
	SubmitOld   *bool  `json:"submitold,omitempty"`

	// Basic pool extension from BIP 0023.
	Target  string `json:"target,omitempty"`
	Expires int64  `json:"expires,omitempty"`

	// Mutations from BIP 0023.
	MaxTime    int64    `json:"maxtime,omitempty"`
	MinTime    int64    `json:"mintime,omitempty"`
	Mutable    []string `json:"mutable,omitempty"`
	NonceRange string   `json:"noncerange,omitempty"`

	// Block proposal from BIP 0023.
	Capabilities []string `json:"capabilities,omitempty"`
	RejectReason string   `json:"reject-reason,omitempty"`

	BTCAux string `json:"btcAux"`
	K      uint32 `json:"k"`
	VoteK  uint32 `json:"vote_k"`
}

// GetMempoolEntryResult models the data returned from the getmempoolentry's
// fee field

type MempoolFees struct {
	Base       float64 `json:"base"`
	Modified   float64 `json:"modified"`
	Ancestor   float64 `json:"ancestor"`
	Descendant float64 `json:"descendant"`
}

// GetMempoolEntryResult models the data returned from the getmempoolentry
// command.
// easyjson:json
type GetMempoolEntryResult struct {
	VSize       int32   `json:"vsize"`
	Size        int32   `json:"size"`
	Weight      int64   `json:"weight"`
	Fee         float64 `json:"fee"`
	ModifiedFee float64 `json:"modifiedfee"`
	Time        int64   `json:"time"`
	// Height          int64       `json:"height"`
	SerialID        int64       `json:"serialID"`
	PrevSerialID    int64       `json:"prevSerialID"`
	DescendantCount int64       `json:"descendantcount"`
	DescendantSize  int64       `json:"descendantsize"`
	DescendantFees  float64     `json:"descendantfees"`
	AncestorCount   int64       `json:"ancestorcount"`
	AncestorSize    int64       `json:"ancestorsize"`
	AncestorFees    float64     `json:"ancestorfees"`
	WTxID           string      `json:"wtxid"`
	Fees            MempoolFees `json:"fees"`
	Depends         []string    `json:"depends"`
}

// GetMempoolInfoResult models the data returned from the getmempoolinfo
// command.
type GetMempoolInfoResult struct {
	Size  int64 `json:"size"`
	Bytes int64 `json:"bytes"`
}

// NetworksResult models the networks data from the getnetworkinfo command.
type NetworksResult struct {
	Name                      string `json:"name"`
	Limited                   bool   `json:"limited"`
	Reachable                 bool   `json:"reachable"`
	Proxy                     string `json:"proxy"`
	ProxyRandomizeCredentials bool   `json:"proxy_randomize_credentials"`
}

// LocalAddressesResult models the localaddresses data from the getnetworkinfo
// command.
type LocalAddressesResult struct {
	Address string `json:"address"`
	Port    uint16 `json:"port"`
	Score   int32  `json:"score"`
}

// GetNetworkInfoResult models the data returned from the getnetworkinfo
// command.
// easyjson:json
type GetNetworkInfoResult struct {
	Version         int32                  `json:"version"`
	SubVersion      string                 `json:"subversion"`
	ProtocolVersion int32                  `json:"protocolversion"`
	LocalServices   string                 `json:"localservices"`
	LocalRelay      bool                   `json:"localrelay"`
	TimeOffset      int64                  `json:"timeoffset"`
	Connections     int32                  `json:"connections"`
	NetworkActive   bool                   `json:"networkactive"`
	Networks        []NetworksResult       `json:"networks"`
	RelayFee        float64                `json:"relayfee"`
	IncrementalFee  float64                `json:"incrementalfee"`
	LocalAddresses  []LocalAddressesResult `json:"localaddresses"`
	Warnings        string                 `json:"warnings"`
}

// GetPeerInfoResult models the data returned from the getpeerinfo command.
// easyjson:json
type GetPeerInfoResult struct {
	ID             int32   `json:"id"`
	Addr           string  `json:"addr"`
	AddrLocal      string  `json:"addrlocal,omitempty"`
	Services       string  `json:"services"`
	RelayTxes      bool    `json:"relaytxes"`
	LastSend       int64   `json:"lastsend"`
	LastRecv       int64   `json:"lastrecv"`
	BytesSent      uint64  `json:"bytessent"`
	BytesRecv      uint64  `json:"bytesrecv"`
	ConnTime       int64   `json:"conntime"`
	TimeOffset     int64   `json:"timeoffset"`
	PingTime       float64 `json:"pingtime"`
	PingWait       float64 `json:"pingwait,omitempty"`
	Version        uint32  `json:"version"`
	SubVer         string  `json:"subver"`
	Inbound        bool    `json:"inbound"`
	StartingHeight int32   `json:"startingheight"`
	CurrentHeight  int32   `json:"currentheight,omitempty"`
	BanScore       int32   `json:"banscore"`
	FeeFilter      int64   `json:"feefilter"`
	SyncNode       bool    `json:"syncnode"`
}

// easyjson:json
type GetPeerInfoResults []GetPeerInfoResult

// GetRawMempoolVerboseResult models the data returned from the getrawmempool
// command when the verbose flag is set.  When the verbose flag is not set,
// getrawmempool returns an array of transaction hashes.
// easyjson:json
type GetRawMempoolVerboseResult struct {
	Size             int32    `json:"size"`
	Vsize            int32    `json:"vsize"`
	Weight           int32    `json:"weight"`
	Fee              float64  `json:"fee"`
	Time             int64    `json:"time"`
	Height           int64    `json:"height"`
	StartingPriority float64  `json:"startingpriority"`
	CurrentPriority  float64  `json:"currentpriority"`
	Depends          []string `json:"depends"`
	PreciseFee       int64    `json:"precise_fee"`
}

// easyjson:json
type GetRawMempoolVerboseResultMap map[string]GetRawMempoolVerboseResult

// ScriptPubKeyResult models the scriptPubKey data of a tx script.  It is
// defined separately since it is used by multiple commands.
type ScriptPubKeyResult struct {
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex,omitempty"`
	ReqSigs   int32    `json:"reqSigs,omitempty"`
	Type      string   `json:"type"`
	Addresses []string `json:"addresses,omitempty"`
}

// GetTxOutResult models the data from the gettxout command.
// easyjson:json
type GetTxOutResult struct {
	BestBlock     string             `json:"bestblock"`
	Confirmations int64              `json:"confirmations"`
	Value         float64            `json:"value"`
	ScriptPubKey  ScriptPubKeyResult `json:"scriptPubKey"`
	Coinbase      bool               `json:"coinbase"`
	IsSpent       bool               `json:"isSpent"`
	PreciseValue  int64              `json:"precise_value"`
}

// GetTxResult models the data from the gettx command.
// easyjson:json
type GetTxResult struct {
	BestBlock     string `json:"bestblock"`
	Block         string `json:"block"`
	Height        int64  `json:"height"`
	Confirmations int64  `json:"confirmations"`
	Coinbase      bool   `json:"coinbase"`
	RawTx         string `json:"raw_tx"`
}

type GetTxOutStatus struct {
	Outs        []TxOutKey
	OnlyMempool *bool `jsonrpcdefault:"true"`
}

type TxOutKey struct {
	TxHash string `json:"outTxHash"`
	Index  uint32 `json:"outIndex"`
}

//easyjson:json
type TxOutStatus struct {
	OutTxHash string `json:"outTxHash"`
	OutIndex  uint32 `json:"outIndex"`
	Found     bool   `json:"found"`
	IsSpent   bool   `json:"isSpent"`
	InMempool bool   `json:"isMempool"`
}

//easyjson:json
type TxOutStatuses []TxOutStatus

// ExtendedTxOutResult models the data from the gettxout command.
// easyjson:json
type ExtendedTxOutResult struct {
	TxHash        string             `json:"txHash"`
	Index         uint32             `json:"index"`
	BlockHeight   int64              `json:"blockHeight"`
	Confirmations int64              `json:"confirmations"`
	Value         int64              `json:"value"`
	ScriptPubKey  ScriptPubKeyResult `json:"scriptPubKey"`
	Coinbase      bool               `json:"coinbase"`
	Used          bool               `json:"used"`
}

// easyjson:json
type ListTxOutResult struct {
	List []ExtendedTxOutResult
}

// EADAddresses models the data from the listeadaddresses command.
type EADAddresses struct {
	ID        uint64       `json:"id"`
	PublicKey string       `json:"publicKey"`
	IPs       []EADAddress `json:"ips"`
}

type EADAddress struct {
	// IP address of the server.
	IP  string `json:"ip"`
	URL string `json:"url"`

	// Port the server is using.
	Port uint16 `json:"port"`
	// ExpiresAt Address expiration time.
	ExpiresAt time.Time `json:"expires_at"`
	// Shard shows what shards the agent works with.
	Shard      uint32 `json:"shards"`
	TxHash     string `json:"tx_hash"`
	TxOutIndex int    `json:"tx_out_index"`
}

//easyjson:json
type ListEADAddresses struct {
	Agents map[string]EADAddresses `json:"agents"`
}

// GetNetTotalsResult models the data returned from the getnettotals command.
type GetNetTotalsResult struct {
	TotalBytesRecv uint64 `json:"totalbytesrecv"`
	TotalBytesSent uint64 `json:"totalbytessent"`
	TimeMillis     int64  `json:"timemillis"`
}

// ScriptSig models a signature script.  It is defined separately since it only
// applies to non-coinbase.  Therefore the field in the Vin structure needs
// to be a pointer.
type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

// Vin models parts of the tx data.  It is defined separately since
// getrawtransaction, decoderawtransaction, and searchrawtransaction use the
// same structure.
type Vin struct {
	Coinbase         string     `json:"coinbase"`
	Txid             string     `json:"txid"`
	Vout             uint32     `json:"vout"`
	ScriptSig        *ScriptSig `json:"scriptSig"`
	Sequence         uint32     `json:"sequence"`
	Witness          []string   `json:"txinwitness"`
	Age              int32      `json:"age"`
	Amount           int64      `json:"amount"`
	FromAnotherShard bool       `json:"from_another_shard,omitempty"`
}

// IsCoinBase returns a bool to show if a Vin is a Coinbase one or not.
func (v *Vin) IsCoinBase() bool {
	return len(v.Coinbase) > 0
}

// HasWitness returns a bool to show if a Vin has any witness data associated
// with it or not.
func (v *Vin) HasWitness() bool {
	return len(v.Witness) > 0
}

// MarshalJSON provides a custom Marshal method for Vin.
func (v *Vin) MarshalJSON() ([]byte, error) {
	if v.IsCoinBase() {
		coinbaseStruct := struct {
			Coinbase string   `json:"coinbase"`
			Sequence uint32   `json:"sequence"`
			Witness  []string `json:"witness,omitempty"`
		}{
			Coinbase: v.Coinbase,
			Sequence: v.Sequence,
			Witness:  v.Witness,
		}
		return json.Marshal(coinbaseStruct)
	}

	if v.HasWitness() {
		txStruct := struct {
			Txid      string     `json:"txid"`
			Vout      uint32     `json:"vout"`
			ScriptSig *ScriptSig `json:"scriptSig"`
			Witness   []string   `json:"txinwitness"`
			Sequence  uint32     `json:"sequence"`
		}{
			Txid:      v.Txid,
			Vout:      v.Vout,
			ScriptSig: v.ScriptSig,
			Witness:   v.Witness,
			Sequence:  v.Sequence,
		}
		return json.Marshal(txStruct)
	}

	txStruct := struct {
		Txid      string     `json:"txid"`
		Vout      uint32     `json:"vout"`
		ScriptSig *ScriptSig `json:"scriptSig"`
		Sequence  uint32     `json:"sequence"`
	}{
		Txid:      v.Txid,
		Vout:      v.Vout,
		ScriptSig: v.ScriptSig,
		Sequence:  v.Sequence,
	}
	return json.Marshal(txStruct)
}

// PrevOut represents previous output for an input Vin.
type PrevOut struct {
	Addresses    []string `json:"addresses,omitempty"`
	Value        float64  `json:"value"`
	PreciseValue int64    `json:"precise_value"`
}

// VinPrevOut is like Vin except it includes PrevOut.  It is used by searchrawtransaction
type VinPrevOut struct {
	Coinbase  string     `json:"coinbase"`
	Txid      string     `json:"txid"`
	Vout      uint32     `json:"vout"`
	ScriptSig *ScriptSig `json:"scriptSig"`
	Witness   []string   `json:"txinwitness"`
	PrevOut   *PrevOut   `json:"prevOut"`
	Sequence  uint32     `json:"sequence"`
}

// IsCoinBase returns a bool to show if a Vin is a Coinbase one or not.
func (v *VinPrevOut) IsCoinBase() bool {
	return len(v.Coinbase) > 0
}

// HasWitness returns a bool to show if a Vin has any witness data associated
// with it or not.
func (v *VinPrevOut) HasWitness() bool {
	return len(v.Witness) > 0
}

// MarshalJSON provides a custom Marshal method for VinPrevOut.
func (v *VinPrevOut) MarshalJSON() ([]byte, error) {
	if v.IsCoinBase() {
		coinbaseStruct := struct {
			Coinbase string `json:"coinbase"`
			Sequence uint32 `json:"sequence"`
		}{
			Coinbase: v.Coinbase,
			Sequence: v.Sequence,
		}
		return json.Marshal(coinbaseStruct)
	}

	if v.HasWitness() {
		txStruct := struct {
			Txid      string     `json:"txid"`
			Vout      uint32     `json:"vout"`
			ScriptSig *ScriptSig `json:"scriptSig"`
			Witness   []string   `json:"txinwitness"`
			PrevOut   *PrevOut   `json:"prevOut,omitempty"`
			Sequence  uint32     `json:"sequence"`
		}{
			Txid:      v.Txid,
			Vout:      v.Vout,
			ScriptSig: v.ScriptSig,
			Witness:   v.Witness,
			PrevOut:   v.PrevOut,
			Sequence:  v.Sequence,
		}
		return json.Marshal(txStruct)
	}

	txStruct := struct {
		Txid      string     `json:"txid"`
		Vout      uint32     `json:"vout"`
		ScriptSig *ScriptSig `json:"scriptSig"`
		PrevOut   *PrevOut   `json:"prevOut,omitempty"`
		Sequence  uint32     `json:"sequence"`
	}{
		Txid:      v.Txid,
		Vout:      v.Vout,
		ScriptSig: v.ScriptSig,
		PrevOut:   v.PrevOut,
		Sequence:  v.Sequence,
	}
	return json.Marshal(txStruct)
}

// Vout models parts of the tx data.  It is defined separately since both
// getrawtransaction and decoderawtransaction use the same structure.
type Vout struct {
	Value            float64            `json:"value"`
	N                uint32             `json:"n"`
	ScriptPubKey     ScriptPubKeyResult `json:"scriptPubKey"`
	PreciseValue     int64              `json:"precise_value"`
	FromAnotherShard bool               `json:"from_another_shard,omitempty"`
}

// GetMiningInfoResult models the data from the getmininginfo command.
// easyjson:json
type GetMiningInfoResult struct {
	Blocks             int64   `json:"blocks"`
	CurrentBlockSize   uint64  `json:"currentblocksize"`
	CurrentBlockWeight uint64  `json:"currentblockweight"`
	CurrentBlockTx     uint64  `json:"currentblocktx"`
	Difficulty         float64 `json:"difficulty"`
	Errors             string  `json:"errors"`
	NetworkHashPS      int64   `json:"networkhashps"`
	PooledTx           uint64  `json:"pooledtx"`
	TestNet            bool    `json:"testnet"`
}

// GetWorkResult models the data from the getwork command.
type GetWorkResult struct {
	Data     string `json:"data"`
	Hash1    string `json:"hash1"`
	Midstate string `json:"midstate"`
	Target   string `json:"target"`
}

// InfoChainResult models the data returned by the chain server getinfo command.
type InfoChainResult struct {
	Version         int32   `json:"version"`
	ProtocolVersion int32   `json:"protocolversion"`
	Blocks          int32   `json:"blocks"`
	TimeOffset      int64   `json:"timeoffset"`
	Connections     int32   `json:"connections"`
	Proxy           string  `json:"proxy"`
	Difficulty      float64 `json:"difficulty"`
	TestNet         bool    `json:"testnet"`
	RelayFee        float64 `json:"relayfee"`
	Errors          string  `json:"errors"`
}

// TxRawResult models the data from the getrawtransaction command.
// easyjson:json
type TxRawResult struct {
	Hex           string `json:"hex"`
	Txid          string `json:"txid"`
	Hash          string `json:"hash,omitempty"`
	Size          int32  `json:"size,omitempty"`
	Vsize         int32  `json:"vsize,omitempty"`
	Weight        int32  `json:"weight,omitempty"`
	Version       int32  `json:"version"`
	LockTime      uint32 `json:"locktime"`
	Vin           []Vin  `json:"vin"`
	Vout          []Vout `json:"vout"`
	InAmount      int64  `json:"in_amount,omitempty"` // will be not empty only when GetTxDetails
	OutAmount     int64  `json:"out_amount,omitempty"`
	Fee           int64  `json:"fee,omitempty"` // will be not empty only when GetTxDetails
	BlockHash     string `json:"blockhash,omitempty"`
	Confirmations uint64 `json:"confirmations,omitempty"`
	Time          int64  `json:"time,omitempty"`
	Blocktime     int64  `json:"blocktime,omitempty"`
	ChainName     string `json:"chainName"`
	CoinbaseTx    bool   `json:"coinbase_tx"`
	OrphanTx      bool   `json:"orphan_tx"`
}

// SearchRawTransactionsResult models the data from the searchrawtransaction
// command.
// easyjson:json
type SearchRawTransactionsResult struct {
	Hex           string       `json:"hex,omitempty"`
	Txid          string       `json:"txid"`
	Hash          string       `json:"hash"`
	Size          string       `json:"size"`
	Vsize         string       `json:"vsize"`
	Weight        string       `json:"weight"`
	Version       int32        `json:"version"`
	LockTime      uint32       `json:"locktime"`
	Vin           []VinPrevOut `json:"vin"`
	Vout          []Vout       `json:"vout"`
	BlockHash     string       `json:"blockhash,omitempty"`
	Confirmations uint64       `json:"confirmations,omitempty"`
	Time          int64        `json:"time,omitempty"`
	Blocktime     int64        `json:"blocktime,omitempty"`
}

// easyjson:json
type SearchRawTransactionsResults []*SearchRawTransactionsResult

// TxRawDecodeResult models the data from the decoderawtransaction command.
type TxRawDecodeResult struct {
	Txid     string `json:"txid"`
	Version  int32  `json:"version"`
	Locktime uint32 `json:"locktime"`
	Vin      []Vin  `json:"vin"`
	Vout     []Vout `json:"vout"`
}

// ValidateAddressChainResult models the data returned by the chain server
// validateaddress command.
type ValidateAddressChainResult struct {
	IsValid bool   `json:"isvalid"`
	Address string `json:"address,omitempty"`
}

// EstimateSmartFeeResult models the data returned buy the chain server
// estimatesmartfee command
// easyjson:json
type EstimateSmartFeeResult struct {
	BtcPerKB    *float64 `json:"btc_per_kb"`
	SatoshiPerB *float64 `json:"satoshi_per_b"`
	Errors      []string `json:"errors,omitempty"`
	Blocks      int64    `json:"blocks"`
}

// easyjson:json
type Fee struct {
	CoinsPerKB  float64 `json:"coins_per_kb"`
	SatoshiPerB float64 `json:"satoshi_per_b"`
	Blocks      int64   `json:"blocks"`
	Estimated   bool    `json:"estimated"`
}

// easyjson:json
type ExtendedFeeFeeResult struct {
	Fast     Fee `json:"fast"`
	Moderate Fee `json:"moderate"`
	Slow     Fee `json:"slow"`
}

// easyjson:json
type ShardListResult struct {
	Shards map[uint32]ShardInfo `json:"shards"`
}

// easyjson:json
type ShardInfo struct {
	ID                    uint32 `json:"id"`
	LastVersion           int32  `json:"last_version"`
	BeaconExpansionHeight int32  `json:"beacon_expansion_height"`
	BeaconExpansionHash   string `json:"beacon_expansion_hash"`
	GenesisHash           string `json:"genesis_hash"`
	Enabled               bool   `json:"enabled"`
	P2PPort               int    `json:"p2p_port"`
}

// GetLastSerialBlockNumberResult ...
type GetLastSerialBlockNumberResult struct {
	LastSerial int64 `json:"lastserial"`
}

// easyjson:json
type GetBlockResult struct {
	Block        string `json:"block"`
	Height       int32  `json:"height"`
	SerialID     int64  `json:"serial_id"`
	PrevSerialID int64  `json:"prev_serial_id"`
}

// easyjson:json
type GetBlockResults []GetBlockResult

// easyjson:json
type TxOperation struct {
	Input        bool     `json:"input"`        // indicates is this tx input or output
	PkScript     string   `json:"pkScript"`     // hex encoded pkScript
	Addresses    []string `json:"addresses"`    // list of addresses extracted from pkScript
	Idx          uint32   `json:"idx"`          // serial number of input in tx or output if !input
	Amount       int64    `json:"amount"`       // amount of coins in satoshi
	TxHash       string   `json:"txHash"`       // hash of this tx
	TxIndex      uint32   `json:"txIndex"`      // serial number of tx in block
	Coinbase     bool     `json:"coinbase"`     // indicates is this coinbase tx
	OriginTxHash string   `json:"originTxHash"` // not empty if input; tx hash of parent utxo
	OriginIdx    uint32   `json:"originIdx"`    // not empty if input; serial number of output in tx if input

	CSTx    bool   `json:"csTx"`
	ShardID uint32 `json:"shardId"`
}

// easyjson:json
type BlockTxOperations struct {
	BlockHash     string        `json:"blockHash"`
	BlockHeight   int64         `json:"blockHeight"`
	Confirmations int64         `json:"confirmations"`
	Ops           []TxOperation `json:"ops"`
}

// GetBlockTxOpsCmd defines the getblocktxops JSON-RPC command.
type GetBlockTxOpsCmd struct {
	BlockHash string `json:"blockHash"`
}

// easyjson:json
type MempoolUTXO struct {
	UTXOHash      string `json:"utxoHash"`
	UTXOIndex     uint32 `json:"utxoIndex"`
	UsedByTxHash  string `json:"usedByTxHash"`
	UsedByTxIndex uint32 `json:"usedByTxIndex"`
}

// easyjson:json
type MempoolUTXOs []MempoolUTXO

type EstimateLockTimeResult struct {
	NBlocks int64 `json:"nBlocks"`
}

type EstimateSwapLockTimeResult struct {
	NBlocksAtSource int64 `json:"NBlocksAtSource"`
	NBlocksAtDest   int64 `json:"NBlocksAtDest"`
}

// easyjson:json
type GetNodeMetricsResult struct {
	Stats   map[string]float64 `json:"stats"`
	Version string             `json:"version"`
}

// easyjson:json
type GetChainMetricsResult struct {
	ChainStats map[uint32]map[string]float64 `json:"chainStats"`
	NetName    string                        `json:"netName"`
}
