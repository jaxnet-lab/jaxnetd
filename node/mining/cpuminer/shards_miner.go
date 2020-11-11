/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package cpuminer

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
)

type MultiMiner struct {
	sync.Mutex
	log         zerolog.Logger
	wg          sync.WaitGroup
	shardMiners map[string]*CPUMiner
}

func NewMiner(cfgMap map[string]*Config, log zerolog.Logger) *MultiMiner {
	miner := &MultiMiner{
		shardMiners: map[string]*CPUMiner{},
		log:         log,
	}

	for i, i2 := range cfgMap {
		miner.shardMiners[i] = New(i2, log.With().Str("chain", i).Logger())
	}
	return miner

}
func (miner *MultiMiner) AddChainMiner(name string, config *Config) {
	miner.Lock()
	miner.shardMiners[name] = New(config, miner.log.With().Str("chain", name).Logger())
	miner.shardMiners[name].Start()
	miner.Unlock()

}

func (miner *MultiMiner) Run(ctx context.Context) {
	miner.Lock()
	for i := range miner.shardMiners {
		miner.shardMiners[i].Start()
	}
	miner.Unlock()

	<-ctx.Done()

	for i := range miner.shardMiners {
		name := i

		miner.wg.Add(1)
		go func() {
			miner.shardMiners[name].Stop()
			miner.wg.Done()
		}()
	}

	miner.wg.Wait()
}
