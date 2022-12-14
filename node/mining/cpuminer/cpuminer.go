// Copyright (c) 2014-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cpuminer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	mergedMiningTree "gitlab.com/jaxnet/jaxnetd/types/merge_mining_tree"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
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

// defaultNumWorkers is the default number of workers to use for mining
// and is based on the number of processor cores.  This helps ensure the
// system stays reasonably responsive under heavy load.
var defaultNumWorkers = uint32(1) // uint32(runtime.NumCPU())

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
	MiningAddrs []jaxutil.Address

	// ProcessBlock defines the function to call with any solved blocks.
	// It typically must run the provided block through the same set of
	// rules and handling as any other block coming from the network.
	ProcessBlock func(*jaxutil.Block, chaindata.BehaviorFlags) (bool, error)

	// IsCurrent defines the function to use to obtain whether or not the
	// block chain is current.  This is used by the automatic persistent
	// mining routine to determine whether or it should attempt mining.
	// This is useful because there is no point in mining if the chain is
	// not current since any solved blocks would be on a side chain and and
	// up orphaned anyways.
	IsCurrent func() bool

	AutominingEnabled   bool
	AutominingThreshold int32
}

// CPUMiner provides facilities for solving blocks (mining) using the CPU in
// a concurrency-safe manner.  It consists of two main goroutines -- a speed
// monitor and a controller for worker goroutines which generate and solve
// blocks.  The number of goroutines can be set via the SetMaxGoRoutines
// function, but the default is based on the number of processor cores in the
// system which is typically sufficient.
type CPUMiner struct {
	sync.Mutex

	beacon      Config
	shards      map[uint32]Config
	miningAddrs jaxutil.Address

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
	log               zerolog.Logger

	autominingEnabled   bool
	autominingThreshold int32
}

const hashesInKilohash = 1_000

// speedMonitor handles tracking the number of hashes per second the mining
// process is performing.  It must be run as a goroutine.
func (miner *CPUMiner) speedMonitor() {
	miner.log.Debug().Msg("CPU miner speed monitor started")

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
				miner.log.Debug().Msgf("Hash speed: %6.0f kilohashes/s", hashesPerSec/hashesInKilohash)
			}

		// Request for the number of hashes per second.
		case miner.queryHashesPerSec <- hashesPerSec:
			// Nothing to do.

		case <-miner.speedMonitorQuit:
			break out
		}
	}

	miner.wg.Done()
	miner.log.Debug().Msg("CPU miner speed monitor done")
}

