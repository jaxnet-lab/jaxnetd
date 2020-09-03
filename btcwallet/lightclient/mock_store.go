package neutrino

import (
	"fmt"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire/chain"

	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/btcwallet/lightclient/headerfs"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
)

// mockBlockHeaderStore is an implementation of the BlockHeaderStore backed by
// a simple map.
type mockBlockHeaderStore struct {
	headers map[chainhash.Hash]chain.BlockHeader
	heights map[uint32]chain.BlockHeader
}

// A compile-time check to ensure the mockBlockHeaderStore adheres to the
// BlockHeaderStore interface.
var _ headerfs.BlockHeaderStore = (*mockBlockHeaderStore)(nil)

// NewMockBlockHeaderStore returns a version of the BlockHeaderStore that's
// backed by an in-memory map. This instance is meant to be used by callers
// outside the package to unit test components that require a BlockHeaderStore
// interface.
func newMockBlockHeaderStore() *mockBlockHeaderStore {
	return &mockBlockHeaderStore{
		headers: make(map[chainhash.Hash]chain.BlockHeader),
		heights: make(map[uint32]chain.BlockHeader),
	}
}

func (m *mockBlockHeaderStore) ChainTip() (chain.BlockHeader,
	uint32, error) {
	return nil, 0, nil

}
func (m *mockBlockHeaderStore) LatestBlockLocator() (
	blockchain.BlockLocator, error) {
	return nil, nil
}

func (m *mockBlockHeaderStore) FetchHeaderByHeight(height uint32) (
	chain.BlockHeader, error) {

	if header, ok := m.heights[height]; ok {
		return header, nil
	}

	return nil, headerfs.ErrHeightNotFound
}

func (m *mockBlockHeaderStore) FetchHeaderAncestors(uint32,
	*chainhash.Hash) ([]chain.BlockHeader, uint32, error) {

	return nil, 0, nil
}
func (m *mockBlockHeaderStore) HeightFromHash(*chainhash.Hash) (uint32, error) {
	return 0, nil

}
func (m *mockBlockHeaderStore) RollbackLastBlock() (*headerfs.BlockStamp,
	error) {
	return nil, nil
}

func (m *mockBlockHeaderStore) FetchHeader(h *chainhash.Hash) (
	chain.BlockHeader, uint32, error) {
	if header, ok := m.headers[*h]; ok {
		return header, 0, nil
	}
	return nil, 0, fmt.Errorf("not found")
}

func (m *mockBlockHeaderStore) WriteHeaders(headers ...headerfs.BlockHeader) error {
	for _, h := range headers {
		m.headers[h.BlockHash()] = h.BlockHeader
	}

	return nil
}
