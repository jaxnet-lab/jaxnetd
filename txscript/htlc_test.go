/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package txscript

import (
	"testing"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
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
	multiSigLockScript, err := MultiSigLockScript(
		address0, []*btcutil.AddressPubKey{address1, address2}, 2, int32(refundDeferringPeriod))
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
