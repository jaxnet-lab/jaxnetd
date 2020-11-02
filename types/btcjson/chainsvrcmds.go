// Copyright (c) 2014-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// NOTE: This file is intended to house the RPC commands that are supported by
// a chain server.

package btcjson

import (
	"encoding/json"
	"fmt"

	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

// AddNodeSubCmd defines the type used in the addnode JSON-RPC command for the
// sub command field.
type AddNodeSubCmd string

const (
	// ANAdd indicates the specified host should be added as a persistent
	// peer.
	ANAdd AddNodeSubCmd = "add"

	// ANRemove indicates the specified peer should be removed.
	ANRemove AddNodeSubCmd = "remove"

	// ANOneTry indicates the specified host should try to connect once,
	// but it should not be made persistent.
	ANOneTry AddNodeSubCmd = "onetry"
)

// AddNodeCmd defines the addnode JSON-RPC command.
type AddNodeCmd struct {
	Addr   string
	SubCmd AddNodeSubCmd `jsonrpcusage:"\"add|remove|onetry\""`
}

// NewAddNodeCmd returns a new instance which can be used to issue an addnode
// JSON-RPC command.
func NewAddNodeCmd(addr string, subCmd AddNodeSubCmd) *AddNodeCmd {
	return &AddNodeCmd{
		Addr:   addr,
		SubCmd: subCmd,
	}
}

// TransactionInput represents the inputs to a transaction.  Specifically a
// transaction hash and output number pair.
type TransactionInput struct {
	Txid string `json:"txid"`
	Vout uint32 `json:"vout"`
}

// CreateRawTransactionCmd defines the createrawtransaction JSON-RPC command.
type CreateRawTransactionCmd struct {
	Inputs   []TransactionInput
	Amounts  map[string]float64 `jsonrpcusage:"{\"address\":amount,...}"` // In BTC
	LockTime *int64
}

// NewCreateRawTransactionCmd returns a new instance which can be used to issue
// a createrawtransaction JSON-RPC command.
//
// Amounts are in BTC. Passing in nil and the empty slice as inputs is equivalent,
// both gets interpreted as the empty slice.
func NewCreateRawTransactionCmd(inputs []TransactionInput, amounts map[string]float64,
	lockTime *int64) *CreateRawTransactionCmd {
	// to make sure we're serializing this to the empty list and not null, we
	// explicitly initialize the list
	if inputs == nil {
		inputs = []TransactionInput{}
	}
	return &CreateRawTransactionCmd{
		Inputs:   inputs,
		Amounts:  amounts,
		LockTime: lockTime,
	}
}

// DecodeRawTransactionCmd defines the decoderawtransaction JSON-RPC command.
type DecodeRawTransactionCmd struct {
	HexTx string
}

// NewDecodeRawTransactionCmd returns a new instance which can be used to issue
// a decoderawtransaction JSON-RPC command.
func NewDecodeRawTransactionCmd(hexTx string) *DecodeRawTransactionCmd {
	return &DecodeRawTransactionCmd{
		HexTx: hexTx,
	}
}

// DecodeScriptCmd defines the decodescript JSON-RPC command.
type DecodeScriptCmd struct {
	HexScript string
}

// NewDecodeScriptCmd returns a new instance which can be used to issue a
// decodescript JSON-RPC command.
func NewDecodeScriptCmd(hexScript string) *DecodeScriptCmd {
	return &DecodeScriptCmd{
		HexScript: hexScript,
	}
}

// GetAddedNodeInfoCmd defines the getaddednodeinfo JSON-RPC command.
type GetAddedNodeInfoCmd struct {
	DNS  bool
	Node *string
}

// NewGetAddedNodeInfoCmd returns a new instance which can be used to issue a
// getaddednodeinfo JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetAddedNodeInfoCmd(dns bool, node *string) *GetAddedNodeInfoCmd {
	return &GetAddedNodeInfoCmd{
		DNS:  dns,
		Node: node,
	}
}

// GetBestBlockHashCmd defines the getbestblockhash JSON-RPC command.
type GetBestBlockHashCmd struct{}

// NewGetBestBlockHashCmd returns a new instance which can be used to issue a
// getbestblockhash JSON-RPC command.
func NewGetBestBlockHashCmd() *GetBestBlockHashCmd {
	return &GetBestBlockHashCmd{}
}

// GetBeaconBlockCmd defines the getBeaconBlock JSON-RPC command.
type GetBeaconBlockCmd struct {
	Hash      string
	Verbosity *int `jsonrpcdefault:"1"`
}