// submitBlock submits the passed block to network after ensuring it passes all
// of the consensus validation rules.
func (miner *CPUMiner) submitBlock(chainID uint32, block *jaxutil.Block) bool {
	miner.submitBlockLock.Lock()
	defer miner.submitBlockLock.Unlock()

	if chainID == 0 {
		curHeight := miner.beacon.BlockTemplateGenerator.BestSnapshot().Height
		if curHeight != 0 && !miner.beacon.IsCurrent() {
			miner.log.Warn().Msgf("Beacon is not in sync state, skip submission")
			return true
		}
	} else {
		curHeight := miner.shards[chainID].BlockTemplateGenerator.BestSnapshot().Height
		if curHeight != 0 && !miner.shards[chainID].IsCurrent() {
			miner.log.Warn().Uint32("chain_id", chainID).
				Msgf("Shard is not in sync state, skip submission")
			return true
		}
	}

	// Ensure the block is not stale since a new block could have shown up
	// while the solution was being found.  Typically that condition is
	// detected and all work on the stale block is halted to start work on
	// a new block, but the check only happens periodically, so it is
	// possible a block was found and submitted in between.
	msgBlock := block.MsgBlock()
	merkleMountainRoot := msgBlock.Header.PrevBlocksMMRRoot()

	var bestMMR chainhash.Hash
	var bestHeight int32
	if chainID == 0 {
		bestMMR = miner.beacon.BlockTemplateGenerator.BestSnapshot().CurrentMMRRoot
		bestHeight = miner.beacon.BlockTemplateGenerator.BestSnapshot().Height
	} else {
		bestMMR = miner.shards[chainID].BlockTemplateGenerator.BestSnapshot().CurrentMMRRoot
		bestHeight = miner.shards[chainID].BlockTemplateGenerator.BestSnapshot().Height
	}

	if !merkleMountainRoot.IsEqual(&bestMMR) {
		miner.log.Debug().Msgf("Block submitted via CPU miner with previous block %s is stale",
			msgBlock.Header.PrevBlocksMMRRoot())
		return false
	}

	// Process this block using the same rules as blocks coming from other
	// nodes.  This will in turn relay it to the network like normal.
	var err error
	var isOrphan bool

	if chainID == 0 {
		isOrphan, err = miner.beacon.ProcessBlock(block, chaindata.BFNone)
	} else {
		isOrphan, err = miner.shards[chainID].ProcessBlock(block, chaindata.BFNone)
	}

	if err != nil {
		// Anything other than a rule violation is an unexpected error,
		// so log that error as an internal error.
		// miner.log.Error().Err(err).Msg("Unexpected error while processingblock submitted via CPU miner")
		miner.log.Error().Err(err).Uint32("chain_id", chainID).Msg("Block submitted via CPU miner rejected")
		// miner.log.Debug().Err(err).Msg("Block submitted via CPU miner rejected")
		return false
	}
	if isOrphan {
		miner.log.Debug().Uint32("chain_id", chainID).Msg("Block submitted via CPU miner is an orphan")
		return false
	}

	// The block was accepted.
	reward := block.MsgBlock().Transactions[0].TxOut[1].Value
	if chainID == 0 {
		reward += block.MsgBlock().Transactions[0].TxOut[2].Value
	}

	miner.log.Info().Uint32("chain_id", chainID).
		Stringer("hash", block.Hash()).
		Stringer("pow_hash", block.PowHash()).
		Stringer("amount", jaxutil.IAmountVal(reward, chainID == 0)).
		Int32("height", bestHeight+1).
		Msgf("Block submitted via CPU miner accepted")
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
func (miner *CPUMiner) solveBlock(job *miningJob,
	ticker *time.Ticker, quit chan struct{}) bool {
	// Choose a random extra nonce offset for this block template and
	// worker.
	// enOffset, err := encoder.RandomUint64()
	// if err != nil {
	// 	miner.log.Error().Err(err).Msg("Unexpected error while generating random extra nonce offset")
	// 	enOffset = 0
	// }
	enOffset := uint64(0)

	// Create some convenience variables.
	targetDifficulty := pow.CompactToBig(job.beacon.block.Header.Bits())

	// Initial state.
	lastGenerated := time.Now()
	lastTxUpdate := miner.beacon.BlockTemplateGenerator.TxSource().LastUpdated()
	hashesCompleted := uint64(0)

	// Note that the entire extra nonce range is iterated and the offset is
	// added relying on the fact that overflow will wrap around 0 as
	// provided by the Go spec.
	for extraNonce := uint64(0); extraNonce < maxExtraNonce; extraNonce++ {
		// Update the extra nonce in the block template with the
		// new value by regenerating the coinbase script and
		// setting the merkle root to the new value.

		block, beaconCoinbaseAux, _ := updateBeaconExtraNonce(job.beacon.block,
			int64(job.beacon.blockHeight), extraNonce+enOffset)

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
				best := miner.beacon.BlockTemplateGenerator.BestSnapshot()
				h := block.Header.PrevBlocksMMRRoot()
				if !(&h).IsEqual(&best.CurrentMMRRoot) {
					return false
				}

				// The current block is stale if the memory pool
				// has been updated since the block template was
				// generated and it has been at least one
				// minute.
				if lastTxUpdate != miner.beacon.BlockTemplateGenerator.TxSource().LastUpdated() &&
					time.Now().After(lastGenerated.Add(time.Minute)) {

					return false
				}

				if err := miner.beacon.BlockTemplateGenerator.UpdateBlockTime(&job.beacon.block); err != nil {
					miner.log.Error().Err(err).Msg("cannot update block time")
				}

			default:
				// Non-blocking select to fall through
			}

			// Update the nonce and hash the block header.  Each
			// hash is actually a double sha256 (two hashes), so
			// increment the number of hashes completed for each
			// attempt accordingly.
			job.beacon.block.Header.SetNonce(i)
			hash := job.beacon.block.Header.BeaconHeader().PoWHash()
			hashesCompleted += 2

			atLeastOneMined := false
			// The block is solved when the new block hash is less
			// than the target difficulty.  Yay!

			if pow.HashToBig(&hash).Cmp(targetDifficulty) <= 0 {
				if miner.beacon.ChainParams.PowParams.HashSorting {
					if pow.ValidateHashSortingRule(pow.HashToBig(&hash),
						miner.beacon.ChainParams.PowParams.HashSortingSlotNumber, 0) {
						atLeastOneMined = true
						miner.updateHashes <- hashesCompleted
						job.beacon.notSolved = false
					}
				} else {
					atLeastOneMined = true
					miner.updateHashes <- hashesCompleted
					job.beacon.notSolved = false

				}
			}

			for shardID, task := range job.shards {
				targetDifficulty := pow.CompactToBig(task.block.Header.Bits())

				if pow.HashToBig(&hash).Cmp(targetDifficulty) > 0 {
					continue
				}

				if miner.beacon.ChainParams.PowParams.HashSorting {
					if pow.ValidateHashSortingRule(pow.HashToBig(&hash),
						miner.beacon.ChainParams.PowParams.HashSortingSlotNumber, shardID) {

						atLeastOneMined = true
						task.notSolved = false

						task.block.Header.SetBeaconHeader(job.beacon.block.Header.BeaconHeader(), beaconCoinbaseAux)
						job.shards[shardID] = task

					}
				} else {
					atLeastOneMined = true
					task.notSolved = false
					task.block.Header.SetBeaconHeader(job.beacon.block.Header.BeaconHeader(), beaconCoinbaseAux)
					job.shards[shardID] = task
				}
			}

			if atLeastOneMined {
				return true
			}
		}
	}

	return false
}

