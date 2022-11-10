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
	ok, class := isHTLC(pops)
	if !ok {
		t.Fatal(errors.New("not htlc1"))
	}
	if class != PubKeyTy {
		t.Fatal(errors.New("invalid class"))
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
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: 0}, Sequence: 4294967295},
		},
		TxOut:    []*wire.TxOut{{Value: 1}, {Value: 2}, {Value: 3}},
		LockTime: 0,
	}

	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			if !testHtlcWithAddressPubKey(t, tx.Copy(), inputAmounts, hashType, i) {
				break
			}
			if !testHtlcWithMultiSig(t, tx.Copy(), inputAmounts, hashType, i) {
				break
			}
			if !testHtlcWithWitnessPubKeyHash(t, tx.Copy(), inputAmounts, hashType, i) {
				break
			}
		}
	}
}

// nolint: errcheck
func testHtlcWithAddressPubKey(t *testing.T, tx *wire.MsgTx, inputAmounts []int64, hashType SigHashType, i int) bool {
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

	t.Logf("lock: %s", hex.EncodeToString(htlcScript))

	parentTx := &wire.MsgTx{
		Version: wire.TxVerRegular,
		TxIn: []*wire.TxIn{
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32}, Sequence: 4294967295},
		},
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
		sigScript, err := SignTxOutput(&chaincfg.TestNetParams, tx, i, htlcScript, hashType,
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
		sigScript, err := SignTxOutput(&chaincfg.TestNetParams, tx, i, htlcScript, hashType,
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

func testHtlcWithMultiSig(t *testing.T, tx *wire.MsgTx, inputAmounts []int64, hashType SigHashType, i int) bool {
	msg := fmt.Sprintf("%d:%d", hashType, i)
	alice, addressAlice, err := genKeys(t, msg)
	if err != nil {
		return false
	}

	bob, addressBob, err := genKeys(t, msg)
	if err != nil {
		return false
	}

	_, addressEva, err := genKeys(t, msg)
	if err != nil {
		return false
	}

	pkScript, err := MultiSigScript([]*jaxutil.AddressPubKey{addressAlice, addressBob, addressEva}, 2, true)
	if err != nil {
		t.Errorf("failed to make pkscript for %s: %v", msg, err)
		return false

	}

	scriptAddr, err := jaxutil.NewAddressScriptHash(pkScript, &chaincfg.TestNetParams)
	if err != nil {
		t.Errorf("failed to make p2sh addr for %s: %v", msg, err)
		return false
	}

	lockPeriod := int32(24000)

	htlcScript, err := HTLCScript(scriptAddr, lockPeriod)
	if err != nil {
		t.Errorf("failed to make pkscript for %s: %v", msg, err)
		return false
	}

	// t.Logf("lock: %s", hex.EncodeToString(htlcScript))

	parentTx := &wire.MsgTx{
		Version: wire.TxVerRegular,
		TxIn: []*wire.TxIn{
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32}, Sequence: 4294967295},
		},
		TxOut:    []*wire.TxOut{{Value: 100000, PkScript: htlcScript}},
		LockTime: 0,
	}

	buf := bytes.NewBuffer(nil)
	parentTx.Serialize(buf)

	// t.Logf("parent tx[%d]: %s", i, hex.EncodeToString(buf.Bytes()))

	tx.TxIn[i].PreviousOutPoint.Hash = parentTx.TxHash()
	tx.TxIn[i].PreviousOutPoint.Index = 0

	buf = bytes.NewBuffer(nil)
	tx.Serialize(buf)

	// t.Logf("tx[%d] before signing: %s", i, hex.EncodeToString(buf.Bytes()))

	{ // check the strategy of the multi sig spend
		sigScript, err := SignTxOutput(&chaincfg.TestNetParams, tx, i, htlcScript, hashType,
			mkGetKey(map[string]addressToKey{
				addressAlice.EncodeAddress(): {alice, true},
				addressBob.EncodeAddress():   {bob, true},
			}),
			mkGetScript(map[string][]byte{
				scriptAddr.EncodeAddress(): pkScript,
			}), nil)
		if err != nil {
			t.Errorf("failed to sign output %s: %v", msg, err)
			return false
		}

		tx.TxIn[i].SignatureScript = sigScript
		buf = bytes.NewBuffer(nil)
		tx.Serialize(buf)
		// t.Logf("tx[%d] with 1 sig: %s", i, hex.EncodeToString(buf.Bytes()))

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
		sigScript, err := SignTxOutput(&chaincfg.TestNetParams, tx, i, htlcScript, hashType,
			mkGetKey(map[string]addressToKey{
				addressAlice.EncodeAddress(): {alice, true},
				addressBob.EncodeAddress():   {bob, true},
			}),
			mkGetScript(map[string][]byte{
				scriptAddr.EncodeAddress(): pkScript,
			}), nil)
		if err != nil {
			t.Errorf("failed to sign output %s: %v", msg, err)
			return false
		}

		in := tx.TxIn[i]
		in.Age = lockPeriod + 5
		in.SignatureScript = nil
		tx.TxIn[i] = in
		// activateTraceLogger()

		asm, _ := DisasmString(sigScript)
		fmt.Println("refund_signature_asm: ", asm)

		err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, htlcScript)
		if err != nil {
			t.Errorf("fully signed refund script invalid for %s: %v\n", msg, err)
			return false
		}

		// fmt.Println("refund_signature_hex: ", hex.EncodeToString(sigScript))
	}
	return true
}