// NewGetBeaconBlockCmd returns a new instance which can be used to issue a getBeaconBlock
// JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetBeaconBlockCmd(hash string, verbosity *int) *GetBeaconBlockCmd {
	return &GetBeaconBlockCmd{
		Hash:      hash,
		Verbosity: verbosity,
	}
}

// GetShardBlockCmd defines the getShardBlock JSON-RPC command.
type GetShardBlockCmd struct {
	Hash      string
	Verbosity *int `jsonrpcdefault:"1"`
}

// NewGetShardBlockCmd returns a new instance which can be used to issue a getShardBlock
// JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetShardBlockCmd(hash string, verbosity *int) *GetShardBlockCmd {
	return &GetShardBlockCmd{
		Hash:      hash,
		Verbosity: verbosity,
	}
}

// GetBlockChainInfoCmd defines the getblockchaininfo JSON-RPC command.
type GetBlockChainInfoCmd struct{}

// NewGetBlockChainInfoCmd returns a new instance which can be used to issue a
// getblockchaininfo JSON-RPC command.
func NewGetBlockChainInfoCmd() *GetBlockChainInfoCmd {
	return &GetBlockChainInfoCmd{}
}

// GetBlockCountCmd defines the getblockcount JSON-RPC command.
type GetBlockCountCmd struct{}

// NewGetBlockCountCmd returns a new instance which can be used to issue a
// getblockcount JSON-RPC command.
func NewGetBlockCountCmd() *GetBlockCountCmd {
	return &GetBlockCountCmd{}
}

// GetBlockHashCmd defines the getblockhash JSON-RPC command.
type GetBlockHashCmd struct {
	Index int64
}

// NewGetBlockHashCmd returns a new instance which can be used to issue a
// getblockhash JSON-RPC command.
func NewGetBlockHashCmd(index int64) *GetBlockHashCmd {
	return &GetBlockHashCmd{
		Index: index,
	}
}

// GetBeaconBlockHeaderCmd defines the getBeaconBlockHeader JSON-RPC command.
type GetBeaconBlockHeaderCmd struct {
	Hash    string
	Verbose *bool `jsonrpcdefault:"true"`
}

// NewGetBeaconBlockHeaderCmd returns a new instance which can be used to issue a
// getBeaconBlockHeader JSON-RPC command.
func NewGetBeaconBlockHeaderCmd(hash string, verbose *bool) *GetBeaconBlockHeaderCmd {
	return &GetBeaconBlockHeaderCmd{
		Hash:    hash,
		Verbose: verbose,
	}
}

// GetShardBlockHeaderCmd defines the getShardBlockHeader JSON-RPC command.
type GetShardBlockHeaderCmd struct {
	Hash    string
	Verbose *bool `jsonrpcdefault:"true"`
}

// NewGetShardBlockHeaderCmd returns a new instance which can be used to issue a
// getShardBlockHeader JSON-RPC command.
func NewGetShardBlockHeaderCmd(hash string, verbose *bool) *GetShardBlockHeaderCmd {
	return &GetShardBlockHeaderCmd{
		Hash:    hash,
		Verbose: verbose,
	}
}

// HashOrHeight defines a type that can be used as hash_or_height value in JSON-RPC commands.
type HashOrHeight struct {
	Value interface{}
}

// MarshalJSON implements the json.Marshaler interface
func (h HashOrHeight) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.Value)
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (h *HashOrHeight) UnmarshalJSON(data []byte) error {
	var unmarshalled interface{}
	if err := json.Unmarshal(data, &unmarshalled); err != nil {
		return err
	}

	switch v := unmarshalled.(type) {
	case float64:
		h.Value = int(v)
	case string:
		h.Value = v
	default:
		return fmt.Errorf("invalid hash_or_height value: %v", unmarshalled)
	}

	return nil
}

// GetBlockStatsCmd defines the getblockstats JSON-RPC command.
type GetBlockStatsCmd struct {
	HashOrHeight HashOrHeight
	Stats        *[]string
}

// GetBlockStatsCmd defines the getblockstats JSON-RPC command.
type GetChainStatsCmd struct {
	HashOrHeight HashOrHeight
	Stats        *[]string
}

// NewGetBlockStatsCmd returns a new instance which can be used to issue a
// getblockstats JSON-RPC command. Either height or hash must be specified.
func NewGetBlockStatsCmd(hashOrHeight HashOrHeight, stats *[]string) *GetBlockStatsCmd {
	return &GetBlockStatsCmd{
		HashOrHeight: hashOrHeight,
		Stats:        stats,
	}
}

