/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaindata

import "gitlab.com/jaxnet/jaxnetd/types/wire"

// BlockTemplate houses a block that has yet to be solved along with additional
// details about the fees and the number of signature operations for each
// transaction in the block.
type BlockTemplate struct {
	// Block is a block that is ready to be solved by miners.  Thus, it is
	// completely valid with the exception of satisfying the proof-of-work
	// requirement.
	Block *wire.MsgBlock

	// Fees contains the amount of fees each transaction in the generated
	// template pays in base units.  Since the first transaction is the
	// coinbase, the first entry (offset 0) will contain the negative of the
	// sum of the fees of all other transactions.
	Fees []int64

	// SigOpCosts contains the number of signature operations each
	// transaction in the generated template performs.
	SigOpCosts []int64

	// Height is the height at which the block template connects to the main
	// chain.
	Height int32

	// ValidPayAddress indicates whether or not the template coinbase pays
	// to an address or is redeemable by anyone.  See the documentation on
	// NewBlockTemplate for details on which this can be useful to generate
	// templates without a coinbase payment address.
	ValidPayAddress bool

	// WitnessCommitment is a commitment to the witness data (if any)
	// within the block. This field will only be populted once segregated
	// witness has been activated, and the block contains a transaction
	// which has witness data.
	WitnessCommitment []byte
}
