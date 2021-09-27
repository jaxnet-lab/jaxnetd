/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package txscript

import (
	"encoding/hex"
	"fmt"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
)

func Test_isMultiSigLock(t *testing.T) {
	refundDeferringPeriod := 10

	_, address0, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}

	_, address1, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}

	_, address2, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}
	multiSigLockScript, err := MultiSigLockScript([]*jaxutil.AddressPubKey{address1, address2}, 2, address0, int32(refundDeferringPeriod), false)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := parseScript(multiSigLockScript)
	if err != nil {
		t.Fatal(err)
	}

	if !isMultiSigLock(parsed) {
		t.Fatal("expect multiSig")
	}
}

func Test_extractMultiSigLockAddrs(t *testing.T) {
	refundDeferringPeriod := 10

	_, address0, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}

	_, address1, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}

	_, address2, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}

	multiSigLockScript, err := MultiSigLockScript([]*jaxutil.AddressPubKey{address1, address2}, 2, address0, int32(refundDeferringPeriod), false)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := parseScript(multiSigLockScript)
	if err != nil {
		t.Fatal(err)
	}

	addresses, nReq := extractMultiSigLockAddrs(parsed, &chaincfg.TestNet3Params)
	if nReq != 2 {
		t.Fatalf("expected 2, has %d", nReq)
	}
	if len(addresses) != 3 {
		t.Fatalf("expected 3, has %d", len(addresses))
	}
}

func Test_pubKeysToAddresses(t *testing.T) {
	_, address0, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}

	_, address1, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}

	_, address2, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}

	scripts := []string{
		hex.EncodeToString(address0.ScriptAddress()),
		hex.EncodeToString(address1.ScriptAddress()),
		hex.EncodeToString(address2.ScriptAddress()),
	}
	for _, bytes := range scripts {
		fmt.Println(bytes)
	}

	notSorted := pubKeysToAddresses([]*jaxutil.AddressPubKey{address0, address1, address2}, false)
	fmt.Println("notSorted")
	for _, bytes := range notSorted {
		fmt.Println(hex.EncodeToString(bytes))
	}

	sorted := pubKeysToAddresses([]*jaxutil.AddressPubKey{address0, address1, address2}, true)

	fmt.Println("sorted")
	for _, bytes := range sorted {
		fmt.Println(hex.EncodeToString(bytes))
	}
}
