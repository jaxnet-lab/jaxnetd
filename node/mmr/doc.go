/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

// Package mmr provides implementation of the Merkle Mountain Range.
//
// Block Leaf is structure of that contains Block Hash and Block PowWeight aka weight:
// 	      block = { hash; weight }
//
// How To calculate Root Leaf for two parents:
// 	 1. concat hash and weight for each parent
// 	 2. concat resulting values and hash them.
// 	 3. sum weights
//
// 	      root = block1 + block2
// 	   where:
// 	      bv1 = concat(block1.hash, block1.weight)
// 	      bv2 = concat(block2.hash, block2.weight)
// 	      root.hash   = HASH( concat(bv1, bv2) )
// 	      root.weight = block1.weight+block2.weight
//
// Tree Topology:
//
// For 1 leaf:
// 	      0: root = block1
//
// For 2 leaves:
//	      1:      root = node12 = block1 + block2
//	             /       \
//	      0:   block1   block2
//
// For 3 leaves:
//	      2:          root = node12 + block3
//	                   /      \
//	      1:       node12      \
//	              /     \       \
//	      0:   block1  block2   block3
//
// For 4 leaves:
//	      2:            root = node12 + node34
//	                    /           \
//	      1:        node12         node34
//	               /     \        /      \
//	      0:   block1  block2   block3   block4
//
// For 5 leaves:
//	      3:                    root = node12_34 + block5
//	                              /       \
//	      2:              node12_34         \
//	                     /         \          \
//	      1:        node12         node34       \
//	               /     \        /      \        \
//	      0:   block1  block2   block3   block4   block5
//
// For 6 leaves:
//	      3:                     root = node12_34 + node56
//	                              /              \
//	      2:               node12_34               \
//	                     /         \                 \
//	      1:        node12         node34            node56
//	               /     \        /      \          /      \
//	      0:   block1  block2   block3   block4   block5  block6
//
// For 7 leaves:
//	      3:                      root = node12_34 + node56_7
//	                              /                        \
//	      2:               node12_34                      node56_7
//	                     /         \                      /       \
//	      1:        node12         node34            node56        \
//	               /     \        /      \          /      \        \
//	      0:   block1  block2   block3  block4   block5  block6   block7
//
// For 8 leaves:
//	      3:                      root = node12_34 + node56_78
//	                              /                        \
//	      2:              node12_34                         node56_78
//	                     /         \                      /          \
//	      1:        node12         node34            node56           node78
//	               /     \        /      \          /      \         /     \
//	      0:   block1  block2   block3  block4   block5  block6   block7  block8
package mmr