func testHtlcWithWitnessPubKeyHash(t *testing.T, tx *wire.MsgTx, inputAmounts []int64, hashType SigHashType, i int) bool {
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

	t.Logf("lock: %s", hex.EncodeToString(htlcScript))

	parentTx := &wire.MsgTx{
		Version: wire.TxVerRegular,
		TxIn: []*wire.TxIn{
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32}, Sequence: 4294967295},
		},
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
		sigScript, err := SignTxOutput(&chaincfg.TestNetParams, tx, i, htlcScript, hashType,
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
		sigScript, err := SignTxOutput(&chaincfg.TestNetParams, tx, i, htlcScript, hashType,
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

// nolint: gocritic
func failErr(t *testing.T, err error, msg string, args ...interface{}) {
	if err != nil {
		if msg != "" && len(args) > 0 {
			t.Errorf(msg, args...)
		} else if msg != "" && len(args) == 0 {
			t.Errorf(msg)
		} else {
			t.Errorf(err.Error())
		}

		t.FailNow()
	}
}

func TestMultisigSanbbox(t *testing.T) {
	// t.Parallel()

	// make key
	// make script based on key.
	// sign with magic pixie dust.
	hashType := SigHashAll

	i := 0

	msg := fmt.Sprintf("%d:%d", hashType, i)

	alice, addressAlice, err := genKeys(t, msg)
	failErr(t, err, "")

	bob, addressBob, err := genKeys(t, msg)
	failErr(t, err, "")

	multiSigScript, err := MultiSigScript([]*jaxutil.AddressPubKey{addressAlice, addressBob}, 2, true)
	failErr(t, err, "failed to make pkscript for %s: %v", msg, err)

	pay2MultisigScriptAddr, err := jaxutil.NewAddressScriptHash(multiSigScript, &chaincfg.TestNetParams)
	failErr(t, err, "failed to make p2sh addr for %s: %v", msg, err)
	pay2MultisigPkScript, err := PayToAddrScript(pay2MultisigScriptAddr)
	failErr(t, err, "failed to make script pkscript for %s: %v", msg, err)

	parentTx := &wire.MsgTx{
		Version: wire.TxVerRegular,
		TxIn: []*wire.TxIn{
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32}, Sequence: 4294967295},
		},
		TxOut:    []*wire.TxOut{{Value: 100000, PkScript: pay2MultisigPkScript}},
		LockTime: 0,
	}

	inputAmounts := []int64{5, 10, 15}
	tx := &wire.MsgTx{
		Version: wire.TxVerRegular,
		TxIn: []*wire.TxIn{
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: 0}, Sequence: 4294967295},
		},
		TxOut:    []*wire.TxOut{{Value: 1}, {Value: 2}, {Value: 3}},
		LockTime: 0,
	}

	tx.TxIn[i].PreviousOutPoint.Hash = parentTx.TxHash()
	tx.TxIn[i].PreviousOutPoint.Index = 0

	{
		// check multisig script
		sigScript, err := SignTxOutput(&chaincfg.TestNetParams, tx, i,
			pay2MultisigPkScript,
			hashType,
			mkGetKey(map[string]addressToKey{
				addressAlice.EncodeAddress(): {alice, true},
				addressBob.EncodeAddress():   {bob, true},
			}),
			mkGetScript(map[string][]byte{
				pay2MultisigScriptAddr.EncodeAddress(): multiSigScript,
			}), nil)
		failErr(t, err, "failed to sign output %s: %v", msg, err)

		// in := tx.TxIn[i]
		// in.Age = lockPeriod + 5
		// in.SignatureScript = nil
		// tx.TxIn[i] = in
		// activateTraceLogger()

		asm, _ := DisasmString(sigScript)
		fmt.Println("refund_signature_asm: ", asm)
		//
		err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, pay2MultisigPkScript)
		failErr(t, err, "fully signed refund script invalid for %s: %v\n", msg, err)

		// fmt.Println("refund_signature_hex: ", hex.EncodeToString(sigScript))
	}
	{
		lockPeriod := int32(24000)
		htlcScript, err := HTLCScriptAddress(pay2MultisigScriptAddr, lockPeriod, &chaincfg.TestNetParams)
		failErr(t, err, "failed to make pkscript for %s: %v", msg, err)
		pay2HTLCScript := htlcScript.ScriptAddress()

		parentTx.TxOut[0].PkScript = pay2HTLCScript
		tx.TxIn[i].PreviousOutPoint.Hash = parentTx.TxHash()

		// check multisig script
		sigScript, err := SignTxOutput(&chaincfg.TestNetParams, tx, i,
			pay2HTLCScript,
			hashType,
			mkGetKey(map[string]addressToKey{
				addressAlice.EncodeAddress(): {alice, true},
				addressBob.EncodeAddress():   {bob, true},
			}),
			mkGetScript(map[string][]byte{
				pay2MultisigScriptAddr.EncodeAddress(): multiSigScript,
				htlcScript.EncodeAddress():             multiSigScript,
			}), nil)
		failErr(t, err, "failed to sign output %s: %v", msg, err)

		in := tx.TxIn[i]
		in.Age = lockPeriod + 5
		// in.SignatureScript = nil
		tx.TxIn[i] = in
		// activateTraceLogger()

		asm, _ := DisasmString(sigScript)
		fmt.Println("refund_signature_asm: ", asm)
		//
		err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, pay2HTLCScript)
		failErr(t, err, "fully signed refund script invalid for %s: %v\n", msg, err)

		fmt.Println("refund_signature_hex: ", hex.EncodeToString(sigScript))
	}
}
