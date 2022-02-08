// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcclient

import (
	"bytes"
	"encoding/hex"
	"encoding/json"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
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
func (r FutureGetShardBlockResult) Receive() (*BlockResult, error) {
	res, err := r.client.waitForGetBlockRes(r.Response, r.hash, "getShardBlock", false, false)
	if err != nil {
		return nil, err
	}

	// Unmarshal the raw result into a BlockResult.
	var blockResult jaxjson.GetBlockResult
	err = blockResult.UnmarshalJSON(res)
	if err != nil {
		return nil, err
	}

	// Decode the serialized block hex to raw bytes.
	serializedBlock, err := hex.DecodeString(blockResult.Block)
	if err != nil {
		return nil, err
	}

	// Deserialize the block and return it.
	msgBlock := wire.EmptyShardBlock()
	err = msgBlock.Deserialize(bytes.NewReader(serializedBlock))
	if err != nil {
		return nil, err
	}
	return &BlockResult{
		Block:        &msgBlock,
		Height:       blockResult.Height,
		SerialID:     blockResult.SerialID,
		PrevSerialID: blockResult.PrevSerialID,
	}, nil
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

	cmd := jaxjson.NewGetShardBlockCmd(hash, jaxjson.Int(0))
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
func (c *Client) GetShardBlock(blockHash *chainhash.Hash) (*BlockResult, error) {
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
func (r FutureGetShardBlockVerboseResult) Receive() (*jaxjson.GetShardBlockVerboseResult, error) {
	res, err := r.client.waitForGetBlockRes(r.Response, "getShardBlock", r.hash, true, false)
	if err != nil {
		return nil, err
	}

	// Unmarshal the raw result into a BlockResult.
	var blockResult jaxjson.GetShardBlockVerboseResult
	err = blockResult.UnmarshalJSON(res)
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
	cmd := jaxjson.NewGetShardBlockCmd(hash, jaxjson.Int(1))
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
func (c *Client) GetShardBlockVerbose(blockHash *chainhash.Hash) (*jaxjson.GetShardBlockVerboseResult, error) {
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
func (r FutureGetShardBlockVerboseTxResult) Receive() (*jaxjson.GetShardBlockVerboseResult, error) {
	res, err := r.client.waitForGetBlockRes(r.Response, "getShardBlock", r.hash, true, true)
	if err != nil {
		return nil, err
	}

	var blockResult jaxjson.GetShardBlockVerboseResult
	err = blockResult.UnmarshalJSON(res)
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
	cmd := jaxjson.NewGetShardBlockCmd(hash, jaxjson.Int(2))
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
func (c *Client) GetShardBlockVerboseTx(blockHash *chainhash.Hash) (*jaxjson.GetShardBlockVerboseResult, error) {
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

	cmd := jaxjson.NewGetShardBlockHeaderCmd(hash, jaxjson.Bool(false))
	return c.sendCmd(cmd)
}

// GetShardBlockHeader returns the getShardBlockHeader from the server given its hash.
//
// See GetShardBlockHeaderVerbose to retrieve a data structure with information about the
// block instead.
func (c *Client) GetShardBlockHeader(blockHash *chainhash.Hash) (wire.BlockHeader, error) {
	return c.GetShardBlockHeaderAsync(blockHash).Receive()
}

// FutureGetShardBlockHeaderVerboseResult is a future promise to deliver the result of a
// GetShardBlockAsync RPC invocation (or an applicable error).
type FutureGetShardBlockHeaderVerboseResult chan *response

// Receive waits for the response promised by the future and returns the
// data structure of the blockheader requested from the server given its hash.
func (r FutureGetShardBlockHeaderVerboseResult) Receive() (*jaxjson.GetShardBlockHeaderVerboseResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var bh jaxjson.GetShardBlockHeaderVerboseResult
	err = bh.UnmarshalJSON(res)
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

	cmd := jaxjson.NewGetShardBlockHeaderCmd(hash, jaxjson.Bool(true))
	return c.sendCmd(cmd)
}

// GetShardBlockHeaderVerbose returns a data structure with information about the
// blockheader from the server given its hash.
//
// See GetShardBlockHeader to retrieve a blockheader instead.
func (c *Client) GetShardBlockHeaderVerbose(blockHash *chainhash.Hash) (*jaxjson.GetShardBlockHeaderVerboseResult, error) {
	return c.GetShardBlockHeaderVerboseAsync(blockHash).Receive()
}

// FutureGetBlockTemplateAsync is a future promise to deliver the result of a
// GetWorkAsync RPC invocation (or an applicable error).
type FutureGetBlockTemplateAsync chan *response

// Receive waits for the response promised by the future and returns the hash
// data to work on.
func (r FutureGetBlockTemplateAsync) Receive() (*jaxjson.GetBlockTemplateResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a getwork result object.
	var result jaxjson.GetBlockTemplateResult
	err = result.UnmarshalJSON(res)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetBlockTemplateAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetWork for the blocking version and more details.
func (c *Client) GetBlockTemplateAsync(reqData *jaxjson.TemplateRequest) FutureGetBlockTemplateAsync {
	cmd := &jaxjson.GetBlockTemplateCmd{Request: reqData}
	return c.sendCmd(cmd)
}

// GetBlockTemplate deals with generating and returning block templates to the caller.
func (c *Client) GetBlockTemplate(reqData *jaxjson.TemplateRequest) (*jaxjson.GetBlockTemplateResult, error) {
	return c.GetBlockTemplateAsync(reqData).Receive()
}

// FutureGetShardBlockTemplateAsync is a future promise to deliver the result of a
// GetWorkAsync RPC invocation (or an applicable error).
type FutureGetShardBlockTemplateAsync chan *response

// Receive waits for the response promised by the future and returns the hash
// data to work on.
func (r FutureGetShardBlockTemplateAsync) Receive() (*jaxjson.GetBlockTemplateResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a getwork result object.
	var result jaxjson.GetBlockTemplateResult
	err = result.UnmarshalJSON(res)
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
func (c *Client) GetShardBlockTemplateAsync(reqData *jaxjson.TemplateRequest) FutureGetShardBlockTemplateAsync {
	cmd := jaxjson.NewGetShardBlockTemplateCmd(reqData)
	return c.sendCmd(cmd)
}

// GetShardBlockTemplate deals with generating and returning block templates to the caller.
// DEPRECATED: use the GetBlockTemplate insead.
func (c *Client) GetShardBlockTemplate(reqData *jaxjson.TemplateRequest) (*jaxjson.GetBlockTemplateResult, error) {
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
	cmd := jaxjson.NewGetShardHeadersCmd(locators, hash)
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

// FutureGetShardBlockBySerialNumberResult is a future promise to deliver the result of a
// GetShardBlockAsync RPC invocation (or an applicable error).
type FutureGetShardBlockBySerialNumberResult struct {
	client   *Client
	serialID int64
	Response chan *response
}

type BlockResult struct {
	Block        *wire.MsgBlock
	Height       int32
	SerialID     int64
	PrevSerialID int64
}

// Receive waits for the response promised by the future and returns the raw
// block requested from the server given its hash.
func (r FutureGetShardBlockBySerialNumberResult) Receive() (*BlockResult, error) {
	res, err := r.client.waitForGetBlockBySerialNumberRes(r.Response, "getShardBlockBySerialNumber", r.serialID, false, false)
	if err != nil {
		return nil, err
	}

	// Unmarshal the raw result into a BlockResult.
	var blockResult jaxjson.GetBlockResult
	err = blockResult.UnmarshalJSON(res)
	if err != nil {
		return nil, err
	}

	// Decode the serialized block hex to raw bytes.
	serializedBlock, err := hex.DecodeString(blockResult.Block)
	if err != nil {
		return nil, err
	}

	// Deserialize the block and return it.
	msgBlock := wire.EmptyShardBlock()
	err = msgBlock.Deserialize(bytes.NewReader(serializedBlock))
	if err != nil {
		return nil, err
	}
	return &BlockResult{
		Block:        &msgBlock,
		Height:       blockResult.Height,
		SerialID:     blockResult.SerialID,
		PrevSerialID: blockResult.PrevSerialID,
	}, nil
}

// GetShardBlockBySerialNumberAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetShardBlockBySerialNumber for the blocking version and more details.
func (c *Client) GetShardBlockBySerialNumberAsync(serialID int64) FutureGetShardBlockBySerialNumberResult {
	cmd := jaxjson.NewGetShardBlockBySerialNumberCmd(serialID, jaxjson.Int(0))
	return FutureGetShardBlockBySerialNumberResult{
		client:   c,
		serialID: serialID,
		Response: c.sendCmd(cmd),
	}
}

// GetShardBlockBySerialNumber returns a list of raw blocks from the server given its id and limit.
//
// See GetShardBlockBySerialNumberVerbose to retrieve a data structure with information about the
// block instead.
func (c *Client) GetShardBlockBySerialNumber(serialID int64) (*BlockResult, error) {
	return c.GetShardBlockBySerialNumberAsync(serialID).Receive()
}

// FutureListShardBlocksBySerialNumberResult is a future promise to deliver the result of a
// GetBeaconBlockAsync RPC invocation (or an applicable error).
type FutureListShardBlocksBySerialNumberResult struct {
	client   *Client
	serialID int64
	Response chan *response
}

// Receive waits for the response promised by the future and returns the raw
// block requested from the server given its hash.
func (r FutureListShardBlocksBySerialNumberResult) Receive() ([]*BlockResult, error) {
	res, err := r.client.waitForGetBlockBySerialNumberRes(r.Response, "listShardBlocksBySerialNumber", r.serialID, false, false)
	if err != nil {
		return nil, err
	}
	// Unmarshal the raw result into a BlockResult.
	var blockResults jaxjson.GetBlockResults
	err = blockResults.UnmarshalJSON(res)
	if err != nil {
		return nil, err
	}

	var output []*BlockResult
	for i := 0; i < len(blockResults); i++ {
		serializedBlock, err := hex.DecodeString(blockResults[i].Block)
		if err != nil {
			return nil, err
		}

		// Deserialize the block and return it.
		msgBlock := wire.EmptyShardBlock()
		err = msgBlock.Deserialize(bytes.NewReader(serializedBlock))
		if err != nil {
			return nil, err
		}

		output = append(output, &BlockResult{
			Block:        &msgBlock,
			Height:       blockResults[i].Height,
			SerialID:     blockResults[i].SerialID,
			PrevSerialID: blockResults[i].PrevSerialID,
		})
	}

	return output, nil
}

// ListShardBlocksBySerialNumberAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ListShardBlocksBySerialNumber for the blocking version and more details.
func (c *Client) ListShardBlocksBySerialNumberAsync(serialID int64, limit int) FutureListShardBlocksBySerialNumberResult {
	cmd := jaxjson.NewListShardBlocksBySerialNumberCmd(serialID, jaxjson.Int(0), jaxjson.Int(limit))
	return FutureListShardBlocksBySerialNumberResult{
		client:   c,
		serialID: serialID,
		Response: c.sendCmd(cmd),
	}
}

// ListShardBlocksBySerialNumber returns a raw block from the server given its id.
//
// See GetBeaconBlockBySerialNumberVerbose to retrieve a data structure with information about the
// block instead.
func (c *Client) ListShardBlocksBySerialNumber(serialID int64, limit int) ([]*BlockResult, error) {
	return c.ListShardBlocksBySerialNumberAsync(serialID, limit).Receive()
}

// FutureGetShardBlockVerboseBySerialNumberResult is a future promise to deliver the result of a
// GetShardBlockBySerialNumberAsync RPC invocation (or an applicable error).
type FutureGetShardBlockVerboseBySerialNumberResult struct {
	client   *Client
	serialID int64
	Response chan *response
}

// Receive waits for the response promised by the future and returns the data
// structure from the server with information about the requested block.
func (r FutureGetShardBlockVerboseBySerialNumberResult) Receive() (*jaxjson.GetShardBlockVerboseResult, error) {
	res, err := r.client.waitForGetBlockBySerialNumberRes(r.Response, "getShardBlockBySerialNumber", r.serialID, true, false)
	if err != nil {
		return nil, err
	}

	// Unmarshal the raw result into a BlockResult.
	var blockResult jaxjson.GetShardBlockVerboseResult
	err = blockResult.UnmarshalJSON(res)
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
func (c *Client) GetShardBlockVerboseBySerialNumberAsync(serialID int64) FutureGetShardBlockVerboseBySerialNumberResult {
	// From the bitcoin-cli getblock documentation:
	// "If verbosity is 1, returns an Object with information about block ."
	cmd := jaxjson.NewGetShardBlockBySerialNumberCmd(serialID, jaxjson.Int(1))
	return FutureGetShardBlockVerboseBySerialNumberResult{
		client:   c,
		serialID: serialID,
		Response: c.sendCmd(cmd),
	}
}

// GetShardBlockVerboseBySerialNumber returns a data structure from the server with information
// about a block given its hash.
//
// See GetShardBlockVerboseTx to retrieve transaction data structures as well.
// See GetShardBlockBySerialNumber to retrieve a raw block instead.
func (c *Client) GetShardBlockVerboseBySerialNumber(serialID int64) (*jaxjson.GetShardBlockVerboseResult, error) {
	return c.GetShardBlockVerboseBySerialNumberAsync(serialID).Receive()
}
