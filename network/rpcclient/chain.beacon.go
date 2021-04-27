// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package rpcclient

import (
	"bytes"
	"encoding/hex"
	"encoding/json"

	"gitlab.com/jaxnet/core/shard.core/types/btcjson"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

// FutureGetBeaconBlockResult is a future promise to deliver the result of a
// GetBeaconBlockAsync RPC invocation (or an applicable error).
type FutureGetBeaconBlockResult struct {
	client   *Client
	hash     string
	Response chan *response
}

// Receive waits for the response promised by the future and returns the raw
// block requested from the server given its hash.
func (r FutureGetBeaconBlockResult) Receive() (*wire.MsgBlock, error) {
	res, err := r.client.waitForGetBlockRes(r.Response, r.hash, "getBeaconBlock", false, false)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var blockHex string
	err = json.Unmarshal(res, &blockHex)
	if err != nil {
		return nil, err
	}

	// Decode the serialized block hex to raw bytes.
	serializedBlock, err := hex.DecodeString(blockHex)
	if err != nil {
		return nil, err
	}

	// Deserialize the block and return it.
	var msgBlock = wire.EmptyBeaconBlock()
	err = msgBlock.Deserialize(bytes.NewReader(serializedBlock))
	if err != nil {
		return nil, err
	}
	return &msgBlock, nil
}

// GetBeaconBlockAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBeaconBlock for the blocking version and more details.
func (c *Client) GetBeaconBlockAsync(blockHash *chainhash.Hash) FutureGetBeaconBlockResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	cmd := btcjson.NewGetBeaconBlockCmd(hash, btcjson.Int(0))
	return FutureGetBeaconBlockResult{
		client:   c,
		hash:     hash,
		Response: c.ForBeacon().sendCmd(cmd),
	}
}

// GetBeaconBlock returns a raw block from the server given its hash.
//
// See GetBeaconBlockVerbose to retrieve a data structure with information about the
// block instead.
func (c *Client) GetBeaconBlock(blockHash *chainhash.Hash) (*wire.MsgBlock, error) {
	return c.GetBeaconBlockAsync(blockHash).Receive()
}

// FutureGetBeaconBlockVerboseResult is a future promise to deliver the result of a
// GetBeaconBlockVerboseAsync RPC invocation (or an applicable error).
type FutureGetBeaconBlockVerboseResult struct {
	client   *Client
	hash     string
	Response chan *response
}

// Receive waits for the response promised by the future and returns the data
// structure from the server with information about the requested block.
func (r FutureGetBeaconBlockVerboseResult) Receive() (*btcjson.GetBeaconBlockVerboseResult, error) {
	res, err := r.client.waitForGetBlockRes(r.Response, "getBeaconBlock", r.hash, true, false)
	if err != nil {
		return nil, err
	}

	// Unmarshal the raw result into a BlockResult.
	var blockResult btcjson.GetBeaconBlockVerboseResult
	err = json.Unmarshal(res, &blockResult)
	if err != nil {
		return nil, err
	}
	return &blockResult, nil
}

// GetBeaconBlockVerboseAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetBeaconBlockVerbose for the blocking version and more details.
func (c *Client) GetBeaconBlockVerboseAsync(blockHash *chainhash.Hash) FutureGetBeaconBlockVerboseResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}
	// From the bitcoin-cli getblock documentation:
	// "If verbosity is 1, returns an Object with information about block ."
	cmd := btcjson.NewGetBeaconBlockCmd(hash, btcjson.Int(1))
	return FutureGetBeaconBlockVerboseResult{
		client:   c,
		hash:     hash,
		Response: c.ForBeacon().sendCmd(cmd),
	}
}

// GetBeaconBlockVerbose returns a data structure from the server with information
// about a block given its hash.
//
// See GetBeaconBlockVerboseTx to retrieve transaction data structures as well.
// See GetBeaconBlock to retrieve a raw block instead.
func (c *Client) GetBeaconBlockVerbose(blockHash *chainhash.Hash) (*btcjson.GetBeaconBlockVerboseResult, error) {
	return c.GetBeaconBlockVerboseAsync(blockHash).Receive()
}

// FutureGetBeaconBlockVerboseTxResult is a future promise to deliver the result of a
// GetBeaconBlockVerboseTxResult RPC invocation (or an applicable error).
type FutureGetBeaconBlockVerboseTxResult struct {
	client   *Client
	hash     string
	Response chan *response
}