// TemplateRequest is a request object as defined in BIP22
// (https://en.bitcoin.it/wiki/BIP_0022), it is optionally provided as an
// pointer argument to GetBeaconBlockTemplateCmd.
type TemplateRequest struct {
	Mode         string   `json:"mode,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`

	// Optional long polling.
	LongPollID string `json:"longpollid,omitempty"`

	// Optional template tweaking.  SigOpLimit and SizeLimit can be int64
	// or bool.
	SigOpLimit interface{} `json:"sigoplimit,omitempty"`
	SizeLimit  interface{} `json:"sizelimit,omitempty"`
	MaxVersion uint32      `json:"maxversion,omitempty"`

	// Basic pool extension from BIP 0023.
	Target string `json:"target,omitempty"`

	// Block proposal from BIP 0023.  Data is only provided when Mode is
	// "proposal".
	Data   string `json:"data,omitempty"`
	WorkID string `json:"workid,omitempty"`
}

// convertTemplateRequestField potentially converts the provided value as
// needed.
func convertTemplateRequestField(fieldName string, iface interface{}) (interface{}, error) {
	switch val := iface.(type) {
	case nil:
		return nil, nil
	case bool:
		return val, nil
	case float64:
		if val == float64(int64(val)) {
			return int64(val), nil
		}
	}

	str := fmt.Sprintf("the %s field must be unspecified, a boolean, or "+
		"a 64-bit integer", fieldName)
	return nil, makeError(ErrInvalidType, str)
}

// UnmarshalJSON provides a custom Unmarshal method for TemplateRequest.  This
// is necessary because the SigOpLimit and SizeLimit fields can only be specific
// types.
func (t *TemplateRequest) UnmarshalJSON(data []byte) error {
	type templateRequest TemplateRequest

	request := (*templateRequest)(t)
	if err := json.Unmarshal(data, &request); err != nil {
		return err
	}

	// The SigOpLimit field can only be nil, bool, or int64.
	val, err := convertTemplateRequestField("sigoplimit", request.SigOpLimit)
	if err != nil {
		return err
	}
	request.SigOpLimit = val

	// The SizeLimit field can only be nil, bool, or int64.
	val, err = convertTemplateRequestField("sizelimit", request.SizeLimit)
	if err != nil {
		return err
	}
	request.SizeLimit = val

	return nil
}

// GetBeaconBlockTemplateCmd defines the getBeaconBlockTemplate JSON-RPC command.
type GetBeaconBlockTemplateCmd struct {
	Request *TemplateRequest
}

// NewGetBeaconBlockTemplateCmd returns a new instance which can be used to issue a
// getblocktemplate JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetBeaconBlockTemplateCmd(request *TemplateRequest) *GetBeaconBlockTemplateCmd {
	return &GetBeaconBlockTemplateCmd{
		Request: request,
	}
}

// GetShardBlockTemplateCmd defines the getShardBlockTemplate JSON-RPC command.
type GetShardBlockTemplateCmd struct {
	Request *TemplateRequest
}

// NewGetShardBlockTemplateCmd returns a new instance which can be used to issue a
// getblocktemplate JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetShardBlockTemplateCmd(request *TemplateRequest) *GetShardBlockTemplateCmd {
	return &GetShardBlockTemplateCmd{
		Request: request,
	}
}

// GetCFilterCmd defines the getcfilter JSON-RPC command.
type GetCFilterCmd struct {
	Hash       string
	FilterType wire.FilterType
}

// NewGetCFilterCmd returns a new instance which can be used to issue a
// getcfilter JSON-RPC command.
func NewGetCFilterCmd(hash string, filterType wire.FilterType) *GetCFilterCmd {
	return &GetCFilterCmd{
		Hash:       hash,
		FilterType: filterType,
	}
}

// GetCFilterHeaderCmd defines the getcfilterheader JSON-RPC command.
type GetCFilterHeaderCmd struct {
	Hash       string
	FilterType wire.FilterType
}

// NewGetCFilterHeaderCmd returns a new instance which can be used to issue a
// getcfilterheader JSON-RPC command.
func NewGetCFilterHeaderCmd(hash string,
	filterType wire.FilterType) *GetCFilterHeaderCmd {
	return &GetCFilterHeaderCmd{
		Hash:       hash,
		FilterType: filterType,
	}
}

// GetChainTipsCmd defines the getchaintips JSON-RPC command.
type GetChainTipsCmd struct{}

// NewGetChainTipsCmd returns a new instance which can be used to issue a
// getchaintips JSON-RPC command.
func NewGetChainTipsCmd() *GetChainTipsCmd {
	return &GetChainTipsCmd{}
}

// GetConnectionCountCmd defines the getconnectioncount JSON-RPC command.
type GetConnectionCountCmd struct{}

