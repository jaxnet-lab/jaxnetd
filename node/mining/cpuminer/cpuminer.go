// Copyright (c) 2014-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cpuminer

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/minio/sha256-simd"
	"gitlab.com/jaxnet/core/shard.core/btcutil"
	"gitlab.com/jaxnet/core/shard.core/node/chaindata"
	"gitlab.com/jaxnet/core/shard.core/node/encoder"
	"gitlab.com/jaxnet/core/shard.core/node/mining"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/pow"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
	"go.uber.org/zap"
)

const (
	// maxNonce is the maximum value a nonce can be in a block header.
	maxNonce = ^uint32(0) // 2^32 - 1

	// maxExtraNonce is the maximum value an extra nonce used in a coinbase
	// transaction can be.
	maxExtraNonce = ^uint64(0) // 2^64 - 1

	// hpsUpdateSecs is the number of seconds to wait in between each
	// update to the hashes per second monitor.
	hpsUpdateSecs = 10

	// hashUpdateSec is the number of seconds each worker waits in between
	// notifying the speed monitor with how many hashes have been completed
	// while they are actively searching for a solution.  This is done to
	// reduce the amount of syncs between the workers that must be done to
	// keep track of the hashes per second.
	hashUpdateSecs = 15
)

var (
	// defaultNumWorkers is the default number of workers to use for mining
	// and is based on the number of processor cores.  This helps ensure the
	// system stays reasonably responsive under heavy load.
	defaultNumWorkers = uint32(1) // uint32(runtime.NumCPU())
)

// Config is a descriptor containing the cpu miner configuration.
type Config struct {
	// ChainParams identifies which chain parameters the cpu miner is
	// associated with.
	ChainParams *chaincfg.Params

	// BlockTemplateGenerator identifies the instance to use in order to
	// generate block templates that the miner will attempt to solve.
	BlockTemplateGenerator *mining.BlkTmplGenerator

	// MiningAddrs is a list of payment addresses to use for the generated
	// blocks.  Each generated block will randomly choose one of them.
	MiningAddrs []btcutil.Address

	// ProcessBlock defines the function to call with any solved blocks.
	// It typically must run the provided block through the same set of
	// rules and handling as any other block coming from the network.
	ProcessBlock func(*btcutil.Block, chaindata.BehaviorFlags) (bool, error)

	// ConnectedCount defines the function to use to obtain how many other
	// peers the server is connected to.  This is used by the automatic
	// persistent mining routine to determine whether or it should attempt
	// mining.  This is useful because there is no point in mining when not
	// connected to any peers since there would no be anyone to send any
	// found blocks to.
	ConnectedCount func() int32

	// IsCurrent defines the function to use to obtain whether or not the
	// block chain is current.  This is used by the automatic persistent
	// mining routine to determine whether or it should attempt mining.
	// This is useful because there is no point in mining if the chain is
	// not current since any solved blocks would be on a side chain and and
	// up orphaned anyways.
	IsCurrent func() bool
}

// CPUMiner provides facilities for solving blocks (mining) using the CPU in
// a concurrency-safe manner.  It consists of two main goroutines -- a speed
// monitor and a controller for worker goroutines which generate and solve
// blocks.  The number of goroutines can be set via the SetMaxGoRoutines
// function, but the default is based on the number of processor cores in the
// system which is typically sufficient.
type CPUMiner struct {
	sync.Mutex
	generator         *mining.BlkTmplGenerator
	cfg               Config
	numWorkers        uint32
	started           bool
	discreteMining    bool
	submitBlockLock   sync.Mutex
	wg                sync.WaitGroup
	workerWg          sync.WaitGroup
	updateNumWorkers  chan struct{}
	queryHashesPerSec chan float64
	updateHashes      chan uint64
	speedMonitorQuit  chan struct{}
	quit              chan struct{}
	log               *zap.Logger
}

