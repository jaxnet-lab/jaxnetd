// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package p2p

import (
	"crypto/rand"
	"encoding/binary"
	"math"
	"sync"

	"github.com/rs/zerolog/log"
)

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
	limitRange := (math.MaxUint16 / max) * max
	for {
		if err := binary.Read(rand.Reader, binary.LittleEndian, &randomNumber); err != nil {
			log.Error().Err(err).Msg("cannot read random number from reader")
		}
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
