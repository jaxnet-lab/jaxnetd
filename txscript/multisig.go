package txscript

import (
	"fmt"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

// MultiSigScript returns a valid script for a multisignature redemption where
// nrequired of the keys in pubkeys are required to have signed the transaction
// for success.  An Error with the error code ErrTooManyRequiredSigs will be
// returned if nrequired is larger than the number of keys provided.
func MultiSigScript(pubkeys []*btcutil.AddressPubKey, nrequired int) ([]byte, error) {
	if len(pubkeys) < nrequired {
		str := fmt.Sprintf("unable to generate multisig script with "+
			"%d required signatures when there are only %d public "+
			"keys available", nrequired, len(pubkeys))
		return nil, scriptError(ErrTooManyRequiredSigs, str)
	}

	builder := NewScriptBuilder().AddInt64(int64(nrequired))
	for _, key := range pubkeys {
		builder.AddData(key.ScriptAddress())
	}
	builder.AddInt64(int64(len(pubkeys)))
	builder.AddOp(OP_CHECKMULTISIG)

	return builder.Script()
}

// MultiSigLockScript is a multi-signature lock script is of the form:
//
// OP_INPUTAGE <required_age_for_refund>  OP_LESSTHAN
// OP_IF
//     <numsigs> <pubkey> <pubkey> <pubkey>... <numpubkeys>
//     OP_CHECKMULTISIG
// OP_ELSE
//     <refund_pubkey> OP_CHECKSIG OP_NIP
// OP_ENDIF
//
// Can be spent in two ways:
// 1. as a multi-signature script, if tx.TxIn[i].Age less than <required_age_for_refund>.
// 2. as pay-to-pubkey ("refund to owner") script, if tx.TxIn[i].Age greater than <required_age_for_refund>.
//
// "Refund to owner" can be spend ONLY to address derived from the refund_pubkey:
// pay-to-pubkey or pay-to-pubkey-hash
//
// In case of MultiSig activation the number of required signatures is the 4th item
// on the stack and the number of public keys is the 5th to last
// item on the stack.
// Otherwise, in case of "refund to owner" (pay-to-pubkey), requires one signature
// and pubkey is the second to last item on the stack.
func MultiSigLockScript(pubkeys []*btcutil.AddressPubKey, sigRequired int,
	refundAddress *btcutil.AddressPubKey, refundDeferringPeriod int32) ([]byte, error) {
	if len(pubkeys) < sigRequired {
		str := fmt.Sprintf("unable to generate multisig script with "+
			"%d required signatures when there are only %d public "+
			"keys available", sigRequired, len(pubkeys))
		return nil, scriptError(ErrTooManyRequiredSigs, str)
	}

	builder := NewScriptBuilder()
	// lock
	builder.AddOp(OP_INPUTAGE)
	builder.AddInt64(int64(refundDeferringPeriod))
	builder.AddOp(OP_LESSTHAN)
	// ---

	builder.AddOp(OP_IF)

	// multisig statement
	builder.AddInt64(int64(sigRequired))
	for _, key := range pubkeys {
		builder.AddData(key.ScriptAddress())
	}
	builder.AddInt64(int64(len(pubkeys)))
	builder.AddOp(OP_CHECKMULTISIG)
	// ---

	builder.AddOp(OP_ELSE)

	// refund statement
	builder.AddData(refundAddress.ScriptAddress())
	builder.AddOp(OP_CHECKSIG)
	builder.AddOp(OP_NIP)
	// ---
	builder.AddOp(OP_ENDIF)

	return builder.Script()
}

const (

	// mslFirstMSigOpI is the index of the first parsedOpcode for MultiSigTy
	mslFirstMSigOpI = 4

	// mslTailLen is the len of parsedOpcode set associated with the "refund tail" of the script.
	mslTailLen = 5
)

// isMultiSigLock returns true if the passed script is a MultiSigLockTy transaction, false
// otherwise.
// The minimal valid MultiSigLockTy:
// [ 0] OP_INPUTAGE
// [ 1] <required_age_for_refund>
// [ 2] OP_LESSTHAN
// [ 3] OP_IF
// [ 4]    OP_1
// [..]    <pubkey>
// [-7]    OP_1
// [-6]    OP_CHECKMULTISIG
// [-5] OP_ELSE
// [-4]    <refund_pubkey>
// [-3]    OP_CHECKSIG
// [-2]    OP_NIP
// [-1] OP_ENDIF
func isMultiSigLock(pops []parsedOpcode) bool {
	l := len(pops)
	if l < 13 {
		return false
	}
	containsStatements :=
		isOpCode(pops[0], OP_INPUTAGE) &&
			isNumber(pops[1].opcode) &&
			isOpCode(pops[2], OP_LESSTHAN) &&
			isOpCode(pops[3], OP_IF) &&
			isOpCode(pops[l-5], OP_ELSE) &&
			(len(pops[l-4].data) == 33 || len(pops[l-4].data) == 65) &&
			isOpCode(pops[l-3], OP_CHECKSIG) &&
			isOpCode(pops[l-2], OP_NIP) &&
			isOpCode(pops[l-1], OP_ENDIF)
	if !containsStatements {
		return false
	}

	multiSigEnd := l - mslTailLen
	return isMultiSig(pops[mslFirstMSigOpI:multiSigEnd])
}

// extractMultiSigLockAddrs collect all possible addresses from multi sig lock script
// A multi-signature lock script is of the form:
// OP_INPUTAGE <required_age_for_refund> OP_LESSTHAN
// OP_IF <numsigs> <pubkey> <pubkey> <pubkey>... <numpubkeys> OP_CHECKMULTISIG
// OP_ELSE <refund_pubkey> OP_CHECKSIG OP_NIP
// OP_ENDIF
func extractMultiSigLockAddrs(pops []parsedOpcode,
	chainParams *chaincfg.Params) (addrs []btcutil.Address, requiredSigs int) {
	// In case of MultiSig activation the number of required signatures is the 5th item
	// on the stack and the number of public keys is the 5th to last
	// item on the stack.
	// Otherwise, in case of refund (pay-to-pubkey),
	// requires one signature and address is the second to last item on the stack.
	requiredSigs = asSmallInt(pops[mslFirstMSigOpI].opcode)
	numPubKeys := asSmallInt(pops[len(pops)-mslTailLen-2].opcode)

	addrIndex := map[string]struct{}{}
	// Extract the public keys while skipping any that are invalid.
	addrs = make([]btcutil.Address, 0, numPubKeys+1)
	for i := 0; i < numPubKeys; i++ {
		addr, err := btcutil.NewAddressPubKey(pops[mslFirstMSigOpI+1+i].data, chainParams)
		if err == nil {
			if _, ok := addrIndex[addr.String()]; !ok {
				addrs = append(addrs, addr)
				addrIndex[addr.String()] = struct{}{}
			}
		}
	}

	addr, err := btcutil.NewAddressPubKey(pops[len(pops)-4].data, chainParams)
	if err == nil {
		if _, ok := addrIndex[addr.String()]; !ok {
			addrs = append(addrs, addr)
		}
	}

	return
}

// signMultiSigLock signs as many of the outputs in the provided multisig script as
// possible. It returns the generated script and a boolean if the script fulfils
// the contract (i.e. nrequired signatures are provided).  Since it is arguably
// legal to not be able to sign any of the outputs, no error is returned.
func signMultiSigLock(tx *wire.MsgTx, idx int, subScript []byte, hashType SigHashType,
	addresses []btcutil.Address, nRequired int, kdb KeyDB) ([]byte, bool) {

	// here is safe to ignore error, it checked previously.
	parsed, _ := parseScript(subScript)

	var refundLock int32
	if isSmallInt(parsed[1].opcode) {
		refundLock = int32(asSmallInt(parsed[1].opcode))
	} else {
		num, _ := makeScriptNum(parsed[1].data, true, 5)
		refundLock = num.Int32()
	}

	if tx.TxIn[idx].Age >= refundLock {
		nRequired = 1
	}

	// We start with a single OP_FALSE to work around the (now standard)
	// but in the reference implementation that causes a spurious pop at
	// the end of OP_CHECKMULTISIG.
	builder := NewScriptBuilder()
	builder.AddOp(OP_FALSE)

	signed := 0
	for _, addr := range addresses {
		key, _, err := kdb.GetKey(addr)
		if err != nil {
			continue
		}
		sig, err := RawTxInSignature(tx, idx, subScript, hashType, key)
		if err != nil {
			continue
		}

		builder.AddData(sig)
		signed++
		if signed == nRequired {
			break
		}
	}

	script, _ := builder.Script()
	return script, signed == nRequired
}
