// Copyright (c) 2014-2017 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcclient

import (
	"encoding/hex"
	"encoding/json"
	"errors"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// FutureGetBestBlockHashResult is a future promise to deliver the result of a
// GetBestBlockAsync RPC invocation (or an applicable error).
type FutureGetBestBlockHashResult chan *response

// Receive waits for the response promised by the future and returns the hash of
// the best block in the longest block chain.
func (r FutureGetBestBlockHashResult) Receive() (*chainhash.Hash, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var txHashStr string
	err = json.Unmarshal(res, &txHashStr)
	if err != nil {
		return nil, err
	}
	return chainhash.NewHashFromStr(txHashStr)
}

// GetBestBlockHashAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetBestBlockHash for the blocking version and more details.
func (c *Client) GetBestBlockHashAsync() FutureGetBestBlockHashResult {
	cmd := jaxjson.NewGetBestBlockHashCmd()
	return c.sendCmd(cmd)
}

// GetBestBlockHash returns the hash of the best block in the longest block
// chain.
func (c *Client) GetBestBlockHash() (*chainhash.Hash, error) {
	return c.GetBestBlockHashAsync().Receive()
}

// legacyGetBlockRequest constructs and sends a legacy getblock request which
// contains two separate bools to denote verbosity, in contract to a single int
// parameter.
func (c *Client) legacyGetBlockRequest(cmd, hash string, verbose,
	verboseTx bool) ([]byte, error) {

	hashJSON, err := json.Marshal(hash)
	if err != nil {
		return nil, err
	}
	verboseJSON, err := json.Marshal(jaxjson.Bool(verbose))
	if err != nil {
		return nil, err
	}
	verboseTxJSON, err := json.Marshal(jaxjson.Bool(verboseTx))
	if err != nil {
		return nil, err
	}
	return c.RawRequest(cmd, []json.RawMessage{
		hashJSON, verboseJSON, verboseTxJSON,
	})
}

// waitForGetBlockRes waits for the response of a getblock request. If the
// response indicates an invalid parameter was provided, a legacy style of the
// request is resent and its response is returned instead.
func (c *Client) waitForGetBlockRes(respChan chan *response, cmd, hash string,
	verbose, verboseTx bool) ([]byte, error) {

	res, err := receiveFuture(respChan)

	// If we receive an invalid parameter error, then we may be
	// communicating with a jaxnetd node which only understands the legacy
	// request, so we'll try that.
	if err, ok := err.(*jaxjson.RPCError); ok &&
		err.Code == jaxjson.ErrRPCInvalidParams.Code {
		return c.legacyGetBlockRequest(cmd, hash, verbose, verboseTx)
	}

	// Otherwise, we can return the response as is.
	return res, err
}

// FutureGetBlockCountResult is a future promise to deliver the result of a
// GetBlockCountAsync RPC invocation (or an applicable error).
type FutureGetBlockCountResult chan *response

// Receive waits for the response promised by the future and returns the number
// of blocks in the longest block chain.
func (r FutureGetBlockCountResult) Receive() (int64, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return 0, err
	}

	// Unmarshal the result as an int64.
	var count int64
	err = json.Unmarshal(res, &count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetBlockCountAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBlockCount for the blocking version and more details.
func (c *Client) GetBlockCountAsync() FutureGetBlockCountResult {
	cmd := jaxjson.NewGetBlockCountCmd()
	return c.sendCmd(cmd)
}

// GetBlockCount returns the number of blocks in the longest block chain.
func (c *Client) GetBlockCount() (int64, error) {
	return c.GetBlockCountAsync().Receive()
}

// FutureGetDifficultyResult is a future promise to deliver the result of a
// GetDifficultyAsync RPC invocation (or an applicable error).
type FutureGetDifficultyResult chan *response

// Receive waits for the response promised by the future and returns the
// proof-of-work difficulty as a multiple of the minimum difficulty.
func (r FutureGetDifficultyResult) Receive() (float64, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return 0, err
	}

	// Unmarshal the result as a float64.
	var difficulty float64
	err = json.Unmarshal(res, &difficulty)
	if err != nil {
		return 0, err
	}
	return difficulty, nil
}

// GetDifficultyAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetDifficulty for the blocking version and more details.
func (c *Client) GetDifficultyAsync() FutureGetDifficultyResult {
	cmd := jaxjson.NewGetDifficultyCmd()
	return c.sendCmd(cmd)
}

// GetDifficulty returns the proof-of-work difficulty as a multiple of the
// minimum difficulty.
func (c *Client) GetDifficulty() (float64, error) {
	return c.GetDifficultyAsync().Receive()
}

// FutureGetBlockChainInfoResult is a promise to deliver the result of a
// GetBlockChainInfoAsync RPC invocation (or an applicable error).
type FutureGetBlockChainInfoResult struct {
	client   *Client
	Response chan *response
}

// unmarshalPartialGetBlockChainInfoResult unmarshals the response into an
// instance of GetBlockChainInfoResult without populating the SoftForks and
// UnifiedSoftForks fields.
func unmarshalPartialGetBlockChainInfoResult(res []byte) (*jaxjson.GetBlockChainInfoResult, error) {
	var chainInfo jaxjson.GetBlockChainInfoResult
	if err := json.Unmarshal(res, &chainInfo); err != nil {
		return nil, err
	}
	return &chainInfo, nil
}

// unmarshalGetBlockChainInfoResultSoftForks properly unmarshals the softforks
// related fields into the GetBlockChainInfoResult instance.
func unmarshalGetBlockChainInfoResultSoftForks(chainInfo *jaxjson.GetBlockChainInfoResult,
	version BackendVersion, res []byte) error {

	switch version {
	// Versions of jaxnetd on or after v0.19.0 use the unified format.
	case JaxnetdPost19:
		var softForks jaxjson.UnifiedSoftForks
		if err := json.Unmarshal(res, &softForks); err != nil {
			return err
		}
		chainInfo.UnifiedSoftForks = &softForks

	// All other versions use the original format.
	default:
		var softForks jaxjson.SoftForks
		if err := json.Unmarshal(res, &softForks); err != nil {
			return err
		}
		chainInfo.SoftForks = &softForks
	}

	return nil
}

// Receive waits for the response promised by the future and returns chain info
// result provided by the server.
func (r FutureGetBlockChainInfoResult) Receive() (*jaxjson.GetBlockChainInfoResult, error) {
	res, err := receiveFuture(r.Response)
	if err != nil {
		return nil, err
	}
	chainInfo, err := unmarshalPartialGetBlockChainInfoResult(res)
	if err != nil {
		return nil, err
	}

	// Inspect the version to determine how we'll need to parse the
	// softforks from the response.
	version, err := r.client.BackendVersion()
	if err != nil {
		return nil, err
	}

	err = unmarshalGetBlockChainInfoResultSoftForks(chainInfo, version, res)
	if err != nil {
		return nil, err
	}

	return chainInfo, nil
}

// GetBlockChainInfoAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function
// on the returned instance.
//
// See GetBlockChainInfo for the blocking version and more details.
func (c *Client) GetBlockChainInfoAsync() FutureGetBlockChainInfoResult {
	cmd := jaxjson.NewGetBlockChainInfoCmd()
	return FutureGetBlockChainInfoResult{
		client:   c,
		Response: c.sendCmd(cmd),
	}
}

// GetBlockChainInfo returns information related to the processing state of
// various chain-specific details such as the current difficulty from the tip
// of the main chain.
func (c *Client) GetBlockChainInfo() (*jaxjson.GetBlockChainInfoResult, error) {
	return c.GetBlockChainInfoAsync().Receive()
}

// FutureGetBlockHashResult is a future promise to deliver the result of a
// GetBlockHashAsync RPC invocation (or an applicable error).
type FutureGetBlockHashResult chan *response

// Receive waits for the response promised by the future and returns the hash of
// the block in the best block chain at the given height.
func (r FutureGetBlockHashResult) Receive() (*chainhash.Hash, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal the result as a string-encoded sha.
	var txHashStr string
	err = json.Unmarshal(res, &txHashStr)
	if err != nil {
		return nil, err
	}
	return chainhash.NewHashFromStr(txHashStr)
}

// GetBlockHashAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBlockHash for the blocking version and more details.
func (c *Client) GetBlockHashAsync(blockHeight int64) FutureGetBlockHashResult {
	cmd := jaxjson.NewGetBlockHashCmd(blockHeight)
	return c.sendCmd(cmd)
}

// GetBlockHash returns the hash of the block in the best block chain at the
// given height.
func (c *Client) GetBlockHash(blockHeight int64) (*chainhash.Hash, error) {
	return c.GetBlockHashAsync(blockHeight).Receive()
}

// FutureGetMempoolEntryResult is a future promise to deliver the result of a
// GetMempoolEntryAsync RPC invocation (or an applicable error).
type FutureGetMempoolEntryResult chan *response

// Receive waits for the response promised by the future and returns a data
// structure with information about the transaction in the memory pool given
// its hash.
func (r FutureGetMempoolEntryResult) Receive() (*jaxjson.GetMempoolEntryResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal the result as an array of strings.
	var mempoolEntryResult jaxjson.GetMempoolEntryResult
	err = json.Unmarshal(res, &mempoolEntryResult)
	if err != nil {
		return nil, err
	}

	return &mempoolEntryResult, nil
}

// GetMempoolEntryAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetMempoolEntry for the blocking version and more details.
func (c *Client) GetMempoolEntryAsync(txHash string) FutureGetMempoolEntryResult {
	cmd := jaxjson.NewGetMempoolEntryCmd(txHash)
	return c.sendCmd(cmd)
}

// GetMempoolEntry returns a data structure with information about the
// transaction in the memory pool given its hash.
func (c *Client) GetMempoolEntry(txHash string) (*jaxjson.GetMempoolEntryResult, error) {
	return c.GetMempoolEntryAsync(txHash).Receive()
}

// FutureGetRawMempoolResult is a future promise to deliver the result of a
// GetRawMempoolAsync RPC invocation (or an applicable error).
type FutureGetRawMempoolResult chan *response

// Receive waits for the response promised by the future and returns the hashes
// of all transactions in the memory pool.
func (r FutureGetRawMempoolResult) Receive() ([]*chainhash.Hash, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal the result as an array of strings.
	var txHashStrs []string
	err = json.Unmarshal(res, &txHashStrs)
	if err != nil {
		return nil, err
	}

	// Create a slice of ShaHash arrays from the string slice.
	txHashes := make([]*chainhash.Hash, 0, len(txHashStrs))
	for _, hashStr := range txHashStrs {
		txHash, err := chainhash.NewHashFromStr(hashStr)
		if err != nil {
			return nil, err
		}
		txHashes = append(txHashes, txHash)
	}

	return txHashes, nil
}

// GetRawMempoolAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetRawMempool for the blocking version and more details.
func (c *Client) GetRawMempoolAsync() FutureGetRawMempoolResult {
	cmd := jaxjson.NewGetRawMempoolCmd(jaxjson.Bool(false))
	return c.sendCmd(cmd)
}

// GetRawMempool returns the hashes of all transactions in the memory pool.
//
// See GetRawMempoolVerbose to retrieve data structures with information about
// the transactions instead.
func (c *Client) GetRawMempool() ([]*chainhash.Hash, error) {
	return c.GetRawMempoolAsync().Receive()
}

// FutureGetRawMempoolVerboseResult is a future promise to deliver the result of
// a GetRawMempoolVerboseAsync RPC invocation (or an applicable error).
type FutureGetRawMempoolVerboseResult chan *response

// Receive waits for the response promised by the future and returns a map of
// transaction hashes to an associated data structure with information about the
// transaction for all transactions in the memory pool.
func (r FutureGetRawMempoolVerboseResult) Receive() (map[string]jaxjson.GetRawMempoolVerboseResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal the result as a map of strings (tx shas) to their detailed
	// results.
	var mempoolItems map[string]jaxjson.GetRawMempoolVerboseResult
	err = json.Unmarshal(res, &mempoolItems)
	if err != nil {
		return nil, err
	}
	return mempoolItems, nil
}

// GetRawMempoolVerboseAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetRawMempoolVerbose for the blocking version and more details.
func (c *Client) GetRawMempoolVerboseAsync() FutureGetRawMempoolVerboseResult {
	cmd := jaxjson.NewGetRawMempoolCmd(jaxjson.Bool(true))
	return c.sendCmd(cmd)
}

// GetRawMempoolVerbose returns a map of transaction hashes to an associated
// data structure with information about the transaction for all transactions in
// the memory pool.
//
// See GetRawMempool to retrieve only the transaction hashes instead.
func (c *Client) GetRawMempoolVerbose() (map[string]jaxjson.GetRawMempoolVerboseResult, error) {
	return c.GetRawMempoolVerboseAsync().Receive()
}

// FutureEstimateFeeResult is a future promise to deliver the result of a
// EstimateFeeAsync RPC invocation (or an applicable error).
type FutureEstimateFeeResult chan *response

// Receive waits for the response promised by the future and returns the info
// provided by the server.
func (r FutureEstimateFeeResult) Receive() (float64, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return -1, err
	}

	// Unmarshal result as a getinfo result object.
	var fee float64
	err = json.Unmarshal(res, &fee)
	if err != nil {
		return -1, err
	}

	return fee, nil
}

// EstimateFeeAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See EstimateFee for the blocking version and more details.
func (c *Client) EstimateFeeAsync(numBlocks int64) FutureEstimateFeeResult {
	cmd := jaxjson.NewEstimateFeeCmd(numBlocks)
	return c.sendCmd(cmd)
}

// EstimateFee provides an estimated fee  in bitcoins per kilobyte.
func (c *Client) EstimateFee(numBlocks int64) (float64, error) {
	return c.EstimateFeeAsync(numBlocks).Receive()
}

// FutureEstimateSmartFeeResult is a future promise to deliver the result of a
// EstimateSmartFeeAsync RPC invocation (or an applicable error).
type FutureEstimateSmartFeeResult chan *response

// Receive waits for the response promised by the future and returns the
// estimated fee.
func (r FutureEstimateSmartFeeResult) Receive() (*jaxjson.EstimateSmartFeeResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	var verified jaxjson.EstimateSmartFeeResult
	err = json.Unmarshal(res, &verified)
	if err != nil {
		return nil, err
	}
	return &verified, nil
}

// EstimateSmartFeeAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See EstimateSmartFee for the blocking version and more details.
func (c *Client) EstimateSmartFeeAsync(confTarget int64, mode *jaxjson.EstimateSmartFeeMode) FutureEstimateSmartFeeResult {
	cmd := jaxjson.NewEstimateSmartFeeCmd(confTarget, mode)
	return c.sendCmd(cmd)
}

