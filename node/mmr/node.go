/*
 * Copyright (c) 2022 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"encoding/json"
	"math/big"

	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

type nodeValue []byte

func (v nodeValue) intoNode() (b TreeNode) {
	copy(b.Hash[:], v[:chainhash.HashSize])
	b.Weight = new(big.Int).SetBytes(v[chainhash.HashSize:])
	return b
}

type TreeNode struct {
	Hash   chainhash.Hash
	Weight *big.Int
	Height int32

	v     nodeValue
	final bool
}

func (node *TreeNode) Bytes() []byte {
	if len(node.v) != 0 {
		return node.v
	}

	wBytes := node.Weight.Bytes()

	node.v = make([]byte, chainhash.HashSize+len(wBytes))
	copy(node.v[:chainhash.HashSize], node.Hash[:])
	copy(node.v[chainhash.HashSize:], wBytes)

	return node.v
}

func (node *TreeNode) MarshalJSON() ([]byte, error) {
	type dto struct {
		BlockHash string
		Weight    string
		Height    int32
	}

	d := dto{
		BlockHash: node.Hash.String(),
		Weight:    node.Weight.String(),
		Height:    node.Height,
	}

	return json.Marshal(d)
}

func (node *TreeNode) Clone() *TreeNode {
	if node == nil {
		return nil
	}

	clone := *node
	return &clone
}
