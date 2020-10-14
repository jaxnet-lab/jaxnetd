package p2p

import (
	"crypto/rand"
	"encoding/binary"
	"math"
	"sort"
	"sync"

	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
)

// checkpointSorter implements sort.Interface to allow a slice of checkpoints to
// be sorted.
type checkpointSorter []chaincfg.Checkpoint

// Len returns the number of checkpoints in the slice.  It is part of the
// sort.Interface implementation.
func (s checkpointSorter) Len() int {
	return len(s)
}

// Swap swaps the checkpoints at the passed indices.  It is part of the
// sort.Interface implementation.
func (s checkpointSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns whether the checkpoint with index i should sort before the
// checkpoint with index j.  It is part of the sort.Interface implementation.
func (s checkpointSorter) Less(i, j int) bool {
	return s[i].Height < s[j].Height
}

// mergeCheckpoints returns two slices of checkpoints merged into one slice
// such that the checkpoints are sorted by height.  In the case the additional
// checkpoints contain a checkpoint with the same height as a checkpoint in the
// default checkpoints, the additional checkpoint will take precedence and
// overwrite the default one.
func mergeCheckpoints(defaultCheckpoints, additional []chaincfg.Checkpoint) []chaincfg.Checkpoint {
	// Create a map of the additional checkpoints to remove duplicates while
	// leaving the most recently-specified checkpoint.
	extra := make(map[int32]chaincfg.Checkpoint)
	for _, checkpoint := range additional {
		extra[checkpoint.Height] = checkpoint
	}

	// Add all default checkpoints that do not have an override in the
	// additional checkpoints.
	numDefault := len(defaultCheckpoints)
	checkpoints := make([]chaincfg.Checkpoint, 0, numDefault+len(extra))
	for _, checkpoint := range defaultCheckpoints {
		if _, exists := extra[checkpoint.Height]; !exists {
			checkpoints = append(checkpoints, checkpoint)
		}
	}

	// Append the additional checkpoints and return the sorted results.
	for _, checkpoint := range extra {
		checkpoints = append(checkpoints, checkpoint)
	}
	sort.Sort(checkpointSorter(checkpoints))
	return checkpoints
}

// on the count n.
func pickNoun(n uint64, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// randomUint16Number returns a random uint16 in a specified input range.  Note
// that the range is in zeroth ordering; if you pass it 1800, you will get
// values from 0 to 1800.
func randomUint16Number(max uint16) uint16 {
	// In order to avoid modulo bias and ensure every possible outcome in
	// [0, max) has equal probability, the random number must be sampled
	// from a random source that has a range limited to a multiple of the
	// modulus.
	var randomNumber uint16
	var limitRange = (math.MaxUint16 / max) * max
	for {
		binary.Read(rand.Reader, binary.LittleEndian, &randomNumber)
		if randomNumber < limitRange {
			return (randomNumber % max)
		}
	}
}

type ChainPort struct {
	ShardID uint32
	Port    int
	NetName string
}

type ChainsPortIndex struct {
	mutex       sync.RWMutex
	shardsPorts map[uint32]int
}

func NewPortsIndex() *ChainsPortIndex {
	return &ChainsPortIndex{
		mutex:       sync.RWMutex{},
		shardsPorts: map[uint32]int{},
	}
}

func (ind *ChainsPortIndex) Add(shardID uint32, port int) {
	ind.mutex.Lock()
	ind.shardsPorts[shardID] = port
	ind.mutex.Unlock()
}

func (ind *ChainsPortIndex) Get(shardID uint32) (int, bool) {
	ind.mutex.RLock()
	port, ok := ind.shardsPorts[shardID]
	ind.mutex.RUnlock()
	return port, ok

}

func (ind *ChainsPortIndex) Delete(shardID uint32) {
	ind.mutex.Lock()
	delete(ind.shardsPorts, shardID)
	ind.mutex.Unlock()
}