func updateBeaconExtraNonce(beaconBlock wire.MsgBlock, height int64, extraNonce uint64) (wire.MsgBlock, wire.CoinbaseAux, error) {
	bh := beaconBlock.Header.BeaconHeader().BeaconExclusiveHash()
	coinbaseScript, err := chaindata.BTCCoinbaseScript(height, packUint64LE(extraNonce), bh.CloneBytes())
	if err != nil {
		return wire.MsgBlock{}, wire.CoinbaseAux{}, err
	}

	beaconBlock.Header.UpdateCoinbaseScript(coinbaseScript)
	aux := wire.CoinbaseAux{}.FromBlock(&beaconBlock, true) // todo:

	// updateMerkleRoot(&beaconBlock)

	return beaconBlock, aux, nil
}

func packUint64LE(n uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, n)
	return b
}

const mineAfterAutominingCheck = 10

// generateBlocks is a worker that is controlled by the miningWorkerController.
// It is self contained in that it creates block templates and attempts to solve
// them while detecting when it is performing stale work and reacting
// accordingly by generating a new block template.  When a block is solved, it
// is submitted.
//
// It must be run as a goroutine.
// nolint: gomnd
func (miner *CPUMiner) generateBlocks(quit chan struct{}) {
	// Start a ticker which is used to signal checks for stale work and
	// updates to the speed monitor.
	ticker := time.NewTicker(time.Second * hashUpdateSecs)
	defer ticker.Stop()

	job := new(miningJob)
	job.beacon = chainTask{}
	job.shards = make(map[uint32]chainTask, len(miner.shards))
	time.Sleep(5 * time.Second)

	var toMineWithoutCheck int
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
		// if miner.beacon.ConnectedCount() == 0 {
		// 	time.Sleep(time.Second)
		// 	continue
		// }

		// No point in searching for a solution before the chain is
		// synced.  Also, grab the same lock as used for block
		// submission, since the current block will be changing and
		// this would otherwise end up building a new block template on
		// a block that is in the process of becoming stale.
		miner.submitBlockLock.Lock()

		needToSleep := miner.updateTasks(job)
		if needToSleep {
			miner.submitBlockLock.Unlock()
			time.Sleep(time.Second)
			continue
		}

		if toMineWithoutCheck > 0 {
			toMineWithoutCheck--
		}

		if miner.autominingEnabled && haveEnoughBlocks(job, miner.autominingThreshold) && toMineWithoutCheck == 0 {
			if !haveEnoughTxs(job) {
				miner.submitBlockLock.Unlock()
				time.Sleep(10 * time.Second)
				continue
			}
			toMineWithoutCheck = mineAfterAutominingCheck
		}

		curHeight := miner.beacon.BlockTemplateGenerator.BestSnapshot().Height
		if curHeight != 0 && !miner.beacon.IsCurrent() {
			miner.submitBlockLock.Unlock()
			time.Sleep(time.Second)
			continue
		}
		miner.submitBlockLock.Unlock()

		// Attempt to solve the block.  The function will exit early
		// with false when conditions that trigger a stale block, so
		// a new block template can be generated.  When the return is
		// true a solution was found, so submit the solved block.
		if miner.solveBlock(job, ticker, quit) {
			miner.submitTask(job)
		}
	}

	miner.workerWg.Done()
	miner.log.Debug().Msg("Generate blocks worker done")
}

