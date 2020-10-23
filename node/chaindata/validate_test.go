// Copyright (c) 2013-2017 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// +build deprecated_tests

package chaindata

import (
	"math"
	"reflect"
	"testing"
	"time"

	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

// TestSequenceLocksActive tests the SequenceLockActive function to ensure it
// works as expected in all possible combinations/scenarios.
func TestSequenceLocksActive(t *testing.T) {
	seqLock := func(h int32, s int64) *SequenceLock {
		return &SequenceLock{
			Seconds:     s,
			BlockHeight: h,
		}
	}

	tests := []struct {
		seqLock     *SequenceLock
		blockHeight int32
		mtp         time.Time

		want bool
	}{
		// Block based sequence lock with equal block height.
		{seqLock: seqLock(1000, -1), blockHeight: 1001, mtp: time.Unix(9, 0), want: true},

		// Time based sequence lock with mtp past the absolute time.
		{seqLock: seqLock(-1, 30), blockHeight: 2, mtp: time.Unix(31, 0), want: true},

		// Block based sequence lock with current height below seq lock block height.
		{seqLock: seqLock(1000, -1), blockHeight: 90, mtp: time.Unix(9, 0), want: false},

		// Time based sequence lock with current time before lock time.
		{seqLock: seqLock(-1, 30), blockHeight: 2, mtp: time.Unix(29, 0), want: false},

		// Block based sequence lock at the same height, so shouldn't yet be active.
		{seqLock: seqLock(1000, -1), blockHeight: 1000, mtp: time.Unix(9, 0), want: false},

		// Time based sequence lock with current time equal to lock time, so shouldn't yet be active.
		{seqLock: seqLock(-1, 30), blockHeight: 2, mtp: time.Unix(30, 0), want: false},
	}

	t.Logf("Running %d sequence locks tests", len(tests))
	for i, test := range tests {
		got := SequenceLockActive(test.seqLock,
			test.blockHeight, test.mtp)
		if got != test.want {
			t.Fatalf("SequenceLockActive #%d got %v want %v", i,
				got, test.want)
		}
	}
}

// TestCheckConnectBlockTemplate tests the CheckConnectBlockTemplate function to
// ensure it fails.
func TestCheckConnectBlockTemplate(t *testing.T) {
	// Create a new database and chain instance to run tests against.
	chain, teardownFunc, err := chainSetup("checkconnectblocktemplate",
		&chaincfg.MainNetParams)
	if err != nil {
		t.Errorf("Failed to setup chain instance: %v", err)
		return
	}
	defer teardownFunc()

	// Since we're not dealing with the real block chain, set the coinbase
	// maturity to 1.
	chain.TstSetCoinbaseMaturity(1)

	// Load up blocks such that there is a side chain.
	// (genesis block) -> 1 -> 2 -> 3 -> 4
	//                          \-> 3a
	testFiles := []string{
		"blk_0_to_4.dat.bz2",
		"blk_3A.dat.bz2",
	}

	var blocks []*btcutil.Block
	for _, file := range testFiles {
		blockTmp, err := loadBlocks(file)
		if err != nil {
			t.Fatalf("Error loading file: %v\n", err)
		}
		blocks = append(blocks, blockTmp...)
	}

	for i := 1; i <= 3; i++ {
		isMainChain, _, err := chain.ProcessBlock(blocks[i], BFNone)
		if err != nil {
			t.Fatalf("CheckConnectBlockTemplate: Received unexpected error "+
				"processing block %d: %v", i, err)
		}
		if !isMainChain {
			t.Fatalf("CheckConnectBlockTemplate: Expected block %d to connect "+
				"to main chain", i)
		}
	}

	// Block 3 should fail to connect since it's already inserted.
	err = chain.CheckConnectBlockTemplate(blocks[3])
	if err == nil {
		t.Fatal("CheckConnectBlockTemplate: Did not received expected error " +
			"on block 3")
	}

	// Block 4 should connect successfully to tip of chain.
	err = chain.CheckConnectBlockTemplate(blocks[4])
	if err != nil {
		t.Fatalf("CheckConnectBlockTemplate: Received unexpected error on "+
			"block 4: %v", err)
	}

	// Block 3a should fail to connect since does not build on chain tip.
	err = chain.CheckConnectBlockTemplate(blocks[5])
	if err == nil {
		t.Fatal("CheckConnectBlockTemplate: Did not received expected error " +
			"on block 3a")
	}

	// Block 4 should connect even if proof of work is invalid.
	invalidPowBlock := *blocks[4].MsgBlock()
	invalidPowBlock.Header.SetNonce(invalidPowBlock.Header.Nonce() + 1)
	err = chain.CheckConnectBlockTemplate(btcutil.NewBlock(&invalidPowBlock))
	if err != nil {
		t.Fatalf("CheckConnectBlockTemplate: Received unexpected error on "+
			"block 4 with bad nonce: %v", err)
	}

	// Invalid block building on chain tip should fail to connect.
	invalidBlock := *blocks[4].MsgBlock()
	invalidBlock.Header.SetBits(invalidBlock.Header.Bits() - 1)
	err = chain.CheckConnectBlockTemplate(btcutil.NewBlock(&invalidBlock))
	if err == nil {
		t.Fatal("CheckConnectBlockTemplate: Did not received expected error " +
			"on block 4 with invalid difficulty bits")
	}
}

// TestCheckBlockSanity tests the CheckBlockSanity function to ensure it works
// as expected.
func TestCheckBlockSanity(t *testing.T) {
	powLimit := chaincfg.MainNetParams.PowLimit
	block := btcutil.NewBlock(&Block100000)
	timeSource := NewMedianTime()
	err := CheckBlockSanity(block, powLimit, timeSource)
	if err != nil {
		t.Errorf("CheckBlockSanity: %v", err)
	}

	// Ensure a block that has a timestamp with a precision higher than one
	// second fails.
	timestamp := block.MsgBlock().Header.Timestamp()
	block.MsgBlock().Header.SetTimestamp(timestamp.Add(time.Nanosecond))
	err = CheckBlockSanity(block, powLimit, timeSource)
	if err == nil {
		t.Errorf("CheckBlockSanity: error is nil when it shouldn't be")
	}
}

// TestCheckSerializedHeight tests the checkSerializedHeight function with
// various serialized heights and also does negative tests to ensure errors
// and handled properly.
func TestCheckSerializedHeight(t *testing.T) {
	// Create an empty coinbase template to be used in the tests below.
	coinbaseOutpoint := wire.NewOutPoint(&chainhash.Hash{}, math.MaxUint32)
	coinbaseTx := wire.NewMsgTx(1)
	coinbaseTx.AddTxIn(wire.NewTxIn(coinbaseOutpoint, nil, nil))

	// Expected rule errors.
	missingHeightError := RuleError{
		ErrorCode: ErrMissingCoinbaseHeight,
	}
	badHeightError := RuleError{
		ErrorCode: ErrBadCoinbaseHeight,
	}

	tests := []struct {
		sigScript  []byte // Serialized data
		wantHeight int32  // Expected height
		err        error  // Expected error type
	}{
		// No serialized height length.
		{[]byte{}, 0, missingHeightError},
		// Serialized height length with no height bytes.
		{[]byte{0x02}, 0, missingHeightError},
		// Serialized height length with too few height bytes.
		{[]byte{0x02, 0x4a}, 0, missingHeightError},
		// Serialized height that needs 2 bytes to encode.
		{[]byte{0x02, 0x4a, 0x52}, 21066, nil},
		// Serialized height that needs 2 bytes to encode, but backwards
		// endianness.
		{[]byte{0x02, 0x4a, 0x52}, 19026, badHeightError},
		// Serialized height that needs 3 bytes to encode.
		{[]byte{0x03, 0x40, 0x0d, 0x03}, 200000, nil},
		// Serialized height that needs 3 bytes to encode, but backwards
		// endianness.
		{[]byte{0x03, 0x40, 0x0d, 0x03}, 1074594560, badHeightError},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		msgTx := coinbaseTx.Copy()
		msgTx.TxIn[0].SignatureScript = test.sigScript
		tx := btcutil.NewTx(msgTx)

		err := checkSerializedHeight(tx, test.wantHeight)
		if reflect.TypeOf(err) != reflect.TypeOf(test.err) {
			t.Errorf("checkSerializedHeight #%d wrong error type "+
				"got: %v <%T>, want: %T", i, err, err, test.err)
			continue
		}

		if rerr, ok := err.(RuleError); ok {
			trerr := test.err.(RuleError)
			if rerr.ErrorCode != trerr.ErrorCode {
				t.Errorf("checkSerializedHeight #%d wrong "+
					"error code got: %v, want: %v", i,
					rerr.ErrorCode, trerr.ErrorCode)
				continue
			}
		}
	}
}
