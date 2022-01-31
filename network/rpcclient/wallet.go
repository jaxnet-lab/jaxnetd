// Copyright (c) 2014-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcclient

import (
	"encoding/json"
	"strconv"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// *****************************
// Transaction Listing Functions
// *****************************

// FutureGetTransactionResult is a future promise to deliver the result
// of a GetTransactionAsync or GetTransactionWatchOnlyAsync RPC invocation
// (or an applicable error).
type FutureGetTransactionResult chan *response

// Receive waits for the response promised by the future and returns detailed
// information about a wallet transaction.
func (r FutureGetTransactionResult) Receive() (*jaxjson.GetTransactionResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a gettransaction result object
	var getTx jaxjson.GetTransactionResult
	err = json.Unmarshal(res, &getTx)
	if err != nil {
		return nil, err
	}

	return &getTx, nil
}

// GetTransactionAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetTransaction for the blocking version and more details.
func (c *Client) GetTransactionAsync(txHash *chainhash.Hash) FutureGetTransactionResult {
	hash := ""
	if txHash != nil {
		hash = txHash.String()
	}

	cmd := jaxjson.NewGetTransactionCmd(hash, nil)
	return c.sendCmd(cmd)
}

// GetTransaction returns detailed information about a wallet transaction.
//
// See GetRawTransaction to return the raw transaction instead.
func (c *Client) GetTransaction(txHash *chainhash.Hash) (*jaxjson.GetTransactionResult, error) {
	return c.GetTransactionAsync(txHash).Receive()
}

// GetTransactionWatchOnlyAsync returns an instance of a type that can be used
// to get the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetTransactionWatchOnly for the blocking version and more details.
func (c *Client) GetTransactionWatchOnlyAsync(txHash *chainhash.Hash, watchOnly bool) FutureGetTransactionResult {
	hash := ""
	if txHash != nil {
		hash = txHash.String()
	}

	cmd := jaxjson.NewGetTransactionCmd(hash, &watchOnly)
	return c.sendCmd(cmd)
}

// GetTransactionWatchOnly returns detailed information about a wallet
// transaction, and allow including watch-only addresses in balance
// calculation and details.
func (c *Client) GetTransactionWatchOnly(txHash *chainhash.Hash, watchOnly bool) (*jaxjson.GetTransactionResult, error) {
	return c.GetTransactionWatchOnlyAsync(txHash, watchOnly).Receive()
}

// FutureListTransactionsResult is a future promise to deliver the result of a
// ListTransactionsAsync, ListTransactionsCountAsync, or
// ListTransactionsCountFromAsync RPC invocation (or an applicable error).
type FutureListTransactionsResult chan *response

// Receive waits for the response promised by the future and returns a list of
// the most recent transactions.
func (r FutureListTransactionsResult) Receive() ([]jaxjson.ListTransactionsResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as an array of listtransaction result objects.
	var transactions []jaxjson.ListTransactionsResult
	err = json.Unmarshal(res, &transactions)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

// ListTransactionsAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See ListTransactions for the blocking version and more details.
func (c *Client) ListTransactionsAsync(account string) FutureListTransactionsResult {
	cmd := jaxjson.NewListTransactionsCmd(&account, nil, nil, nil)
	return c.sendCmd(cmd)
}

// ListTransactions returns a list of the most recent transactions.
//
// See the ListTransactionsCount and ListTransactionsCountFrom to control the
// number of transactions returned and starting point, respectively.
func (c *Client) ListTransactions(account string) ([]jaxjson.ListTransactionsResult, error) {
	return c.ListTransactionsAsync(account).Receive()
}

// ListTransactionsCountAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListTransactionsCount for the blocking version and more details.
func (c *Client) ListTransactionsCountAsync(account string, count int) FutureListTransactionsResult {
	cmd := jaxjson.NewListTransactionsCmd(&account, &count, nil, nil)
	return c.sendCmd(cmd)
}

// ListTransactionsCount returns a list of the most recent transactions up
// to the passed count.
//
// See the ListTransactions and ListTransactionsCountFrom functions for
// different options.
func (c *Client) ListTransactionsCount(account string, count int) ([]jaxjson.ListTransactionsResult, error) {
	return c.ListTransactionsCountAsync(account, count).Receive()
}

// ListTransactionsCountFromAsync returns an instance of a type that can be used
// to get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListTransactionsCountFrom for the blocking version and more details.
func (c *Client) ListTransactionsCountFromAsync(account string, count, from int) FutureListTransactionsResult {
	cmd := jaxjson.NewListTransactionsCmd(&account, &count, &from, nil)
	return c.sendCmd(cmd)
}

// ListTransactionsCountFrom returns a list of the most recent transactions up
// to the passed count while skipping the first 'from' transactions.
//
// See the ListTransactions and ListTransactionsCount functions to use defaults.
func (c *Client) ListTransactionsCountFrom(account string, count, from int) ([]jaxjson.ListTransactionsResult, error) {
	return c.ListTransactionsCountFromAsync(account, count, from).Receive()
}

// ListTransactionsCountFromWatchOnlyAsync returns an instance of a type that can be used
// to get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListTransactionsCountFromWatchOnly for the blocking version and more details.
func (c *Client) ListTransactionsCountFromWatchOnlyAsync(account string, count, from int, watchOnly bool) FutureListTransactionsResult {
	cmd := jaxjson.NewListTransactionsCmd(&account, &count, &from, &watchOnly)
	return c.sendCmd(cmd)
}

// ListTransactionsCountFromWatchOnly returns a list of the most recent transactions up
// to the passed count while skipping the first 'from' transactions. It will include or
// exclude transactions from watch-only addresses based on the passed value for the watchOnly parameter
//
// See the ListTransactions and ListTransactionsCount functions to use defaults.
func (c *Client) ListTransactionsCountFromWatchOnly(account string, count, from int, watchOnly bool) ([]jaxjson.ListTransactionsResult, error) {
	return c.ListTransactionsCountFromWatchOnlyAsync(account, count, from, watchOnly).Receive()
}

// FutureListUnspentResult is a future promise to deliver the result of a
// ListUnspentAsync, ListUnspentMinAsync, ListUnspentMinMaxAsync, or
// ListUnspentMinMaxAddressesAsync RPC invocation (or an applicable error).
type FutureListUnspentResult chan *response

// Receive waits for the response promised by the future and returns all
// unspent wallet transaction outputs returned by the RPC call.  If the
// future wac returned by a call to ListUnspentMinAsync, ListUnspentMinMaxAsync,
// or ListUnspentMinMaxAddressesAsync, the range may be limited by the
// parameters of the RPC invocation.
func (r FutureListUnspentResult) Receive() ([]jaxjson.ListUnspentResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as an array of listunspent results.
	var unspent []jaxjson.ListUnspentResult
	err = json.Unmarshal(res, &unspent)
	if err != nil {
		return nil, err
	}

	return unspent, nil
}

// ListUnspentAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function
// on the returned instance.
//
// See ListUnspent for the blocking version and more details.
func (c *Client) ListUnspentAsync() FutureListUnspentResult {
	cmd := jaxjson.NewListUnspentCmd(nil, nil, nil)
	return c.sendCmd(cmd)
}

// ListUnspentMinAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function
// on the returned instance.
//
// See ListUnspentMin for the blocking version and more details.
func (c *Client) ListUnspentMinAsync(minConf int) FutureListUnspentResult {
	cmd := jaxjson.NewListUnspentCmd(&minConf, nil, nil)
	return c.sendCmd(cmd)
}

// ListUnspentMinMaxAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function
// on the returned instance.
//
// See ListUnspentMinMax for the blocking version and more details.
func (c *Client) ListUnspentMinMaxAsync(minConf, maxConf int) FutureListUnspentResult {
	cmd := jaxjson.NewListUnspentCmd(&minConf, &maxConf, nil)
	return c.sendCmd(cmd)
}

// ListUnspentMinMaxAddressesAsync returns an instance of a type that can be
// used to get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListUnspentMinMaxAddresses for the blocking version and more details.
func (c *Client) ListUnspentMinMaxAddressesAsync(minConf, maxConf int, addrs []jaxutil.Address) FutureListUnspentResult {
	addrStrs := make([]string, 0, len(addrs))
	for _, a := range addrs {
		addrStrs = append(addrStrs, a.EncodeAddress())
	}

	cmd := jaxjson.NewListUnspentCmd(&minConf, &maxConf, &addrStrs)
	return c.sendCmd(cmd)
}

// ListUnspent returns all unspent transaction outputs known to a wallet, using
// the default number of minimum and maximum number of confirmations as a
// filter (1 and 999999, respectively).
func (c *Client) ListUnspent() ([]jaxjson.ListUnspentResult, error) {
	return c.ListUnspentAsync().Receive()
}

// ListUnspentMin returns all unspent transaction outputs known to a wallet,
// using the specified number of minimum confirmations and default number of
// maximum confirmations (999999) as a filter.
func (c *Client) ListUnspentMin(minConf int) ([]jaxjson.ListUnspentResult, error) {
	return c.ListUnspentMinAsync(minConf).Receive()
}

// ListUnspentMinMax returns all unspent transaction outputs known to a wallet,
// using the specified number of minimum and maximum number of confirmations as
// a filter.
func (c *Client) ListUnspentMinMax(minConf, maxConf int) ([]jaxjson.ListUnspentResult, error) {
	return c.ListUnspentMinMaxAsync(minConf, maxConf).Receive()
}

// ListUnspentMinMaxAddresses returns all unspent transaction outputs that pay
// to any of specified addresses in a wallet using the specified number of
// minimum and maximum number of confirmations as a filter.
func (c *Client) ListUnspentMinMaxAddresses(minConf, maxConf int, addrs []jaxutil.Address) ([]jaxjson.ListUnspentResult, error) {
	return c.ListUnspentMinMaxAddressesAsync(minConf, maxConf, addrs).Receive()
}

// FutureListSinceBlockResult is a future promise to deliver the result of a
// ListSinceBlockAsync or ListSinceBlockMinConfAsync RPC invocation (or an
// applicable error).
type FutureListSinceBlockResult chan *response

// Receive waits for the response promised by the future and returns all
// transactions added in blocks since the specified block hash, or all
// transactions if it is nil.
func (r FutureListSinceBlockResult) Receive() (*jaxjson.ListSinceBlockResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a listsinceblock result object.
	var listResult jaxjson.ListSinceBlockResult
	err = json.Unmarshal(res, &listResult)
	if err != nil {
		return nil, err
	}

	return &listResult, nil
}

// ListSinceBlockAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See ListSinceBlock for the blocking version and more details.
func (c *Client) ListSinceBlockAsync(blockHash *chainhash.Hash) FutureListSinceBlockResult {
	var hash *string
	if blockHash != nil {
		hash = jaxjson.String(blockHash.String())
	}

	cmd := jaxjson.NewListSinceBlockCmd(hash, nil, nil)
	return c.sendCmd(cmd)
}

// ListSinceBlock returns all transactions added in blocks since the specified
// block hash, or all transactions if it is nil, using the default number of
// minimum confirmations as a filter.
//
// See ListSinceBlockMinConf to override the minimum number of confirmations.
// See ListSinceBlockMinConfWatchOnly to override the minimum number of confirmations and watch only parameter.
func (c *Client) ListSinceBlock(blockHash *chainhash.Hash) (*jaxjson.ListSinceBlockResult, error) {
	return c.ListSinceBlockAsync(blockHash).Receive()
}

// ListSinceBlockMinConfAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListSinceBlockMinConf for the blocking version and more details.
func (c *Client) ListSinceBlockMinConfAsync(blockHash *chainhash.Hash, minConfirms int) FutureListSinceBlockResult {
	var hash *string
	if blockHash != nil {
		hash = jaxjson.String(blockHash.String())
	}

	cmd := jaxjson.NewListSinceBlockCmd(hash, &minConfirms, nil)
	return c.sendCmd(cmd)
}

// ListSinceBlockMinConf returns all transactions added in blocks since the
// specified block hash, or all transactions if it is nil, using the specified
// number of minimum confirmations as a filter.
//
// See ListSinceBlock to use the default minimum number of confirmations.
func (c *Client) ListSinceBlockMinConf(blockHash *chainhash.Hash, minConfirms int) (*jaxjson.ListSinceBlockResult, error) {
	return c.ListSinceBlockMinConfAsync(blockHash, minConfirms).Receive()
}

// ListSinceBlockMinConfWatchOnlyAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListSinceBlockMinConfWatchOnly for the blocking version and more details.
func (c *Client) ListSinceBlockMinConfWatchOnlyAsync(blockHash *chainhash.Hash, minConfirms int, watchOnly bool) FutureListSinceBlockResult {
	var hash *string
	if blockHash != nil {
		hash = jaxjson.String(blockHash.String())
	}

	cmd := jaxjson.NewListSinceBlockCmd(hash, &minConfirms, &watchOnly)
	return c.sendCmd(cmd)
}

// ListSinceBlockMinConfWatchOnly returns all transactions added in blocks since the
// specified block hash, or all transactions if it is nil, using the specified
// number of minimum confirmations as a filter.
//
// See ListSinceBlock to use the default minimum number of confirmations and default watch only parameter.
func (c *Client) ListSinceBlockMinConfWatchOnly(blockHash *chainhash.Hash, minConfirms int, watchOnly bool) (*jaxjson.ListSinceBlockResult, error) {
	return c.ListSinceBlockMinConfWatchOnlyAsync(blockHash, minConfirms, watchOnly).Receive()
}

// **************************
// Transaction Send Functions
// **************************

// FutureLockUnspentResult is a future promise to deliver the error result of a
// LockUnspentAsync RPC invocation.
type FutureLockUnspentResult chan *response

// Receive waits for the response promised by the future and returns the result
// of locking or unlocking the unspent output(s).
func (r FutureLockUnspentResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// LockUnspentAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See LockUnspent for the blocking version and more details.
func (c *Client) LockUnspentAsync(unlock bool, ops []*wire.OutPoint) FutureLockUnspentResult {
	outputs := make([]jaxjson.TransactionInput, len(ops))
	for i, op := range ops {
		outputs[i] = jaxjson.TransactionInput{
			Txid: op.Hash.String(),
			Vout: op.Index,
		}
	}
	cmd := jaxjson.NewLockUnspentCmd(unlock, outputs)
	return c.sendCmd(cmd)
}

// LockUnspent marks outputs as locked or unlocked, depending on the value of
// the unlock bool.  When locked, the unspent output will not be selected as
// input for newly created, non-raw transactions, and will not be returned in
// future ListUnspent results, until the output is marked unlocked again.
//
// If unlock is false, each outpoint in ops will be marked locked.  If unlocked
// is true and specific outputs are specified in ops (len != 0), exactly those
// outputs will be marked unlocked.  If unlocked is true and no outpoints are
// specified, all previous locked outputs are marked unlocked.
//
// The locked or unlocked state of outputs are not written to disk and after
// restarting a wallet process, this data will be reset (every output unlocked).
//
// NOTE: While this method would be a bit more readable if the unlock bool was
// reversed (that is, LockUnspent(true, ...) locked the outputs), it has been
// left as unlock to keep compatibility with the reference client API and to
// avoid confusion for those who are already familiar with the lockunspent RPC.
func (c *Client) LockUnspent(unlock bool, ops []*wire.OutPoint) error {
	return c.LockUnspentAsync(unlock, ops).Receive()
}

// FutureListLockUnspentResult is a future promise to deliver the result of a
// ListLockUnspentAsync RPC invocation (or an applicable error).
type FutureListLockUnspentResult chan *response

// Receive waits for the response promised by the future and returns the result
// of all currently locked unspent outputs.
func (r FutureListLockUnspentResult) Receive() ([]*wire.OutPoint, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal as an array of transaction inputs.
	var inputs []jaxjson.TransactionInput
	err = json.Unmarshal(res, &inputs)
	if err != nil {
		return nil, err
	}

	// Create a slice of outpoints from the transaction input structs.
	ops := make([]*wire.OutPoint, len(inputs))
	for i, input := range inputs {
		sha, err := chainhash.NewHashFromStr(input.Txid)
		if err != nil {
			return nil, err
		}
		ops[i] = wire.NewOutPoint(sha, input.Vout)
	}

	return ops, nil
}

// ListLockUnspentAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See ListLockUnspent for the blocking version and more details.
func (c *Client) ListLockUnspentAsync() FutureListLockUnspentResult {
	cmd := jaxjson.NewListLockUnspentCmd()
	return c.sendCmd(cmd)
}

// ListLockUnspent returns a slice of outpoints for all unspent outputs marked
// as locked by a wallet.  Unspent outputs may be marked locked using
// LockOutput.
func (c *Client) ListLockUnspent() ([]*wire.OutPoint, error) {
	return c.ListLockUnspentAsync().Receive()
}

// FutureSetTxFeeResult is a future promise to deliver the result of a
// SetTxFeeAsync RPC invocation (or an applicable error).
type FutureSetTxFeeResult chan *response

// Receive waits for the response promised by the future and returns the result
// of setting an optional transaction fee per KB that helps ensure transactions
// are processed quickly.  Most transaction are 1KB.
func (r FutureSetTxFeeResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// SetTxFeeAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See SetTxFee for the blocking version and more details.
func (c *Client) SetTxFeeAsync(fee jaxutil.Amount) FutureSetTxFeeResult {
	cmd := jaxjson.NewSetTxFeeCmd(fee.ToBTC())
	return c.sendCmd(cmd)
}

// SetTxFee sets an optional transaction fee per KB that helps ensure
// transactions are processed quickly.  Most transaction are 1KB.
func (c *Client) SetTxFee(fee jaxutil.Amount) error {
	return c.SetTxFeeAsync(fee).Receive()
}

// FutureSendToAddressResult is a future promise to deliver the result of a
// SendToAddressAsync RPC invocation (or an applicable error).
type FutureSendToAddressResult chan *response

// Receive waits for the response promised by the future and returns the hash
// of the transaction sending the passed amount to the given address.
func (r FutureSendToAddressResult) Receive() (*chainhash.Hash, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var txHash string
	err = json.Unmarshal(res, &txHash)
	if err != nil {
		return nil, err
	}

	return chainhash.NewHashFromStr(txHash)
}

// SendToAddressAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See SendToAddress for the blocking version and more details.
func (c *Client) SendToAddressAsync(address jaxutil.Address, amount jaxutil.Amount) FutureSendToAddressResult {
	addr := address.EncodeAddress()
	cmd := jaxjson.NewSendToAddressCmd(addr, amount.ToBTC(), nil, nil)
	return c.sendCmd(cmd)
}

// SendToAddress sends the passed amount to the given address.
//
// See SendToAddressComment to associate comments with the transaction in the
// wallet.  The comments are not part of the transaction and are only internal
// to the wallet.
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) SendToAddress(address jaxutil.Address, amount jaxutil.Amount) (*chainhash.Hash, error) {
	return c.SendToAddressAsync(address, amount).Receive()
}

// SendToAddressCommentAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See SendToAddressComment for the blocking version and more details.
func (c *Client) SendToAddressCommentAsync(address jaxutil.Address,
	amount jaxutil.Amount, comment,
	commentTo string) FutureSendToAddressResult {

	addr := address.EncodeAddress()
	cmd := jaxjson.NewSendToAddressCmd(addr, amount.ToBTC(), &comment,
		&commentTo)
	return c.sendCmd(cmd)
}

// SendToAddressComment sends the passed amount to the given address and stores
// the provided comment and comment to in the wallet.  The comment parameter is
// intended to be used for the purpose of the transaction while the commentTo
// parameter is intended to be used for who the transaction is being sent to.
//
// The comments are not part of the transaction and are only internal
// to the wallet.
//
// See SendToAddress to avoid using comments.
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) SendToAddressComment(address jaxutil.Address, amount jaxutil.Amount, comment, commentTo string) (*chainhash.Hash, error) {
	return c.SendToAddressCommentAsync(address, amount, comment,
		commentTo).Receive()
}

// FutureSendFromResult is a future promise to deliver the result of a
// SendFromAsync, SendFromMinConfAsync, or SendFromCommentAsync RPC invocation
// (or an applicable error).
type FutureSendFromResult chan *response

// Receive waits for the response promised by the future and returns the hash
// of the transaction sending amount to the given address using the provided
// account as a source of funds.
func (r FutureSendFromResult) Receive() (*chainhash.Hash, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var txHash string
	err = json.Unmarshal(res, &txHash)
	if err != nil {
		return nil, err
	}

	return chainhash.NewHashFromStr(txHash)
}

// SendFromAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See SendFrom for the blocking version and more details.
func (c *Client) SendFromAsync(fromAccount string, toAddress jaxutil.Address, amount jaxutil.Amount) FutureSendFromResult {
	addr := toAddress.EncodeAddress()
	cmd := jaxjson.NewSendFromCmd(fromAccount, addr, amount.ToBTC(), nil,
		nil, nil)
	return c.sendCmd(cmd)
}

// SendFrom sends the passed amount to the given address using the provided
// account as a source of funds.  Only funds with the default number of minimum
// confirmations will be used.
//
// See SendFromMinConf and SendFromComment for different options.
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) SendFrom(fromAccount string, toAddress jaxutil.Address, amount jaxutil.Amount) (*chainhash.Hash, error) {
	return c.SendFromAsync(fromAccount, toAddress, amount).Receive()
}

// SendFromMinConfAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See SendFromMinConf for the blocking version and more details.
func (c *Client) SendFromMinConfAsync(fromAccount string, toAddress jaxutil.Address, amount jaxutil.Amount, minConfirms int) FutureSendFromResult {
	addr := toAddress.EncodeAddress()
	cmd := jaxjson.NewSendFromCmd(fromAccount, addr, amount.ToBTC(),
		&minConfirms, nil, nil)
	return c.sendCmd(cmd)
}

// SendFromMinConf sends the passed amount to the given address using the
// provided account as a source of funds.  Only funds with the passed number of
// minimum confirmations will be used.
//
// See SendFrom to use the default number of minimum confirmations and
// SendFromComment for additional options.
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) SendFromMinConf(fromAccount string, toAddress jaxutil.Address, amount jaxutil.Amount, minConfirms int) (*chainhash.Hash, error) {
	return c.SendFromMinConfAsync(fromAccount, toAddress, amount,
		minConfirms).Receive()
}

// SendFromCommentAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See SendFromComment for the blocking version and more details.
func (c *Client) SendFromCommentAsync(fromAccount string,
	toAddress jaxutil.Address, amount jaxutil.Amount, minConfirms int,
	comment, commentTo string) FutureSendFromResult {

	addr := toAddress.EncodeAddress()
	cmd := jaxjson.NewSendFromCmd(fromAccount, addr, amount.ToBTC(),
		&minConfirms, &comment, &commentTo)
	return c.sendCmd(cmd)
}

// SendFromComment sends the passed amount to the given address using the
// provided account as a source of funds and stores the provided comment and
// comment to in the wallet.  The comment parameter is intended to be used for
// the purpose of the transaction while the commentTo parameter is intended to
// be used for who the transaction is being sent to.  Only funds with the passed
// number of minimum confirmations will be used.
//
// See SendFrom and SendFromMinConf to use defaults.
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) SendFromComment(fromAccount string, toAddress jaxutil.Address,
	amount jaxutil.Amount, minConfirms int,
	comment, commentTo string) (*chainhash.Hash, error) {

	return c.SendFromCommentAsync(fromAccount, toAddress, amount,
		minConfirms, comment, commentTo).Receive()
}

// FutureSendManyResult is a future promise to deliver the result of a
// SendManyAsync, SendManyMinConfAsync, or SendManyCommentAsync RPC invocation
// (or an applicable error).
type FutureSendManyResult chan *response

