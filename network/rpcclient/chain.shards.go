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

// FutureGetShardBlockResult is a future promise to deliver the result of a
// GetShardBlockAsync RPC invocation (or an applicable error).
type FutureGetShardBlockResult struct {
	client   *Client
	hash     string
	Response chan *response
}

// Receive waits for the response promised by the future and returns the raw
// block requested from the server given its hash.
func (r FutureGetShardBlockResult) Receive() (*wire.MsgBlock, error) {
	res, err := r.client.waitForGetBlockRes(r.Response, r.hash, "getShardBlock", false, false)
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
	var msgBlock = wire.EmptyShardBlock()
	err = msgBlock.Deserialize(bytes.NewReader(serializedBlock))
	if err != nil {
		return nil, err
	}
	return &msgBlock, nil
}

// GetShardBlockAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetShardBlock for the blocking version and more details.
func (c *Client) GetShardBlockAsync(blockHash *chainhash.Hash) FutureGetShardBlockResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	cmd := btcjson.NewGetShardBlockCmd(hash, btcjson.Int(0))
	return FutureGetShardBlockResult{
		client:   c,
		hash:     hash,
		Response: c.sendCmd(cmd),
	}
}

// GetShardBlock returns a raw block from the server given its hash.
//
// See GetShardBlockVerbose to retrieve a data structure with information about the
// block instead.
func (c *Client) GetShardBlock(blockHash *chainhash.Hash) (*wire.MsgBlock, error) {
	return c.GetShardBlockAsync(blockHash).Receive()
}

// FutureGetShardBlockVerboseResult is a future promise to deliver the result of a
// GetShardBlockVerboseAsync RPC invocation (or an applicable error).
type FutureGetShardBlockVerboseResult struct {
	client   *Client
	hash     string
	Response chan *response
}

// Receive waits for the response promised by the future and returns the data
// structure from the server with information about the requested block.
func (r FutureGetShardBlockVerboseResult) Receive() (*btcjson.GetShardBlockVerboseResult, error) {
	res, err := r.client.waitForGetBlockRes(r.Response, "getShardBlock", r.hash, true, false)
	if err != nil {
		return nil, err
	}

	// Unmarshal the raw result into a BlockResult.
	var blockResult btcjson.GetShardBlockVerboseResult
	err = json.Unmarshal(res, &blockResult)
	if err != nil {
		return nil, err
	}
	return &blockResult, nil
}

// GetShardBlockVerboseAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetShardBlockVerbose for the blocking version and more details.
func (c *Client) GetShardBlockVerboseAsync(blockHash *chainhash.Hash) FutureGetShardBlockVerboseResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}
	// From the bitcoin-cli getblock documentation:
	// "If verbosity is 1, returns an Object with information about block ."
	cmd := btcjson.NewGetShardBlockCmd(hash, btcjson.Int(1))
	return FutureGetShardBlockVerboseResult{
		client:   c,
		hash:     hash,
		Response: c.sendCmd(cmd),
	}
}

// GetShardBlockVerbose returns a data structure from the server with information
// about a block given its hash.
//
// See GetShardBlockVerboseTx to retrieve transaction data structures as well.
// See GetShardBlock to retrieve a raw block instead.
func (c *Client) GetShardBlockVerbose(blockHash *chainhash.Hash) (*btcjson.GetShardBlockVerboseResult, error) {
	return c.GetShardBlockVerboseAsync(blockHash).Receive()
}

// FutureGetShardBlockVerboseTxResult is a future promise to deliver the result of a
// GetShardBlockVerboseTxResult RPC invocation (or an applicable error).
type FutureGetShardBlockVerboseTxResult struct {
	client   *Client
	hash     string
	Response chan *response
}

// Receive waits for the response promised by the future and returns a verbose
// version of the block including detailed information about its transactions.
func (r FutureGetShardBlockVerboseTxResult) Receive() (*btcjson.GetShardBlockVerboseTxResult, error) {
	res, err := r.client.waitForGetBlockRes(r.Response, "getShardBlock", r.hash, true, true)
	if err != nil {
		return nil, err
	}

	var blockResult btcjson.GetShardBlockVerboseTxResult
	err = json.Unmarshal(res, &blockResult)
	if err != nil {
		return nil, err
	}

	return &blockResult, nil
}

// GetShardBlockVerboseTxAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetShardBlockVerboseTx or the blocking version and more details.
func (c *Client) GetShardBlockVerboseTxAsync(blockHash *chainhash.Hash) FutureGetShardBlockVerboseTxResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	// From the bitcoin-cli getblock documentation:
	//
	// If verbosity is 2, returns an Object with information about block
	// and information about each transaction.
	cmd := btcjson.NewGetShardBlockCmd(hash, btcjson.Int(2))
	return FutureGetShardBlockVerboseTxResult{
		client:   c,
		hash:     hash,
		Response: c.sendCmd(cmd),
	}
}