// NewGetConnectionCountCmd returns a new instance which can be used to issue a
// getconnectioncount JSON-RPC command.
func NewGetConnectionCountCmd() *GetConnectionCountCmd {
	return &GetConnectionCountCmd{}
}

// GetDifficultyCmd defines the getdifficulty JSON-RPC command.
type GetDifficultyCmd struct{}

// NewGetDifficultyCmd returns a new instance which can be used to issue a
// getdifficulty JSON-RPC command.
func NewGetDifficultyCmd() *GetDifficultyCmd {
	return &GetDifficultyCmd{}
}

// GetGenerateCmd defines the getgenerate JSON-RPC command.
type GetGenerateCmd struct{}

// NewGetGenerateCmd returns a new instance which can be used to issue a
// getgenerate JSON-RPC command.
func NewGetGenerateCmd() *GetGenerateCmd {
	return &GetGenerateCmd{}
}

// GetHashesPerSecCmd defines the gethashespersec JSON-RPC command.
type GetHashesPerSecCmd struct{}

// NewGetHashesPerSecCmd returns a new instance which can be used to issue a
// gethashespersec JSON-RPC command.
func NewGetHashesPerSecCmd() *GetHashesPerSecCmd {
	return &GetHashesPerSecCmd{}
}

// GetInfoCmd defines the getinfo JSON-RPC command.
type GetInfoCmd struct{}

// NewGetInfoCmd returns a new instance which can be used to issue a
// getinfo JSON-RPC command.
func NewGetInfoCmd() *GetInfoCmd {
	return &GetInfoCmd{}
}

// GetMempoolEntryCmd defines the getmempoolentry JSON-RPC command.
type GetMempoolEntryCmd struct {
	TxID string
}

// NewGetMempoolEntryCmd returns a new instance which can be used to issue a
// getmempoolentry JSON-RPC command.
func NewGetMempoolEntryCmd(txHash string) *GetMempoolEntryCmd {
	return &GetMempoolEntryCmd{
		TxID: txHash,
	}
}

// GetMempoolInfoCmd defines the getmempoolinfo JSON-RPC command.
type GetMempoolInfoCmd struct{}

// NewGetMempoolInfoCmd returns a new instance which can be used to issue a
// getmempool JSON-RPC command.
func NewGetMempoolInfoCmd() *GetMempoolInfoCmd {
	return &GetMempoolInfoCmd{}
}

// GetMiningInfoCmd defines the getmininginfo JSON-RPC command.
type GetMiningInfoCmd struct{}

// NewGetMiningInfoCmd returns a new instance which can be used to issue a
// getmininginfo JSON-RPC command.
func NewGetMiningInfoCmd() *GetMiningInfoCmd {
	return &GetMiningInfoCmd{}
}

// GetNetworkInfoCmd defines the getnetworkinfo JSON-RPC command.
type GetNetworkInfoCmd struct {
	// Subversion string `json:"subversion"`
}

// NewGetNetworkInfoCmd returns a new instance which can be used to issue a
// getnetworkinfo JSON-RPC command.
func NewGetNetworkInfoCmd() *GetNetworkInfoCmd {
	return &GetNetworkInfoCmd{
		// Subversion: "1.3.0",
	}
}

// GetNetTotalsCmd defines the getnettotals JSON-RPC command.
type GetNetTotalsCmd struct{}

// NewGetNetTotalsCmd returns a new instance which can be used to issue a
// getnettotals JSON-RPC command.
func NewGetNetTotalsCmd() *GetNetTotalsCmd {
	return &GetNetTotalsCmd{}
}

// GetNetworkHashPSCmd defines the getnetworkhashps JSON-RPC command.
type GetNetworkHashPSCmd struct {
	Blocks *int `jsonrpcdefault:"120"`
	Height *int `jsonrpcdefault:"-1"`
}

// NewGetNetworkHashPSCmd returns a new instance which can be used to issue a
// getnetworkhashps JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetNetworkHashPSCmd(numBlocks, height *int) *GetNetworkHashPSCmd {
	return &GetNetworkHashPSCmd{
		Blocks: numBlocks,
		Height: height,
	}
}

// GetPeerInfoCmd defines the getpeerinfo JSON-RPC command.
type GetPeerInfoCmd struct{}

// NewGetPeerInfoCmd returns a new instance which can be used to issue a getpeer
// JSON-RPC command.
func NewGetPeerInfoCmd() *GetPeerInfoCmd {
	return &GetPeerInfoCmd{}
}

// GetRawMempoolCmd defines the getmempool JSON-RPC command.
type GetRawMempoolCmd struct {
	Verbose *bool `jsonrpcdefault:"false"`
}

