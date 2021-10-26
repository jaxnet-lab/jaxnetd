// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"compress/bzip2"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/jaxnet/jaxnetd/database"
	_ "gitlab.com/jaxnet/jaxnetd/database/ffldb"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/blocknodes"
	"gitlab.com/jaxnet/jaxnetd/node/chainctx"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const (
	// testDbType is the database backend type to use for the tests.
	testDbType = "ffldb"

	// testDbRoot is the root directory used to create all test databases.
	testDbRoot = "testdbs"

	// blockDataNet is the expected network in the test block data.
	blockDataNet = wire.MainNet
)

// filesExists returns whether or not the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// isSupportedDbType returns whether or not the passed database type is
// currently supported.
func isSupportedDbType(dbType string) bool {
	supportedDrivers := database.SupportedDrivers()
	for _, driver := range supportedDrivers {
		if dbType == driver {
			return true
		}
	}

	return false
}

// loadBlocks reads files containing bitcoin block data (gzipped but otherwise
// in the format bitcoind writes) from disk and returns them as an array of
// jaxutil.Block.  This is largely borrowed from the test code in btcdb.
func loadBlocks(chainCtx chainctx.IChainCtx, filename string) (blocks []*jaxutil.Block, err error) {
	filename = filepath.Join("testdata/", filename)

	var network = wire.MainNet
	var dr io.Reader
	var fi io.ReadCloser

	fi, err = os.Open(filename)
	if err != nil {
		return
	}

	if strings.HasSuffix(filename, ".bz2") {
		dr = bzip2.NewReader(fi)
	} else {
		dr = fi
	}
	defer fi.Close()

	var block *jaxutil.Block

	err = nil
	for height := int64(1); err == nil; height++ {
		var rintbuf uint32
		err = binary.Read(dr, binary.LittleEndian, &rintbuf)
		if err == io.EOF {
			// hit end of file at expected offset: no warning
			height--
			err = nil
			break
		}
		if err != nil {
			break
		}
		if rintbuf != uint32(network) {
			break
		}
		err = binary.Read(dr, binary.LittleEndian, &rintbuf)
		blocklen := rintbuf

		rbytes := make([]byte, blocklen)

		// read block
		dr.Read(rbytes)

		block, err = jaxutil.NewBlockFromBytes(chainCtx, rbytes)
		if err != nil {
			return
		}
		blocks = append(blocks, block)
	}

	return
}

// chainSetup is used to create a new db and chain instance with the genesis
// block already inserted.  In addition to the new chain instance, it returns
// a teardown function the caller should invoke when done testing to clean up.
func chainSetup(dbName string, chainCtx chainctx.IChainCtx) (*BlockChain, func(), error) {
	if !isSupportedDbType(testDbType) {
		return nil, nil, fmt.Errorf("unsupported db type %v", testDbType)
	}

	// Handle memory database specially since it doesn't need the disk
	// specific handling.
	var db database.DB
	var teardown func()
	if testDbType == "memdb" {
		ndb, err := database.Create(testDbType, chainCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating db: %v", err)
		}
		db = ndb

		// Setup a teardown function for cleaning up.  This function is
		// returned to the caller to be invoked when it is done testing.
		teardown = func() {
			db.Close()
		}
	} else {
		// Create the root directory for test databases.
		if !fileExists(testDbRoot) {
			if err := os.MkdirAll(testDbRoot, 0700); err != nil {
				err := fmt.Errorf("unable to create test db "+
					"root: %v", err)
				return nil, nil, err
			}
		}

		// Create a new database to store the accepted blocks into.
		dbPath := filepath.Join(testDbRoot, dbName)
		_ = os.RemoveAll(dbPath)
		ndb, err := database.Create(testDbType, chainCtx, dbPath)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating db: %v", err)
		}
		db = ndb

		// Setup a teardown function for cleaning up.  This function is
		// returned to the caller to be invoked when it is done testing.
		teardown = func() {
			db.Close()
			os.RemoveAll(dbPath)
			os.RemoveAll(testDbRoot)
		}
	}

	// Copy the chain params to ensure any modifications the tests do to
	// the chain parameters do not affect the global instance.
	paramsCopy := *chainCtx.Params()

	// Create the main chain instance.
	chain, err := New(&Config{
		DB:          db,
		ChainCtx:    chainCtx,
		BlockGen:    newBeaconBlockGen(),
		ChainParams: &paramsCopy,
		Checkpoints: nil,
		TimeSource:  chaindata.NewMedianTime(),
		SigCache:    txscript.NewSigCache(1000),
	})
	if err != nil {
		teardown()
		err := fmt.Errorf("failed to create chain instance: %v", err)
		return nil, nil, err
	}
	return chain, teardown, nil
}