func (miner *CPUMiner) updateTasks(job *miningJob) bool {
	curHeight := miner.beacon.BlockTemplateGenerator.BestSnapshot().Height
	if curHeight != 0 && !miner.beacon.IsCurrent() {
		return true
	}

	burnReward := types.BurnJaxReward
	burnJXN := (curHeight+1)%2 == 0
	if burnJXN {
		burnReward = types.BurnJaxNetReward
	}

	if job.beacon.blockHeight < curHeight+1 {
		template, err := miner.beacon.BlockTemplateGenerator.NewBlockTemplate(miner.miningAddrs, burnReward)
		if err != nil {
			miner.log.Error().Err(err).Msg("Failed to create new beacon block template")
			return false
		}

		job.beacon = chainTask{
			blockHeight: curHeight + 1,
			block:       *template.Block,
			notSolved:   true,
			burnReward:  burnReward,
		}
	}

	for shardID := range miner.shards {
		curHeight := miner.shards[shardID].BlockTemplateGenerator.BestSnapshot().Height
		if curHeight != 0 && !miner.beacon.IsCurrent() {
			return true
		}

		needToRefillBlock := job.shards[shardID].submitted && job.shards[shardID].burnReward != job.beacon.burnReward

		if needToRefillBlock || job.shards[shardID].blockHeight < curHeight+1 {
			template, err := miner.shards[shardID].BlockTemplateGenerator.NewBlockTemplate(miner.miningAddrs, burnReward)
			if err != nil {
				miner.log.Error().Err(err).Uint32("shard_id", shardID).Msg("Failed to create new block shard template")
				return false
			}

			job.shards[shardID] = chainTask{
				blockHeight: curHeight + 1,
				block:       *template.Block,
				notSolved:   true,
				burnReward:  burnReward,
			}
		}
	}

	err := miner.updateMergedMiningProof(job)
	if err != nil {
		miner.log.Warn().Err(err).Msg("Failed to update merged mining proof")
	}

	return false
}

func (miner *CPUMiner) updateMergedMiningProof(job *miningJob) error {
	knownShardsCount := len(miner.shards)
	fetchedShardsCount := len(job.shards)

	if knownShardsCount == 0 || fetchedShardsCount == 0 {
		return nil
	}

	tree := mergedMiningTree.NewSparseMerkleTree(uint32(knownShardsCount))
	for id, shard := range job.shards {
		// Shard IDs are going to be indexed from 1,
		// but the tree expects slots to be indexed from 0.
		slotIndex := id - 1

		shardHeader, _ := shard.block.Header.(*wire.ShardHeader)
		shardBlockHash := shardHeader.ExclusiveHash()
		err := tree.SetShardHash(slotIndex, shardBlockHash)
		if err != nil {
			return err
		}
	}

	root, err := tree.Root()
	if err != nil {
		return err
	}

	rootHash, err := chainhash.NewHash(root[:])
	if err != nil {
		return err
	}

	coding, codingBitLength, err := tree.CatalanNumbersCoding()
	if err != nil {
		return err
	}

	hashes := tree.MarshalOrangeTreeLeafs()

	job.beacon.block.Header.BeaconHeader().SetMergeMiningRoot(*rootHash)
	job.beacon.block.Header.BeaconHeader().SetMergeMiningNumber(uint32(len(job.shards)))
	job.beacon.block.Header.BeaconHeader().SetMergedMiningTreeCodingProof(hashes, coding, codingBitLength)

	for id := range job.shards {
		// Shard IDs are going to be indexed from 1,
		// but the tree expects slots to be indexed from 0.
		slotIndex := id - 1
		path, err := tree.MerkleProofPath(slotIndex)
		if err != nil {
			return err
		}

		job.shards[id].block.Header.SetShardMerkleProof(path)
		job.shards[id].block.Header.BeaconHeader().SetMergeMiningRoot(*rootHash)
		job.shards[id].block.Header.BeaconHeader().SetMergeMiningNumber(uint32(len(job.shards)))
		job.shards[id].block.Header.BeaconHeader().SetMergedMiningTreeCodingProof(hashes, coding, codingBitLength)
	}

	return nil
}

func (miner *CPUMiner) submitTask(job *miningJob) {
	if !job.beacon.notSolved && !job.beacon.submitted {
		block := jaxutil.NewBlock(&job.beacon.block)
		job.beacon.submitted = miner.submitBlock(0, block)
	}

	for shardID := range job.shards {
		task := job.shards[shardID]
		if task.burnReward != job.beacon.burnReward {
			task.submitted = true
			continue
		}
		if task.notSolved || task.submitted {
			continue
		}

		block := jaxutil.NewBlock(&task.block)
		miner.submitBlock(shardID, block)
		task.submitted = true
		job.shards[shardID] = task
	}
}

