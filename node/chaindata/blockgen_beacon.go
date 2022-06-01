/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaindata

import (
	"fmt"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	mmtree "gitlab.com/jaxnet/jaxnetd/types/merge_mining_tree"
	"gitlab.com/jaxnet/jaxnetd/types/pow"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type StateProvider struct {
	ShardCount func() (uint32, error)
	BTCGen     BtcGen
}

type BeaconBlockGenerator struct {
	stateInfo             StateProvider
	powLimit              *big.Int
	hashSortingSlotNumber uint32
}

type BtcGen interface {
	NewBlockTemplate(burnReward int, beaconHash chainhash.Hash) (wire.BTCBlockAux, bool, error)
}

func NewBeaconBlockGen(stateInfo StateProvider, powParams chaincfg.PowParams) *BeaconBlockGenerator {
	return &BeaconBlockGenerator{
		stateInfo:             stateInfo,
		powLimit:              pow.CompactToBig(powParams.PowLimitBits),
		hashSortingSlotNumber: powParams.HashSortingSlotNumber,
	}
}

func (c *BeaconBlockGenerator) NewBlockHeader(version wire.BVersion, height int32,
	blocksMMRRoot, prevHash, merkleRootHash chainhash.Hash,
	timestamp time.Time, bits uint32, prevWeight *big.Int, nonce uint32, burnReward int) (wire.BlockHeader, error) {
	// Limit the timestamp to one second precision since the protocol
	// doesn't support better.
	header := wire.NewBeaconBlockHeader(
		version,
		height,
		blocksMMRRoot,
		prevHash,
		merkleRootHash,
		chainhash.Hash{},
		timestamp,
		bits,
		prevWeight,
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
		aux.Nonce = header.Nonce()
	}
	header.SetBTCAux(aux)

	return header, nil
}

func validateMergeMiningData(header wire.BlockHeader, beacon bool) error {
	beaconAux := header.BeaconHeader()

	mmNumber := header.MergeMiningNumber()
	mergeMiningRoot := beaconAux.MergeMiningRoot()
	miningTreeCodingProof, orangeTreeEncoding, codingLen := beaconAux.MergedMiningTreeCodingProof()
	emptyData := chainhash.ZeroHash.IsEqual(&mergeMiningRoot) && len(miningTreeCodingProof) == 0 && len(orangeTreeEncoding) == 0 && codingLen == 0

	if mmNumber > beaconAux.Shards() {
		return fmt.Errorf("MergeMiningNumber(%v) more than beaconAux.Shards(%v)",
			mmNumber, beaconAux.Shards())
	}

	if beacon && mmNumber == 0 {
		if !emptyData {
			return fmt.Errorf("MergeMiningNumber is 0, but MergeMining data not empty")
		}
		return nil
	}

	tree := mmtree.NewSparseMerkleTree(beaconAux.Shards())
	orangeTreeEmpty := chainhash.NextPowerOfTwo(int(mmNumber)) == int(mmNumber) && mmNumber == beaconAux.Shards()
	if !orangeTreeEmpty {
		err := tree.ValidateOrangeTree(codingLen, orangeTreeEncoding, miningTreeCodingProof, mmNumber, mergeMiningRoot, beacon)
		if err != nil {
			return errors.Wrap(err, "invalid orange tree")
		}
		return nil
	}

	return nil
}

func (c *BeaconBlockGenerator) ValidateJaxAuxRules(block *wire.MsgBlock, height int32) error {
	if err := validateMergeMiningData(block.Header, true); err != nil {
		return err
	}

	_, err := ValidateBeaconCoinbase(block.Header.BeaconHeader(), block.Transactions[0], calcBlockSubsidy(height))
	return err
}

// CalcBlockSubsidy returns the subsidy amount a block at the provided Height
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
func (c *BeaconBlockGenerator) CalcBlockSubsidy(height int32, difficulty uint32, header wire.BlockHeader) int64 {
	return calcBlockSubsidy(height)
}

// nolint:gomnd
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
		return (340 - 10*((x-1+pow10)/pow11)) * chaincfg.HaberStornettaPerJAXNETCoin

	// Year 2
	case height > endOfEpoch && height <= endOfEpoch*2:
		return (260 - 5*((x-endOfEpoch-1+pow10)/pow11)) * chaincfg.HaberStornettaPerJAXNETCoin

	// Year 3
	case height > endOfEpoch*2 && height <= endOfEpoch*3:
		return (220 - 15*((x-(endOfEpoch*2+1)+pow10)/pow11)) * chaincfg.HaberStornettaPerJAXNETCoin

	// Year 4
	case height > endOfEpoch*3 && height <= endOfEpoch*4:
		return (100 - 5*((x-(endOfEpoch*3+1)+pow10)/pow11)) * chaincfg.HaberStornettaPerJAXNETCoin

	// Year 5
	case height > endOfEpoch*4 && height <= endOfEpoch*5:
		return (60 - 5*((x-(endOfEpoch*4+1)+pow10)/pow11)) * chaincfg.HaberStornettaPerJAXNETCoin

	// Year 6+
	case height > endOfEpoch*5:
		return chaincfg.BeaconBaseReward * chaincfg.HaberStornettaPerJAXNETCoin
	default:
		return chaincfg.BeaconBaseReward * chaincfg.HaberStornettaPerJAXNETCoin
	}
}

type BTCBlockGen struct {
	MinerAddress jaxutil.Address
}

// nolint: gomnd
func (bg *BTCBlockGen) NewBlockTemplate(burnRewardFlag int, beaconHash chainhash.Hash) (wire.BTCBlockAux, bool, error) {
	burnReward := burnRewardFlag&types.BurnJaxNetReward == types.BurnJaxNetReward
	tx, err := CreateBitcoinCoinbaseTx(6_2500_0000, 0, int32(-1),
		bg.MinerAddress, beaconHash.CloneBytes(), burnReward)
	if err != nil {
		return wire.BTCBlockAux{}, false, err
	}

	return wire.BTCBlockAux{
		CoinbaseAux: wire.CoinbaseAux{
			Tx:            *tx.MsgTx(),
			TxMerkleProof: []chainhash.Hash{},
		},
		Bits:       0x170e2632,
		MerkleRoot: *tx.Hash(),
		Timestamp:  time.Unix(time.Now().Unix(), 0),
	}, false, nil
}
