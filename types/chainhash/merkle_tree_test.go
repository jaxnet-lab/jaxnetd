/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chainhash

import (
	"reflect"
	"testing"
)

func TestBuildMerkleTreeProof(t *testing.T) {
	s2h := func(h string) Hash {
		return HashH([]byte(h))
	}
	leafHash := func(h1, h2 string) Hash {
		ch1 := s2h(h1)
		ch2 := s2h(h2)
		return *HashMerkleBranches(&ch1, &ch2)
	}

	tests := []struct {
		name     string
		txHashes []Hash
		want     []Hash
	}{
		{
			name:     "0",
			txHashes: []Hash{s2h("leaf_0")},
			want:     []Hash{},
		},
		{
			name:     "1",
			txHashes: []Hash{s2h("leaf_0"), s2h("leaf_1")},
			want:     []Hash{s2h("leaf_1")},
		},
		{
			name:     "2",
			txHashes: []Hash{s2h("leaf_0"), s2h("leaf_1"), s2h("leaf_3")},
			want:     []Hash{s2h("leaf_1"), leafHash("leaf_3", "leaf_3")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildMerkleTreeProof(tt.txHashes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildMerkleTreeProof() = %v, want %v", got, tt.want)
			}

			root := MerkleTreeRoot(tt.txHashes)

			if !ValidateMerkleTreeProof(tt.txHashes[0], tt.want, root) {
				t.Error("ValidateMerkleTreeProof() = false, want true")
			}

		})
	}
}