// EstimateSmartFee requests the server to estimate a fee level based on the given parameters.
func (c *Client) EstimateSmartFee(confTarget int64, mode *jaxjson.EstimateSmartFeeMode) (*jaxjson.EstimateSmartFeeResult, error) {
	return c.EstimateSmartFeeAsync(confTarget, mode).Receive()
}

// FutureGetExtendedFeeResult is a future promise to deliver the result of a
// EstimateSmartFeeAsync RPC invocation (or an applicable error).
type FutureGetExtendedFeeResult chan *response

// Receive waits for the response promised by the future and returns the
// estimated fee.
func (r FutureGetExtendedFeeResult) Receive() (*jaxjson.ExtendedFeeFeeResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	var result jaxjson.ExtendedFeeFeeResult
	err = json.Unmarshal(res, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetExtendedFeeAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See EstimateSmartFee for the blocking version and more details.
func (c *Client) GetExtendedFeeAsync() FutureGetExtendedFeeResult {
	cmd := &jaxjson.GetExtendedFee{}
	return c.sendCmd(cmd)
}

// GetExtendedFee requests the server to estimate a fee level based on the given parameters.
func (c *Client) GetExtendedFee() (*jaxjson.ExtendedFeeFeeResult, error) {
	return c.GetExtendedFeeAsync().Receive()
}

// FutureVerifyChainResult is a future promise to deliver the result of a
// VerifyChainAsync, VerifyChainLevelAsyncRPC, or VerifyChainBlocksAsync
// invocation (or an applicable error).
type FutureVerifyChainResult chan *response

// Receive waits for the response promised by the future and returns whether
// or not the chain verified based on the check level and number of blocks
// to verify specified in the original call.
func (r FutureVerifyChainResult) Receive() (bool, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return false, err
	}

	// Unmarshal the result as a boolean.
	var verified bool
	err = json.Unmarshal(res, &verified)
	if err != nil {
		return false, err
	}
	return verified, nil
}

// VerifyChainAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See VerifyChain for the blocking version and more details.
func (c *Client) VerifyChainAsync() FutureVerifyChainResult {
	cmd := jaxjson.NewVerifyChainCmd(nil, nil)
	return c.sendCmd(cmd)
}

// VerifyChain requests the server to verify the block chain database using
// the default check level and number of blocks to verify.
//
// See VerifyChainLevel and VerifyChainBlocks to override the defaults.
func (c *Client) VerifyChain() (bool, error) {
	return c.VerifyChainAsync().Receive()
}

// VerifyChainLevelAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See VerifyChainLevel for the blocking version and more details.
func (c *Client) VerifyChainLevelAsync(checkLevel int32) FutureVerifyChainResult {
	cmd := jaxjson.NewVerifyChainCmd(&checkLevel, nil)
	return c.sendCmd(cmd)
}

// VerifyChainLevel requests the server to verify the block chain database using
// the passed check level and default number of blocks to verify.
//
// The check level controls how thorough the verification is with higher numbers
// increasing the amount of checks done as consequently how long the
// verification takes.
//
// See VerifyChain to use the default check level and VerifyChainBlocks to
// override the number of blocks to verify.
func (c *Client) VerifyChainLevel(checkLevel int32) (bool, error) {
	return c.VerifyChainLevelAsync(checkLevel).Receive()
}

// VerifyChainBlocksAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See VerifyChainBlocks for the blocking version and more details.
func (c *Client) VerifyChainBlocksAsync(checkLevel, numBlocks int32) FutureVerifyChainResult {
	cmd := jaxjson.NewVerifyChainCmd(&checkLevel, &numBlocks)
	return c.sendCmd(cmd)
}

// VerifyChainBlocks requests the server to verify the block chain database
// using the passed check level and number of blocks to verify.
//
// The check level controls how thorough the verification is with higher numbers
// increasing the amount of checks done as consequently how long the
// verification takes.
//
// The number of blocks refers to the number of blocks from the end of the
// current longest chain.
//
// See VerifyChain and VerifyChainLevel to use defaults.
func (c *Client) VerifyChainBlocks(checkLevel, numBlocks int32) (bool, error) {
	return c.VerifyChainBlocksAsync(checkLevel, numBlocks).Receive()
}

// FutureGetTxOutResult is a future promise to deliver the result of a
// GetTxOutAsync RPC invocation (or an applicable error).
type FutureGetTxOutResult chan *response

// Receive waits for the response promised by the future and returns a
// transaction given its hash.
func (r FutureGetTxOutResult) Receive() (*jaxjson.GetTxOutResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// take care of the special case where the output has been spent already
	// it should return the string "null"
	if string(res) == "null" {
		return nil, nil
	}

	// Unmarshal result as an gettxout result object.
	var txOutInfo *jaxjson.GetTxOutResult
	err = json.Unmarshal(res, &txOutInfo)
	if err != nil {
		return nil, err
	}

	return txOutInfo, nil
}

// GetTxOutAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetTxOut for the blocking version and more details.
func (c *Client) GetTxOutAsync(txHash *chainhash.Hash, index uint32, mempool, orphan bool) FutureGetTxOutResult {
	hash := ""
	if txHash != nil {
		hash = txHash.String()
	}

	cmd := jaxjson.NewGetTxOutCmd(hash, index, &mempool, &orphan)
	return c.sendCmd(cmd)
}

// GetTxOut returns the transaction output info if it's unspent and
// nil, otherwise.
func (c *Client) GetTxOut(txHash *chainhash.Hash, index uint32, mempool, orphan bool) (*jaxjson.GetTxOutResult, error) {
	return c.GetTxOutAsync(txHash, index, mempool, orphan).Receive()
}

type FutureGetTxOutStatusResult chan *response

// Receive waits for the response promised by the future and returns a
// transaction given its hash.
func (r FutureGetTxOutStatusResult) Receive() ([]jaxjson.TxOutStatus, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// take care of the special case where the output has been spent already
	// it should return the string "null"
	if string(res) == "null" {
		return nil, nil
	}

	// Unmarshal result as an gettxout result object.
	var result = make([]jaxjson.TxOutStatus, 0)
	err = json.Unmarshal(res, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) GetTxOutStatusAsync(outs []jaxjson.TxOutKey, onlyMempool bool) FutureGetTxOutStatusResult {
	cmd := &jaxjson.GetTxOutStatus{Outs: outs, OnlyMempool: &onlyMempool}
	return c.sendCmd(cmd)
}

func (c *Client) GetTxOutStatus(outs []jaxjson.TxOutKey, onlyMempool bool) ([]jaxjson.TxOutStatus, error) {
	return c.GetTxOutStatusAsync(outs, onlyMempool).Receive()
}

// FutureListTxOutResult is a future promise to deliver the result of a
// GetTxOutAsync RPC invocation (or an applicable error).
type FutureListTxOutResult chan *response

// Receive waits for the response promised by the future and returns a
// transaction given its hash.
func (r FutureListTxOutResult) Receive() (*jaxjson.ListTxOutResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// take care of the special case where the output has been spent already
	// it should return the string "null"
	if string(res) == "null" {
		return nil, nil
	}

	// Unmarshal result as an gettxout result object.
	listTxOut := &jaxjson.ListTxOutResult{}
	err = json.Unmarshal(res, listTxOut)
	if err != nil {
		return nil, err
	}

	return listTxOut, nil
}

// ListTxOutAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetTxOut for the blocking version and more details.
func (c *Client) ListTxOutAsync() FutureListTxOutResult {
	cmd := jaxjson.NewListTxOutCmd()
	return c.sendCmd(cmd)
}

// ListTxOut returns the transaction output info if it's unspent and
// nil, otherwise.
func (c *Client) ListTxOut() (*jaxjson.ListTxOutResult, error) {
	return c.ListTxOutAsync().Receive()
}

// FutureListEADAddressesResult is a future promise to deliver the result of a
// ListEADAddressesAsync RPC invocation (or an applicable error).
type FutureListEADAddressesResult chan *response

// Receive waits for the response promised by the future and returns a
// transaction given its hash.
func (r FutureListEADAddressesResult) Receive() (*jaxjson.ListEADAddresses, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// take care of the special case where the output has been spent already
	// it should return the string "null"
	if string(res) == "null" {
		return nil, nil
	}

	// Unmarshal result as an ListEADAddresses result object.
	listTxOut := &jaxjson.ListEADAddresses{}
	err = json.Unmarshal(res, listTxOut)
	if err != nil {
		return nil, err
	}

	return listTxOut, nil
}

// EADAddressesAsync ...
func (c *Client) ListEADAddressesAsync(shards []uint32, eadPublicKey *string) FutureListEADAddressesResult {
	cmd := jaxjson.NewListEADAddressesCmd(shards, eadPublicKey)
	return c.sendCmd(cmd)
}

// ListEADAddresses ...
func (c *Client) ListEADAddresses(shards []uint32, eadPublicKey *string) (*jaxjson.ListEADAddresses, error) {
	return c.ListEADAddressesAsync(shards, eadPublicKey).Receive()
}

// FutureRescanBlocksResult is a future promise to deliver the result of a
// RescanBlocksAsync RPC invocation (or an applicable error).
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
type FutureRescanBlocksResult chan *response

// Receive waits for the response promised by the future and returns the
// discovered rescanblocks data.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (r FutureRescanBlocksResult) Receive() ([]jaxjson.RescannedBlock, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	var rescanBlocksResult []jaxjson.RescannedBlock
	err = json.Unmarshal(res, &rescanBlocksResult)
	if err != nil {
		return nil, err
	}

	return rescanBlocksResult, nil
}

// RescanBlocksAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See RescanBlocks for the blocking version and more details.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (c *Client) RescanBlocksAsync(blockHashes []chainhash.Hash) FutureRescanBlocksResult {
	strBlockHashes := make([]string, len(blockHashes))
	for i := range blockHashes {
		strBlockHashes[i] = blockHashes[i].String()
	}

	cmd := jaxjson.NewRescanBlocksCmd(strBlockHashes)
	return c.sendCmd(cmd)
}