// speedMonitor handles tracking the number of hashes per second the mining
// process is performing.  It must be run as a goroutine.
func (miner *CPUMiner) speedMonitor() {
	miner.log.Debug("CPU miner speed monitor started")

	var hashesPerSec float64
	var totalHashes uint64
	ticker := time.NewTicker(time.Second * hpsUpdateSecs)
	defer ticker.Stop()

out:
	for {
		select {
		// Periodic updates from the workers with how many hashes they
		// have performed.
		case numHashes := <-miner.updateHashes:
			totalHashes += numHashes

		// Time to update the hashes per second.
		case <-ticker.C:
			curHashesPerSec := float64(totalHashes) / hpsUpdateSecs
			if hashesPerSec == 0 {
				hashesPerSec = curHashesPerSec
			}
			hashesPerSec = (hashesPerSec + curHashesPerSec) / 2
			totalHashes = 0
			if hashesPerSec != 0 {
				miner.log.Debug(fmt.Sprintf("Hash speed: %6.0f kilohashes/s",
					hashesPerSec/1000))
			}

		// Request for the number of hashes per second.
		case miner.queryHashesPerSec <- hashesPerSec:
			// Nothing to do.

		case <-miner.speedMonitorQuit:
			break out
		}
	}

	miner.wg.Done()
	miner.log.Debug("CPU miner speed monitor done")
}

// submitBlock submits the passed block to network after ensuring it passes all
// of the consensus validation rules.
func (miner *CPUMiner) submitBlock(block *btcutil.Block) bool {
	miner.submitBlockLock.Lock()
	defer miner.submitBlockLock.Unlock()

	// Ensure the block is not stale since a new block could have shown up
	// while the solution was being found.  Typically that condition is
	// detected and all work on the stale block is halted to start work on
	// a new block, but the check only happens periodically, so it is
	// possible a block was found and submitted in between.
	msgBlock := block.MsgBlock()
	h := msgBlock.Header.PrevBlock()
	if !h.IsEqual(&miner.generator.BestSnapshot().Hash) {
		miner.log.Debug(fmt.Sprintf("Block submitted via CPU miner with previous block %s is stale",
			msgBlock.Header.PrevBlock))
		return false
	}

	// Process this block using the same rules as blocks coming from other
	// nodes.  This will in turn relay it to the network like normal.
	isOrphan, err := miner.cfg.ProcessBlock(block, chaindata.BFNone)
	if err != nil {
		// Anything other than a rule violation is an unexpected error,
		// so log that error as an internal error.
		if _, ok := err.(chaindata.RuleError); !ok {
			miner.log.Error("Unexpected error while processing "+
				"block submitted via CPU miner", zap.Error(err))
			return false
		}

		miner.log.Debug("Block submitted via CPU miner rejected", zap.Error(err))
		return false
	}
	if isOrphan {
		miner.log.Debug("Block submitted via CPU miner is an orphan")
		return false
	}

	// The block was accepted.
	coinbaseTx := block.MsgBlock().Transactions[0].TxOut[0]
	miner.log.Info(fmt.Sprintf("Block submitted via CPU miner accepted (hash %s, amount %v)",
		block.Hash(), btcutil.Amount(coinbaseTx.Value)))
	return true
}

// solveBlock attempts to find some combination of a nonce, extra nonce, and
// current timestamp which makes the passed block hash to a value less than the
// target difficulty.  The timestamp is updated periodically and the passed
// block is modified with all tweaks during this process.  This means that
// when the function returns true, the block is ready for submission.
//
// This function will return early with false when conditions that trigger a
// stale block such as a new block showing up or periodically when there are
// new transactions and enough time has elapsed without finding a solution.
func (miner *CPUMiner) solveBlock(msgBlock *wire.MsgBlock, blockHeight int32,
	ticker *time.Ticker, quit chan struct{}, worker uint32) bool {

	// Choose a random extra nonce offset for this block template and
	// worker.
	enOffset, err := encoder.RandomUint64()
	if err != nil {
		miner.log.Error("Unexpected error while generating random "+
			"extra nonce offset", zap.Error(err))
		enOffset = 0
	}

	// Create some convenience variables.
	header := msgBlock.Header
	targetDifficulty := pow.CompactToBig(header.Bits())

	// Initial state.
	lastGenerated := time.Now()
	lastTxUpdate := miner.generator.TxSource().LastUpdated()
	hashesCompleted := uint64(0)

	// Note that the entire extra nonce range is iterated and the offset is
	// added relying on the fact that overflow will wrap around 0 as
	// provided by the Go spec.
	for extraNonce := uint64(0); extraNonce < maxExtraNonce; extraNonce++ {
		// Update the extra nonce in the block template with the
		// new value by regenerating the coinbase script and
		// setting the merkle root to the new value.
		miner.generator.UpdateExtraNonce(msgBlock, blockHeight, extraNonce+enOffset)

		// fmt.Printf("BlockData %x (%d)\n", bd, len(bd))
		// Search through the entire nonce range for a solution while
		// periodically checking for early quit and stale block
		// conditions along with updates to the speed monitor.
		for i := uint32(0); i <= maxNonce; i++ {
			select {
			case <-quit:
				return false

			case <-ticker.C:
				miner.updateHashes <- hashesCompleted
				hashesCompleted = 0

				// The current block is stale if the best block
				// has changed.
				best := miner.generator.BestSnapshot()
				h := header.PrevBlock()
				if !(&h).IsEqual(&best.Hash) {
					return false
				}

				// The current block is stale if the memory pool
				// has been updated since the block template was
				// generated and it has been at least one
				// minute.
				if lastTxUpdate != miner.generator.TxSource().LastUpdated() &&
					time.Now().After(lastGenerated.Add(time.Minute)) {

					return false
				}

				miner.generator.UpdateBlockTime(msgBlock)

			default:
				// Non-blocking select to fall through
			}

			// Update the nonce and hash the block header.  Each
			// hash is actually a double sha256 (two hashes), so
			// increment the number of hashes completed for each
			// attempt accordingly.
			header.SetNonce(i)
			hash := header.BlockHash()
			hashesCompleted += 2

			// The block is solved when the new block hash is less
			// than the target difficulty.  Yay!
			if pow.HashToBig(&hash).Cmp(targetDifficulty) <= 0 {
				miner.updateHashes <- hashesCompleted
				return true
			}
		}
	}

	return false
}

