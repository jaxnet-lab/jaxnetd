package txscript

import (
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// HTLCScript is a multi-signature lock script is of the form:
//
// OP_INPUTAGE <required_age_for_refund>  OP_LESSTHAN
// OP_IF
//     OP_RETURN
// OP_ELSE
//     <refund_pubkey> OP_CHECKSIG OP_NIP
// OP_ENDIF
// ...
func HTLCScript(pubkey jaxutil.Address, lockPeriod int32) ([]byte, error) {
	builder := NewScriptBuilder()
	// lock
	builder.AddOp(OP_INPUTAGE)
	builder.AddInt64(int64(lockPeriod))
	builder.AddOp(OP_LESSTHAN)
	builder.AddOp(OP_IF)
	builder.AddOp(OP_CHECKMULTISIG)
	builder.AddOp(OP_ELSE)
	builder.AddData(pubkey.ScriptAddress())
	builder.AddOp(OP_CHECKSIG)
	builder.AddOp(OP_NIP)
	builder.AddOp(OP_ENDIF)

	return builder.Script()
}

// isMultiSigLock returns true if the passed script is a MultiSigLockTy transaction, false
// otherwise.
// The minimal valid MultiSigLockTy:
// [ 0] OP_INPUTAGE
// [ 1] <required_age_for_refund>
// [ 2] OP_LESSTHAN
// [ 3] OP_IF
// [ 4]     OR_RETURN
// [ 5] OP_ELSE
// [ 6]    <refund_pubkey>
// [ 7]    OP_CHECKSIG
// [ 8]    OP_NIP
// [ 9] OP_ENDIF
func isHTLC(pops []parsedOpcode) bool {
	l := len(pops)
	if l != 10 {
		return false
	}
	return isOpCode(pops[0], OP_INPUTAGE) &&
		isNumber(pops[1].opcode) &&
		isOpCode(pops[2], OP_LESSTHAN) &&
		isOpCode(pops[3], OP_IF) &&
		isOpCode(pops[4], OP_RETURN) &&
		isOpCode(pops[5], OP_ELSE) &&
		(len(pops[6].data) == 33 || len(pops[6].data) == 65) &&
		isOpCode(pops[7], OP_CHECKSIG) &&
		isOpCode(pops[8], OP_NIP) &&
		isOpCode(pops[9], OP_ENDIF)

}

// extractHTLCAddrs
func extractHTLCAddrs(pops []parsedOpcode,
	chainParams *chaincfg.Params) (addr jaxutil.Address, err error) {

	return jaxutil.NewAddressPubKey(pops[6].data, chainParams)
}

// signHTLC ...
func signHTLC(tx *wire.MsgTx, idx int, subScript []byte, hashType SigHashType,
	address jaxutil.Address, kdb KeyDB) ([]byte, error) {

	// We start with a single OP_FALSE to work around the (now standard)
	// but in the reference implementation that causes a spurious pop at
	// the end of OP_CHECKMULTISIG.

	key, _, err := kdb.GetKey(address)
	if err != nil {
		return nil, err
	}
	sig, err := RawTxInSignature(tx, idx, subScript, hashType, key)
	if err != nil {
		return nil, err
	}

	return NewScriptBuilder().AddData(sig).Script()
}