// NewGetRawMempoolCmd returns a new instance which can be used to issue a
// getrawmempool JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetRawMempoolCmd(verbose *bool) *GetRawMempoolCmd {
	return &GetRawMempoolCmd{
		Verbose: verbose,
	}
}

// GetRawTransactionCmd defines the getrawtransaction JSON-RPC command.
//
// NOTE: This field is an int versus a bool to remain compatible with Bitcoin
// Core even though it really should be a bool.
type GetRawTransactionCmd struct {
	Txid    string
	Verbose *int `jsonrpcdefault:"0"`
}

// NewGetRawTransactionCmd returns a new instance which can be used to issue a
// getrawtransaction JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetRawTransactionCmd(txHash string, verbose *int) *GetRawTransactionCmd {
	return &GetRawTransactionCmd{
		Txid:    txHash,
		Verbose: verbose,
	}
}

// GetTxOutCmd defines the gettxout JSON-RPC command.
type GetTxOutCmd struct {
	Txid           string
	Vout           uint32
	IncludeMempool *bool `jsonrpcdefault:"true"`
}

// NewGetTxOutCmd returns a new instance which can be used to issue a gettxout
// JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetTxOutCmd(txHash string, vout uint32, includeMempool *bool) *GetTxOutCmd {
	return &GetTxOutCmd{
		Txid:           txHash,
		Vout:           vout,
		IncludeMempool: includeMempool,
	}
}

// GetTxOutProofCmd defines the gettxoutproof JSON-RPC command.
type GetTxOutProofCmd struct {
	TxIDs     []string
	BlockHash *string
}

// NewGetTxOutProofCmd returns a new instance which can be used to issue a
// gettxoutproof JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetTxOutProofCmd(txIDs []string, blockHash *string) *GetTxOutProofCmd {
	return &GetTxOutProofCmd{
		TxIDs:     txIDs,
		BlockHash: blockHash,
	}
}

// GetTxOutSetInfoCmd defines the gettxoutsetinfo JSON-RPC command.
type GetTxOutSetInfoCmd struct{}

// NewGetTxOutSetInfoCmd returns a new instance which can be used to issue a
// gettxoutsetinfo JSON-RPC command.
func NewGetTxOutSetInfoCmd() *GetTxOutSetInfoCmd {
	return &GetTxOutSetInfoCmd{}
}

// GetWorkCmd defines the getwork JSON-RPC command.
type GetWorkCmd struct {
	Data *string
}

// NewGetWorkCmd returns a new instance which can be used to issue a getwork
// JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewGetWorkCmd(data *string) *GetWorkCmd {
	return &GetWorkCmd{
		Data: data,
	}
}

// HelpCmd defines the help JSON-RPC command.
type HelpCmd struct {
	Scope   *string
	Command *string
}

// NewHelpCmd returns a new instance which can be used to issue a help JSON-RPC
// command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewHelpCmd(command *string) *HelpCmd {
	return &HelpCmd{
		Command: command,
	}
}

// InvalidateBlockCmd defines the invalidateblock JSON-RPC command.
type InvalidateBlockCmd struct {
	BlockHash string
}

// NewInvalidateBlockCmd returns a new instance which can be used to issue a
// invalidateblock JSON-RPC command.
func NewInvalidateBlockCmd(blockHash string) *InvalidateBlockCmd {
	return &InvalidateBlockCmd{
		BlockHash: blockHash,
	}
}

// PingCmd defines the ping JSON-RPC command.
type PingCmd struct{}

// NewPingCmd returns a new instance which can be used to issue a ping JSON-RPC
// command.
func NewPingCmd() *PingCmd {
	return &PingCmd{}
}

// PreciousBlockCmd defines the preciousblock JSON-RPC command.
type PreciousBlockCmd struct {
	BlockHash string
}

// NewPreciousBlockCmd returns a new instance which can be used to issue a
// preciousblock JSON-RPC command.
func NewPreciousBlockCmd(blockHash string) *PreciousBlockCmd {
	return &PreciousBlockCmd{
		BlockHash: blockHash,
	}
}

// ReconsiderBlockCmd defines the reconsiderblock JSON-RPC command.
type ReconsiderBlockCmd struct {
	BlockHash string
}

// NewReconsiderBlockCmd returns a new instance which can be used to issue a
// reconsiderblock JSON-RPC command.
func NewReconsiderBlockCmd(blockHash string) *ReconsiderBlockCmd {
	return &ReconsiderBlockCmd{
		BlockHash: blockHash,
	}
}

