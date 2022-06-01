package txscript

import (
	"errors"

	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

// HTLCScript is a multi-signature lock script is of the form:
//
// OP_INPUTAGE <lockPeriod> OP_GREATERTHAN
// OP_IF
//     OP_DUP  OP_HASH160  <AddressPubKeyHash> OP_EQUALVERIFY OP_CHECKSIG
// or
//     <AddressPubKey> OP_CHECKSIG
// or
//     OP_HASH160  <AddressScriptHash> OP_EQUAL
// OP_ELSE
//     OP_RETURN
// OP_ENDIF
// ...
func HTLCScript(address jaxutil.Address, lockPeriod int32) ([]byte, error) {
	builder := NewScriptBuilder()
	// lock
	builder.AddOp(OP_INPUTAGE)
	builder.AddInt64(int64(lockPeriod))
	builder.AddOp(OP_GREATERTHAN)
	builder.AddOp(OP_IF)

	switch addr := address.(type) {
	case *jaxutil.AddressPubKey:
		builder.AddData(addr.ScriptAddress())
		builder.AddOp(OP_CHECKSIG)
	case *jaxutil.AddressPubKeyHash:
		builder.AddOp(OP_DUP)
		builder.AddOp(OP_HASH160)
		builder.AddData(addr.ScriptAddress())
		builder.AddOp(OP_EQUALVERIFY)
		builder.AddOp(OP_CHECKSIG)
	case *jaxutil.AddressScriptHash:
		builder.AddOp(OP_HASH160)
		builder.AddData(addr.ScriptAddress())
		builder.AddOp(OP_EQUAL)
	default:
		return nil, errors.New("unsupported address type")
	}

	builder.AddOp(OP_ELSE)
	builder.AddOp(OP_RETURN)
	// builder.AddOp(OP_NIP)

	builder.AddOp(OP_ENDIF)

	return builder.Script()
}

func HTLCScriptAddress(address jaxutil.Address, lockPeriod int32, params *chaincfg.Params) (*jaxutil.HTLCAddress, error) {
	script, err := HTLCScript(address, lockPeriod)
	if err != nil {
		return nil, err
	}

	return jaxutil.NewHTLCAddress(script, params)
}

// isMultiSigLock returns true if the passed script is a MultiSigLockTy transaction, false
// otherwise.
// The minimal valid MultiSigLockTy:
// [ 0] OP_INPUTAGE
// [ 1] <lockPeriod>
// [ 2] OP_GREATERTHAN
// [ 3] OP_IF
// [--]     todo:
// [ 9] OP_ELSE
// [10]     OR_RETURN
// [11] OP_ENDIF
func isHTLC(pops []parsedOpcode) (bool, ScriptClass) {
	l := len(pops)
	if l != 9 && l != 10 && l != 12 {
		return false, NonStandardTy
	}

	templateMatch := isOpCode(pops[0], OP_INPUTAGE) &&
		isNumber(pops[1].opcode) &&
		isOpCode(pops[2], OP_GREATERTHAN) &&
		isOpCode(pops[3], OP_IF) &&
		isOpCode(pops[l-3], OP_ELSE) &&
		isOpCode(pops[l-2], OP_RETURN) &&
		isOpCode(pops[l-1], OP_ENDIF)
	if !templateMatch {
		return false, NonStandardTy
	}

	switch len(pops) {
	case 9: // isPubkey
		templateMatch = (len(pops[4].data) == 33 || len(pops[4].data) == 65) && pops[5].opcode.value == OP_CHECKSIG
		if templateMatch {
			return true, PubKeyTy
		}
	case 10: // isScriptHash
		templateMatch = pops[4].opcode.value == OP_HASH160 &&
			pops[5].opcode.value == OP_DATA_20 &&
			pops[6].opcode.value == OP_EQUAL
		if templateMatch {
			return true, ScriptHashTy
		}
	case 12: // isPubkeyHash
		templateMatch = pops[4].opcode.value == OP_DUP &&
			pops[5].opcode.value == OP_HASH160 &&
			pops[6].opcode.value == OP_DATA_20 &&
			pops[7].opcode.value == OP_EQUALVERIFY &&
			pops[8].opcode.value == OP_CHECKSIG
		if templateMatch {
			return true, PubKeyHashTy
		}
	}

	return false, NonStandardTy
}

func isHTLCWithScriptHash(pops []parsedOpcode) bool {
	_, innerType := isHTLC(pops)
	return innerType == ScriptHashTy
}

// extractHTLCAddrs ...
func extractHTLCAddrs(pops []parsedOpcode, chainParams *chaincfg.Params) (ScriptClass, []jaxutil.Address, int, error) {
	var addr jaxutil.Address
	var err error
	switch len(pops) {
	case 9:
		// OP_INPUTAGE <lockPeriod> OP_GREATERTHAN OP_IF
		// <pubkey> OP_CHECKSIG
		// OP_ELSE OP_RETURN OP_ENDIF
		addr, err = jaxutil.NewAddressPubKey(pops[4].data, chainParams)
	case 10:
		// OP_INPUTAGE <lockPeriod> OP_GREATERTHAN OP_IF
		// OP_HASH160 <scripthash> OP_EQUAL
		// OP_ELSE OP_RETURN OP_ENDIF
		addr, err = jaxutil.NewAddressScriptHashFromHash(pops[5].data, chainParams)
	case 12:
		// OP_INPUTAGE <lockPeriod> OP_GREATERTHAN OP_IF
		// OP_DUP OP_HASH160 <hash> OP_EQUALVERIFY OP_CHECKSIG
		// OP_ELSE OP_RETURN OP_ENDIF
		addr, err = jaxutil.NewAddressPubKeyHash(pops[6].data, chainParams)
	}

	return HTLCScriptTy, []jaxutil.Address{addr}, 1, err
}

// signHTLC ...
// nolint: gomnd
func signHTLC(tx *wire.MsgTx, idx int, subScript []byte, hashType SigHashType,
	address []jaxutil.Address, kdb KeyDB, sdb ScriptDB,
) ([]byte, ScriptClass, error) {
	// look up key for address

	pops, _ := parseScript(subScript)
	switch len(pops) {
	case 9:
		key, _, err := kdb.GetKey(address[0])
		if err != nil {
			return nil, 0, err
		}

		sig, err := p2pkSignatureScript(tx, idx, subScript, hashType, key)
		return sig, HTLCScriptTy, err
	case 10:
		sig, err := sdb.GetScript(address[0])
		return sig, ScriptHashTy, err
	case 12:
		key, compressed, err := kdb.GetKey(address[0])
		if err != nil {
			return nil, 0, err
		}
		sig, err := SignatureScript(tx, idx, subScript, hashType, key, compressed)
		return sig, HTLCScriptTy, err
	}

	return nil, NonStandardTy, errors.New("invalid htlc script")
}

// ExtractHTLCData extract full info from HTLC pkScript:
// 1) subScript - the original Address,
// 2) subScriptClass -  class of th inner pkScript,
// 3) lockTime
func ExtractHTLCData(pkScript []byte, params *chaincfg.Params) (jaxutil.Address, ScriptClass, int32, error) {
	pops, err := parseScript(pkScript)
	if err != nil {
		return nil, 0, 0, err
	}

	var (
		addr        jaxutil.Address
		scriptClass ScriptClass
		lockTime    int32
	)

	if isSmallInt(pops[1].opcode) {
		rawShardID := asSmallInt(pops[1].opcode)
		lockTime = int32(rawShardID)
	} else {
		var rawShardID scriptNum
		rawShardID, err = makeScriptNum(pops[1].data, true, 5)
		if err != nil {
			return nil, 0, 0, err
		}
		lockTime = int32(rawShardID)
	}

	switch len(pops) {
	case 9:
		addr, err = jaxutil.NewAddressPubKey(pops[4].data, params)
		scriptClass = PubKeyTy
	case 10:
		addr, err = jaxutil.NewAddressScriptHashFromHash(pops[5].data, params)
		scriptClass = ScriptHashTy
	case 12:
		addr, err = jaxutil.NewAddressPubKeyHash(pops[6].data, params)
		scriptClass = PubKeyHashTy
	}

	return addr, scriptClass, lockTime, err
}

func ExtractHTLCLockTime(pkScript []byte) (int32, error) {
	pops, err := parseScript(pkScript)
	if err != nil {
		return 0, err
	}

	var lockTime int32
	if isSmallInt(pops[1].opcode) {
		rawShardID := asSmallInt(pops[1].opcode)
		lockTime = int32(rawShardID)
		return lockTime, nil
	}

	var rawLockTime scriptNum
	rawLockTime, err = makeScriptNum(pops[1].data, true, 5)
	if err != nil {
		return 0, err
	}
	lockTime = int32(rawLockTime)

	return lockTime, nil
}
