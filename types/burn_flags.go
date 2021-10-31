/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package types

const (
	BurnJaxNetReward = 1 << iota
	BurnJaxReward    = 1 << iota
)

const (
	JaxBurnAddr    = "1JAXNETJAXNETJAXNETJAXNETJAXW3bkUN"
	JaxBCHBurnAddr = "qqjaxnetjaxnetjaxnetjaxnetjaxnetju326ted65" // 14T1986qwbxxLRxekqBxC4PmdTCWHL7GpK
	JaxBurnAddrTy  = "jax-burn"
)

var RawJaxBurnScript = []byte{
	0x76, 0xa9, 0x14, 0xbc, 0x47, 0x3a, 0xf4, 0xc7,
	0x1c, 0x45, 0xd5, 0xaa, 0x32, 0x78, 0xad, 0xc9,
	0x97, 0x1, 0xde, 0xd3, 0x74, 0xa, 0x54, 0x88, 0xac}

var RawBCHJaxBurnScript = []byte{
	0x76, 0xa9, 0x14, 0x25, 0xd3, 0x4f, 0x2b, 0x97,
	0x4d, 0x3c, 0xae, 0x5d, 0x34, 0xf2, 0xb9, 0x74,
	0xd3, 0xca, 0xe5, 0xd3, 0x4f, 0x2b, 0x97, 0x88, 0xac}
