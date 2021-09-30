package txscript

import (
	"errors"
	"testing"
)

func TestHTLCScript(t *testing.T) {
	_, addrPubKey, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}

	addrPubKeyHash := addrPubKey.AddressPubKeyHash()

	htlc1, err := HTLCScript(addrPubKey.AddressPubKeyHash(), 2400000)
	if err != nil {
		t.Fatal(err)
	}

	htlc2, err := HTLCScript(addrPubKeyHash, 2400000)
	if err != nil {
		t.Fatal(err)
	}

	println(DisasmString(htlc1))
	println(DisasmString(htlc2))
	pops, err := parseScript(htlc1)
	if err != nil {
		t.Fatal(err)
	}

	if !isHTLC(pops) {
		t.Fatal(errors.New("not htlc1"))
	}

	lockTime, err := ExtractHTLCLockTime(htlc1)
	if err != nil {
		t.Fatal(err)
	}
	if lockTime != 2400000 {
		t.Fatal(errors.New("invalid locktime"))
	}

}
