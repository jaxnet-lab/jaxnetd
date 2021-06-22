/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package btcd

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"

	btcdjson "github.com/btcsuite/btcd/btcjson"
	btcdchainhash "github.com/btcsuite/btcd/chaincfg/chainhash"
	btcrpc "github.com/btcsuite/btcd/rpcclient"
	btcdwire "github.com/btcsuite/btcd/wire"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/node/mining"
	"gitlab.com/jaxnet/jaxnetd/types"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type RPCCfg struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"pass"`
}

type Configuration struct {
	Network string `yaml:"network"`
	RPC     RPCCfg `yaml:"rpc"`
	Enable  bool   `yaml:"enable"`
}

type BlockProvider struct {
	client       *btcrpc.Client
	offline      bool
	minerAddress jaxutil.Address
}

func NewBlockProvider(cfg Configuration, minerAddress jaxutil.Address) (*BlockProvider, error) {
	if !cfg.Enable {
		return &BlockProvider{offline: true}, nil
	}
	client, err := btcrpc.New(&btcrpc.ConnConfig{
		Host:         cfg.RPC.Host,
		User:         cfg.RPC.User,
		Pass:         cfg.RPC.Password,
		Params:       cfg.Network,
		DisableTLS:   true,
		HTTPPostMode: true,
	}, nil)

	return &BlockProvider{
		client:       client,
		offline:      false,
		minerAddress: minerAddress,
	}, err
}

func (bg *BlockProvider) NewBlockTemplate(burnRewardFlag int) (wire.BTCBlockAux, error) {
	if bg.offline || bg.client == nil {
		return wire.BTCBlockAux{
			Timestamp: time.Unix(time.Now().Unix(), 0),
		}, nil
	}

	template, err := bg.client.GetBlockTemplate(
		&btcdjson.TemplateRequest{
			Mode:         "template",
			Capabilities: []string{"coinbasevalue"},
		})
	if err != nil {
		return wire.BTCBlockAux{}, err
	}

	block, height, err := DecodeBitcoinResponse(template)
	if err != nil {
		return wire.BTCBlockAux{}, err
	}

	aux := wire.BTCBlockAux{
		Version:    block.Header.Version,
		PrevBlock:  chainhash.Hash(block.Header.PrevBlock),
		MerkleRoot: chainhash.Hash(block.Header.MerkleRoot),
		Timestamp:  block.Header.Timestamp,
		Bits:       block.Header.Bits,
		Nonce:      block.Header.Nonce,
		CoinbaseAux: wire.CoinbaseAux{
			Tx:       wire.MsgTx{},
			TxMerkle: make([]chainhash.Hash, 0, len(template.Transactions)),
		},
	}

	totalFee := int64(0)
	reward := int64(0)
	if template.CoinbaseValue != nil {
		reward = *template.CoinbaseValue
	}

	for _, tx := range template.Transactions {
		totalFee += tx.Fee
		txHash, _ := chainhash.NewHashFromStr(tx.Hash)
		aux.TxMerkle = append(aux.TxMerkle, *txHash)
	}

	burnReward := burnRewardFlag&types.BurnBtcReward == types.BurnBtcReward
	tx, err := mining.CreateJaxCoinbaseTx(reward, totalFee, int32(height), bg.minerAddress, burnReward)
	if err != nil {
		return wire.BTCBlockAux{}, err
	}

	aux.Tx = *tx.MsgTx()
	return aux, nil
}

func DecodeBitcoinResponse(c *btcdjson.GetBlockTemplateResult) (
	block *btcdwire.MsgBlock, height int64, err error) {

	// Block initialisation.
	height = c.Height

	bitcoinBlock := btcdwire.MsgBlock{}
	block = &bitcoinBlock

	// Transactions processing.
	block.Transactions, err = unmarshalBitcoinTransactions(c.CoinbaseTxn, c.Transactions)
	if err != nil {
		return
	}

	// Block header processing.
	previousBlockHash, err := btcdchainhash.NewHashFromStr(c.PreviousHash)
	if err != nil {
		return
	}

	bits, err := unmarshalBits(c.Bits)
	if err != nil {
		return
	}

	block.Header = *btcdwire.NewBlockHeader(c.Version, previousBlockHash, &btcdchainhash.Hash{}, bits, uint32(0))
	return
}

func unmarshalBits(hexBits string) (bits uint32, err error) {
	bitsHex, err := hex.DecodeString(hexBits)
	if err != nil {
		return
	}

	if len(bitsHex) != 4 {
		err = errors.New("invalid header bits format")
		return
	}

	bits = binary.BigEndian.Uint32(bitsHex)
	return
}

func unmarshalBitcoinTransactions(coinbaseTx *btcdjson.GetBlockTemplateResultTx,
	txs []btcdjson.GetBlockTemplateResultTx) (transactions []*btcdwire.MsgTx, err error) {

	unmarshalBitcoinTx := func(txHash string) (tx *btcdwire.MsgTx, err error) {
		txBinary, err := hex.DecodeString(txHash)
		if err != nil {
			return
		}

		tx = &btcdwire.MsgTx{}
		txReader := bytes.NewReader(txBinary)
		err = tx.Deserialize(txReader)
		return
	}

	// Coinbase transaction must be processed first.
	// (transactions order in transactions slice is significant)
	if coinbaseTx != nil {
		cTX, err := unmarshalBitcoinTx(coinbaseTx.Data)
		if err != nil {
			return nil, err
		}

		transactions = make([]*btcdwire.MsgTx, 0)
		transactions = append(transactions, cTX)
	}

	// Regular transactions processing.
	for _, marshalledTx := range txs {
		tx, err := unmarshalBitcoinTx(marshalledTx.Data)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, tx)
	}

	return
}
