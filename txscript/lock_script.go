package txscript

import (
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// HTLCScript is a multi-signature lock script is of the form:
//
// OP_INPUTAGE <lockPeriod> OP_GREATERTHAN
// OP_IF
//     OP_DUP
//     OP_HASH160
//     <scriptAddress>
//     OP_EQUALVERIFY
//     OP_CHECKSIG
// OP_ELSE
//     OP_RETURN
// OP_ENDIF
// ...
func HTLCScript(address *jaxutil.AddressPubKeyHash, lockPeriod int32) ([]byte, error) {
	builder := NewScriptBuilder()
	// lock
	builder.AddOp(OP_INPUTAGE)
	builder.AddInt64(int64(lockPeriod))
	builder.AddOp(OP_GREATERTHAN)
	builder.AddOp(OP_IF)

	builder.AddOp(OP_DUP)
	builder.AddOp(OP_HASH160)
	builder.AddData(address.ScriptAddress())
	builder.AddOp(OP_EQUALVERIFY)
	builder.AddOp(OP_CHECKSIG)

	// payScript, err := PayToAddrScript(address)
	// if err != nil {
	// 	return nil, err
	// }

	// println(DisasmString(payScript))
	// builder.AddData(payScript)

	// builder.AddOp(OP_NIP)
	builder.AddOp(OP_ELSE)
	builder.AddOp(OP_RETURN)
	// builder.AddOp(OP_NIP)

	builder.AddOp(OP_ENDIF)

	return builder.Script()
}

// isMultiSigLock returns true if the passed script is a MultiSigLockTy transaction, false
// otherwise.
// The minimal valid MultiSigLockTy:
// [ 0] OP_INPUTAGE
// [ 1] <lockPeriod>
// [ 2] OP_GREATERTHAN
// [ 3] OP_IF
// [ 4]    OP_DUP
// [ 5]    OP_HASH160
// [ 6]    <scriptAddress>
// [ 7]    OP_EQUALVERIFY
// [ 8]    OP_CHECKSIG
// [ 9] OP_ELSE
// [10]     OR_RETURN
// [11] OP_ENDIF
func isHTLC(pops []parsedOpcode) bool {
	l := len(pops)
	if l != 12 {
		return false
	}

	return isOpCode(pops[0], OP_INPUTAGE) &&
		isNumber(pops[1].opcode) &&
		isOpCode(pops[2], OP_GREATERTHAN) &&
		isOpCode(pops[3], OP_IF) &&
		pops[4].opcode.value == OP_DUP &&
		pops[5].opcode.value == OP_HASH160 &&
		pops[6].opcode.value == OP_DATA_20 &&
		pops[7].opcode.value == OP_EQUALVERIFY &&
		pops[8].opcode.value == OP_CHECKSIG &&
		isOpCode(pops[9], OP_ELSE) &&
		isOpCode(pops[10], OP_RETURN) &&
		// isOpCode(pops[11], OP_NIP) &&
		isOpCode(pops[11], OP_ENDIF)

}

// extractHTLCAddrs
func extractHTLCAddrs(pops []parsedOpcode, chainParams *chaincfg.Params) (ScriptClass, []jaxutil.Address, int, error) {
	println(DisasmString(pops[4].data))
	// A pay-to-pubkey-hash script is of the form:
	//  OP_DUP OP_HASH160 <hash> OP_EQUALVERIFY OP_CHECKSIG
	// Therefore the pubkey hash is the 3rd item on the stack.
	// Skip the pubkey hash if it's invalid for some reason.
	addr, err := jaxutil.NewAddressPubKeyHash(pops[6].data, chainParams)

	return HTLCScriptTy, []jaxutil.Address{addr}, 1, err
}

// signHTLC ...
func signHTLC(tx *wire.MsgTx, idx int, subScript []byte, hashType SigHashType,
	address jaxutil.Address, kdb KeyDB) ([]byte, error) {

	// look up key for address
	key, compressed, err := kdb.GetKey(address)
	if err != nil {
		return nil, err
	}

	return SignatureScript(tx, idx, subScript, hashType, key, compressed)
}

func ExtractHTLCLockTime(data []byte) (int32, error) {
	pops, err := parseScript(data)
	if err != nil {
		return 0, err
	}

	var lockTime int32
	if isSmallInt(pops[1].opcode) {
		rawShardID := asSmallInt(pops[1].opcode)
		lockTime = int32(rawShardID)
		return lockTime, nil
	}

	var rawShardID scriptNum
	rawShardID, err = makeScriptNum(pops[1].data, true, 5)
	if err != nil {
		return 0, err
	}
	lockTime = int32(rawShardID)

	return lockTime, nil
}