// Receive waits for the response promised by the future and returns a verbose
// version of the block including detailed information about its transactions.
func (r FutureGetBeaconBlockVerboseTxResult) Receive() (*btcjson.GetBeaconBlockVerboseTxResult, error) {
	res, err := r.client.waitForGetBlockRes(r.Response, "getBeaconBlock", r.hash, true, true)
	if err != nil {
		return nil, err
	}

	var blockResult btcjson.GetBeaconBlockVerboseTxResult
	err = json.Unmarshal(res, &blockResult)
	if err != nil {
		return nil, err
	}

	return &blockResult, nil
}

// GetBeaconBlockVerboseTxAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetBeaconBlockVerboseTx or the blocking version and more details.
func (c *Client) GetBeaconBlockVerboseTxAsync(blockHash *chainhash.Hash) FutureGetBeaconBlockVerboseTxResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	// From the bitcoin-cli getblock documentation:
	//
	// If verbosity is 2, returns an Object with information about block
	// and information about each transaction.
	cmd := btcjson.NewGetBeaconBlockCmd(hash, btcjson.Int(2))
	return FutureGetBeaconBlockVerboseTxResult{
		client:   c,
		hash:     hash,
		Response: c.ForBeacon().sendCmd(cmd),
	}
}

// GetBeaconBlockVerboseTx returns a data structure from the server with information
// about a block and its transactions given its hash.
//
// See GetBeaconBlockVerbose if only transaction hashes are preferred.
// See GetBeaconBlock to retrieve a raw block instead.
func (c *Client) GetBeaconBlockVerboseTx(blockHash *chainhash.Hash) (*btcjson.GetBeaconBlockVerboseTxResult, error) {
	return c.GetBeaconBlockVerboseTxAsync(blockHash).Receive()
}

// FutureGetBeaconBlockHeaderResult is a future promise to deliver the result of a
// GetBeaconBlockHeaderAsync RPC invocation (or an applicable error).
type FutureGetBeaconBlockHeaderResult chan *response

// Receive waits for the response promised by the future and returns the
// blockheader requested from the server given its hash.
func (r FutureGetBeaconBlockHeaderResult) Receive() (wire.BlockHeader, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var bhHex string
	err = json.Unmarshal(res, &bhHex)
	if err != nil {
		return nil, err
	}

	serializedBH, err := hex.DecodeString(bhHex)
	if err != nil {
		return nil, err
	}

	// Deserialize the blockheader and return it.
	bh := wire.EmptyBeaconHeader()
	err = bh.Read(bytes.NewReader(serializedBH))
	if err != nil {
		return nil, err
	}

	return bh, err
}

// GetBeaconBlockHeaderAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBeaconBlockHeader for the blocking version and more details.
func (c *Client) GetBeaconBlockHeaderAsync(blockHash *chainhash.Hash) FutureGetBeaconBlockHeaderResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	cmd := btcjson.NewGetBeaconBlockHeaderCmd(hash, btcjson.Bool(false))
	return c.ForBeacon().sendCmd(cmd)
}

// GetBeaconBlockHeader returns the getBeaconBlockHeader from the server given its hash.
//
// See GetBeaconBlockHeaderVerbose to retrieve a data structure with information about the
// block instead.
func (c *Client) GetBeaconBlockHeader(blockHash *chainhash.Hash) (wire.BlockHeader, error) {
	return c.GetBeaconBlockHeaderAsync(blockHash).Receive()
}

// FutureGetBeaconBlockHeaderVerboseResult is a future promise to deliver the result of a
// GetBeaconBlockAsync RPC invocation (or an applicable error).
type FutureGetBeaconBlockHeaderVerboseResult chan *response

// Receive waits for the response promised by the future and returns the
// data structure of the blockheader requested from the server given its hash.
func (r FutureGetBeaconBlockHeaderVerboseResult) Receive() (*btcjson.GetBeaconBlockHeaderVerboseResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var bh btcjson.GetBeaconBlockHeaderVerboseResult
	err = json.Unmarshal(res, &bh)
	if err != nil {
		return nil, err
	}

	return &bh, nil
}

// GetBeaconBlockHeaderVerboseAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBeaconBlockHeader for the blocking version and more details.
func (c *Client) GetBeaconBlockHeaderVerboseAsync(blockHash *chainhash.Hash) FutureGetBeaconBlockHeaderVerboseResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	cmd := btcjson.NewGetBeaconBlockHeaderCmd(hash, btcjson.Bool(true))
	return c.ForBeacon().sendCmd(cmd)
}