// GetShardBlockVerboseTx returns a data structure from the server with information
// about a block and its transactions given its hash.
//
// See GetShardBlockVerbose if only transaction hashes are preferred.
// See GetShardBlock to retrieve a raw block instead.
func (c *Client) GetShardBlockVerboseTx(blockHash *chainhash.Hash) (*btcjson.GetShardBlockVerboseTxResult, error) {
	return c.GetShardBlockVerboseTxAsync(blockHash).Receive()
}

// FutureGetShardBlockHeaderResult is a future promise to deliver the result of a
// GetShardBlockHeaderAsync RPC invocation (or an applicable error).
type FutureGetShardBlockHeaderResult chan *response

// Receive waits for the response promised by the future and returns the
// blockheader requested from the server given its hash.
func (r FutureGetShardBlockHeaderResult) Receive() (wire.BlockHeader, error) {
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
	bh := wire.EmptyShardHeader()
	err = bh.Read(bytes.NewReader(serializedBH))
	if err != nil {
		return nil, err
	}

	return bh, err
}

// GetShardBlockHeaderAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetShardBlockHeader for the blocking version and more details.
func (c *Client) GetShardBlockHeaderAsync(blockHash *chainhash.Hash) FutureGetShardBlockHeaderResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	cmd := btcjson.NewGetShardBlockHeaderCmd(hash, btcjson.Bool(false))
	return c.sendCmd(cmd)
}

// GetShardBlockHeader returns the getShardBlockHeader from the server given its hash.
//
// See GetShardBlockHeaderVerbose to retrieve a data structure with information about the
// block instead.
func (c *Client) GetShardBlockHeader(blockHash *chainhash.Hash) (wire.BlockHeader, error) {
	return c.GetShardBlockHeaderAsync(blockHash).Receive()
}

// FutureGetBeaconBlockHeaderVerboseResult is a future promise to deliver the result of a
// GetShardBlockAsync RPC invocation (or an applicable error).
type FutureGetShardBlockHeaderVerboseResult chan *response

// Receive waits for the response promised by the future and returns the
// data structure of the blockheader requested from the server given its hash.
func (r FutureGetShardBlockHeaderVerboseResult) Receive() (*btcjson.GetShardBlockHeaderVerboseResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var bh btcjson.GetShardBlockHeaderVerboseResult
	err = json.Unmarshal(res, &bh)
	if err != nil {
		return nil, err
	}

	return &bh, nil
}

// GetShardBlockHeaderVerboseAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetShardBlockHeader for the blocking version and more details.
func (c *Client) GetShardBlockHeaderVerboseAsync(blockHash *chainhash.Hash) FutureGetShardBlockHeaderVerboseResult {
	hash := ""
	if blockHash != nil {
		hash = blockHash.String()
	}

	cmd := btcjson.NewGetShardBlockHeaderCmd(hash, btcjson.Bool(true))
	return c.sendCmd(cmd)
}

// GetShardBlockHeaderVerbose returns a data structure with information about the
// blockheader from the server given its hash.
//
// See GetShardBlockHeader to retrieve a blockheader instead.
func (c *Client) GetShardBlockHeaderVerbose(blockHash *chainhash.Hash) (*btcjson.GetShardBlockHeaderVerboseResult, error) {
	return c.GetShardBlockHeaderVerboseAsync(blockHash).Receive()
}

// FutureGetShardBlockTemplateAsync is a future promise to deliver the result of a
// GetWorkAsync RPC invocation (or an applicable error).
type FutureGetShardBlockTemplateAsync chan *response

// Receive waits for the response promised by the future and returns the hash
// data to work on.
func (r FutureGetShardBlockTemplateAsync) Receive() (*btcjson.GetShardBlockTemplateResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a getwork result object.
	var result btcjson.GetShardBlockTemplateResult
	err = json.Unmarshal(res, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetShardBlockTemplateAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetWork for the blocking version and more details.
func (c *Client) GetShardBlockTemplateAsync(reqData *btcjson.TemplateRequest) FutureGetShardBlockTemplateAsync {
	cmd := btcjson.NewGetShardBlockTemplateCmd(reqData)
	return c.sendCmd(cmd)
}

// GetShardBlockTemplate deals with generating and returning block templates to the caller.
func (c *Client) GetShardBlockTemplate(reqData *btcjson.TemplateRequest) (*btcjson.GetShardBlockTemplateResult, error) {
	return c.GetShardBlockTemplateAsync(reqData).Receive()
}

// FutureGetShardHeadersResult is a future promise to deliver the result of a
// getheaders RPC invocation (or an applicable error).
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
type FutureGetShardHeadersResult chan *response

// Receive waits for the response promised by the future and returns the
// getheaders result.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (r FutureGetShardHeadersResult) Receive() ([]wire.ShardHeader, error) {
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
	headers := make([]wire.ShardHeader, len(result))
	for i, headerHex := range result {
		serialized, err := hex.DecodeString(headerHex)
		if err != nil {
			return nil, err
		}
		header := wire.EmptyShardHeader()
		err = header.Read(bytes.NewReader(serialized))
		if err != nil {
			return nil, err
		}
		headers[i] = *header
	}
	return headers, nil
}

// GetShardHeadersAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the returned instance.
//
// See GetShardHeaders for the blocking version and more details.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (c *Client) GetShardHeadersAsync(blockLocators []chainhash.Hash, hashStop *chainhash.Hash) FutureGetShardHeadersResult {
	locators := make([]string, len(blockLocators))
	for i := range blockLocators {
		locators[i] = blockLocators[i].String()
	}
	hash := ""
	if hashStop != nil {
		hash = hashStop.String()
	}
	cmd := btcjson.NewGetShardHeadersCmd(locators, hash)
	return c.sendCmd(cmd)
}

