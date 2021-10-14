/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package main

import (
	"github.com/davecgh/go-spew/spew"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

func main() {
	btcBlock := "000000000000000000040760e91d914ce615b1968cedd3ad53cf34be9c0ff72b"
	hash, _ := chainhash.NewHashFromStr(btcBlock)

	script, _ := txscript.NewScriptBuilder().
		AddData(hash[:]).
		AddData([]byte("Jax.Network enters the race! ")).
		AddData([]byte{0x73, 0x68, 0x65, 0x62}).
		Script()

	spew.Dump(script)
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{Index: 0xffffffff},
				SignatureScript:  script,
				Sequence:         0xffffffff,
			},
		},
		LockTime: 0,
	}

	address, _ := jaxutil.DecodeAddress("mxQsksaTJb11i7vSxAUL6VBjoQnhP3bfFz", &chaincfg.FastNetParams)
	script, _ = txscript.PayToAddrScript(address)

	spew.Dump(script)

	tx.AddTxOut(&wire.TxOut{
		Value:    50_0000_0000,
		PkScript: script,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    50_0000_0000,
		PkScript: script,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    50_0000_0000,
		PkScript: script,
	})

	spew.Dump(tx)
	hexTx, _ := tx.SerializeToHex()
	println(hexTx)
}