func DoubleHashH(b []byte) chainhash.Hash {
	first := sha256.Sum256(b)
	return sha256.Sum256(first[:])
}

// generateBlocks is a worker that is controlled by the miningWorkerController.
// It is self contained in that it creates block templates and attempts to solve
// them while detecting when it is performing stale work and reacting
// accordingly by generating a new block template.  When a block is solved, it
// is submitted.
//
// It must be run as a goroutine.
func (miner *CPUMiner) generateBlocks(quit chan struct{}, worker uint32) {

	// Start a ticker which is used to signal checks for stale work and
	// updates to the speed monitor.
	ticker := time.NewTicker(time.Second * hashUpdateSecs)
	defer ticker.Stop()
out:
	for {
		// Quit when the miner is stopped.
		select {
		case <-quit:
			break out
		default:
			// Non-blocking select to fall through
		}

		// Wait until there is a connection to at least one other peer
		// since there is no way to relay a found block or receive
		// transactions to work on when there are no connected peers.
		if miner.cfg.ConnectedCount() == 0 {
			time.Sleep(time.Second)
			continue
		}

		// No point in searching for a solution before the chain is
		// synced.  Also, grab the same lock as used for block
		// submission, since the current block will be changing and
		// this would otherwise end up building a new block template on
		// a block that is in the process of becoming stale.
		miner.submitBlockLock.Lock()
		curHeight := miner.generator.BestSnapshot().Height
		if curHeight != 0 && !miner.cfg.IsCurrent() {
			miner.submitBlockLock.Unlock()
			time.Sleep(time.Second)
			continue
		}

		// Choose a payment address at random.
		rand.Seed(time.Now().UnixNano())
		// fmt.Println("miner.cfg.MiningAddrs", miner.cfg.MiningAddrs)
		// fmt.Println("rand.Intn(len(miner.cfg.MiningAddrs)) ", rand.Intn(len(miner.cfg.MiningAddrs)))
		payToAddr := miner.cfg.MiningAddrs[rand.Intn(len(miner.cfg.MiningAddrs))]

		// fmt.Println("payToAddr: ", payToAddr.String())

		// Create a new block template using the available transactions
		// in the memory pool as a source of transactions to potentially
		// include in the block.
		template, err := miner.generator.NewBlockTemplate(payToAddr)
		miner.submitBlockLock.Unlock()
		if err != nil {
			errStr := fmt.Sprintf("Failed to create new block "+
				"template: %v", err)
			miner.log.Error(errStr)
			continue
		}

		// Attempt to solve the block.  The function will exit early
		// with false when conditions that trigger a stale block, so
		// a new block template can be generated.  When the return is
		// true a solution was found, so submit the solved block.
		if miner.solveBlock(template.Block, curHeight+1, ticker, quit, worker) {
			block := btcutil.NewBlock(template.Block)
			miner.submitBlock(block)
		}
	}

	miner.workerWg.Done()
	miner.log.Debug("Generate blocks worker done")
}