// GetBeaconBlockHeaderVerbose returns a data structure with information about the
// blockheader from the server given its hash.
//
// See GetBeaconBlockHeader to retrieve a blockheader instead.
func (c *Client) GetBeaconBlockHeaderVerbose(blockHash *chainhash.Hash) (*btcjson.GetBeaconBlockHeaderVerboseResult, error) {
	return c.GetBeaconBlockHeaderVerboseAsync(blockHash).Receive()
}

// FutureGetBeaconBlockTemplateAsync is a future promise to deliver the result of a
// GetWorkAsync RPC invocation (or an applicable error).
type FutureGetBeaconBlockTemplateAsync chan *response

// Receive waits for the response promised by the future and returns the hash
// data to work on.
func (r FutureGetBeaconBlockTemplateAsync) Receive() (*btcjson.GetBeaconBlockTemplateResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a getwork result object.
	var result btcjson.GetBeaconBlockTemplateResult
	err = json.Unmarshal(res, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetBeaconBlockTemplateAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetWork for the blocking version and more details.
func (c *Client) GetBeaconBlockTemplateAsync(reqData *btcjson.TemplateRequest) FutureGetBeaconBlockTemplateAsync {
	cmd := btcjson.NewGetBeaconBlockTemplateCmd(reqData)
	return c.ForBeacon().sendCmd(cmd)
}

// GetBeaconBlockTemplate deals with generating and returning block templates to the caller.
func (c *Client) GetBeaconBlockTemplate(reqData *btcjson.TemplateRequest) (*btcjson.GetBeaconBlockTemplateResult, error) {
	return c.GetBeaconBlockTemplateAsync(reqData).Receive()
}

// FutureGetBeaconHeadersResult is a future promise to deliver the result of a
// getheaders RPC invocation (or an applicable error).
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
type FutureGetBeaconHeadersResult chan *response

// Receive waits for the response promised by the future and returns the
// getheaders result.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (r FutureGetBeaconHeadersResult) Receive() ([]wire.BeaconHeader, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a slice of strings.
	var result []string
	err = json.Unmarshal(res, &result)
	if err != nil {
		return nil, err
	}

	// Deserialize the []string into []chain.BlockHeader.
	headers := make([]wire.BeaconHeader, len(result))
	for i, headerHex := range result {
		serialized, err := hex.DecodeString(headerHex)
		if err != nil {
			return nil, err
		}
		header := wire.EmptyBeaconHeader()
		err = header.Read(bytes.NewReader(serialized))
		if err != nil {
			return nil, err
		}
		headers[i] = *header
	}
	return headers, nil
}

// GetBeaconHeadersAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the returned instance.
//
// See GetBeaconHeaders for the blocking version and more details.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (c *Client) GetBeaconHeadersAsync(blockLocators []chainhash.Hash, hashStop *chainhash.Hash) FutureGetBeaconHeadersResult {
	locators := make([]string, len(blockLocators))
	for i := range blockLocators {
		locators[i] = blockLocators[i].String()
	}
	hash := ""
	if hashStop != nil {
		hash = hashStop.String()
	}
	cmd := btcjson.NewGetBeaconHeadersCmd(locators, hash)
	return c.ForBeacon().sendCmd(cmd)
}

// GetBeaconHeaders mimics the wire protocol getheaders and headers messages by
// returning all headers on the main chain after the first known block in the
// locators, up until a block hash matches hashStop.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (c *Client) GetBeaconHeaders(blockLocators []chainhash.Hash, hashStop *chainhash.Hash) ([]wire.BeaconHeader, error) {
	return c.GetBeaconHeadersAsync(blockLocators, hashStop).Receive()
}

// FutureGetBeaconBlockBySerialNumberResult is a future promise to deliver the result of a
// GetBeaconBlockAsync RPC invocation (or an applicable error).
type FutureGetBeaconBlockBySerialNumberResult struct {
	client   *Client
	serialID int64
	Response chan *response
}

// Receive waits for the response promised by the future and returns the raw
// block requested from the server given its hash.
func (r FutureGetBeaconBlockBySerialNumberResult) Receive() (*wire.MsgBlock, int64, int64, error) {
	res, err := r.client.waitForGetBlockBySerialNumberRes(r.Response, "getBeaconBlockBySerialNumber", r.serialID, false, false)
	if err != nil {
		return nil, r.serialID, -1, err
	}
	// Unmarshal the raw result into a BlockResult.
	var blockResult btcjson.GetBeaconBlockBySerialNumberResult
	err = json.Unmarshal(res, &blockResult)
	if err != nil {
		return nil, r.serialID, -1, err
	}

	// Decode the serialized block hex to raw bytes.
	serializedBlock, err := hex.DecodeString(blockResult.Block)
	if err != nil {
		return nil, r.serialID, -1, err
	}

	// Deserialize the block and return it.
	var msgBlock = wire.EmptyBeaconBlock()
	err = msgBlock.Deserialize(bytes.NewReader(serializedBlock))
	if err != nil {
		return nil, r.serialID, -1, err
	}
	return &msgBlock, blockResult.SerialID, blockResult.PrevSerialID, nil
}

// GetBeaconBlockBySerialNumberAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBeaconBlockBySerialNumber for the blocking version and more details.
func (c *Client) GetBeaconBlockBySerialNumberAsync(serialID int64) FutureGetBeaconBlockBySerialNumberResult {
	cmd := btcjson.NewGetBeaconBlockBySerialNumberCmd(serialID, btcjson.Int(0))
	return FutureGetBeaconBlockBySerialNumberResult{
		client:   c,
		serialID: serialID,
		Response: c.ForBeacon().sendCmd(cmd),
	}
}

// GetBeaconBlockBySerialNumber returns a raw block from the server given its id.
//
// See GetBeaconBlockBySerialNumberVerbose to retrieve a data structure with information about the
// block instead.
func (c *Client) GetBeaconBlockBySerialNumber(serialID int64) (*wire.MsgBlock, int64, int64, error) {
	return c.GetBeaconBlockBySerialNumberAsync(serialID).Receive()
}

// FutureGetBeaconBlockVerboseBySerialNumberResult is a future promise to deliver the result of a
// GetBeaconBlockBySerialNumberAsync RPC invocation (or an applicable error).
type FutureGetBeaconBlockVerboseBySerialNumberResult struct {
	client   *Client
	serialID int64
	Response chan *response
}

// Receive waits for the response promised by the future and returns the data
// structure from the server with information about the requested block.
func (r FutureGetBeaconBlockVerboseBySerialNumberResult) Receive() (*btcjson.GetBeaconBlockVerboseResult, error) {
	res, err := r.client.waitForGetBlockBySerialNumberRes(r.Response, "getBeaconBlockBySerialNumber", r.serialID, true, false)
	if err != nil {
		return nil, err
	}

	// Unmarshal the raw result into a BlockResult.
	var blockResult btcjson.GetBeaconBlockVerboseResult
	err = json.Unmarshal(res, &blockResult)
	if err != nil {
		return nil, err
	}
	return &blockResult, nil
}

// GetBeaconBlockVerboseBySerialNumberAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetBeaconBlockVerboseBySerialNumber for the blocking version and more details.
func (c *Client) GetBeaconBlockVerboseBySerialNumberAsync(serialID int64) FutureGetBeaconBlockVerboseBySerialNumberResult {
	// From the bitcoin-cli getblock documentation:
	// "If verbosity is 1, returns an Object with information about block ."
	cmd := btcjson.NewGetBeaconBlockBySerialNumberCmd(serialID, btcjson.Int(1))
	return FutureGetBeaconBlockVerboseBySerialNumberResult{
		client:   c,
		serialID: serialID,
		Response: c.ForBeacon().sendCmd(cmd),
	}
}

// GetBeaconBlockVerboseBySerialNumber returns a data structure from the server with information
// about a block given its hash.
//
// See GetBeaconBlockVerboseTx to retrieve transaction data structures as well.
// See GetBeaconBlockBySerialNumber to retrieve a raw block instead.
func (c *Client) GetBeaconBlockVerboseBySerialNumber(serialID int64) (*btcjson.GetBeaconBlockVerboseResult, error) {
	return c.GetBeaconBlockVerboseBySerialNumberAsync(serialID).Receive()
}

// waitForGetBlockBySerialNumberRes waits for the response of a getblock request. If the
// response indicates an invalid parameter was provided, a legacy style of the
// request is resent and its response is returned instead.
func (c *Client) waitForGetBlockBySerialNumberRes(respChan chan *response, cmd string, serialID int64,
	verbose, verboseTx bool) ([]byte, error) {

	res, err := receiveFuture(respChan)

	// If we receive an invalid parameter error, then we may be
	// communicating with a btcd node which only understands the legacy
	// request, so we'll try that.
	// if err, ok := err.(*btcjson.RPCError); ok &&
	// 	err.Code == btcjson.ErrRPCInvalidParams.Code {
	// 	return c.legacyGetBlockRequest(cmd, hash, verbose, verboseTx)
	// }

	// Otherwise, we can return the response as is.
	return res, err
}