// Receive waits for the response promised by the future and returns the hash
// of the transaction sending multiple amounts to multiple addresses using the
// provided account as a source of funds.
func (r FutureSendManyResult) Receive() (*chainhash.Hash, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmashal result as a string.
	var txHash string
	err = json.Unmarshal(res, &txHash)
	if err != nil {
		return nil, err
	}

	return chainhash.NewHashFromStr(txHash)
}

// SendManyAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See SendMany for the blocking version and more details.
func (c *Client) SendManyAsync(fromAccount string, amounts map[jaxutil.Address]jaxutil.Amount) FutureSendManyResult {
	convertedAmounts := make(map[string]float64, len(amounts))
	for addr, amount := range amounts {
		convertedAmounts[addr.EncodeAddress()] = amount.ToBTC()
	}
	cmd := jaxjson.NewSendManyCmd(fromAccount, convertedAmounts, nil, nil)
	return c.sendCmd(cmd)
}

// SendMany sends multiple amounts to multiple addresses using the provided
// account as a source of funds in a single transaction.  Only funds with the
// default number of minimum confirmations will be used.
//
// See SendManyMinConf and SendManyComment for different options.
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) SendMany(fromAccount string, amounts map[jaxutil.Address]jaxutil.Amount) (*chainhash.Hash, error) {
	return c.SendManyAsync(fromAccount, amounts).Receive()
}

// SendManyMinConfAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See SendManyMinConf for the blocking version and more details.
func (c *Client) SendManyMinConfAsync(fromAccount string,
	amounts map[jaxutil.Address]jaxutil.Amount,
	minConfirms int) FutureSendManyResult {

	convertedAmounts := make(map[string]float64, len(amounts))
	for addr, amount := range amounts {
		convertedAmounts[addr.EncodeAddress()] = amount.ToBTC()
	}
	cmd := jaxjson.NewSendManyCmd(fromAccount, convertedAmounts,
		&minConfirms, nil)
	return c.sendCmd(cmd)
}

// SendManyMinConf sends multiple amounts to multiple addresses using the
// provided account as a source of funds in a single transaction.  Only funds
// with the passed number of minimum confirmations will be used.
//
// See SendMany to use the default number of minimum confirmations and
// SendManyComment for additional options.
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) SendManyMinConf(fromAccount string,
	amounts map[jaxutil.Address]jaxutil.Amount,
	minConfirms int) (*chainhash.Hash, error) {

	return c.SendManyMinConfAsync(fromAccount, amounts, minConfirms).Receive()
}

// SendManyCommentAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See SendManyComment for the blocking version and more details.
func (c *Client) SendManyCommentAsync(fromAccount string,
	amounts map[jaxutil.Address]jaxutil.Amount, minConfirms int,
	comment string) FutureSendManyResult {

	convertedAmounts := make(map[string]float64, len(amounts))
	for addr, amount := range amounts {
		convertedAmounts[addr.EncodeAddress()] = amount.ToBTC()
	}
	cmd := jaxjson.NewSendManyCmd(fromAccount, convertedAmounts,
		&minConfirms, &comment)
	return c.sendCmd(cmd)
}

// SendManyComment sends multiple amounts to multiple addresses using the
// provided account as a source of funds in a single transaction and stores the
// provided comment in the wallet.  The comment parameter is intended to be used
// for the purpose of the transaction   Only funds with the passed number of
// minimum confirmations will be used.
//
// See SendMany and SendManyMinConf to use defaults.
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) SendManyComment(fromAccount string,
	amounts map[jaxutil.Address]jaxutil.Amount, minConfirms int,
	comment string) (*chainhash.Hash, error) {

	return c.SendManyCommentAsync(fromAccount, amounts, minConfirms,
		comment).Receive()
}

// *************************
// Address/Account Functions
// *************************

// FutureAddMultisigAddressResult is a future promise to deliver the result of a
// AddMultisigAddressAsync RPC invocation (or an applicable error).
type FutureAddMultisigAddressResult struct {
	responseChannel chan *response
	network         *chaincfg.Params
}

// Receive waits for the response promised by the future and returns the
// multisignature address that requires the specified number of signatures for
// the provided addresses.
func (r FutureAddMultisigAddressResult) Receive() (jaxutil.Address, error) {
	res, err := receiveFuture(r.responseChannel)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var addr string
	err = json.Unmarshal(res, &addr)
	if err != nil {
		return nil, err
	}

	return jaxutil.DecodeAddress(addr, r.network)
}

// AddMultisigAddressAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See AddMultisigAddress for the blocking version and more details.
func (c *Client) AddMultisigAddressAsync(requiredSigs int, addresses []jaxutil.Address, account string) FutureAddMultisigAddressResult {
	addrs := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		addrs = append(addrs, addr.String())
	}

	cmd := jaxjson.NewAddMultisigAddressCmd(requiredSigs, addrs, &account)
	result := FutureAddMultisigAddressResult{
		network:         c.chainParams,
		responseChannel: c.sendCmd(cmd),
	}
	return result
}

// AddMultisigAddress adds a multisignature address that requires the specified
// number of signatures for the provided addresses to the wallet.
func (c *Client) AddMultisigAddress(requiredSigs int, addresses []jaxutil.Address, account string) (jaxutil.Address, error) {
	return c.AddMultisigAddressAsync(requiredSigs, addresses, account).Receive()
}

// FutureCreateMultisigResult is a future promise to deliver the result of a
// CreateMultisigAsync RPC invocation (or an applicable error).
type FutureCreateMultisigResult chan *response

// Receive waits for the response promised by the future and returns the
// multisignature address and script needed to redeem it.
func (r FutureCreateMultisigResult) Receive() (*jaxjson.CreateMultiSigResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a createmultisig result object.
	var multisigRes jaxjson.CreateMultiSigResult
	err = json.Unmarshal(res, &multisigRes)
	if err != nil {
		return nil, err
	}

	return &multisigRes, nil
}

// CreateMultisigAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See CreateMultisig for the blocking version and more details.
func (c *Client) CreateMultisigAsync(requiredSigs int, addresses []jaxutil.Address) FutureCreateMultisigResult {
	addrs := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		addrs = append(addrs, addr.String())
	}

	cmd := jaxjson.NewCreateMultisigCmd(requiredSigs, addrs)
	return c.sendCmd(cmd)
}

// CreateMultisig creates a multisignature address that requires the specified
// number of signatures for the provided addresses and returns the
// multisignature address and script needed to redeem it.
func (c *Client) CreateMultisig(requiredSigs int, addresses []jaxutil.Address) (*jaxjson.CreateMultiSigResult, error) {
	return c.CreateMultisigAsync(requiredSigs, addresses).Receive()
}

// FutureCreateNewAccountResult is a future promise to deliver the result of a
// CreateNewAccountAsync RPC invocation (or an applicable error).
type FutureCreateNewAccountResult chan *response

