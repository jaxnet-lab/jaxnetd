/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package mmr

import (
	"math/big"

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

func (mmrTree *TreeContainer) SetNodeQuick(node blocknodes.IBlockNode) {
	mmrTree.RootToBlock[node.ActualMMRRoot()] = node.GetHash()
	mmrTree.AddBlockWithoutRebuild(node.GetHash(), node.ActualMMRRoot(), node.Height(), node.PowWeight())
}

func (mmrTree *TreeContainer) SetNodeToMmrWithReorganization(blockNode blocknodes.IBlockNode) bool {
	prevNodesMMRRoot := blockNode.PrevMMRRoot()
	currentMMRRoot := mmrTree.CurrentRoot()

	// 1) Good Case: if a new node is next in the current chain,
	// then just push it to the MMR tree as the last leaf.
	if prevNodesMMRRoot.IsEqual(&currentMMRRoot) {
		mmrTree.AppendBlock(blockNode.GetHash(), blockNode.PowWeight())
		mmrTree.RootToBlock[mmrTree.CurrentRoot()] = blockNode.GetHash()
		blockNode.SetActualMMRRoot(mmrTree.CurrentRoot())
		return true
	}
	type treeNode struct {
		hash          chainhash.Hash
		weight        *big.Int
		actualMMRRoot chainhash.Hash
	}

	lifoToAdd := []treeNode{
		{hash: blockNode.GetHash(), weight: blockNode.PowWeight(), actualMMRRoot: blockNode.ActualMMRRoot()},
	}

	// reject non-genesis block without parent
	if blockNode.Parent() == nil && blockNode.Height() > 0 {
		return false
	}

	// 2) OrphanAdd Case: if a node is not next in the current chain,
	// then looking for the first ancestor (<fork root>) that is present in current chain,
	// resetting MMR tree state to this <fork root> as the last leaf
	// and adding all blocks between <fork root> and a new node.
	iterNode := blockNode.Parent()
	iterMMRRoot := blockNode.PrevMMRRoot()
	for iterNode != nil {
		prevHash := iterNode.GetHash()
		bNode, topPresent := mmrTree.LookupNodeByRoot(iterMMRRoot)
		if topPresent {
			if !bNode.Hash.IsEqual(&prevHash) || iterNode.Height() != bNode.Height {
				// todo: impossible in normal world situation
				return false
			}

			mmrTree.ResetRootTo(bNode.Hash, bNode.Height)
			break
		}

		lifoToAdd = append(lifoToAdd, treeNode{
			hash:          iterNode.GetHash(),
			weight:        iterNode.PowWeight(),
			actualMMRRoot: iterNode.ActualMMRRoot()})

		iterMMRRoot = iterNode.PrevMMRRoot()
		iterNode = iterNode.Parent()
	}

	for i := len(lifoToAdd) - 1; i >= 0; i-- {
		bNode := lifoToAdd[i]
		if bNode.actualMMRRoot.IsZero() {
			mmrTree.AppendBlock(bNode.hash, bNode.weight)
			mmrTree.RootToBlock[mmrTree.CurrentRoot()] = bNode.hash
			blockNode.SetActualMMRRoot(mmrTree.CurrentRoot())
		} else {
			mmrTree.AddBlockWithoutRebuild(bNode.hash, bNode.actualMMRRoot, mmrTree.nextHeight, bNode.weight)
			mmrTree.RootToBlock[bNode.actualMMRRoot] = bNode.hash
		}
	}

	return true
}