// SearchRawTransactionsCmd defines the searchrawtransactions JSON-RPC command.
type SearchRawTransactionsCmd struct {
	Address     string
	Verbose     *int  `jsonrpcdefault:"1"`
	Skip        *int  `jsonrpcdefault:"0"`
	Count       *int  `jsonrpcdefault:"100"`
	VinExtra    *int  `jsonrpcdefault:"0"`
	Reverse     *bool `jsonrpcdefault:"false"`
	FilterAddrs *[]string
}

// NewSearchRawTransactionsCmd returns a new instance which can be used to issue a
// sendrawtransaction JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewSearchRawTransactionsCmd(address string, verbose, skip, count *int, vinExtra *int, reverse *bool, filterAddrs *[]string) *SearchRawTransactionsCmd {
	return &SearchRawTransactionsCmd{
		Address:     address,
		Verbose:     verbose,
		Skip:        skip,
		Count:       count,
		VinExtra:    vinExtra,
		Reverse:     reverse,
		FilterAddrs: filterAddrs,
	}
}

// SendRawTransactionCmd defines the sendrawtransaction JSON-RPC command.
type SendRawTransactionCmd struct {
	HexTx         string
	AllowHighFees *bool `jsonrpcdefault:"false"`
	MaxFeeRate    *int32
}

// NewSendRawTransactionCmd returns a new instance which can be used to issue a
// sendrawtransaction JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewSendRawTransactionCmd(hexTx string, allowHighFees *bool) *SendRawTransactionCmd {
	return &SendRawTransactionCmd{
		HexTx:         hexTx,
		AllowHighFees: allowHighFees,
	}
}

// NewSendRawTransactionCmd returns a new instance which can be used to issue a
// sendrawtransaction JSON-RPC command to a bitcoind node.
//
// A 0 maxFeeRate indicates that a maximum fee rate won't be enforced.
func NewBitcoindSendRawTransactionCmd(hexTx string, maxFeeRate int32) *SendRawTransactionCmd {
	return &SendRawTransactionCmd{
		HexTx:      hexTx,
		MaxFeeRate: &maxFeeRate,
	}
}

// SetGenerateCmd defines the setgenerate JSON-RPC command.
type SetGenerateCmd struct {
	Generate     bool
	GenProcLimit *int `jsonrpcdefault:"-1"`
}

// NewSetGenerateCmd returns a new instance which can be used to issue a
// setgenerate JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewSetGenerateCmd(generate bool, genProcLimit *int) *SetGenerateCmd {
	return &SetGenerateCmd{
		Generate:     generate,
		GenProcLimit: genProcLimit,
	}
}

// StopCmd defines the stop JSON-RPC command.
type StopCmd struct{}

// NewStopCmd returns a new instance which can be used to issue a stop JSON-RPC
// command.
func NewStopCmd() *StopCmd {
	return &StopCmd{}
}

// SubmitBlockOptions represents the optional options struct provided with a
// SubmitBlockCmd command.
type SubmitBlockOptions struct {
	// must be provided if server provided a workid with template.
	WorkID string `json:"workid,omitempty"`
}

// SubmitBlockCmd defines the submitblock JSON-RPC command.
type SubmitBlockCmd struct {
	HexBlock string
	Options  *SubmitBlockOptions
}

// NewSubmitBlockCmd returns a new instance which can be used to issue a
// submitblock JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewSubmitBlockCmd(hexBlock string, options *SubmitBlockOptions) *SubmitBlockCmd {
	return &SubmitBlockCmd{
		HexBlock: hexBlock,
		Options:  options,
	}
}

// UptimeCmd defines the uptime JSON-RPC command.
type UptimeCmd struct{}

// NewUptimeCmd returns a new instance which can be used to issue an uptime JSON-RPC command.
func NewUptimeCmd() *UptimeCmd {
	return &UptimeCmd{}
}

// ValidateAddressCmd defines the validateaddress JSON-RPC command.
type ValidateAddressCmd struct {
	Address string
}

// NewValidateAddressCmd returns a new instance which can be used to issue a
// validateaddress JSON-RPC command.
func NewValidateAddressCmd(address string) *ValidateAddressCmd {
	return &ValidateAddressCmd{
		Address: address,
	}
}

// VerifyChainCmd defines the verifychain JSON-RPC command.
type VerifyChainCmd struct {
	CheckLevel *int32 `jsonrpcdefault:"3"`
	CheckDepth *int32 `jsonrpcdefault:"288"` // 0 = all
}

