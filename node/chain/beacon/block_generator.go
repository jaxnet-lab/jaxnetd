/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package beacon

import (
	"time"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const (
	// baseSubsidy is the starting subsidy amount for mined blocks.  This
	// value is halved every SubsidyHalvingInterval blocks.
	baseSubsidy = 20
)

type StateProvider struct {
	ShardCount func() (uint32, error)
	BTCGen     btcGen
}

type BlockGenerator struct {
	stateInfo StateProvider
}

type btcGen interface {
	NewBlockTemplate(burnReward int, beaconHash chainhash.Hash) (wire.BTCBlockAux, bool, error)
}

func NewChainBlockGenerator(stateInfo StateProvider) *BlockGenerator {
	return &BlockGenerator{
		stateInfo: stateInfo,
	}
}

func (c *BlockGenerator) NewBlockHeader(version wire.BVersion, mmrRoot, merkleRootHash chainhash.Hash,
	timestamp time.Time, bits, nonce uint32, burnReward int) (wire.BlockHeader, error) {

	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	header := wire.NewBeaconBlockHeader(
		version,
		mmrRoot,
		merkleRootHash,
		chainhash.Hash{},
		timestamp,
		bits,
		nonce,
	)

	count, err := c.stateInfo.ShardCount()
	if err != nil {
		// an error will occur if it is impossible
		// to get the last block from the chain state
		return header, err
	}

	header.SetShards(count)

	if version.ExpansionMade() {
		header.SetShards(count + 1)
	}

	header.SetK(pow.PackK(pow.K1))
	header.SetVoteK(pow.PackK(pow.K1))

	aux, full, err := c.stateInfo.BTCGen.NewBlockTemplate(burnReward, header.BeaconExclusiveHash())
	if err != nil {
		return nil, errors.Wrap(err, "unable to generate btc block aux")
	}
	if !full {
		aux.Version = header.Version().Version()
		aux.Bits = header.Bits()
		aux.Nonce = header.Nonce()
	}
	header.SetBTCAux(aux)

	return header, nil
}

func (c *BlockGenerator) ValidateBlockHeader(_ wire.BlockHeader) error {
	return nil
}

func (c *BlockGenerator) ValidateCoinbaseTx(block *wire.MsgBlock, height int32) error {
	_, err := mining.ValidateBeaconCoinbase(block.Header.BeaconHeader(), block.Transactions[0], calcBlockSubsidy(height))
	return err
}

func (c *BlockGenerator) AcceptBlock(wire.BlockHeader) error {
	return nil
}

// CalcBlockSubsidy returns the subsidy amount a block at the provided height
// should have. This is mainly used for determining how much the coinbase for
// newly generated blocks awards as well as validating the coinbase for blocks
// has the expected value.
//
// | Year | First Block | Last block | Formula based on block number         | First Block reward | Last block reward |
// |------|-------------|------------|---------------------------------------|--------------------|-------------------|
// | 1    | 1           | 49152      | `340-10*([(x-1+3*2^10)/(3*2^11])`     | 340                | 260               |
// | 2    | 49153       | 98304      | `260-5*([(x-49153+3*2^10)/(3*2^11])`  | 260                | 220               |
// | 3    | 98305       | 147456     | `220-15*([(x-98305+3*2^10)/(3*2^11])` | 220                | 100               |
// | 4    | 147457      | 196608     | `100-5*([(x-147157+3*2^10)/(3*2^11])` | 100                | 60                |
// | 5    | 196609      | 245760     | `60-5*([(x-196609+3*2^10)/(3*2^11])`  | 60                 | 20                |
// | 6+   | 245761      |            | 20                                    | 20                 |                   |
func (c *BlockGenerator) CalcBlockSubsidy(height int32, genesisBits uint32, header wire.BlockHeader) int64 {
	return calcBlockSubsidy(height)
}

func calcBlockSubsidy(height int32) int64 {
	const (
		pow10      = 3072 // 3*2^10
		pow11      = 6144 // 3*2^11
		endOfEpoch = 49152
	)
	x := int64(height)

	switch {

	// Year 1
	case height >= 0 && height <= endOfEpoch:
		return (340 - 10*((x-1+pow10)/pow11)) * jaxutil.HaberStornettaPerJAXNETCoin

	// Year 2
	case height > endOfEpoch && height <= endOfEpoch*2:
		return (260 - 5*((x-endOfEpoch-1+pow10)/pow11)) * jaxutil.HaberStornettaPerJAXNETCoin

	// Year 3
	case height > endOfEpoch*2 && height <= endOfEpoch*3:
		return (220 - 15*((x-(endOfEpoch*2+1)+pow10)/pow11)) * jaxutil.HaberStornettaPerJAXNETCoin

	// Year 4
	case height > endOfEpoch*3 && height <= endOfEpoch*4:
		return (100 - 5*((x-(endOfEpoch*3+1)+pow10)/pow11)) * jaxutil.HaberStornettaPerJAXNETCoin

	// Year 5
	case height > endOfEpoch*4 && height <= endOfEpoch*5:
		return (60 - 5*((x-(endOfEpoch*4+1)+pow10)/pow11)) * jaxutil.HaberStornettaPerJAXNETCoin

	// Year 6+
	case height > endOfEpoch*5:
		return baseSubsidy * jaxutil.HaberStornettaPerJAXNETCoin
	default:
		return baseSubsidy * jaxutil.HaberStornettaPerJAXNETCoin
	}

}
