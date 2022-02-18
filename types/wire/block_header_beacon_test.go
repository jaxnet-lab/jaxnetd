/*
 * Copyright (c) 2022 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package wire

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"
)

func TestWriteBigInt(t *testing.T) {
	val := big.NewInt(1)
	buf := bytes.NewBuffer(nil)
	err := WriteBigInt(buf, val)
	if err != nil {
		t.Fatalf(err.Error())
	}

	for _, b := range buf.Bytes() {
		fmt.Printf("0x%02x, ", b)
	}

	fmt.Println("")
	nVal, err := ReadBigInt(buf)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if val.String() != nVal.String() {
		t.Fatalf("values are not equal: %s != %s", val.String(), nVal.String())
	}
}