// RescanBlocks rescans the blocks identified by blockHashes, in order, using
// the client's loaded transaction filter.  The blocks do not need to be on the
// main chain, but they do need to be adjacent to each other.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (c *Client) RescanBlocks(blockHashes []chainhash.Hash) ([]jaxjson.RescannedBlock, error) {
	return c.RescanBlocksAsync(blockHashes).Receive()
}

// FutureInvalidateBlockResult is a future promise to deliver the result of a
// InvalidateBlockAsync RPC invocation (or an applicable error).
type FutureInvalidateBlockResult chan *response

// Receive waits for the response promised by the future and returns the raw
// block requested from the server given its hash.
func (r FutureInvalidateBlockResult) Receive() error {
	_, err := receiveFuture(r)

	return err
}

// InvalidateBlockAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See InvalidateBlock for the blocking version and more details.
func (c *Client) InvalidateBlockAsync(blockHash *chainhash.Hash) FutureInvalidateBlockResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	cmd := jaxjson.NewInvalidateBlockCmd(hash)
	return c.sendCmd(cmd)
}

// InvalidateBlock invalidates a specific block.
func (c *Client) InvalidateBlock(blockHash *chainhash.Hash) error {
	return c.InvalidateBlockAsync(blockHash).Receive()
}

// FutureGetCFilterResult is a future promise to deliver the result of a
// GetCFilterAsync RPC invocation (or an applicable error).
type FutureGetCFilterResult chan *response