// GetShardHeaders mimics the wire protocol getheaders and headers messages by
// returning all headers on the main chain after the first known block in the
// locators, up until a block hash matches hashStop.
//
// NOTE: This is a btcsuite extension ported from
// github.com/decred/dcrrpcclient.
func (c *Client) GetShardHeaders(blockLocators []chainhash.Hash, hashStop *chainhash.Hash) ([]wire.ShardHeader, error) {
	return c.GetShardHeadersAsync(blockLocators, hashStop).Receive()
}

// FutureGetShardBlockResult is a future promise to deliver the result of a
// GetShardBlockAsync RPC invocation (or an applicable error).
type FutureGetShardBlockBySerialNumberResult struct {
	client   *Client
	serialID int
	Response chan *response
}

// Receive waits for the response promised by the future and returns the raw
// block requested from the server given its hash.
func (r FutureGetShardBlockBySerialNumberResult) Receive() (*wire.MsgBlock, int, int, error) {
	res, err := r.client.waitForGetBlockBySerialNumberRes(r.Response, "getShardBlockBySerialNumber", r.serialID, false, false)
	if err != nil {
		return nil, r.serialID, -1, err
	}

	// Unmarshal the raw result into a BlockResult.
	var blockResult btcjson.GetShardBlockBySerialNumberResult
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
	var msgBlock = wire.EmptyShardBlock()
	err = msgBlock.Deserialize(bytes.NewReader(serializedBlock))
	if err != nil {
		return nil, r.serialID, -1, err
	}
	return &msgBlock, blockResult.SerialID, blockResult.PrevSerialID, nil
}

// GetShardBlockBySerialNumberAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetShardBlockBySerialNumber for the blocking version and more details.
func (c *Client) GetShardBlockBySerialNumberAsync(serialID int) FutureGetShardBlockBySerialNumberResult {
	cmd := btcjson.NewGetShardBlockBySerialNumberCmd(serialID, btcjson.Int(0))
	return FutureGetShardBlockBySerialNumberResult{
		client:   c,
		serialID: serialID,
		Response: c.sendCmd(cmd),
	}
}

// GetShardBlockBySerialNumber returns a raw block from the server given its id.
//
// See GetShardBlockBySerialNumberVerbose to retrieve a data structure with information about the
// block instead.
func (c *Client) GetShardBlockBySerialNumber(serialID int) (*wire.MsgBlock, int, int, error) {
	return c.GetShardBlockBySerialNumberAsync(serialID).Receive()
}


// FutureGetShardBlockVerboseBySerialNumberResult is a future promise to deliver the result of a
// GetShardBlockBySerialNumberAsync RPC invocation (or an applicable error).
type FutureGetShardBlockVerboseBySerialNumberResult struct {
	client   *Client
	serialID     int
	Response chan *response
}

// Receive waits for the response promised by the future and returns the data
// structure from the server with information about the requested block.
func (r FutureGetShardBlockVerboseBySerialNumberResult) Receive() (*btcjson.GetShardBlockBySerialNumberVerboseResult, error) {
	res, err := r.client.waitForGetBlockBySerialNumberRes(r.Response, "getShardBlockBySerialNumber", r.serialID, true, false)
	if err != nil {
		return nil, err
	}

	// Unmarshal the raw result into a BlockResult.
	var blockResult btcjson.GetShardBlockBySerialNumberVerboseResult
	err = json.Unmarshal(res, &blockResult)
	if err != nil {
		return nil, err
	}
	return &blockResult, nil
}

// GetShardBlockVerboseBySerialNumberAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetShardBlockVerboseBySerialNumber for the blocking version and more details.
func (c *Client) GetShardBlockVerboseBySerialNumberAsync(serialID int) FutureGetShardBlockVerboseBySerialNumberResult {
	// From the bitcoin-cli getblock documentation:
	// "If verbosity is 1, returns an Object with information about block ."
	cmd := btcjson.NewGetShardBlockBySerialNumberCmd(serialID, btcjson.Int(1))
	return FutureGetShardBlockVerboseBySerialNumberResult{
		client:   c,
		serialID:     serialID,
		Response: c.sendCmd(cmd),
	}
}

// GetShardBlockVerboseBySerialNumber returns a data structure from the server with information
// about a block given its hash.
//
// See GetShardBlockVerboseTx to retrieve transaction data structures as well.
// See GetShardBlockBySerialNumber to retrieve a raw block instead.
func (c *Client) GetShardBlockVerboseBySerialNumber(serialID int) (*btcjson.GetShardBlockBySerialNumberVerboseResult, error) {
	return c.GetShardBlockVerboseBySerialNumberAsync(serialID).Receive()
}