// loadUtxoView returns a utxo view loaded from a file.
func loadUtxoView(filename string) (*chaindata.UtxoViewpoint, error) {
	// The utxostore file format is:
	// <tx hash><output index><serialized utxo len><serialized utxo>
	//
	// The output index and serialized utxo len are little endian uint32s
	// and the serialized utxo uses the format described in chainio.go.

	filename = filepath.Join("testdata", filename)
	fi, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	// Choose read based on whether the file is compressed or not.
	var r io.Reader
	if strings.HasSuffix(filename, ".bz2") {
		r = bzip2.NewReader(fi)
	} else {
		r = fi
	}
	defer fi.Close()

	view := chaindata.NewUtxoViewpoint(true)
	for {
		// Hash of the utxo entry.
		var hash chainhash.Hash
		_, err := io.ReadAtLeast(r, hash[:], len(hash[:]))
		if err != nil {
			// Expected EOF at the right offset.
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Output index of the utxo entry.
		var index uint32
		err = binary.Read(r, binary.LittleEndian, &index)
		if err != nil {
			return nil, err
		}

		// Num of serialized utxo entry bytes.
		var numBytes uint32
		err = binary.Read(r, binary.LittleEndian, &numBytes)
		if err != nil {
			return nil, err
		}

		// Serialized utxo entry.
		serialized := make([]byte, numBytes)
		_, err = io.ReadAtLeast(r, serialized, int(numBytes))
		if err != nil {
			return nil, err
		}

		// Deserialize it and add it to the view.
		entry, err := chaindata.DeserializeUtxoEntry(serialized)
		if err != nil {
			return nil, err
		}
		view.Entries()[wire.OutPoint{Hash: hash, Index: index}] = entry
	}

	return view, nil
}

// TstSetCoinbaseMaturity makes the ability to set the coinbase maturity
// available when running tests.
func (b *BlockChain) TstSetCoinbaseMaturity(maturity uint16) {
	b.Chain().Params().CoinbaseMaturity = maturity
}

// newFakeChain returns a chain that is usable for syntetic tests.  It is
// important to note that this chain has no database associated with it, so
// it is not usable with all functions and the tests must take care when making
// use of it.
func newFakeChain(ctx chainctx.IChainCtx) *BlockChain {
	// Create a genesis block node and block index index populated with it
	// for use when creating the fake chain below.
	node := ctx.NewNode(ctx.GenesisBlock().Header, nil, 0)

	index := newBlockIndex(nil, ctx.Params())
	index.AddNode(node)

	targetTimespan := int64(ctx.Params().PowParams.TargetTimespan / time.Second)
	adjustmentFactor := ctx.Params().PowParams.RetargetAdjustmentFactor
	return &BlockChain{
		chain:      ctx,
		blockGen:   newBeaconBlockGen(),
		TimeSource: chaindata.NewMedianTime(),
		retargetOpts: retargetOpts{
			minRetargetTimespan: targetTimespan / adjustmentFactor,
			maxRetargetTimespan: targetTimespan * adjustmentFactor,
			blocksPerRetarget:   int32(chaincfg.BeaconEpochLength),
		},
		blocksDB: rBlockStorage{
			index:     index,
			bestChain: newChainView(node),
		},
		warningCaches:    newThresholdCaches(vbNumBits),
		deploymentCaches: newThresholdCaches(chaincfg.DefinedDeployments),
	}
}

// newFakeNode creates a block node connected to the passed parent with the
// provided fields populated and fake values for the other fields.
func newFakeNode(ctx chainctx.IChainCtx, parent blocknodes.IBlockNode, blockVersion wire.BVersion, bits uint32, timestamp time.Time) blocknodes.IBlockNode {
	// Make up a header and create a block node from it.
	header := wire.NewBeaconBlockHeader(blockVersion,
		parent.BlocksMMRRoot(), // todo: this is incorrect
		chainhash.ZeroHash,
		chainhash.ZeroHash,
		timestamp,
		bits,
		0,
	)

	return ctx.NewNode(header, parent, 0)
}

func newBeaconBlockGen() chaindata.ChainBlockGenerator {
	minerAddr, _ := jaxutil.DecodeAddress("mxQsksaTJb11i7vSxAUL6VBjoQnhP3bfFz", &chaincfg.TestNetParams)
	return chaindata.NewBeaconBlockGen(
		chaindata.StateProvider{
			ShardCount: func() (uint32, error) { return 0, nil },
			BTCGen:     &chaindata.BTCBlockGen{MinerAddress: minerAddr},
		})
}