// Receive waits for the response promised by the future and returns the
// result of creating new account.
func (r FutureCreateNewAccountResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// CreateNewAccountAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See CreateNewAccount for the blocking version and more details.
func (c *Client) CreateNewAccountAsync(account string) FutureCreateNewAccountResult {
	cmd := jaxjson.NewCreateNewAccountCmd(account)
	return c.sendCmd(cmd)
}

// CreateNewAccount creates a new wallet account.
func (c *Client) CreateNewAccount(account string) error {
	return c.CreateNewAccountAsync(account).Receive()
}

// FutureCreateWalletResult is a future promise to deliver the result of a
// CreateWalletAsync RPC invocation (or an applicable error).
type FutureCreateWalletResult chan *response

// Receive waits for the response promised by the future and returns the
// result of creating a new wallet.
func (r FutureCreateWalletResult) Receive() (*jaxjson.CreateWalletResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	var createWalletResult jaxjson.CreateWalletResult
	err = json.Unmarshal(res, &createWalletResult)
	if err != nil {
		return nil, err
	}
	return &createWalletResult, nil
}

// CreateWalletOpt defines a functional-option to be used with CreateWallet
// method.
type CreateWalletOpt func(*jaxjson.CreateWalletCmd)

// WithCreateWalletDisablePrivateKeys disables the possibility of private keys
// to be used with a wallet created using the CreateWallet method. Using this
// option will make the wallet watch-only.
func WithCreateWalletDisablePrivateKeys() CreateWalletOpt {
	return func(c *jaxjson.CreateWalletCmd) {
		c.DisablePrivateKeys = jaxjson.Bool(true)
	}
}

// WithCreateWalletBlank specifies creation of a blank wallet.
func WithCreateWalletBlank() CreateWalletOpt {
	return func(c *jaxjson.CreateWalletCmd) {
		c.Blank = jaxjson.Bool(true)
	}
}

// WithCreateWalletPassphrase specifies a passphrase to encrypt the wallet
// with.
func WithCreateWalletPassphrase(value string) CreateWalletOpt {
	return func(c *jaxjson.CreateWalletCmd) {
		c.Passphrase = jaxjson.String(value)
	}
}

// WithCreateWalletAvoidReuse specifies keeping track of coin reuse, and
// treat dirty and clean coins differently with privacy considerations in mind.
func WithCreateWalletAvoidReuse() CreateWalletOpt {
	return func(c *jaxjson.CreateWalletCmd) {
		c.AvoidReuse = jaxjson.Bool(true)
	}
}

// CreateWalletAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See CreateWallet for the blocking version and more details.
func (c *Client) CreateWalletAsync(name string, opts ...CreateWalletOpt) FutureCreateWalletResult {
	cmd := jaxjson.NewCreateWalletCmd(name, nil, nil, nil, nil)

	// Apply each specified option to mutate the default command.
	for _, opt := range opts {
		opt(cmd)
	}

	return c.sendCmd(cmd)
}

// CreateWallet creates a new wallet account, with the possibility to use
// private keys.
//
// Optional parameters can be specified using functional-options pattern. The
// following functions are available:
//   * WithCreateWalletDisablePrivateKeys
//   * WithCreateWalletBlank
//   * WithCreateWalletPassphrase
//   * WithCreateWalletAvoidReuse
func (c *Client) CreateWallet(name string, opts ...CreateWalletOpt) (*jaxjson.CreateWalletResult, error) {
	return c.CreateWalletAsync(name, opts...).Receive()
}

// FutureGetAddressInfoResult is a future promise to deliver the result of an
// GetAddressInfoAsync RPC invocation (or an applicable error).
type FutureGetAddressInfoResult chan *response

// Receive waits for the response promised by the future and returns the information
// about the given bitcoin address.
func (r FutureGetAddressInfoResult) Receive() (*jaxjson.GetAddressInfoResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	var getAddressInfoResult jaxjson.GetAddressInfoResult
	err = json.Unmarshal(res, &getAddressInfoResult)
	if err != nil {
		return nil, err
	}
	return &getAddressInfoResult, nil
}

// GetAddressInfoAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetAddressInfo for the blocking version and more details.
func (c *Client) GetAddressInfoAsync(address string) FutureGetAddressInfoResult {
	cmd := jaxjson.NewGetAddressInfoCmd(address)
	return c.sendCmd(cmd)
}

// GetAddressInfo returns information about the given bitcoin address.
func (c *Client) GetAddressInfo(address string) (*jaxjson.GetAddressInfoResult, error) {
	return c.GetAddressInfoAsync(address).Receive()
}

// FutureGetNewAddressResult is a future promise to deliver the result of a
// GetNewAddressAsync RPC invocation (or an applicable error).
type FutureGetNewAddressResult struct {
	responseChannel chan *response
	network         *chaincfg.Params
}

// Receive waits for the response promised by the future and returns a new
// address.
func (r FutureGetNewAddressResult) Receive() (jaxutil.Address, error) {
	res, err := receiveFuture(r.responseChannel)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var addr string
	err = json.Unmarshal(res, &addr)
	if err != nil {
		return nil, err
	}

	return jaxutil.DecodeAddress(addr, r.network)
}

// GetNewAddressAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetNewAddress for the blocking version and more details.
func (c *Client) GetNewAddressAsync(account string) FutureGetNewAddressResult {
	cmd := jaxjson.NewGetNewAddressCmd(&account)
	result := FutureGetNewAddressResult{
		network:         c.chainParams,
		responseChannel: c.sendCmd(cmd),
	}
	return result
}

// GetNewAddress returns a new address, and decodes based on the client's
// chain params.
func (c *Client) GetNewAddress(account string) (jaxutil.Address, error) {
	return c.GetNewAddressAsync(account).Receive()
}

// FutureGetRawChangeAddressResult is a future promise to deliver the result of
// a GetRawChangeAddressAsync RPC invocation (or an applicable error).
type FutureGetRawChangeAddressResult struct {
	responseChannel chan *response
	network         *chaincfg.Params
}

// Receive waits for the response promised by the future and returns a new
// address for receiving change that will be associated with the provided
// account.  Note that this is only for raw transactions and NOT for normal use.
func (r FutureGetRawChangeAddressResult) Receive() (jaxutil.Address, error) {
	res, err := receiveFuture(r.responseChannel)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var addr string
	err = json.Unmarshal(res, &addr)
	if err != nil {
		return nil, err
	}

	return jaxutil.DecodeAddress(addr, r.network)
}

// GetRawChangeAddressAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetRawChangeAddress for the blocking version and more details.
func (c *Client) GetRawChangeAddressAsync(account string) FutureGetRawChangeAddressResult {
	cmd := jaxjson.NewGetRawChangeAddressCmd(&account)
	result := FutureGetRawChangeAddressResult{
		network:         c.chainParams,
		responseChannel: c.sendCmd(cmd),
	}
	return result
}

// GetRawChangeAddress returns a new address for receiving change that will be
// associated with the provided account.  Note that this is only for raw
// transactions and NOT for normal use.
func (c *Client) GetRawChangeAddress(account string) (jaxutil.Address, error) {
	return c.GetRawChangeAddressAsync(account).Receive()
}

// FutureAddWitnessAddressResult is a future promise to deliver the result of
// a AddWitnessAddressAsync RPC invocation (or an applicable error).
type FutureAddWitnessAddressResult struct {
	responseChannel chan *response
	network         *chaincfg.Params
}

// Receive waits for the response promised by the future and returns the new
// address.
func (r FutureAddWitnessAddressResult) Receive() (jaxutil.Address, error) {
	res, err := receiveFuture(r.responseChannel)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var addr string
	err = json.Unmarshal(res, &addr)
	if err != nil {
		return nil, err
	}

	return jaxutil.DecodeAddress(addr, r.network)
}

// AddWitnessAddressAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See AddWitnessAddress for the blocking version and more details.
func (c *Client) AddWitnessAddressAsync(address string) FutureAddWitnessAddressResult {
	cmd := jaxjson.NewAddWitnessAddressCmd(address)
	response := FutureAddWitnessAddressResult{
		network:         c.chainParams,
		responseChannel: c.sendCmd(cmd),
	}
	return response
}

// AddWitnessAddress adds a witness address for a script and returns the new
// address (P2SH of the witness script).
func (c *Client) AddWitnessAddress(address string) (jaxutil.Address, error) {
	return c.AddWitnessAddressAsync(address).Receive()
}

// FutureGetAccountAddressResult is a future promise to deliver the result of a
// GetAccountAddressAsync RPC invocation (or an applicable error).
type FutureGetAccountAddressResult struct {
	responseChannel chan *response
	network         *chaincfg.Params
}

// Receive waits for the response promised by the future and returns the current
// Bitcoin address for receiving payments to the specified account.
func (r FutureGetAccountAddressResult) Receive() (jaxutil.Address, error) {
	res, err := receiveFuture(r.responseChannel)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var addr string
	err = json.Unmarshal(res, &addr)
	if err != nil {
		return nil, err
	}

	return jaxutil.DecodeAddress(addr, r.network)
}

// GetAccountAddressAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetAccountAddress for the blocking version and more details.
func (c *Client) GetAccountAddressAsync(account string) FutureGetAccountAddressResult {
	cmd := jaxjson.NewGetAccountAddressCmd(account)
	result := FutureGetAccountAddressResult{
		network:         c.chainParams,
		responseChannel: c.sendCmd(cmd),
	}
	return result
}

// GetAccountAddress returns the current Bitcoin address for receiving payments
// to the specified account.
func (c *Client) GetAccountAddress(account string) (jaxutil.Address, error) {
	return c.GetAccountAddressAsync(account).Receive()
}

// FutureGetAccountResult is a future promise to deliver the result of a
// GetAccountAsync RPC invocation (or an applicable error).
type FutureGetAccountResult chan *response

// Receive waits for the response promised by the future and returns the account
// associated with the passed address.
func (r FutureGetAccountResult) Receive() (string, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return "", err
	}

	// Unmarshal result as a string.
	var account string
	err = json.Unmarshal(res, &account)
	if err != nil {
		return "", err
	}

	return account, nil
}

// GetAccountAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetAccount for the blocking version and more details.
func (c *Client) GetAccountAsync(address jaxutil.Address) FutureGetAccountResult {
	addr := address.EncodeAddress()
	cmd := jaxjson.NewGetAccountCmd(addr)
	return c.sendCmd(cmd)
}

// GetAccount returns the account associated with the passed address.
func (c *Client) GetAccount(address jaxutil.Address) (string, error) {
	return c.GetAccountAsync(address).Receive()
}

// FutureSetAccountResult is a future promise to deliver the result of a
// SetAccountAsync RPC invocation (or an applicable error).
type FutureSetAccountResult chan *response

// Receive waits for the response promised by the future and returns the result
// of setting the account to be associated with the passed address.
func (r FutureSetAccountResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// SetAccountAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See SetAccount for the blocking version and more details.
func (c *Client) SetAccountAsync(address jaxutil.Address, account string) FutureSetAccountResult {
	addr := address.EncodeAddress()
	cmd := jaxjson.NewSetAccountCmd(addr, account)
	return c.sendCmd(cmd)
}

// SetAccount sets the account associated with the passed address.
func (c *Client) SetAccount(address jaxutil.Address, account string) error {
	return c.SetAccountAsync(address, account).Receive()
}

// FutureGetAddressesByAccountResult is a future promise to deliver the result
// of a GetAddressesByAccountAsync RPC invocation (or an applicable error).
type FutureGetAddressesByAccountResult struct {
	responseChannel chan *response
	network         *chaincfg.Params
}

// Receive waits for the response promised by the future and returns the list of
// addresses associated with the passed account.
func (r FutureGetAddressesByAccountResult) Receive() ([]jaxutil.Address, error) {
	res, err := receiveFuture(r.responseChannel)
	if err != nil {
		return nil, err
	}

	// Unmashal result as an array of string.
	var addrStrings []string
	err = json.Unmarshal(res, &addrStrings)
	if err != nil {
		return nil, err
	}

	addresses := make([]jaxutil.Address, len(addrStrings))
	for i, addrString := range addrStrings {
		addresses[i], err = jaxutil.DecodeAddress(addrString, r.network)
		if err != nil {
			return nil, err
		}
	}

	return addresses, nil
}

// GetAddressesByAccountAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetAddressesByAccount for the blocking version and more details.
func (c *Client) GetAddressesByAccountAsync(account string) FutureGetAddressesByAccountResult {
	cmd := jaxjson.NewGetAddressesByAccountCmd(account)
	result := FutureGetAddressesByAccountResult{
		network:         c.chainParams,
		responseChannel: c.sendCmd(cmd),
	}
	return result
}

// GetAddressesByAccount returns the list of addresses associated with the
// passed account.
func (c *Client) GetAddressesByAccount(account string) ([]jaxutil.Address, error) {
	return c.GetAddressesByAccountAsync(account).Receive()
}

// FutureMoveResult is a future promise to deliver the result of a MoveAsync,
// MoveMinConfAsync, or MoveCommentAsync RPC invocation (or an applicable
// error).
type FutureMoveResult chan *response

// Receive waits for the response promised by the future and returns the result
// of the move operation.
func (r FutureMoveResult) Receive() (bool, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return false, err
	}

	// Unmarshal result as a boolean.
	var moveResult bool
	err = json.Unmarshal(res, &moveResult)
	if err != nil {
		return false, err
	}

	return moveResult, nil
}

// MoveAsync returns an instance of a type that can be used to get the result of
// the RPC at some future time by invoking the Receive function on the returned
// instance.
//
// See Move for the blocking version and more details.
func (c *Client) MoveAsync(fromAccount, toAccount string, amount jaxutil.Amount) FutureMoveResult {
	cmd := jaxjson.NewMoveCmd(fromAccount, toAccount, amount.ToBTC(), nil,
		nil)
	return c.sendCmd(cmd)
}

// Move moves specified amount from one account in your wallet to another.  Only
// funds with the default number of minimum confirmations will be used.
//
// See MoveMinConf and MoveComment for different options.
func (c *Client) Move(fromAccount, toAccount string, amount jaxutil.Amount) (bool, error) {
	return c.MoveAsync(fromAccount, toAccount, amount).Receive()
}

// MoveMinConfAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See MoveMinConf for the blocking version and more details.
func (c *Client) MoveMinConfAsync(fromAccount, toAccount string,
	amount jaxutil.Amount, minConfirms int) FutureMoveResult {

	cmd := jaxjson.NewMoveCmd(fromAccount, toAccount, amount.ToBTC(),
		&minConfirms, nil)
	return c.sendCmd(cmd)
}

// MoveMinConf moves specified amount from one account in your wallet to
// another.  Only funds with the passed number of minimum confirmations will be
// used.
//
// See Move to use the default number of minimum confirmations and MoveComment
// for additional options.
func (c *Client) MoveMinConf(fromAccount, toAccount string, amount jaxutil.Amount, minConf int) (bool, error) {
	return c.MoveMinConfAsync(fromAccount, toAccount, amount, minConf).Receive()
}

// MoveCommentAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See MoveComment for the blocking version and more details.
func (c *Client) MoveCommentAsync(fromAccount, toAccount string,
	amount jaxutil.Amount, minConfirms int, comment string) FutureMoveResult {

	cmd := jaxjson.NewMoveCmd(fromAccount, toAccount, amount.ToBTC(),
		&minConfirms, &comment)
	return c.sendCmd(cmd)
}

// MoveComment moves specified amount from one account in your wallet to
// another and stores the provided comment in the wallet.  The comment
// parameter is only available in the wallet.  Only funds with the passed number
// of minimum confirmations will be used.
//
// See Move and MoveMinConf to use defaults.
func (c *Client) MoveComment(fromAccount, toAccount string, amount jaxutil.Amount,
	minConf int, comment string) (bool, error) {

	return c.MoveCommentAsync(fromAccount, toAccount, amount, minConf,
		comment).Receive()
}

// FutureRenameAccountResult is a future promise to deliver the result of a
// RenameAccountAsync RPC invocation (or an applicable error).
type FutureRenameAccountResult chan *response

// Receive waits for the response promised by the future and returns the
// result of creating new account.
func (r FutureRenameAccountResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// RenameAccountAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See RenameAccount for the blocking version and more details.
func (c *Client) RenameAccountAsync(oldAccount, newAccount string) FutureRenameAccountResult {
	cmd := jaxjson.NewRenameAccountCmd(oldAccount, newAccount)
	return c.sendCmd(cmd)
}

// RenameAccount creates a new wallet account.
func (c *Client) RenameAccount(oldAccount, newAccount string) error {
	return c.RenameAccountAsync(oldAccount, newAccount).Receive()
}

// FutureValidateAddressResult is a future promise to deliver the result of a
// ValidateAddressAsync RPC invocation (or an applicable error).
type FutureValidateAddressResult chan *response

// Receive waits for the response promised by the future and returns information
// about the given bitcoin address.
func (r FutureValidateAddressResult) Receive() (*jaxjson.ValidateAddressWalletResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a validateaddress result object.
	var addrResult jaxjson.ValidateAddressWalletResult
	err = json.Unmarshal(res, &addrResult)
	if err != nil {
		return nil, err
	}

	return &addrResult, nil
}

// ValidateAddressAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See ValidateAddress for the blocking version and more details.
func (c *Client) ValidateAddressAsync(address jaxutil.Address) FutureValidateAddressResult {
	addr := address.EncodeAddress()
	cmd := jaxjson.NewValidateAddressCmd(addr)
	return c.sendCmd(cmd)
}

// ValidateAddress returns information about the given bitcoin address.
func (c *Client) ValidateAddress(address jaxutil.Address) (*jaxjson.ValidateAddressWalletResult, error) {
	return c.ValidateAddressAsync(address).Receive()
}

// FutureKeyPoolRefillResult is a future promise to deliver the result of a
// KeyPoolRefillAsync RPC invocation (or an applicable error).
type FutureKeyPoolRefillResult chan *response

// Receive waits for the response promised by the future and returns the result
// of refilling the key pool.
func (r FutureKeyPoolRefillResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// KeyPoolRefillAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See KeyPoolRefill for the blocking version and more details.
func (c *Client) KeyPoolRefillAsync() FutureKeyPoolRefillResult {
	cmd := jaxjson.NewKeyPoolRefillCmd(nil)
	return c.sendCmd(cmd)
}

// KeyPoolRefill fills the key pool as necessary to reach the default size.
//
// See KeyPoolRefillSize to override the size of the key pool.
func (c *Client) KeyPoolRefill() error {
	return c.KeyPoolRefillAsync().Receive()
}

// KeyPoolRefillSizeAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See KeyPoolRefillSize for the blocking version and more details.
func (c *Client) KeyPoolRefillSizeAsync(newSize uint) FutureKeyPoolRefillResult {
	cmd := jaxjson.NewKeyPoolRefillCmd(&newSize)
	return c.sendCmd(cmd)
}

// KeyPoolRefillSize fills the key pool as necessary to reach the specified
// size.
func (c *Client) KeyPoolRefillSize(newSize uint) error {
	return c.KeyPoolRefillSizeAsync(newSize).Receive()
}

// ************************
// Amount/Balance Functions
// ************************

// FutureListAccountsResult is a future promise to deliver the result of a
// ListAccountsAsync or ListAccountsMinConfAsync RPC invocation (or an
// applicable error).
type FutureListAccountsResult chan *response

// Receive waits for the response promised by the future and returns returns a
// map of account names and their associated balances.
func (r FutureListAccountsResult) Receive() (map[string]jaxutil.Amount, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a json object.
	var accounts map[string]float64
	err = json.Unmarshal(res, &accounts)
	if err != nil {
		return nil, err
	}

	accountsMap := make(map[string]jaxutil.Amount)
	for k, v := range accounts {
		amount, err := jaxutil.NewAmount(v)
		if err != nil {
			return nil, err
		}

		accountsMap[k] = amount
	}

	return accountsMap, nil
}

// ListAccountsAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ListAccounts for the blocking version and more details.
func (c *Client) ListAccountsAsync() FutureListAccountsResult {
	cmd := jaxjson.NewListAccountsCmd(nil)
	return c.sendCmd(cmd)
}

// ListAccounts returns a map of account names and their associated balances
// using the default number of minimum confirmations.
//
// See ListAccountsMinConf to override the minimum number of confirmations.
func (c *Client) ListAccounts() (map[string]jaxutil.Amount, error) {
	return c.ListAccountsAsync().Receive()
}

// ListAccountsMinConfAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListAccountsMinConf for the blocking version and more details.
func (c *Client) ListAccountsMinConfAsync(minConfirms int) FutureListAccountsResult {
	cmd := jaxjson.NewListAccountsCmd(&minConfirms)
	return c.sendCmd(cmd)
}

// ListAccountsMinConf returns a map of account names and their associated
// balances using the specified number of minimum confirmations.
//
// See ListAccounts to use the default minimum number of confirmations.
func (c *Client) ListAccountsMinConf(minConfirms int) (map[string]jaxutil.Amount, error) {
	return c.ListAccountsMinConfAsync(minConfirms).Receive()
}

// FutureGetBalanceResult is a future promise to deliver the result of a
// GetBalanceAsync or GetBalanceMinConfAsync RPC invocation (or an applicable
// error).
type FutureGetBalanceResult chan *response

// Receive waits for the response promised by the future and returns the
// available balance from the server for the specified account.
func (r FutureGetBalanceResult) Receive() (jaxutil.Amount, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return 0, err
	}

	// Unmarshal result as a floating point number.
	var balance float64
	err = json.Unmarshal(res, &balance)
	if err != nil {
		return 0, err
	}

	amount, err := jaxutil.NewAmount(balance)
	if err != nil {
		return 0, err
	}

	return amount, nil
}

// FutureGetBalanceParseResult is same as FutureGetBalanceResult except
// that the result is expected to be a string which is then parsed into
// a float64 value
// This is required for compatibility with servers like blockchain.info
type FutureGetBalanceParseResult chan *response

// Receive waits for the response promised by the future and returns the
// available balance from the server for the specified account.
func (r FutureGetBalanceParseResult) Receive() (jaxutil.Amount, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return 0, err
	}

	// Unmarshal result as a string
	var balanceString string
	err = json.Unmarshal(res, &balanceString)
	if err != nil {
		return 0, err
	}

	balance, err := strconv.ParseFloat(balanceString, 64)
	if err != nil {
		return 0, err
	}
	amount, err := jaxutil.NewAmount(balance)
	if err != nil {
		return 0, err
	}

	return amount, nil
}

// GetBalanceAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBalance for the blocking version and more details.
func (c *Client) GetBalanceAsync(account string) FutureGetBalanceResult {
	cmd := jaxjson.NewGetBalanceCmd(&account, nil)
	return c.sendCmd(cmd)
}

// GetBalance returns the available balance from the server for the specified
// account using the default number of minimum confirmations.  The account may
// be "*" for all accounts.
//
// See GetBalanceMinConf to override the minimum number of confirmations.
func (c *Client) GetBalance(account string) (jaxutil.Amount, error) {
	return c.GetBalanceAsync(account).Receive()
}

// GetBalanceMinConfAsync returns an instance of a type that can be used to get
// the result of the RPC at some future time by invoking the Receive function on
// the returned instance.
//
// See GetBalanceMinConf for the blocking version and more details.
func (c *Client) GetBalanceMinConfAsync(account string, minConfirms int) FutureGetBalanceResult {
	cmd := jaxjson.NewGetBalanceCmd(&account, &minConfirms)
	return c.sendCmd(cmd)
}

// GetBalanceMinConf returns the available balance from the server for the
// specified account using the specified number of minimum confirmations.  The
// account may be "*" for all accounts.
//
// See GetBalance to use the default minimum number of confirmations.
func (c *Client) GetBalanceMinConf(account string, minConfirms int) (jaxutil.Amount, error) {
	if c.config.EnableBCInfoHacks {
		response := c.GetBalanceMinConfAsync(account, minConfirms)
		return FutureGetBalanceParseResult(response).Receive()
	}
	return c.GetBalanceMinConfAsync(account, minConfirms).Receive()
}

// FutureGetBalancesResult is a future promise to deliver the result of a
// GetBalancesAsync RPC invocation (or an applicable error).
type FutureGetBalancesResult chan *response

// Receive waits for the response promised by the future and returns the
// available balances from the server.
func (r FutureGetBalancesResult) Receive() (*jaxjson.GetBalancesResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a floating point number.
	var balances jaxjson.GetBalancesResult
	err = json.Unmarshal(res, &balances)
	if err != nil {
		return nil, err
	}

	return &balances, nil
}

// GetBalancesAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetBalances for the blocking version and more details.
func (c *Client) GetBalancesAsync() FutureGetBalancesResult {
	cmd := jaxjson.NewGetBalancesCmd()
	return c.sendCmd(cmd)
}

// GetBalances returns the available balances from the server.
func (c *Client) GetBalances() (*jaxjson.GetBalancesResult, error) {
	return c.GetBalancesAsync().Receive()
}

// FutureGetReceivedByAccountResult is a future promise to deliver the result of
// a GetReceivedByAccountAsync or GetReceivedByAccountMinConfAsync RPC
// invocation (or an applicable error).
type FutureGetReceivedByAccountResult chan *response

// Receive waits for the response promised by the future and returns the total
// amount received with the specified account.
func (r FutureGetReceivedByAccountResult) Receive() (jaxutil.Amount, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return 0, err
	}

	// Unmarshal result as a floating point number.
	var balance float64
	err = json.Unmarshal(res, &balance)
	if err != nil {
		return 0, err
	}

	amount, err := jaxutil.NewAmount(balance)
	if err != nil {
		return 0, err
	}

	return amount, nil
}

// GetReceivedByAccountAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetReceivedByAccount for the blocking version and more details.
func (c *Client) GetReceivedByAccountAsync(account string) FutureGetReceivedByAccountResult {
	cmd := jaxjson.NewGetReceivedByAccountCmd(account, nil)
	return c.sendCmd(cmd)
}