type miningJob struct {
	beacon chainTask
	shards map[uint32]chainTask
}

type chainTask struct {
	blockHeight int32
	block       wire.MsgBlock
	notSolved   bool
	submitted   bool
	burnReward  int
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
			go miner.generateBlocks(quit)
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

func (miner *CPUMiner) Run(ctx context.Context, wg *sync.WaitGroup) {
	miner.Start()
	wg.Done()
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
	miner.log.Info().Msg("CPU miner started")
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
	miner.log.Info().Msg("CPU miner stopped")
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
// nolint: staticcheck, gomnd
func (miner *CPUMiner) GenerateNBlocks(quit chan struct{}, n uint32) ([]*chainhash.Hash, error) {
	miner.Lock()

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

	miner.log.Debug().Msgf("Generating %d blocks", n)

	i := uint32(0)
	blockHashes := make([]*chainhash.Hash, n)

	// Start a ticker which is used to signal checks for stale work and
	// updates to the speed monitor.
	ticker := time.NewTicker(time.Second * hashUpdateSecs)
	defer ticker.Stop()

	job := new(miningJob)
	job.beacon = chainTask{}
	job.shards = make(map[uint32]chainTask, len(miner.shards))
	time.Sleep(5 * time.Second)
out:
	for i < n {
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
		// if miner.beacon.ConnectedCount() == 0 {
		// 	time.Sleep(time.Second)
		// 	continue
		// }

		// No point in searching for a solution before the chain is
		// synced.  Also, grab the same lock as used for block
		// submission, since the current block will be changing and
		// this would otherwise end up building a new block template on
		// a block that is in the process of becoming stale.
		miner.submitBlockLock.Lock()

		needToSleep := miner.updateTasks(job)
		if needToSleep {
			miner.submitBlockLock.Unlock()
			time.Sleep(time.Second)
			continue
		}

		curHeight := miner.beacon.BlockTemplateGenerator.BestSnapshot().Height
		if curHeight != 0 && !miner.beacon.IsCurrent() {
			miner.submitBlockLock.Unlock()
			time.Sleep(time.Second)
			continue
		}
		miner.submitBlockLock.Unlock()

		// Attempt to solve the block.  The function will exit early
		// with false when conditions that trigger a stale block, so
		// a new block template can be generated.  When the return is
		// true a solution was found, so submit the solved block.
		if miner.solveBlock(job, ticker, quit) {
			miner.submitTask(job)

			if !job.beacon.notSolved {
				hash := job.beacon.block.BlockHash()
				blockHashes[i] = &hash
			}

			for _, shardJob := range job.shards {
				hash := shardJob.block.BlockHash()
				if !shardJob.notSolved {
					blockHashes[i] = &hash
				}
			}
			i++
		}
	}
	miner.log.Debug().Msgf(fmt.Sprintf("Generated %d blocks", i))
	miner.Lock()
	close(miner.speedMonitorQuit)
	miner.wg.Wait()
	miner.started = false
	miner.discreteMining = false
	miner.Unlock()

	return blockHashes, nil
}

func (miner *CPUMiner) AddChain(shardID uint32, cfg Config) {
	miner.Lock()
	miner.shards[shardID] = cfg
	miner.Unlock()
}

// New returns a new instance of a CPU miner for the provided configuration.
// Use Start to begin the mining process.  See the documentation for CPUMiner
// type for more details.
func New(beacon Config, shards map[uint32]Config, address jaxutil.Address, log zerolog.Logger) *CPUMiner {
	return &CPUMiner{
		log:                 log,
		beacon:              beacon,
		shards:              shards,
		miningAddrs:         address,
		numWorkers:          defaultNumWorkers,
		updateNumWorkers:    make(chan struct{}),
		queryHashesPerSec:   make(chan float64),
		updateHashes:        make(chan uint64),
		autominingEnabled:   beacon.AutominingEnabled,
		autominingThreshold: beacon.AutominingThreshold,
	}
}

func haveEnoughBlocks(job *miningJob, threshold int32) bool {
	if job.beacon.blockHeight > threshold {
		return true
	}

	for _, shardJob := range job.shards {
		if shardJob.blockHeight > threshold {
			return true
		}
	}

	return false
}

func haveEnoughTxs(job *miningJob) bool {
	if len(job.beacon.block.Transactions) > 1 {
		return true
	}

	for _, shardJob := range job.shards {
		if len(shardJob.block.Transactions) > 1 {
			return true
		}
	}

	return false
}