// Receive waits for the response promised by the future and returns the raw
// filter requested from the server given its block hash.
func (r FutureGetCFilterResult) Receive() (*wire.MsgCFilter, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var filterHex string
	err = json.Unmarshal(res, &filterHex)
	if err != nil {
		return nil, err
	}

	// Decode the serialized cf hex to raw bytes.
	serializedFilter, err := hex.DecodeString(filterHex)
	if err != nil {
		return nil, err
	}

	// Assign the filter bytes to the correct field of the wire message.
	// We aren't going to set the block hash or extended flag, since we
	// don't actually get that back in the RPC response.
	var msgCFilter wire.MsgCFilter
	msgCFilter.Data = serializedFilter
	return &msgCFilter, nil
}

// GetCFilterAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetCFilter for the blocking version and more details.
func (c *Client) GetCFilterAsync(blockHash *chainhash.Hash,
	filterType wire.FilterType) FutureGetCFilterResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	cmd := jaxjson.NewGetCFilterCmd(hash, filterType)
	return c.sendCmd(cmd)
}

// GetCFilter returns a raw filter from the server given its block hash.
func (c *Client) GetCFilter(blockHash *chainhash.Hash,
	filterType wire.FilterType) (*wire.MsgCFilter, error) {
	return c.GetCFilterAsync(blockHash, filterType).Receive()
}