// GetReceivedByAccount returns the total amount received with the specified
// account with at least the default number of minimum confirmations.
//
// See GetReceivedByAccountMinConf to override the minimum number of
// confirmations.
func (c *Client) GetReceivedByAccount(account string) (jaxutil.Amount, error) {
	return c.GetReceivedByAccountAsync(account).Receive()
}

// GetReceivedByAccountMinConfAsync returns an instance of a type that can be
// used to get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetReceivedByAccountMinConf for the blocking version and more details.
func (c *Client) GetReceivedByAccountMinConfAsync(account string, minConfirms int) FutureGetReceivedByAccountResult {
	cmd := jaxjson.NewGetReceivedByAccountCmd(account, &minConfirms)
	return c.sendCmd(cmd)
}

// GetReceivedByAccountMinConf returns the total amount received with the
// specified account with at least the specified number of minimum
// confirmations.
//
// See GetReceivedByAccount to use the default minimum number of confirmations.
func (c *Client) GetReceivedByAccountMinConf(account string, minConfirms int) (jaxutil.Amount, error) {
	return c.GetReceivedByAccountMinConfAsync(account, minConfirms).Receive()
}

// FutureGetUnconfirmedBalanceResult is a future promise to deliver the result
// of a GetUnconfirmedBalanceAsync RPC invocation (or an applicable error).
type FutureGetUnconfirmedBalanceResult chan *response

// Receive waits for the response promised by the future and returns returns the
// unconfirmed balance from the server for the specified account.
func (r FutureGetUnconfirmedBalanceResult) Receive() (jaxutil.Amount, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return 0, err
	}

	// Unmarshal result as a floating point number.
	var balance float64
	err = json.Unmarshal(res, &balance)
	if err != nil {
		return 0, err
	}

	amount, err := jaxutil.NewAmount(balance)
	if err != nil {
		return 0, err
	}

	return amount, nil
}

// GetUnconfirmedBalanceAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetUnconfirmedBalance for the blocking version and more details.
func (c *Client) GetUnconfirmedBalanceAsync(account string) FutureGetUnconfirmedBalanceResult {
	cmd := jaxjson.NewGetUnconfirmedBalanceCmd(&account)
	return c.sendCmd(cmd)
}

// GetUnconfirmedBalance returns the unconfirmed balance from the server for
// the specified account.
func (c *Client) GetUnconfirmedBalance(account string) (jaxutil.Amount, error) {
	return c.GetUnconfirmedBalanceAsync(account).Receive()
}

// FutureGetReceivedByAddressResult is a future promise to deliver the result of
// a GetReceivedByAddressAsync or GetReceivedByAddressMinConfAsync RPC
// invocation (or an applicable error).
type FutureGetReceivedByAddressResult chan *response

// Receive waits for the response promised by the future and returns the total
// amount received by the specified address.
func (r FutureGetReceivedByAddressResult) Receive() (jaxutil.Amount, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return 0, err
	}

	// Unmarshal result as a floating point number.
	var balance float64
	err = json.Unmarshal(res, &balance)
	if err != nil {
		return 0, err
	}

	amount, err := jaxutil.NewAmount(balance)
	if err != nil {
		return 0, err
	}

	return amount, nil
}

// GetReceivedByAddressAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetReceivedByAddress for the blocking version and more details.
func (c *Client) GetReceivedByAddressAsync(address jaxutil.Address) FutureGetReceivedByAddressResult {
	addr := address.EncodeAddress()
	cmd := jaxjson.NewGetReceivedByAddressCmd(addr, nil)
	return c.sendCmd(cmd)

}

// GetReceivedByAddress returns the total amount received by the specified
// address with at least the default number of minimum confirmations.
//
// See GetReceivedByAddressMinConf to override the minimum number of
// confirmations.
func (c *Client) GetReceivedByAddress(address jaxutil.Address) (jaxutil.Amount, error) {
	return c.GetReceivedByAddressAsync(address).Receive()
}

// GetReceivedByAddressMinConfAsync returns an instance of a type that can be
// used to get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See GetReceivedByAddressMinConf for the blocking version and more details.
func (c *Client) GetReceivedByAddressMinConfAsync(address jaxutil.Address, minConfirms int) FutureGetReceivedByAddressResult {
	addr := address.EncodeAddress()
	cmd := jaxjson.NewGetReceivedByAddressCmd(addr, &minConfirms)
	return c.sendCmd(cmd)
}

// GetReceivedByAddressMinConf returns the total amount received by the specified
// address with at least the specified number of minimum confirmations.
//
// See GetReceivedByAddress to use the default minimum number of confirmations.
func (c *Client) GetReceivedByAddressMinConf(address jaxutil.Address, minConfirms int) (jaxutil.Amount, error) {
	return c.GetReceivedByAddressMinConfAsync(address, minConfirms).Receive()
}

// FutureListReceivedByAccountResult is a future promise to deliver the result
// of a ListReceivedByAccountAsync, ListReceivedByAccountMinConfAsync, or
// ListReceivedByAccountIncludeEmptyAsync RPC invocation (or an applicable
// error).
type FutureListReceivedByAccountResult chan *response

// Receive waits for the response promised by the future and returns a list of
// balances by account.
func (r FutureListReceivedByAccountResult) Receive() ([]jaxjson.ListReceivedByAccountResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal as an array of listreceivedbyaccount result objects.
	var received []jaxjson.ListReceivedByAccountResult
	err = json.Unmarshal(res, &received)
	if err != nil {
		return nil, err
	}

	return received, nil
}

// ListReceivedByAccountAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListReceivedByAccount for the blocking version and more details.
func (c *Client) ListReceivedByAccountAsync() FutureListReceivedByAccountResult {
	cmd := jaxjson.NewListReceivedByAccountCmd(nil, nil, nil)
	return c.sendCmd(cmd)
}

// ListReceivedByAccount lists balances by account using the default number
// of minimum confirmations and including accounts that haven't received any
// payments.
//
// See ListReceivedByAccountMinConf to override the minimum number of
// confirmations and ListReceivedByAccountIncludeEmpty to filter accounts that
// haven't received any payments from the results.
func (c *Client) ListReceivedByAccount() ([]jaxjson.ListReceivedByAccountResult, error) {
	return c.ListReceivedByAccountAsync().Receive()
}

// ListReceivedByAccountMinConfAsync returns an instance of a type that can be
// used to get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListReceivedByAccountMinConf for the blocking version and more details.
func (c *Client) ListReceivedByAccountMinConfAsync(minConfirms int) FutureListReceivedByAccountResult {
	cmd := jaxjson.NewListReceivedByAccountCmd(&minConfirms, nil, nil)
	return c.sendCmd(cmd)
}

// ListReceivedByAccountMinConf lists balances by account using the specified
// number of minimum confirmations not including accounts that haven't received
// any payments.
//
// See ListReceivedByAccount to use the default minimum number of confirmations
// and ListReceivedByAccountIncludeEmpty to also include accounts that haven't
// received any payments in the results.
func (c *Client) ListReceivedByAccountMinConf(minConfirms int) ([]jaxjson.ListReceivedByAccountResult, error) {
	return c.ListReceivedByAccountMinConfAsync(minConfirms).Receive()
}

// ListReceivedByAccountIncludeEmptyAsync returns an instance of a type that can
// be used to get the result of the RPC at some future time by invoking the
// Receive function on the returned instance.
//
// See ListReceivedByAccountIncludeEmpty for the blocking version and more details.
func (c *Client) ListReceivedByAccountIncludeEmptyAsync(minConfirms int, includeEmpty bool) FutureListReceivedByAccountResult {
	cmd := jaxjson.NewListReceivedByAccountCmd(&minConfirms, &includeEmpty,
		nil)
	return c.sendCmd(cmd)
}

// ListReceivedByAccountIncludeEmpty lists balances by account using the
// specified number of minimum confirmations and including accounts that
// haven't received any payments depending on specified flag.
//
// See ListReceivedByAccount and ListReceivedByAccountMinConf to use defaults.
func (c *Client) ListReceivedByAccountIncludeEmpty(minConfirms int, includeEmpty bool) ([]jaxjson.ListReceivedByAccountResult, error) {
	return c.ListReceivedByAccountIncludeEmptyAsync(minConfirms,
		includeEmpty).Receive()
}

// FutureListReceivedByAddressResult is a future promise to deliver the result
// of a ListReceivedByAddressAsync, ListReceivedByAddressMinConfAsync, or
// ListReceivedByAddressIncludeEmptyAsync RPC invocation (or an applicable
// error).
type FutureListReceivedByAddressResult chan *response

// Receive waits for the response promised by the future and returns a list of
// balances by address.
func (r FutureListReceivedByAddressResult) Receive() ([]jaxjson.ListReceivedByAddressResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal as an array of listreceivedbyaddress result objects.
	var received []jaxjson.ListReceivedByAddressResult
	err = json.Unmarshal(res, &received)
	if err != nil {
		return nil, err
	}

	return received, nil
}

// ListReceivedByAddressAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListReceivedByAddress for the blocking version and more details.
func (c *Client) ListReceivedByAddressAsync() FutureListReceivedByAddressResult {
	cmd := jaxjson.NewListReceivedByAddressCmd(nil, nil, nil)
	return c.sendCmd(cmd)
}

// ListReceivedByAddress lists balances by address using the default number
// of minimum confirmations not including addresses that haven't received any
// payments or watching only addresses.
//
// See ListReceivedByAddressMinConf to override the minimum number of
// confirmations and ListReceivedByAddressIncludeEmpty to also include addresses
// that haven't received any payments in the results.
func (c *Client) ListReceivedByAddress() ([]jaxjson.ListReceivedByAddressResult, error) {
	return c.ListReceivedByAddressAsync().Receive()
}

// ListReceivedByAddressMinConfAsync returns an instance of a type that can be
// used to get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See ListReceivedByAddressMinConf for the blocking version and more details.
func (c *Client) ListReceivedByAddressMinConfAsync(minConfirms int) FutureListReceivedByAddressResult {
	cmd := jaxjson.NewListReceivedByAddressCmd(&minConfirms, nil, nil)
	return c.sendCmd(cmd)
}

// ListReceivedByAddressMinConf lists balances by address using the specified
// number of minimum confirmations not including addresses that haven't received
// any payments.
//
// See ListReceivedByAddress to use the default minimum number of confirmations
// and ListReceivedByAddressIncludeEmpty to also include addresses that haven't
// received any payments in the results.
func (c *Client) ListReceivedByAddressMinConf(minConfirms int) ([]jaxjson.ListReceivedByAddressResult, error) {
	return c.ListReceivedByAddressMinConfAsync(minConfirms).Receive()
}

// ListReceivedByAddressIncludeEmptyAsync returns an instance of a type that can
// be used to get the result of the RPC at some future time by invoking the
// Receive function on the returned instance.
//
// See ListReceivedByAccountIncludeEmpty for the blocking version and more details.
func (c *Client) ListReceivedByAddressIncludeEmptyAsync(minConfirms int, includeEmpty bool) FutureListReceivedByAddressResult {
	cmd := jaxjson.NewListReceivedByAddressCmd(&minConfirms, &includeEmpty,
		nil)
	return c.sendCmd(cmd)
}

// ListReceivedByAddressIncludeEmpty lists balances by address using the
// specified number of minimum confirmations and including addresses that
// haven't received any payments depending on specified flag.
//
// See ListReceivedByAddress and ListReceivedByAddressMinConf to use defaults.
func (c *Client) ListReceivedByAddressIncludeEmpty(minConfirms int, includeEmpty bool) ([]jaxjson.ListReceivedByAddressResult, error) {
	return c.ListReceivedByAddressIncludeEmptyAsync(minConfirms,
		includeEmpty).Receive()
}

