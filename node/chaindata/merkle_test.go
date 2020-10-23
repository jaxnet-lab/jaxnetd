// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

//+build deprecated_tests

package chaindata

import (
	"testing"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
)

// TestMerkle tests the BuildMerkleTreeStore API.
func TestMerkle(t *testing.T) {
	block := btcutil.NewBlock(&Block100000)
	merkles := BuildMerkleTreeStore(block.Transactions(), false)
	calculatedMerkleRoot := merkles[len(merkles)-1]
	wantMerkle := Block100000.Header.MerkleRoot()
	if !wantMerkle.IsEqual(calculatedMerkleRoot) {
		t.Errorf("BuildMerkleTreeStore: merkle root mismatch - "+
			"got %v, want %v", calculatedMerkleRoot, wantMerkle)
	}
}