// miningWorkerController launches the worker goroutines that are used to
// generate block templates and solve them.  It also provides the ability to
// dynamically adjust the number of running worker goroutines.
//
// It must be run as a goroutine.
func (miner *CPUMiner) miningWorkerController() {
	// launchWorkers groups common code to launch a specified number of
	// workers for generating blocks.
	var runningWorkers []chan struct{}
	launchWorkers := func(numWorkers uint32) {
		for i := uint32(0); i < numWorkers; i++ {
			quit := make(chan struct{})
			runningWorkers = append(runningWorkers, quit)

			miner.workerWg.Add(1)
			go miner.generateBlocks(quit, i)
		}
	}

	// Launch the current number of workers by default.
	runningWorkers = make([]chan struct{}, 0, miner.numWorkers)
	launchWorkers(miner.numWorkers)

out:
	for {
		select {
		// Update the number of running workers.
		case <-miner.updateNumWorkers:
			// No change.
			numRunning := uint32(len(runningWorkers))
			if miner.numWorkers == numRunning {
				continue
			}

			// Add new workers.
			if miner.numWorkers > numRunning {
				launchWorkers(miner.numWorkers - numRunning)
				continue
			}

			// Signal the most recently created goroutines to exit.
			for i := numRunning - 1; i >= miner.numWorkers; i-- {
				close(runningWorkers[i])
				runningWorkers[i] = nil
				runningWorkers = runningWorkers[:i]
			}

		case <-miner.quit:
			for _, quit := range runningWorkers {
				close(quit)
			}
			break out
		}
	}

	// Wait until all workers shut down to stop the speed monitor since
	// they rely on being able to send updates to it.
	miner.workerWg.Wait()
	close(miner.speedMonitorQuit)
	miner.wg.Done()
}

func (miner *CPUMiner) Run(ctx context.Context) {
	miner.Start()
	<-ctx.Done()
	miner.Stop()
}

// Start begins the CPU mining process as well as the speed monitor used to
// track hashing metrics.  Calling this function when the CPU miner has
// already been started will have no effect.
//
// This function is safe for concurrent access.
func (miner *CPUMiner) Start() {
	miner.Lock()
	defer miner.Unlock()

	// Nothing to do if the miner is already running or if running in
	// discrete mode (using GenerateNBlocks).
	if miner.started || miner.discreteMining {
		return
	}

	miner.quit = make(chan struct{})
	miner.speedMonitorQuit = make(chan struct{})
	miner.wg.Add(2)
	go miner.speedMonitor()
	go miner.miningWorkerController()

	miner.started = true
	miner.log.Info("CPU miner started")
}

// Stop gracefully stops the mining process by signalling all workers, and the
// speed monitor to quit.  Calling this function when the CPU miner has not
// already been started will have no effect.
//
// This function is safe for concurrent access.
func (miner *CPUMiner) Stop() {
	miner.Lock()
	defer miner.Unlock()

	// Nothing to do if the miner is not currently running or if running in
	// discrete mode (using GenerateNBlocks).
	if !miner.started || miner.discreteMining {
		return
	}

	close(miner.quit)
	miner.wg.Wait()
	miner.started = false
	miner.log.Info("CPU miner stopped")
}

// IsMining returns whether or not the CPU miner has been started and is
// therefore currenting mining.
//
// This function is safe for concurrent access.
func (miner *CPUMiner) IsMining() bool {
	miner.Lock()
	defer miner.Unlock()

	return miner.started
}

// HashesPerSecond returns the number of hashes per second the mining process
// is performing.  0 is returned if the miner is not currently running.
//
// This function is safe for concurrent access.
func (miner *CPUMiner) HashesPerSecond() float64 {
	miner.Lock()
	defer miner.Unlock()

	// Nothing to do if the miner is not currently running.
	if !miner.started {
		return 0
	}

	return <-miner.queryHashesPerSec
}

