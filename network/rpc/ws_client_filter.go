// nolint: forcetypeassert
package rpc

import (
	"sync"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
	"golang.org/x/crypto/ripemd160"
)

// wsClientFilter tracks relevant addresses for each websocket client for
// the `rescanblocks` extension. It is modified by the `loadtxfilter` command.
//
// NOTE: This extension was ported from github.com/decred/dcrd
type wsClientFilter struct {
	mu sync.Mutex

	// Implemented fast paths for address lookup.
	pubKeyHashes        map[[ripemd160.Size]byte]struct{}
	scriptHashes        map[[ripemd160.Size]byte]struct{}
	compressedPubKeys   map[[33]byte]struct{}
	uncompressedPubKeys map[[65]byte]struct{}

	// A fallback address lookup map in case a fast path doesn't exist.
	// Only exists for completeness.  If using this shows up in a profile,
	// there's a good chance a fast path should be added.
	otherAddresses map[string]struct{}

	// Outpoints of unspent outputs.
	unspent map[wire.OutPoint]struct{}
}

// newWSClientFilter creates a new, empty wsClientFilter struct to be used
// for a websocket client.
//
// NOTE: This extension was ported from github.com/decred/dcrd
func newWSClientFilter(addresses []string, unspentOutPoints []wire.OutPoint, params *chaincfg.Params) *wsClientFilter {
	filter := &wsClientFilter{
		pubKeyHashes:        map[[ripemd160.Size]byte]struct{}{},
		scriptHashes:        map[[ripemd160.Size]byte]struct{}{},
		compressedPubKeys:   map[[33]byte]struct{}{},
		uncompressedPubKeys: map[[65]byte]struct{}{},
		otherAddresses:      map[string]struct{}{},
		unspent:             make(map[wire.OutPoint]struct{}, len(unspentOutPoints)),
	}

	for _, s := range addresses {
		filter.addAddressStr(s, params)
	}
	for i := range unspentOutPoints {
		filter.addUnspentOutPoint(&unspentOutPoints[i])
	}

	return filter
}

// addAddress adds an address to a wsClientFilter, treating it correctly based
// on the type of address passed as an argument.
//
// NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) addAddress(a jaxutil.Address) {
	switch a := a.(type) {
	case *jaxutil.AddressPubKeyHash:
		f.pubKeyHashes[*a.Hash160()] = struct{}{}
		return
	case *jaxutil.AddressScriptHash:
		f.scriptHashes[*a.Hash160()] = struct{}{}
		return
	case *jaxutil.AddressPubKey:
		serializedPubKey := a.ScriptAddress()
		switch len(serializedPubKey) {
		case compressedKeySize:
			var compressedPubKey [compressedKeySize]byte
			copy(compressedPubKey[:], serializedPubKey)
			f.compressedPubKeys[compressedPubKey] = struct{}{}
			return
		case uncompressedKeySize:
			var uncompressedPubKey [uncompressedKeySize]byte
			copy(uncompressedPubKey[:], serializedPubKey)
			f.uncompressedPubKeys[uncompressedPubKey] = struct{}{}
			return
		}
	}

	f.otherAddresses[a.EncodeAddress()] = struct{}{}
}

// addAddressStr parses an address from a string and then adds it to the
// wsClientFilter using addAddress.
//
// NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) addAddressStr(s string, params *chaincfg.Params) {
	// If address can't be decoded, no point in saving it since it should also
	// impossible to create the address from an inspected transaction output
	// script.
	a, err := jaxutil.DecodeAddress(s, params)
	if err != nil {
		return
	}
	f.addAddress(a)
}

const (
	compressedKeySize   = 33
	uncompressedKeySize = 65
)

// existsAddress returns true if the passed address has been added to the
// wsClientFilter.
//
// NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) existsAddress(a jaxutil.Address) bool {
	switch a := a.(type) {
	case *jaxutil.AddressPubKeyHash:
		_, ok := f.pubKeyHashes[*a.Hash160()]
		return ok
	case *jaxutil.AddressScriptHash:
		_, ok := f.scriptHashes[*a.Hash160()]
		return ok
	case *jaxutil.AddressPubKey:
		serializedPubKey := a.ScriptAddress()
		switch len(serializedPubKey) {
		case compressedKeySize:
			var compressedPubKey [compressedKeySize]byte
			copy(compressedPubKey[:], serializedPubKey)
			_, ok := f.compressedPubKeys[compressedPubKey]
			if !ok {
				_, ok = f.pubKeyHashes[*a.AddressPubKeyHash().Hash160()]
			}
			return ok
		case uncompressedKeySize:
			var uncompressedPubKey [uncompressedKeySize]byte
			copy(uncompressedPubKey[:], serializedPubKey)
			_, ok := f.uncompressedPubKeys[uncompressedPubKey]
			if !ok {
				_, ok = f.pubKeyHashes[*a.AddressPubKeyHash().Hash160()]
			}
			return ok
		}
	}

	_, ok := f.otherAddresses[a.EncodeAddress()]
	return ok
}

// addUnspentOutPoint adds an outpoint to the wsClientFilter.
//
// NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) addUnspentOutPoint(op *wire.OutPoint) {
	f.unspent[*op] = struct{}{}
}

// existsUnspentOutPoint returns true if the passed outpoint has been added to
// the wsClientFilter.
//
// NOTE: This extension was ported from github.com/decred/dcrd
func (f *wsClientFilter) existsUnspentOutPoint(op *wire.OutPoint) bool {
	_, ok := f.unspent[*op]
	return ok
}
