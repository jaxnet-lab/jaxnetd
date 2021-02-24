/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package txscript

import (
	"gitlab.com/jaxnet/core/shard.core/btcec"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

// HtlcScript is the uniform script that's used as the output for
// the second-level HTLC transactions. The second level transaction act as a
// sort of covenant, ensuring that a 2-of-2 multi-sig output can only be
// spent in a particular way, and to a particular output.
//
// Possible Input Scripts:
//  * To revoke an HTLC output that has been transitioned to the claim+delay
//    state:
//    * <revoke sig> 1
//
//  * To claim and HTLC output, either with a pre-image or due to a timeout:
//    * <delay sig> 0
//
// OP_IF
//     <revoke key>
// OP_ELSE
//     <delay in blocks>
//     OP_CHECKSEQUENCEVERIFY
//     OP_DROP
//     <delay key>
// OP_ENDIF
// OP_CHECKSIG
//
// TODO(roasbeef): possible renames for second-level
//  * transition?
//  * covenant output
func HtlcScript(revocationKey, delayKey *btcec.PublicKey, csvDelay uint32) ([]byte, error) {

	builder := NewScriptBuilder()

	// If this is the revocation clause for this script is to be executed,
	// the spender will push a 1, forcing us to hit the true clause of this
	// if statement.
	builder.AddOp(OP_IF)

	// If this this is the revocation case, then we'll push the revocation
	// public key on the stack.
	builder.AddData(revocationKey.SerializeCompressed())

	// Otherwise, this is either the sender or receiver of the HTLC
	// attempting to claim the HTLC output.
	builder.AddOp(OP_ELSE)

	// In order to give the other party time to execute the revocation
	// clause above, we require a relative timeout to pass before the
	// output can be spent.
	builder.AddInt64(int64(csvDelay))
	builder.AddOp(OP_CHECKSEQUENCEVERIFY)
	builder.AddOp(OP_DROP)

	// If the relative timelock passes, then we'll add the delay key to the
	// stack to ensure that we properly authenticate the spending party.
	builder.AddData(delayKey.SerializeCompressed())

	// Close out the if statement.
	builder.AddOp(OP_ENDIF)

	// In either case, we'll ensure that only either the party possessing
	// the revocation private key, or the delay private key is able to
	// spend this output.
	builder.AddOp(OP_CHECKSIG)

	return builder.Script()
}

// HtlcSpendSuccess spends a second-level HTLC output. This function is to be
// used by the sender of an HTLC to claim the output after a relative timeout
// or the receiver of the HTLC to claim on-chain with the pre-image.
func HtlcSpendSuccess(sweepTx *wire.MsgTx, csvDelay uint32, idx int, witnessScript []byte,
	hashType SigHashType, key *btcec.PrivateKey) ([]byte, error) {

	// We're required to wait a relative period of time before we can sweep
	// the output in order to allow the other party to contest our claim of
	// validity to this version of the commitment transaction.
	sweepTx.TxIn[0].Sequence = LockTimeToSequence(false, csvDelay)

	// Finally, OP_CSV requires that the version of the transaction
	// spending a pkscript with OP_CSV within it *must* be >= 2.
	sweepTx.Version = 2

	// As we mutated the transaction, we'll re-calculate the sighashes for
	// this instance.
	// sigHashes := NewTxSigHashes(sweepTx)

	// With the proper sequence and version set, we'll now sign the timeout
	// transaction using the passed signed descriptor. In order to generate
	// a valid signature, then signDesc should be using the base delay
	// public key, and the proper single tweak bytes.
	// Chop off the sighash flag at the end of the signature.
	sig, err := RawTxInSignature(sweepTx, idx, witnessScript, hashType, key)
	if err != nil {
		return nil, err
	}
	// sweepSig, err := btcec.ParseDERSignature(sig[:len(sig)-1], btcec.S256())
	// if err != nil {
	// 	return nil, err
	// }
	//
	// // We set a zero as the first element the witness stack (ignoring the
	// // witness script), in order to force execution to the second portion
	// // of the if clause.
	// witnessStack := wire.TxWitness(make([][]byte, 3))
	// witnessStack[0] = append(sweepSig.Serialize(), byte(hashType))
	// witnessStack[1] = nil
	// witnessStack[2] = witnessScript

	return sig, nil
}

// LockTimeToSequence converts the passed relative locktime to a sequence
// number in accordance to BIP-68.
// See: https://github.com/bitcoin/bips/blob/master/bip-0068.mediawiki
//  * (Compatibility)
func LockTimeToSequence(isSeconds bool, locktime uint32) uint32 {
	// If we're expressing the relative lock time in blocks, then the
	// corresponding sequence number is simply the desired input age.
	if !isSeconds {
		return locktime
	}

	// Set the 22nd bit which indicates the lock time is in seconds, then
	// shift the locktime over by 9 since the time granularity is in
	// 512-second intervals (2^9). This results in a max lock-time of
	// 33,553,920 seconds, or 1.1 years.
	return wire.SequenceLockTimeIsSeconds |
		locktime>>wire.SequenceLockTimeGranularity
}
