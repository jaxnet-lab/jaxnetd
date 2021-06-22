/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package txscript

import (
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
	multiSigLockScript, err := MultiSigLockScript([]*jaxutil.AddressPubKey{address1, address2}, 2,
		address0, int32(refundDeferringPeriod))
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
	multiSigLockScript, err := MultiSigLockScript([]*jaxutil.AddressPubKey{address1, address2}, 2,
		address0, int32(refundDeferringPeriod))
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