// NewVerifyChainCmd returns a new instance which can be used to issue a
// verifychain JSON-RPC command.
//
// The parameters which are pointers indicate they are optional.  Passing nil
// for optional parameters will use the default value.
func NewVerifyChainCmd(checkLevel, checkDepth *int32) *VerifyChainCmd {
	return &VerifyChainCmd{
		CheckLevel: checkLevel,
		CheckDepth: checkDepth,
	}
}

// VerifyMessageCmd defines the verifymessage JSON-RPC command.
type VerifyMessageCmd struct {
	Address   string
	Signature string
	Message   string
}

// NewVerifyMessageCmd returns a new instance which can be used to issue a
// verifymessage JSON-RPC command.
func NewVerifyMessageCmd(address, signature, message string) *VerifyMessageCmd {
	return &VerifyMessageCmd{
		Address:   address,
		Signature: signature,
		Message:   message,
	}
}

// VerifyTxOutProofCmd defines the verifytxoutproof JSON-RPC command.
type VerifyTxOutProofCmd struct {
	Proof string
}

// NewVerifyTxOutProofCmd returns a new instance which can be used to issue a
// verifytxoutproof JSON-RPC command.
func NewVerifyTxOutProofCmd(proof string) *VerifyTxOutProofCmd {
	return &VerifyTxOutProofCmd{
		Proof: proof,
	}
}

type ManageShardsCmd struct {
	ShardID uint32
	Action  string
}

func NewManageShardsCmd(shardID uint32, action string, initialHeight *int32) *ManageShardsCmd {
	return &ManageShardsCmd{
		ShardID: shardID,
		Action:  action,
	}
}

type ListShardsCmd struct{}

