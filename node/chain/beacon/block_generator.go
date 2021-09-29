/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package beacon

import (
	"bytes"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
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
	NewBlockTemplate(burnReward int) (wire.BTCBlockAux, bool, error)
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

	header.SetK(header.Bits() / 2)
	header.SetVoteK(header.Bits() / 2)

	aux, full, err := c.stateInfo.BTCGen.NewBlockTemplate(burnReward)
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

func (c *BlockGenerator) ValidateBlockHeader(_ wire.BlockHeader) error { return nil }

func (c *BlockGenerator) ValidateCoinbaseTx(block *wire.MsgBlock, height int32) error {
	aux := block.Header.BeaconHeader().BTCAux()
	if len(aux.Tx.TxOut) != 3 {
		return errors.New("invalid format of btc aux coinbase tx: less than 3 out")
	}

	jaxNetLink, _ := txscript.NullDataScript([]byte(types.JaxNetLink))
	jaxBurn, _ := txscript.NullDataScript([]byte(types.JaxBurnAddr))

	var btcBurnReward = false

	if len(aux.CoinbaseAux.Tx.TxOut) == 3 {
		const errMsg = "invalid format of btc aux coinbase tx: "
		btcCoinbaseTx := aux.CoinbaseAux.Tx
		btcJaxNetLinkOut := bytes.Equal(btcCoinbaseTx.TxOut[0].PkScript, jaxNetLink) &&
			btcCoinbaseTx.TxOut[0].Value == 0
		if !btcJaxNetLinkOut {
			return errors.New(errMsg + "first out must be zero and have JaxNetLink")
		}

		if btcCoinbaseTx.TxOut[1].Value > 6_2500_0000 {
			return errors.New(errMsg + "reward greater than 6.25 BTC")
		}

		if btcCoinbaseTx.TxOut[1].Value < 6_2500_0000 {
			if btcCoinbaseTx.TxOut[1].Value > 5000_0000 {
				return errors.New(errMsg + "fee greater than 0.5 BTC")
			}
		}

		btcBurnReward = bytes.Equal(btcCoinbaseTx.TxOut[1].PkScript, jaxBurn)
	}

	if len(block.Transactions[0].TxOut) != 3 {
		return errors.New("invalid format of beacon coinbase tx: less than 3 out")
	}

	beaconCoinbaseTx := block.Transactions[0]
	jaxNetLinkOut := bytes.Equal(beaconCoinbaseTx.TxOut[0].PkScript, jaxNetLink) &&
		beaconCoinbaseTx.TxOut[0].Value == 0
	if !jaxNetLinkOut {
		return errors.New("invalid format of beacon coinbase tx: first out must be zero and have JaxNetLink")
	}

	jaxBurnReward := bytes.Equal(beaconCoinbaseTx.TxOut[1].PkScript, jaxBurn)
	if btcBurnReward && !jaxBurnReward {
		return errors.New("invalid format of beacon coinbase tx: BTC burned, JaxNet reward prohibited")
	}
	if !btcBurnReward && jaxBurnReward {
		return errors.New("invalid format of beacon coinbase tx: BTC not burned, JaxNet burn prohibited")
	}

	properReward := beaconCoinbaseTx.TxOut[1].Value == calcBlockSubsidy(height)
	if !properReward {
		return fmt.Errorf("invalid format of beacon coinbase tx: invalid value of second out - has(%d) expected(%d) height(%d)",
			beaconCoinbaseTx.TxOut[1].Value, calcBlockSubsidy(height), height)
	}

	return nil
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
func (c *BlockGenerator) CalcBlockSubsidy(height int32, params chaincfg.PowParams, header wire.BlockHeader) int64 {
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
		return (340 - 10*((x-1+pow10)/pow11)) * jaxutil.SatoshiPerJAXNETCoin

	// Year 2
	case height > endOfEpoch && height <= endOfEpoch*2:
		return (260 - 5*((x-endOfEpoch-1+pow10)/pow11)) * jaxutil.SatoshiPerJAXNETCoin

	// Year 3
	case height > endOfEpoch*2 && height <= endOfEpoch*3:
		return (220 - 15*((x-(endOfEpoch*2+1)+pow10)/pow11)) * jaxutil.SatoshiPerJAXNETCoin

	// Year 4
	case height > endOfEpoch*3 && height <= endOfEpoch*4:
		return (100 - 5*((x-(endOfEpoch*3+1)+pow10)/pow11)) * jaxutil.SatoshiPerJAXNETCoin

	// Year 5
	case height > endOfEpoch*4 && height <= endOfEpoch*5:
		return (60 - 5*((x-(endOfEpoch*4+1)+pow10)/pow11)) * jaxutil.SatoshiPerJAXNETCoin

	// Year 6+
	case height > endOfEpoch*5:
		return baseSubsidy * jaxutil.SatoshiPerJAXNETCoin
	default:
		return baseSubsidy * jaxutil.SatoshiPerJAXNETCoin
	}

}