// ************************
// Wallet Locking Functions
// ************************

// FutureWalletLockResult is a future promise to deliver the result of a
// WalletLockAsync RPC invocation (or an applicable error).
type FutureWalletLockResult chan *response

// Receive waits for the response promised by the future and returns the result
// of locking the wallet.
func (r FutureWalletLockResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// WalletLockAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See WalletLock for the blocking version and more details.
func (c *Client) WalletLockAsync() FutureWalletLockResult {
	cmd := jaxjson.NewWalletLockCmd()
	return c.sendCmd(cmd)
}

// WalletLock locks the wallet by removing the encryption key from memory.
//
// After calling this function, the WalletPassphrase function must be used to
// unlock the wallet prior to calling any other function which requires the
// wallet to be unlocked.
func (c *Client) WalletLock() error {
	return c.WalletLockAsync().Receive()
}

// WalletPassphrase unlocks the wallet by using the passphrase to derive the
// decryption key which is then stored in memory for the specified timeout
// (in seconds).
func (c *Client) WalletPassphrase(passphrase string, timeoutSecs int64) error {
	cmd := jaxjson.NewWalletPassphraseCmd(passphrase, timeoutSecs)
	_, err := c.sendCmdAndWait(cmd)
	return err
}

// FutureWalletPassphraseChangeResult is a future promise to deliver the result
// of a WalletPassphraseChangeAsync RPC invocation (or an applicable error).
type FutureWalletPassphraseChangeResult chan *response

// Receive waits for the response promised by the future and returns the result
// of changing the wallet passphrase.
func (r FutureWalletPassphraseChangeResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// WalletPassphraseChangeAsync returns an instance of a type that can be used to
// get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See WalletPassphraseChange for the blocking version and more details.
func (c *Client) WalletPassphraseChangeAsync(old, new string) FutureWalletPassphraseChangeResult {
	cmd := jaxjson.NewWalletPassphraseChangeCmd(old, new)
	return c.sendCmd(cmd)
}

// WalletPassphraseChange changes the wallet passphrase from the specified old
// to new passphrase.
func (c *Client) WalletPassphraseChange(old, new string) error {
	return c.WalletPassphraseChangeAsync(old, new).Receive()
}

// *************************
// Message Signing Functions
// *************************

// FutureSignMessageResult is a future promise to deliver the result of a
// SignMessageAsync RPC invocation (or an applicable error).
type FutureSignMessageResult chan *response

// Receive waits for the response promised by the future and returns the message
// signed with the private key of the specified address.
func (r FutureSignMessageResult) Receive() (string, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return "", err
	}

	// Unmarshal result as a string.
	var b64 string
	err = json.Unmarshal(res, &b64)
	if err != nil {
		return "", err
	}

	return b64, nil
}

// SignMessageAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See SignMessage for the blocking version and more details.
func (c *Client) SignMessageAsync(address jaxutil.Address, message string) FutureSignMessageResult {
	addr := address.EncodeAddress()
	cmd := jaxjson.NewSignMessageCmd(addr, message)
	return c.sendCmd(cmd)
}

// SignMessage signs a message with the private key of the specified address.
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) SignMessage(address jaxutil.Address, message string) (string, error) {
	return c.SignMessageAsync(address, message).Receive()
}

// FutureVerifyMessageResult is a future promise to deliver the result of a
// VerifyMessageAsync RPC invocation (or an applicable error).
type FutureVerifyMessageResult chan *response

// Receive waits for the response promised by the future and returns whether or
// not the message was successfully verified.
func (r FutureVerifyMessageResult) Receive() (bool, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return false, err
	}

	// Unmarshal result as a boolean.
	var verified bool
	err = json.Unmarshal(res, &verified)
	if err != nil {
		return false, err
	}

	return verified, nil
}

// VerifyMessageAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See VerifyMessage for the blocking version and more details.
func (c *Client) VerifyMessageAsync(address jaxutil.Address, signature, message string) FutureVerifyMessageResult {
	addr := address.EncodeAddress()
	cmd := jaxjson.NewVerifyMessageCmd(addr, signature, message)
	return c.sendCmd(cmd)
}

// VerifyMessage verifies a signed message.
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) VerifyMessage(address jaxutil.Address, signature, message string) (bool, error) {
	return c.VerifyMessageAsync(address, signature, message).Receive()
}

// *********************
// Dump/Import Functions
// *********************

// FutureDumpPrivKeyResult is a future promise to deliver the result of a
// DumpPrivKeyAsync RPC invocation (or an applicable error).
type FutureDumpPrivKeyResult chan *response

// Receive waits for the response promised by the future and returns the private
// key corresponding to the passed address encoded in the wallet import format
// (WIF)
func (r FutureDumpPrivKeyResult) Receive() (*jaxutil.WIF, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a string.
	var privKeyWIF string
	err = json.Unmarshal(res, &privKeyWIF)
	if err != nil {
		return nil, err
	}

	return jaxutil.DecodeWIF(privKeyWIF)
}

// DumpPrivKeyAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See DumpPrivKey for the blocking version and more details.
func (c *Client) DumpPrivKeyAsync(address jaxutil.Address) FutureDumpPrivKeyResult {
	addr := address.EncodeAddress()
	cmd := jaxjson.NewDumpPrivKeyCmd(addr)
	return c.sendCmd(cmd)
}

// DumpPrivKey gets the private key corresponding to the passed address encoded
// in the wallet import format (WIF).
//
// NOTE: This function requires to the wallet to be unlocked.  See the
// WalletPassphrase function for more details.
func (c *Client) DumpPrivKey(address jaxutil.Address) (*jaxutil.WIF, error) {
	return c.DumpPrivKeyAsync(address).Receive()
}

// FutureImportAddressResult is a future promise to deliver the result of an
// ImportAddressAsync RPC invocation (or an applicable error).
type FutureImportAddressResult chan *response

