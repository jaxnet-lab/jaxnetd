package txscript

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

func TestHTLCScript(t *testing.T) {
	_, addrPubKey, err := genKeys(t, "")
	if err != nil {
		t.Fatal(err)
	}

	// addrPubKeyHash := addrPubKey.AddressPubKeyHash()

	htlc1, err := HTLCScript(addrPubKey, 2400000)
	if err != nil {
		t.Fatal(err)
	}

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

	for i := 1; i <= 10; i++ {
		_, addrPubKey, err := genKeys(t, "")
		if err != nil {
			t.Fatal(err)
		}
		htlc1, err := HTLCScript(addrPubKey.AddressPubKeyHash(), int32(2400000+i))
		if err != nil {
			t.Fatal(err)
		}
		addr, _ := jaxutil.NewHTLCAddress(htlc1, &chaincfg.MainNetParams)
		fmt.Printf("%x:\t", i)
		fmt.Println(addr.String())
		a, err := jaxutil.DecodeAddress(addr.String(), &chaincfg.MainNetParams)
		if err != nil {
			t.Fatal(err)
		}
		if a.String() != addr.String() {
			t.Fatal(errors.New("failed"))
		}
	}

}

func TestSignTxOutput_htlc(t *testing.T) {
	// t.Parallel()

	// make key
	// make script based on key.
	// sign with magic pixie dust.
	hashTypes := []SigHashType{
		// SigHashOld, // no longer used but should act like all
		SigHashAll,
		SigHashNone,
		SigHashSingle,
		SigHashAll | SigHashAnyOneCanPay,
		SigHashNone | SigHashAnyOneCanPay,
		SigHashSingle | SigHashAnyOneCanPay,
	}
	inputAmounts := []int64{5, 10, 15}
	tx := &wire.MsgTx{
		Version: wire.TxVerRegular,
		TxIn: []*wire.TxIn{
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: 0}, Sequence: 4294967295}},
		TxOut:    []*wire.TxOut{{Value: 1}, {Value: 2}, {Value: 3}},
		LockTime: 0,
	}

	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			if !testhtlc(t, tx.Copy(), inputAmounts, hashType, i) {
				break
			}
		}
	}
}

func testhtlc(t *testing.T, tx *wire.MsgTx, inputAmounts []int64, hashType SigHashType, i int) bool {
	msg := fmt.Sprintf("%d:%d", hashType, i)
	privateKey, address, err := genKeys(t, msg)
	if err != nil {
		return false
	}

	lockPeriod := int32(24000)

	htlcScript, err := HTLCScript(address, lockPeriod)
	if err != nil {
		t.Errorf("failed to make pkscript for %s: %v", msg, err)
	}

	t.Logf("multisig lock: %s", hex.EncodeToString(htlcScript))

	parentTx := &wire.MsgTx{
		Version: wire.TxVerRegular,
		TxIn: []*wire.TxIn{
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32}, Sequence: 4294967295}},
		TxOut:    []*wire.TxOut{{Value: 100000, PkScript: htlcScript}},
		LockTime: 0,
	}

	buf := bytes.NewBuffer(nil)
	parentTx.Serialize(buf)

	t.Logf("parent tx[%d]: %s", i, hex.EncodeToString(buf.Bytes()))

	tx.TxIn[i].PreviousOutPoint.Hash = parentTx.TxHash()
	tx.TxIn[i].PreviousOutPoint.Index = 0

	buf = bytes.NewBuffer(nil)
	tx.Serialize(buf)

	t.Logf("tx[%d] before signing: %s", i, hex.EncodeToString(buf.Bytes()))

	{ // check the strategy of the multi sig spend
		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params, tx, i, htlcScript, hashType,
			mkGetKey(map[string]addressToKey{address.EncodeAddress(): {privateKey, true}}), nil, nil)
		if err != nil {
			t.Errorf("failed to sign output %s: %v", msg, err)
			return false
		}

		tx.TxIn[i].SignatureScript = sigScript
		buf = bytes.NewBuffer(nil)
		tx.Serialize(buf)
		t.Logf("tx[%d] with 1 sig: %s", i, hex.EncodeToString(buf.Bytes()))

		// asm, _ = DisasmString(sigScript)
		// fmt.Println("partial_signature_asm: ", asm)
		// fmt.Println("partial_signature_hex: ", hex.EncodeToString(sigScript))

		// should fail, because in.Age < lockPeriod
		err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, htlcScript)
		if err == nil {
			t.Errorf("fully signed script invalid with in.Age < for %v is valid\n", lockPeriod)
			return false
		}
	}

	{ // check the strategy of the refund spend
		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params, tx, i, htlcScript, hashType,
			mkGetKey(map[string]addressToKey{address.EncodeAddress(): {privateKey, true}}), nil, nil)
		if err != nil {
			t.Errorf("failed to sign output %s: %v", msg, err)
			return false
		}

		in := tx.TxIn[i]
		in.Age = lockPeriod + 5
		in.SignatureScript = nil
		tx.TxIn[i] = in
		// activateTraceLogger()
		err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, htlcScript)
		if err != nil {
			t.Errorf("fully signed refund script invalid for %s: %v\n", msg, err)
			return false
		}
		// asm, _ = DisasmString(sigScript)
		// fmt.Println("refund_signature_asm: ", asm)
		// fmt.Println("refund_signature_hex: ", hex.EncodeToString(sigScript))
	}
	return true
}
