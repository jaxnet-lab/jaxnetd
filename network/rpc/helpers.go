// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"gitlab.com/jaxnet/jaxnetd/network/netsync"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
)

type (
	commandMux     func(cmd *parsedRPCCmd, closeChan <-chan struct{}) (interface{}, error)
	CommandHandler func(interface{}, <-chan struct{}) (interface{}, error)

	// parsedRPCCmd represents a JSON-RPC request object that has been parsed into
	// a known concrete command along with any error that might have happened while
	// parsing it.
	parsedRPCCmd struct {
		id      interface{}
		scope   string
		method  string
		shardID uint32
		cmd     interface{}
		err     *jaxjson.RPCError
	}
)

// API version constants
const (
	jsonrpcSemverString = "0.3.8"
	jsonrpcSemverMajor  = 0
	jsonrpcSemverMinor  = 3
	jsonrpcSemverPatch  = 8
)

const (
	// rpcAuthTimeoutSeconds is the number of seconds a connection to the
	// RPC server is allowed to stay open without authenticating before it
	// is closed.
	rpcAuthTimeoutSeconds = 10
)

var (
	timeZeroVal time.Time
)

// Errors
var (
	// ErrRPCUnimplemented is an error returned to RPC clients when the
	// provided command is recognized, but not implemented.
	ErrRPCUnimplemented = &jaxjson.RPCError{
		Code:    jaxjson.ErrRPCUnimplemented,
		Message: "Command unimplemented",
	}

	// ErrRPCNoWallet is an error returned to RPC clients when the provided
	// command is recognized as a wallet command.
	ErrRPCNoWallet = &jaxjson.RPCError{
		Code:    jaxjson.ErrRPCNoWallet,
		Message: "This implementation does not implement wallet commands",
	}
)

// parseCmd parses a JSON-RPC request object into known concrete command.  The
// err field of the returned parsedRPCCmd struct will contain an RPC error that
// is suitable for use in replies if the command is invalid in some way such as
// an unregistered command or invalid parameters.
func parseCmd(request *jaxjson.Request) *parsedRPCCmd {
	parsedCmd := parsedRPCCmd{
		id:      request.ID,
		method:  request.Method,
		scope:   request.Scope,
		shardID: request.ShardID,
	}

	cmd, err := jaxjson.UnmarshalCmd(request)
	if err != nil {
		// When the error is because the method is not registered,
		// produce a method not found RPC error.
		if jerr, ok := err.(jaxjson.Error); ok &&
			jerr.ErrorCode == jaxjson.ErrUnregisteredMethod {

			parsedCmd.err = jaxjson.ErrRPCMethodNotFound
			return &parsedCmd
		}

		// Otherwise, some type of invalid parameters is the
		// cause, so produce the equivalent RPC error.
		parsedCmd.err = jaxjson.NewRPCError(
			jaxjson.ErrRPCInvalidParams.Code, err.Error())
		return &parsedCmd
	}
	parsedCmd.cmd = cmd
	return &parsedCmd
}

// jsonAuthFail sends a message back to the client if the http auth is rejected.
func jsonAuthFail(w http.ResponseWriter) {
	w.Header().Add("WWW-Authenticate", `Basic realm="jaxnetd RPC"`)
	http.Error(w, "401 Unauthorized.", http.StatusUnauthorized)
}

// rpcDecodeHexError is a convenience function for returning a nicely formatted
// RPC error which indicates the provided hex string failed to decode.
func rpcDecodeHexError(gotHex string) *jaxjson.RPCError {
	return jaxjson.NewRPCError(jaxjson.ErrRPCDecodeHexString,
		fmt.Sprintf("Argument must be hexadecimal string (not %q)",
			gotHex))
}

// rpcNoTxInfoError is a convenience function for returning a nicely formatted
// RPC error which indicates there is no information available for the provided
// transaction hash.
func rpcNoTxInfoError(txHash *chainhash.Hash) *jaxjson.RPCError {
	return jaxjson.NewRPCError(jaxjson.ErrRPCNoTxInfo,
		fmt.Sprintf("No information available about transaction %v",
			txHash))
}

// peerExists determines if a certain peer is currently connected given
// information about all currently connected peers. Peer existence is
// determined using either a target address or chainProvider id.
func peerExists(connMgr netsync.P2PConnManager, addr string, nodeID int32) bool {
	for _, p := range connMgr.ConnectedPeers() {
		if p.ToPeer().ID() == nodeID || p.ToPeer().Addr() == addr {
			return true
		}
	}
	return false
}

// normalizeAddress returns addr with the passed default port appended if
// there is not already a port specified.
func normalizeAddress(addr, defaultPort string) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}
