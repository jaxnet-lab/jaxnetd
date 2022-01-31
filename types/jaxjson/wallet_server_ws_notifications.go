// Copyright (c) 2014 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// NOTE: This file is intended to house the RPC websocket notifications that are
// supported by a wallet server.

package jaxjson

const (
	// AccountBalanceNtfnMethod is the method used for account balance
	// notifications.
	AccountBalanceNtfnMethod = "accountbalance"

	// JaxnetdConnectedNtfnMethod is the method used for notifications when
	// a wallet server is connected to a chain server.
	JaxnetdConnectedNtfnMethod = "jaxnetdconnected"

	// WalletLockStateNtfnMethod is the method used to notify the lock state
	// of a wallet has changed.
	WalletLockStateNtfnMethod = "walletlockstate"

	// NewTxNtfnMethod is the method used to notify that a wallet server has
	// added a new transaction to the transaction store.
	NewTxNtfnMethod = "newtx"
)

func init() {
	// The commands in this file are only usable with a wallet server via
	// websockets and are notifications.
	flags := UFWalletOnly | UFWebsocketOnly | UFNotification

	MustRegisterCmd("wallet", AccountBalanceNtfnMethod, (*AccountBalanceNtfn)(nil), flags)
	MustRegisterCmd("wallet", JaxnetdConnectedNtfnMethod, (*JaxnetdConnectedNtfn)(nil), flags)
	MustRegisterCmd("wallet", WalletLockStateNtfnMethod, (*WalletLockStateNtfn)(nil), flags)
	MustRegisterCmd("wallet", NewTxNtfnMethod, (*NewTxNtfn)(nil), flags)
}

// AccountBalanceNtfn defines the accountbalance JSON-RPC notification.
type AccountBalanceNtfn struct {
	ChainID   uint32
	Account   string
	Balance   float64 // In JXN or JAX
	Confirmed bool    // Whether Balance is confirmed or unconfirmed.
}

// NewAccountBalanceNtfn returns a new instance which can be used to issue an
// accountbalance JSON-RPC notification.
func NewAccountBalanceNtfn(chainID uint32, account string, balance float64, confirmed bool) *AccountBalanceNtfn {
	return &AccountBalanceNtfn{
		ChainID:   chainID,
		Account:   account,
		Balance:   balance,
		Confirmed: confirmed,
	}
}

// JaxnetdConnectedNtfn defines the jaxnetdconnected JSON-RPC notification.
type JaxnetdConnectedNtfn struct {
	ChainID   uint32
	Connected bool
}

// NewJaxnetdConnectedNtfn returns a new instance which can be used to issue a
// jaxnetdconnected JSON-RPC notification.
func NewJaxnetdConnectedNtfn(chainID uint32, connected bool) *JaxnetdConnectedNtfn {
	return &JaxnetdConnectedNtfn{
		ChainID:   chainID,
		Connected: connected,
	}
}

// WalletLockStateNtfn defines the walletlockstate JSON-RPC notification.
type WalletLockStateNtfn struct {
	ChainID uint32
	Locked  bool
}

// NewWalletLockStateNtfn returns a new instance which can be used to issue a
// walletlockstate JSON-RPC notification.
func NewWalletLockStateNtfn(chainID uint32, locked bool) *WalletLockStateNtfn {
	return &WalletLockStateNtfn{
		ChainID: chainID,
		Locked:  locked,
	}
}

// NewTxNtfn defines the newtx JSON-RPC notification.
type NewTxNtfn struct {
	ChainID uint32
	Account string
	Details ListTransactionsResult
}

// NewNewTxNtfn returns a new instance which can be used to issue a newtx
// JSON-RPC notification.
func NewNewTxNtfn(chainID uint32, account string, details ListTransactionsResult) *NewTxNtfn {
	return &NewTxNtfn{
		ChainID: chainID,
		Account: account,
		Details: details,
	}
}