// FutureGetCFilterHeaderResult is a future promise to deliver the result of a
// GetCFilterHeaderAsync RPC invocation (or an applicable error).
type FutureGetCFilterHeaderResult chan *response

// Receive waits for the response promised by the future and returns the raw
// filter header requested from the server given its block hash.
func (r FutureGetCFilterHeaderResult) Receive() (*wire.MsgCFHeaders, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var headerHex string
	err = json.Unmarshal(res, &headerHex)
	if err != nil {
		return nil, err
	}

	// Assign the decoded header into a hash
	headerHash, err := chainhash.NewHashFromStr(headerHex)
	if err != nil {
		return nil, err
	}

	// Assign the hash to a headers message and return it.
	msgCFHeaders := wire.MsgCFHeaders{PrevFilterHeader: *headerHash}
	return &msgCFHeaders, nil

}

// GetCFilterHeaderAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function
// on the returned instance.
//
// See GetCFilterHeader for the blocking version and more details.
func (c *Client) GetCFilterHeaderAsync(blockHash *chainhash.Hash,
	filterType wire.FilterType) FutureGetCFilterHeaderResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	cmd := jaxjson.NewGetCFilterHeaderCmd(hash, filterType)
	return c.sendCmd(cmd)
}

// GetCFilterHeader returns a raw filter header from the server given its block
// hash.
func (c *Client) GetCFilterHeader(blockHash *chainhash.Hash,
	filterType wire.FilterType) (*wire.MsgCFHeaders, error) {
	return c.GetCFilterHeaderAsync(blockHash, filterType).Receive()
}