func init() {
	// No special flags for commands in this file.
	flags := UsageFlag(0)

	// ---- chain rpc commands -----------------------------------------------------------------------------------------
	MustRegisterCmd("chain", "addNode", (*AddNodeCmd)(nil), flags)
	MustRegisterCmd("chain", "getAddedNodeInfo", (*GetAddedNodeInfoCmd)(nil), flags)
	MustRegisterCmd("chain", "getConnectionCount", (*GetConnectionCountCmd)(nil), flags)
	MustRegisterCmd("chain", "getNetTotals", (*GetNetTotalsCmd)(nil), flags)
	MustRegisterCmd("chain", "getPeerInfo", (*GetPeerInfoCmd)(nil), flags)
	MustRegisterCmd("chain", "node", (*NodeCmd)(nil), flags)
	MustRegisterCmd("chain", "ping", (*PingCmd)(nil), flags)

	MustRegisterCmd("chain", "decodeScript", (*DecodeScriptCmd)(nil), flags)
	MustRegisterCmd("chain", "validateAddress", (*ValidateAddressCmd)(nil), flags)
	MustRegisterCmd("chain", "verifyChain", (*VerifyChainCmd)(nil), flags)
	MustRegisterCmd("chain", "verifyMessage", (*VerifyMessageCmd)(nil), flags)
	MustRegisterCmd("chain", "getCurrentNet", (*GetCurrentNetCmd)(nil), flags)

	MustRegisterCmd("chain", "createRawTransaction", (*CreateRawTransactionCmd)(nil), flags)
	MustRegisterCmd("chain", "decodeRawTransaction", (*DecodeRawTransactionCmd)(nil), flags)
	MustRegisterCmd("chain", "estimateFee", (*EstimateFeeCmd)(nil), flags)
	MustRegisterCmd("chain", "estimatesmartfee", (*EstimateSmartFeeCmd)(nil), flags)
	MustRegisterCmd("chain", "getmempoolinfo", (*GetMempoolInfoCmd)(nil), flags)
	MustRegisterCmd("chain", "getrawmempool", (*GetRawMempoolCmd)(nil), flags)
	MustRegisterCmd("chain", "getrawtransaction", (*GetRawTransactionCmd)(nil), flags)
	MustRegisterCmd("chain", "getTxOut", (*GetTxOutCmd)(nil), flags)
	MustRegisterCmd("chain", "searchRawTransactions", (*SearchRawTransactionsCmd)(nil), flags)
	MustRegisterCmd("chain", "sendRawTransaction", (*SendRawTransactionCmd)(nil), flags)

	MustRegisterCmd("chain", "getBestBlock", (*GetBestBlockCmd)(nil), flags)
	MustRegisterCmd("chain", "getBestBlockHash", (*GetBestBlockHashCmd)(nil), flags)
	MustRegisterCmd("chain", "getblockchaininfo", (*GetBlockChainInfoCmd)(nil), flags)
	MustRegisterCmd("chain", "getblockcount", (*GetBlockCountCmd)(nil), flags)
	MustRegisterCmd("chain", "getblockhash", (*GetBlockHashCmd)(nil), flags)
	MustRegisterCmd("chain", "getCFilter", (*GetCFilterCmd)(nil), flags)
	MustRegisterCmd("chain", "getCFilterHeader", (*GetCFilterHeaderCmd)(nil), flags)
	MustRegisterCmd("chain", "submitBlock", (*SubmitBlockCmd)(nil), flags)

	// ---- node rpc commands ------------------------------------------------------------------------------------------
	MustRegisterCmd("node", "version", (*VersionCmd)(nil), flags)
	MustRegisterCmd("node", "getInfo", (*GetInfoCmd)(nil), flags)
	MustRegisterCmd("node", "getnetworkinfo", (*GetNetworkInfoCmd)(nil), flags)
	MustRegisterCmd("node", "uptime", (*UptimeCmd)(nil), flags)

	MustRegisterCmd("node", "manageShards", (*ManageShardsCmd)(nil), flags)
	MustRegisterCmd("node", "listShards", (*ListShardsCmd)(nil), flags)

	MustRegisterCmd("node", "generate", (*GenerateCmd)(nil), flags)
	MustRegisterCmd("node", "getDifficulty", (*GetDifficultyCmd)(nil), flags)
	MustRegisterCmd("node", "getmininginfo", (*GetMiningInfoCmd)(nil), flags)
	MustRegisterCmd("node", "getnetworkhashps", (*GetNetworkHashPSCmd)(nil), flags)

	MustRegisterCmd("node", "setGenerate", (*SetGenerateCmd)(nil), flags)
	MustRegisterCmd("node", "getBlockStats", (*GetBlockStatsCmd)(nil), flags)
	MustRegisterCmd("node", "getchaintxstats", (*GetChainStatsCmd)(nil), flags)
	MustRegisterCmd("node", "debugLevel", (*DebugLevelCmd)(nil), flags)
	MustRegisterCmd("node", "help", (*HelpCmd)(nil), flags)
	MustRegisterCmd("node", "stop", (*StopCmd)(nil), flags)

	// ---- beacon rpc commands ----------------------------------------------------------------------------------------
	MustRegisterCmd("beacon", "getBeaconBlock", (*GetBeaconBlockCmd)(nil), flags)
	MustRegisterCmd("beacon", "getBeaconBlockHeader", (*GetBeaconBlockHeaderCmd)(nil), flags)
	MustRegisterCmd("beacon", "getblockheader", (*GetBeaconBlockHeaderCmd)(nil), flags)
	MustRegisterCmd("beacon", "getBeaconBlockTemplate", (*GetBeaconBlockTemplateCmd)(nil), flags)
	MustRegisterCmd("beacon", "getBeaconHeaders", (*GetBeaconHeadersCmd)(nil), flags)

	// ---- shard rpc commands -----------------------------------------------------------------------------------------
	MustRegisterCmd("shard", "getShardBlock", (*GetShardBlockCmd)(nil), flags)
	MustRegisterCmd("shard", "getShardBlockHeader", (*GetShardBlockHeaderCmd)(nil), flags)
	MustRegisterCmd("shard", "getshardblockheader", (*GetShardBlockHeaderCmd)(nil), flags)
	MustRegisterCmd("shard", "getShardBlockTemplate", (*GetShardBlockTemplateCmd)(nil), flags)
	MustRegisterCmd("shard", "getShardHeaders", (*GetShardHeadersCmd)(nil), flags)

	// ---- NOT IMPLEMENTED --------------------------------------------------------------------------------------------
	MustRegisterCmd("chain", "getchaintips", (*GetChainTipsCmd)(nil), flags)
	MustRegisterCmd("chain", "gethashespersec", (*GetHashesPerSecCmd)(nil), flags)
	MustRegisterCmd("chain", "getmempoolentry", (*GetMempoolEntryCmd)(nil), flags)
	MustRegisterCmd("chain", "gettxoutproof", (*GetTxOutProofCmd)(nil), flags)
	MustRegisterCmd("chain", "gettxoutsetinfo", (*GetTxOutSetInfoCmd)(nil), flags)
	MustRegisterCmd("chain", "getwork", (*GetWorkCmd)(nil), flags)
	MustRegisterCmd("chain", "invalidateblock", (*InvalidateBlockCmd)(nil), flags)
	MustRegisterCmd("chain", "preciousblock", (*PreciousBlockCmd)(nil), flags)
	MustRegisterCmd("chain", "reconsiderblock", (*ReconsiderBlockCmd)(nil), flags)
	MustRegisterCmd("chain", "verifytxoutproof", (*VerifyTxOutProofCmd)(nil), flags)

}