// SetNumWorkers sets the number of workers to create which solve blocks.  Any
// negative values will cause a default number of workers to be used which is
// based on the number of processor cores in the system.  A value of 0 will
// cause all CPU mining to be stopped.
//
// This function is safe for concurrent access.
func (miner *CPUMiner) SetNumWorkers(numWorkers int32) {
	if numWorkers == 0 {
		miner.Stop()
	}

	// Don't lock until after the first check since Stop does its own
	// locking.
	miner.Lock()
	defer miner.Unlock()

	// Use default if provided value is negative.
	if numWorkers < 0 {
		miner.numWorkers = defaultNumWorkers
	} else {
		miner.numWorkers = uint32(numWorkers)
	}

	// When the miner is already running, notify the controller about the
	// the change.
	if miner.started {
		miner.updateNumWorkers <- struct{}{}
	}
}

// NumWorkers returns the number of workers which are running to solve blocks.
//
// This function is safe for concurrent access.
func (miner *CPUMiner) NumWorkers() int32 {
	miner.Lock()
	defer miner.Unlock()

	return int32(miner.numWorkers)
}

// GenerateNBlocks generates the requested number of blocks. It is self
// contained in that it creates block templates and attempts to solve them while
// detecting when it is performing stale work and reacting accordingly by
// generating a new block template.  When a block is solved, it is submitted.
// The function returns a list of the hashes of generated blocks.
func (miner *CPUMiner) GenerateNBlocks(n uint32) ([]*chainhash.Hash, error) {
	miner.Lock()

	// Respond with an error if server is already mining.
	if miner.started || miner.discreteMining {
		miner.Unlock()
		return nil, errors.New(
			"server is already CPU mining. Please call `setgenerate 0` before calling discrete `generate` commands")
	}

	miner.started = true
	miner.discreteMining = true

	miner.speedMonitorQuit = make(chan struct{})
	miner.wg.Add(1)
	go miner.speedMonitor()

	miner.Unlock()

	miner.log.Debug(fmt.Sprintf("Generating %d blocks", n))

	i := uint32(0)
	blockHashes := make([]*chainhash.Hash, n)

	// Start a ticker which is used to signal checks for stale work and
	// updates to the speed monitor.
	ticker := time.NewTicker(time.Second * hashUpdateSecs)
	defer ticker.Stop()

	for {
		// Read updateNumWorkers in case someone tries a `setgenerate` while
		// we're generating. We can ignore it as the `generate` RPC call only
		// uses 1 worker.
		select {
		case <-miner.updateNumWorkers:
		default:
		}

		// Grab the lock used for block submission, since the current block will
		// be changing and this would otherwise end up building a new block
		// template on a block that is in the process of becoming stale.
		miner.submitBlockLock.Lock()
		curHeight := miner.generator.BestSnapshot().Height

		// Choose a payment address at random.
		rand.Seed(time.Now().UnixNano())
		payToAddr := miner.cfg.MiningAddrs[rand.Intn(len(miner.cfg.MiningAddrs))]

		// Create a new block template using the available transactions
		// in the memory pool as a source of transactions to potentially
		// include in the block.
		template, err := miner.generator.NewBlockTemplate(payToAddr)
		miner.submitBlockLock.Unlock()
		if err != nil {
			errStr := fmt.Sprintf("Failed to create new block "+
				"template: %v", err)
			miner.log.Error(errStr)
			continue
		}

		// Attempt to solve the block.  The function will exit early
		// with false when conditions that trigger a stale block, so
		// a new block template can be generated.  When the return is
		// true a solution was found, so submit the solved block.
		if miner.solveBlock(template.Block, curHeight+1, ticker, nil, 1000) {
			block := btcutil.NewBlock(template.Block)
			miner.submitBlock(block)
			blockHashes[i] = block.Hash()
			i++
			if i == n {
				miner.log.Debug(fmt.Sprintf("Generated %d blocks", i))
				miner.Lock()
				close(miner.speedMonitorQuit)
				miner.wg.Wait()
				miner.started = false
				miner.discreteMining = false
				miner.Unlock()
				return blockHashes, nil
			}
		}
	}
}

// New returns a new instance of a CPU miner for the provided configuration.
// Use Start to begin the mining process.  See the documentation for CPUMiner
// type for more details.
func New(cfg *Config, log *zap.Logger) *CPUMiner {
	return &CPUMiner{
		log:               log,
		generator:         cfg.BlockTemplateGenerator,
		cfg:               *cfg,
		numWorkers:        defaultNumWorkers,
		updateNumWorkers:  make(chan struct{}),
		queryHashesPerSec: make(chan float64),
		updateHashes:      make(chan uint64),
	}
}
