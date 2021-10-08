/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaincfg

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

func TestBeaconGenesisBlock(t *testing.T) {
	for _, net := range []Params{MainNetParams, TestNet3Params, FastNetParams, SimNetParams} {

		cleanState()

		block := net.GenesisBlock()
		bh := block.BlockHash()
		if !bh.IsEqual(net.GenesisHash()) {
			t.Fatalf("genesis hash not equal to itself %s - %s", bh, net.GenesisHash())
		}

		fmt.Println(block.BlockHash())

		buf := bytes.NewBuffer(nil)
		err := block.Serialize(buf)
		if err != nil {
			t.Fatal("unable to serialize genesis block", err)
		}
		hexData := hex.EncodeToString(buf.Bytes())

		rawBlock, _ := hex.DecodeString(hexData)
		newBlock := wire.EmptyBeaconBlock()

		err = newBlock.Deserialize(bytes.NewBuffer(rawBlock))
		if err != nil {
			t.Fatal("unable to deserialize genesis block", err)
		}
		newBH := newBlock.BlockHash()

		if !bh.IsEqual(&newBH) {
			t.Fatalf("genesis hash not equal to itself %s - %s", bh, newBH)
		}
	}

}
