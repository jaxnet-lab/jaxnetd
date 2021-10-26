/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
)

type TreeContainer struct {
	*BlocksMMRTree
	// RootToBlock stores all known pairs of the mmr_root and corresponding block,
	// which was the last leaf in the tree for this root.
	// Here is stored all roots for the main chain and orphans.
	RootToBlock map[chainhash.Hash]chainhash.Hash
}

func (mmrTree *TreeContainer) SetNodeToMmrWithReorganization(node blocknodes.IBlockNode) {
	prevNodesMMRRoot := node.Header().BlocksMerkleMountainRoot()
	currentMMRRoot := mmrTree.CurrentRoot()

	// 1) Good Case: if a new node is next in the current chain,
	// then just push it to the MMR tree as the last leaf.
	if prevNodesMMRRoot.IsEqual(&currentMMRRoot) {
		mmrTree.AddBlock(node.GetHash(), node.Difficulty())
		mmrTree.RootToBlock[mmrTree.CurrentRoot()] = node.GetHash()
		node.SetActualMMRRoot(mmrTree.CurrentRoot())
		return
	}

	lifoToAdd := []Leaf{
		{Hash: node.GetHash(), Weight: node.Difficulty()},
	}

	// 2) OrphanAdd Case: if a node is not next in the current chain,
	// then looking for the first ancestor (<fork root>) that is present in current chain,
	// resetting MMR tree state to this <fork root> as the last leaf
	// and adding all blocks between <fork root> and a new node.
	iterNode := node.Parent()
	iterMMRRoot := node.Header().BlocksMerkleMountainRoot()
	for iterNode != nil {
		prevHash := iterNode.GetHash()
		bNode, topPresent := mmrTree.LookupNodeByRoot(iterMMRRoot)
		if topPresent {
			if !bNode.Hash.IsEqual(&prevHash) || iterNode.Height() != int32(bNode.Height) {
				// todo: impossible in normal world situation
				return
			}

			mmrTree.ResetRootTo(bNode.Hash, int32(bNode.Height))
			break
		}

		lifoToAdd = append(lifoToAdd, Leaf{Hash: iterNode.GetHash(), Weight: iterNode.Difficulty()})

		iterMMRRoot = iterNode.Header().BlocksMerkleMountainRoot()
		iterNode = iterNode.Parent()
	}

	for i := len(lifoToAdd) - 1; i >= 0; i-- {
		bNode := lifoToAdd[i]
		mmrTree.AddBlock(bNode.Hash, bNode.Weight)
		mmrTree.RootToBlock[mmrTree.CurrentRoot()] = node.GetHash()
		node.SetActualMMRRoot(mmrTree.CurrentRoot())
	}
}