// Receive waits for the response promised by the future and returns the result
// of importing the passed public address.
func (r FutureImportAddressResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// ImportAddressAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ImportAddress for the blocking version and more details.
func (c *Client) ImportAddressAsync(address string) FutureImportAddressResult {
	cmd := jaxjson.NewImportAddressCmd(address, "", nil)
	return c.sendCmd(cmd)
}

// ImportAddress imports the passed public address.
func (c *Client) ImportAddress(address string) error {
	return c.ImportAddressAsync(address).Receive()
}

// ImportAddressRescanAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ImportAddress for the blocking version and more details.
func (c *Client) ImportAddressRescanAsync(address string, account string, rescan bool) FutureImportAddressResult {
	cmd := jaxjson.NewImportAddressCmd(address, account, &rescan)
	return c.sendCmd(cmd)
}

// ImportAddressRescan imports the passed public address. When rescan is true,
// the block history is scanned for transactions addressed to provided address.
func (c *Client) ImportAddressRescan(address string, account string, rescan bool) error {
	return c.ImportAddressRescanAsync(address, account, rescan).Receive()
}

// FutureImportMultiResult is a future promise to deliver the result of an
// ImportMultiAsync RPC invocation (or an applicable error).
type FutureImportMultiResult chan *response

// Receive waits for the response promised by the future and returns the result
// of importing multiple addresses/scripts.
func (r FutureImportMultiResult) Receive() (jaxjson.ImportMultiResults, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	var importMultiResults jaxjson.ImportMultiResults
	err = json.Unmarshal(res, &importMultiResults)
	if err != nil {
		return nil, err
	}
	return importMultiResults, nil
}

// ImportMultiAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ImportMulti for the blocking version and more details.
func (c *Client) ImportMultiAsync(requests []jaxjson.ImportMultiRequest, options *jaxjson.ImportMultiOptions) FutureImportMultiResult {
	cmd := jaxjson.NewImportMultiCmd(requests, options)
	return c.sendCmd(cmd)
}

// ImportMulti imports addresses/scripts, optionally rescanning the blockchain
// from the earliest creation time of the imported scripts.
//
// See jaxjson.ImportMultiRequest for details on the requests parameter.
func (c *Client) ImportMulti(requests []jaxjson.ImportMultiRequest, options *jaxjson.ImportMultiOptions) (jaxjson.ImportMultiResults, error) {
	return c.ImportMultiAsync(requests, options).Receive()
}

// FutureImportPrivKeyResult is a future promise to deliver the result of an
// ImportPrivKeyAsync RPC invocation (or an applicable error).
type FutureImportPrivKeyResult chan *response

// Receive waits for the response promised by the future and returns the result
// of importing the passed private key which must be the wallet import format
// (WIF).
func (r FutureImportPrivKeyResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// ImportPrivKeyAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ImportPrivKey for the blocking version and more details.
func (c *Client) ImportPrivKeyAsync(privKeyWIF *jaxutil.WIF) FutureImportPrivKeyResult {
	wif := ""
	if privKeyWIF != nil {
		wif = privKeyWIF.String()
	}

	cmd := jaxjson.NewImportPrivKeyCmd(wif, nil, nil)
	return c.sendCmd(cmd)
}

// ImportPrivKey imports the passed private key which must be the wallet import
// format (WIF).
func (c *Client) ImportPrivKey(privKeyWIF *jaxutil.WIF) error {
	return c.ImportPrivKeyAsync(privKeyWIF).Receive()
}

// ImportPrivKeyLabelAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ImportPrivKey for the blocking version and more details.
func (c *Client) ImportPrivKeyLabelAsync(privKeyWIF *jaxutil.WIF, label string) FutureImportPrivKeyResult {
	wif := ""
	if privKeyWIF != nil {
		wif = privKeyWIF.String()
	}

	cmd := jaxjson.NewImportPrivKeyCmd(wif, &label, nil)
	return c.sendCmd(cmd)
}

// ImportPrivKeyLabel imports the passed private key which must be the wallet import
// format (WIF). It sets the account label to the one provided.
func (c *Client) ImportPrivKeyLabel(privKeyWIF *jaxutil.WIF, label string) error {
	return c.ImportPrivKeyLabelAsync(privKeyWIF, label).Receive()
}

// ImportPrivKeyRescanAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ImportPrivKey for the blocking version and more details.
func (c *Client) ImportPrivKeyRescanAsync(privKeyWIF *jaxutil.WIF, label string, rescan bool) FutureImportPrivKeyResult {
	wif := ""
	if privKeyWIF != nil {
		wif = privKeyWIF.String()
	}

	cmd := jaxjson.NewImportPrivKeyCmd(wif, &label, &rescan)
	return c.sendCmd(cmd)
}

// ImportPrivKeyRescan imports the passed private key which must be the wallet import
// format (WIF). It sets the account label to the one provided. When rescan is true,
// the block history is scanned for transactions addressed to provided privKey.
func (c *Client) ImportPrivKeyRescan(privKeyWIF *jaxutil.WIF, label string, rescan bool) error {
	return c.ImportPrivKeyRescanAsync(privKeyWIF, label, rescan).Receive()
}

// FutureImportPubKeyResult is a future promise to deliver the result of an
// ImportPubKeyAsync RPC invocation (or an applicable error).
type FutureImportPubKeyResult chan *response

// Receive waits for the response promised by the future and returns the result
// of importing the passed public key.
func (r FutureImportPubKeyResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// ImportPubKeyAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ImportPubKey for the blocking version and more details.
func (c *Client) ImportPubKeyAsync(pubKey string) FutureImportPubKeyResult {
	cmd := jaxjson.NewImportPubKeyCmd(pubKey, nil)
	return c.sendCmd(cmd)
}

// ImportPubKey imports the passed public key.
func (c *Client) ImportPubKey(pubKey string) error {
	return c.ImportPubKeyAsync(pubKey).Receive()
}

// ImportPubKeyRescanAsync returns an instance of a type that can be used to get the
// result of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ImportPubKey for the blocking version and more details.
func (c *Client) ImportPubKeyRescanAsync(pubKey string, rescan bool) FutureImportPubKeyResult {
	cmd := jaxjson.NewImportPubKeyCmd(pubKey, &rescan)
	return c.sendCmd(cmd)
}

// ImportPubKeyRescan imports the passed public key. When rescan is true, the
// block history is scanned for transactions addressed to provided pubkey.
func (c *Client) ImportPubKeyRescan(pubKey string, rescan bool) error {
	return c.ImportPubKeyRescanAsync(pubKey, rescan).Receive()
}

// ***********************
// Miscellaneous Functions
// ***********************

// NOTE: While getinfo is implemented here (in wallet.go), a btcd chain server
// will respond to getinfo requests as well, excluding any wallet information.

// FutureGetInfoResult is a future promise to deliver the result of a
// GetInfoAsync RPC invocation (or an applicable error).
type FutureGetInfoResult chan *response

// Receive waits for the response promised by the future and returns the info
// provided by the server.
func (r FutureGetInfoResult) Receive() (*jaxjson.InfoWalletResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a getinfo result object.
	var infoRes jaxjson.InfoWalletResult
	err = json.Unmarshal(res, &infoRes)
	if err != nil {
		return nil, err
	}

	return &infoRes, nil
}

// GetInfoAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetInfo for the blocking version and more details.
func (c *Client) GetInfoAsync() FutureGetInfoResult {
	cmd := jaxjson.NewGetInfoCmd()
	return c.sendCmd(cmd)
}

// GetInfo returns miscellaneous info regarding the RPC server.  The returned
// info object may be void of wallet information if the remote server does
// not include wallet functionality.
func (c *Client) GetInfo() (*jaxjson.InfoWalletResult, error) {
	return c.GetInfoAsync().Receive()
}

// FutureImportPubKeyResult is a future promise to deliver the result of an
// WalletCreateFundedPsbt RPC invocation (or an applicable error).
type FutureWalletCreateFundedPsbtResult chan *response

// Receive waits for the response promised by the future and returns the
// partially signed transaction in PSBT format along with the resulting fee
// and change output index.
func (r FutureWalletCreateFundedPsbtResult) Receive() (*jaxjson.WalletCreateFundedPsbtResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a getinfo result object.
	var psbtRes jaxjson.WalletCreateFundedPsbtResult
	err = json.Unmarshal(res, &psbtRes)
	if err != nil {
		return nil, err
	}

	return &psbtRes, nil
}

// WalletCreateFundedPsbtAsync returns an instance of a type that can be used
// to get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See WalletCreateFundedPsbt for the blocking version and more details.
func (c *Client) WalletCreateFundedPsbtAsync(
	inputs []jaxjson.PsbtInput, outputs []jaxjson.PsbtOutput, locktime *uint32,
	options *jaxjson.WalletCreateFundedPsbtOpts, bip32Derivs *bool,
) FutureWalletCreateFundedPsbtResult {
	cmd := jaxjson.NewWalletCreateFundedPsbtCmd(inputs, outputs, locktime, options, bip32Derivs)
	return c.sendCmd(cmd)
}

// WalletCreateFundedPsbt creates and funds a transaction in the Partially
// Signed Transaction format. Inputs will be added if supplied inputs are not
// enough.
func (c *Client) WalletCreateFundedPsbt(
	inputs []jaxjson.PsbtInput, outputs []jaxjson.PsbtOutput, locktime *uint32,
	options *jaxjson.WalletCreateFundedPsbtOpts, bip32Derivs *bool,
) (*jaxjson.WalletCreateFundedPsbtResult, error) {
	return c.WalletCreateFundedPsbtAsync(inputs, outputs, locktime, options, bip32Derivs).Receive()
}

// FutureWalletProcessPsbtResult is a future promise to deliver the result of a
// WalletCreateFundedPsb RPC invocation (or an applicable error).
type FutureWalletProcessPsbtResult chan *response

// Receive waits for the response promised by the future and returns an updated
// PSBT with signed inputs from the wallet and a boolen indicating if the the
// transaction has a complete set of signatures.
func (r FutureWalletProcessPsbtResult) Receive() (*jaxjson.WalletProcessPsbtResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	// Unmarshal result as a getinfo result object.
	var psbtRes jaxjson.WalletProcessPsbtResult
	err = json.Unmarshal(res, &psbtRes)
	if err != nil {
		return nil, err
	}

	return &psbtRes, nil
}

// WalletProcessPsbtAsync returns an instance of a type that can be used
// to get the result of the RPC at some future time by invoking the Receive
// function on the returned instance.
//
// See WalletProcessPsbt for the blocking version and more details.
func (c *Client) WalletProcessPsbtAsync(
	psbt string, sign *bool, sighashType SigHashType, bip32Derivs *bool,
) FutureWalletProcessPsbtResult {
	cmd := jaxjson.NewWalletProcessPsbtCmd(psbt, sign, jaxjson.String(sighashType.String()), bip32Derivs)
	return c.sendCmd(cmd)
}

// WalletProcessPsbt updates a PSBT with input information from our wallet and
// then signs inputs.
func (c *Client) WalletProcessPsbt(
	psbt string, sign *bool, sighashType SigHashType, bip32Derivs *bool,
) (*jaxjson.WalletProcessPsbtResult, error) {
	return c.WalletProcessPsbtAsync(psbt, sign, sighashType, bip32Derivs).Receive()
}

// FutureGetWalletInfoResult is a future promise to deliver the result of an
// GetWalletInfoAsync RPC invocation (or an applicable error).
type FutureGetWalletInfoResult chan *response

// Receive waits for the response promised by the future and returns the result
// of wallet state info.
func (r FutureGetWalletInfoResult) Receive() (*jaxjson.GetWalletInfoResult, error) {
	res, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	var getWalletInfoResult jaxjson.GetWalletInfoResult
	err = json.Unmarshal(res, &getWalletInfoResult)
	if err != nil {
		return nil, err
	}
	return &getWalletInfoResult, nil
}

// GetWalletInfoAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See GetWalletInfo for the blocking version and more details.
func (c *Client) GetWalletInfoAsync() FutureGetWalletInfoResult {
	cmd := jaxjson.NewGetWalletInfoCmd()
	return c.sendCmd(cmd)
}

// GetWalletInfo returns various wallet state info.
func (c *Client) GetWalletInfo() (*jaxjson.GetWalletInfoResult, error) {
	return c.GetWalletInfoAsync().Receive()
}

// FutureBackupWalletResult is a future promise to deliver the result of an
// BackupWalletAsync RPC invocation (or an applicable error)
type FutureBackupWalletResult chan *response

// Receive waits for the response promised by the future
func (r FutureBackupWalletResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// BackupWalletAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See BackupWallet for the blocking version and more details.
func (c *Client) BackupWalletAsync(destination string) FutureBackupWalletResult {
	return c.sendCmd(jaxjson.NewBackupWalletCmd(destination))
}

// BackupWallet safely copies current wallet file to destination, which can
// be a directory or a path with filename
func (c *Client) BackupWallet(destination string) error {
	return c.BackupWalletAsync(destination).Receive()
}

// FutureDumpWalletResult is a future promise to deliver the result of an
// DumpWallet RPC invocation (or an applicable error)
type FutureDumpWalletResult chan *response

// Receive waits for the response promised by the future
func (r FutureDumpWalletResult) Receive() (*jaxjson.DumpWalletResult, error) {
	bytes, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}

	var res jaxjson.DumpWalletResult
	err = json.Unmarshal(bytes, &res)
	return &res, err
}

// DumpWalletAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See DumpWalletAsync for the blocking version and more details.
func (c *Client) DumpWalletAsync(destination string) FutureDumpWalletResult {
	return c.sendCmd(jaxjson.NewDumpWalletCmd(destination))
}

// DumpWallet dumps all wallet keys in a human-readable format to a server-side file.
func (c *Client) DumpWallet(destination string) (*jaxjson.DumpWalletResult, error) {
	return c.DumpWalletAsync(destination).Receive()
}

// FutureImportWalletResult is a future promise to deliver the result of an
// ImportWalletAsync RPC invocation (or an applicable error)
type FutureImportWalletResult chan *response

// Receive waits for the response promised by the future
func (r FutureImportWalletResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// ImportWalletAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See ImportWallet for the blocking version and more details.
func (c *Client) ImportWalletAsync(filename string) FutureImportWalletResult {
	return c.sendCmd(jaxjson.NewImportWalletCmd(filename))
}

// ImportWallet imports keys from a wallet dump file (see DumpWallet).
func (c *Client) ImportWallet(filename string) error {
	return c.ImportWalletAsync(filename).Receive()
}

// FutureUnloadWalletResult is a future promise to deliver the result of an
// UnloadWalletAsync RPC invocation (or an applicable error)
type FutureUnloadWalletResult chan *response

// Receive waits for the response promised by the future
func (r FutureUnloadWalletResult) Receive() error {
	_, err := receiveFuture(r)
	return err
}

// UnloadWalletAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See UnloadWallet for the blocking version and more details.
func (c *Client) UnloadWalletAsync(walletName *string) FutureUnloadWalletResult {
	return c.sendCmd(jaxjson.NewUnloadWalletCmd(walletName))
}

// UnloadWallet unloads the referenced wallet. If the RPC server URL already
// contains the name of the wallet, like http://127.0.0.1:8332/wallet/<walletname>,
// the parameter must be nil, or it'll return an error.
func (c *Client) UnloadWallet(walletName *string) error {
	return c.UnloadWalletAsync(walletName).Receive()
}

// FutureLoadWalletResult is a future promise to deliver the result of an
// LoadWalletAsync RPC invocation (or an applicable error)
type FutureLoadWalletResult chan *response

// Receive waits for the response promised by the future
func (r FutureLoadWalletResult) Receive() (*jaxjson.LoadWalletResult, error) {
	bytes, err := receiveFuture(r)
	if err != nil {
		return nil, err
	}
	var result jaxjson.LoadWalletResult
	err = json.Unmarshal(bytes, &result)
	return &result, err
}

// LoadWalletAsync returns an instance of a type that can be used to get the result
// of the RPC at some future time by invoking the Receive function on the
// returned instance.
//
// See LoadWallet for the blocking version and more details.
func (c *Client) LoadWalletAsync(walletName string) FutureLoadWalletResult {
	return c.sendCmd(jaxjson.NewLoadWalletCmd(walletName))
}

// LoadWallet loads a wallet from a wallet file or directory.
func (c *Client) LoadWallet(walletName string) (*jaxjson.LoadWalletResult, error) {
	return c.LoadWalletAsync(walletName).Receive()
}

// TODO(davec): Implement
// encryptwallet (Won't be supported by btcwallet since it's always encrypted)
// listaddressgroupings (NYI in btcwallet)
// listreceivedbyaccount (NYI in btcwallet)