// FutureGetBlockStatsResult is a future promise to deliver the result of a
// GetBlockStatsAsync RPC invocation (or an applicable error).
type FutureGetBlockStatsResult chan *response

// Receive waits for the response promised by the future and returns statistics
// of a block at a certain height.
func (r FutureGetBlockStatsResult) Receive() (*jaxjson.GetBlockStatsResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	var blockStats jaxjson.GetBlockStatsResult
	err = json.Unmarshal(res, &blockStats)
	if err != nil {
		return nil, err
	}

	return &blockStats, nil
}

// GetBlockStatsAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetBlockStats or the blocking version and more details.
func (c *Client) GetBlockStatsAsync(hashOrHeight interface{}, stats *[]string) FutureGetBlockStatsResult {
	if hash, ok := hashOrHeight.(*chainhash.Hash); ok {
		hashOrHeight = hash.String()
	}

	cmd := jaxjson.NewGetBlockStatsCmd(jaxjson.HashOrHeight{Value: hashOrHeight}, stats)
	return c.sendCmd(cmd)
}

// GetBlockStats returns block statistics. First argument specifies height or hash of the target block.
// Second argument allows to select certain stats to return.
func (c *Client) GetBlockStats(hashOrHeight interface{}, stats *[]string) (*jaxjson.GetBlockStatsResult, error) {
	return c.GetBlockStatsAsync(hashOrHeight, stats).Receive()
}

// FutureListShards is a future promise to deliver the result of a
// ListShardsAsync RPC invocation (or an applicable error).
type FutureListShards chan *response

// Receive waits for the response promised by the future and returns statistics
// of a block at a certain height.
func (r FutureListShards) Receive() (*jaxjson.ShardListResult, error) {
	res, err := receiveFuture(r)
	var list jaxjson.ShardListResult
	err = json.Unmarshal(res, &list)
	if err != nil {
		return nil, err
	}

	return &list, nil
}

func (c *Client) ListShardsAsync() FutureListShards {
	cmd := &jaxjson.ListShardsCmd{}
	return c.sendCmd(cmd)
}

func (c *Client) ListShards() (*jaxjson.ShardListResult, error) {
	return c.ListShardsAsync().Receive()
}

// FutureManageShards is a future promise to deliver the result of a
// ManageShardsAsync RPC invocation (or an applicable error).
type FutureManageShards chan *response

// Receive waits for the response promised by the future and returns statistics
// of a block at a certain height.
func (r FutureManageShards) Receive() error {
	_, err := receiveFuture(r)

	return err
}

func (c *Client) ManageShardsAsync(action string, shardID uint32) FutureManageShards {
	cmd := jaxjson.NewManageShardsCmd(shardID, action, nil)
	return c.sendCmd(cmd)
}

func (c *Client) ManageShards(action string, shardID uint32) error {
	return c.ManageShardsAsync(action, shardID).Receive()
}

// FutureGetLastSerialBlockNumberResult is a future promise to deliver the result of a
// GetLastSerialBlockNumberAsync RPC invocation (or an applicable error).
type FutureGetLastSerialBlockNumberResult chan *response

// Receive waits for the response promised by the future and returns the number
// of blocks in the longest block chain.
func (r FutureGetLastSerialBlockNumberResult) Receive() (int64, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return 0, err
	}

	// Unmarshal the result as an map.
	var resmap map[string]int64
	err = json.Unmarshal(res, &resmap)
	if err != nil {
		return 0, err
	}
	if _, ok := resmap["lastserial"]; !ok {
		return 0, errors.New("bad response format")
	}
	return resmap["lastserial"], nil
}

// GetLastSerialBlockNumberAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
func (c *Client) GetLastSerialBlockNumberAsync() FutureGetLastSerialBlockNumberResult {
	cmd := jaxjson.NewGetLastSerialBlockNumberCmd()
	return c.sendCmd(cmd)
}

// GetLastSerialBlockNumber returns the number of blocks in the longest block chain.
func (c *Client) GetLastSerialBlockNumber() (int64, error) {
	return c.GetLastSerialBlockNumberAsync().Receive()
}

// FutureGetBlockTxOpsResult is a future promise to deliver the result of a
// GetBlockTxOps RPC invocation (or an applicable error).
type FutureGetBlockTxOpsResult chan *response

// Receive waits for the response promised by the future and returns a
// transaction given its hash.
func (r FutureGetBlockTxOpsResult) Receive() (*jaxjson.BlockTxOperations, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as an getblocktxops result object.
	listTxOut := &jaxjson.BlockTxOperations{}
	err = json.Unmarshal(res, listTxOut)
	if err != nil {
		return nil, err
	}

	return listTxOut, nil
}

// GetBlockTxOperationsAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetBlockTxOperations for the blocking version and more details.
func (c *Client) GetBlockTxOperationsAsync(blockHash *chainhash.Hash) FutureGetBlockTxOpsResult {
	return c.sendCmd(&jaxjson.GetBlockTxOpsCmd{BlockHash: blockHash.String()})
}

// GetBlockTxOperations returns the transaction output info if it's unspent and
// nil, otherwise.
func (c *Client) GetBlockTxOperations(blockHash *chainhash.Hash) (*jaxjson.BlockTxOperations, error) {
	return c.GetBlockTxOperationsAsync(blockHash).Receive()
}

// FutureGetMempoolUTXOs ...
type FutureGetMempoolUTXOs chan *response

// Receive waits for the response promised by the future and returns a
// transaction given its hash.
func (r FutureGetMempoolUTXOs) Receive() ([]jaxjson.MempoolUTXO, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	listTxOut := make([]jaxjson.MempoolUTXO, 0)
	err = json.Unmarshal(res, &listTxOut)
	if err != nil {
		return nil, err
	}

	return listTxOut, nil
}

// GetMempoolUTXOsAsync ...
func (c *Client) GetMempoolUTXOsAsync() FutureGetMempoolUTXOs {
	return c.sendCmd(&jaxjson.GetMempoolUTXOs{})
}

// GetMempoolUTXOs ...
func (c *Client) GetMempoolUTXOs() ([]jaxjson.MempoolUTXO, error) {
	return c.GetMempoolUTXOsAsync().Receive()
}

// FutureEstimateLockTime ...
type FutureEstimateLockTime chan *response

// Receive waits for the response promised by the future and returns a
// transaction given its hash.
func (r FutureEstimateLockTime) Receive() (*jaxjson.EstimateLockTimeResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	listTxOut := new(jaxjson.EstimateLockTimeResult)
	err = json.Unmarshal(res, listTxOut)
	if err != nil {
		return nil, err
	}

	return listTxOut, nil
}

// EstimateLockTimeAsync ...
func (c *Client) EstimateLockTimeAsync(amount int64) FutureEstimateLockTime {
	return c.sendCmd(&jaxjson.EstimateLockTime{Amount: amount})
}

// EstimateLockTime estimates desired period in block for locking funds
// // in shard during the CrossShard Swap Tx.
func (c *Client) EstimateLockTime(amount int64) (*jaxjson.EstimateLockTimeResult, error) {
	return c.EstimateLockTimeAsync(amount).Receive()
}

type FutureSwapEstimateLockTime chan *response

// Receive waits for the response promised by the future and returns a
// transaction given its hash.
func (r FutureSwapEstimateLockTime) Receive() (*jaxjson.EstimateSwapLockTimeResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	listTxOut := new(jaxjson.EstimateSwapLockTimeResult)
	err = json.Unmarshal(res, listTxOut)
	if err != nil {
		return nil, err
	}

	return listTxOut, nil
}

// EstimateSwapLockTimeAsync ...
func (c *Client) EstimateSwapLockTimeAsync(source, dest uint32, amount int64) FutureSwapEstimateLockTime {
	return c.sendCmd(&jaxjson.EstimateSwapLockTime{
		Amount:           amount,
		SourceShard:      source,
		DestinationShard: dest,
	})
}

// EstimateSwapLockTime estimates desired period in block for locking funds
// in source and destination shards during the CrossShard Swap Tx.
func (c *Client) EstimateSwapLockTime(source, dest uint32, amount int64) (*jaxjson.EstimateSwapLockTimeResult, error) {
	return c.EstimateSwapLockTimeAsync(source, dest, amount).Receive()
}

// FutureGetTxResult is a future promise to deliver the result of a
// GetTxOutAsync RPC invocation (or an applicable error).
type FutureGetTxResult chan *response

// Receive waits for the response promised by the future and returns a
// transaction given its hash.
func (r FutureGetTxResult) Receive() (*jaxjson.GetTxResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// take care of the special case where the output has been spent already
	// it should return the string "null"
	if string(res) == "null" {
		return nil, nil
	}

	// Unmarshal result as an gettx result object.
	var txInfo *jaxjson.GetTxResult
	err = json.Unmarshal(res, &txInfo)
	if err != nil {
		return nil, err
	}

	return txInfo, nil
}

// GetTxAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetTxOut for the blocking version and more details.
func (c *Client) GetTxAsync(txHash *chainhash.Hash, mempool, orphan bool) FutureGetTxResult {
	hash := ""
	if txHash != nil {
		hash = txHash.String()
	}

	cmd := &jaxjson.GetTxCmd{Txid: hash, IncludeMempool: &mempool, IncludeOrphan: &orphan}
	return c.sendCmd(cmd)
}

// GetTx returns the transaction info
func (c *Client) GetTx(txHash *chainhash.Hash, mempool, orphan bool) (*jaxjson.GetTxResult, error) {
	return c.GetTxAsync(txHash, mempool, orphan).Receive()
}
