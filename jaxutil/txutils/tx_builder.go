/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package txutils

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/jaxutil/txmodels"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/chaindata"
	"gitlab.com/jaxnet/jaxnetd/node/encoder"
	"gitlab.com/jaxnet/jaxnetd/node/mempool"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

var (
	TxInEstWeight  = 180 // size of regular signed utxo
	TxOutEstWeight = 34  // simple output
	// RegularTxEstWeight = 1040
	// SwapTxEstWeight    = 1800
)

func EstimateFeeForTx(tx *wire.MsgTx, feeRate int64, addChange bool, shardID uint32) int64 {
	if feeRate < mempool.DefaultMinRelayTxFeeSatoshiPerByte {
		feeRate = mempool.DefaultMinRelayTxFeeSatoshiPerByte
	}

	size := chaindata.GetTransactionWeight(jaxutil.NewTx(tx))
	if addChange {
		size += int64(TxInEstWeight + TxOutEstWeight)
	}

	if shardID == 0 { // is beacon
		return feeRate * (size / 1000) // (Satoshi/bytes) * Byte =  satoshi
	}

	return feeRate * (size / 1000) / 8 // (Satoshi/bytes) * 1/8 Byte =  satoshi
}

func EstimateFee(inCount, outCount int, feeRate int64, addChange bool, shardID uint32) int64 {
	if feeRate < mempool.DefaultMinRelayTxFeeSatoshiPerByte {
		feeRate = mempool.DefaultMinRelayTxFeeSatoshiPerByte
	}

	if addChange {
		inCount += 1
		outCount += 1
	}

	// Version 4 bytes + LockTime 4 bytes + Serialized varint size for the
	// number of transaction inputs and outputs.
	baseSize := 8 +
		encoder.VarIntSerializeSize(uint64(inCount)) +
		encoder.VarIntSerializeSize(uint64(outCount)) +
		TxInEstWeight*inCount +
		TxOutEstWeight*outCount

	size := int64(baseSize * chaindata.WitnessScaleFactor)
	if shardID == 0 { // is beacon
		return feeRate * size // (Satoshi/bytes) * Byte = satoshi
	}

	return feeRate * size / 8 // (Satoshi/bytes) * 1/8 Byte = satoshi
}

type TxBuilder interface {
	SetType(txVersion int32) TxBuilder
	SetShardID(shardID uint32) TxBuilder
	SwapTx() TxBuilder

	AddRedeemScripts(redeemScripts ...string) TxBuilder

	SetDestination(destination string, amount int64) TxBuilder
	SetDestinationWithUTXO(destination string, amount int64, utxo txmodels.UTXOs) TxBuilder
	SetDestinationAtShard(destination string, amount int64, shardID uint32) TxBuilder

	SetSenders(addresses ...string) TxBuilder
	SetChangeDestination(changeAddresses ...string) TxBuilder

	// SetFee sets static fee with.
	// If it set FeeProviderFunc result will be ignored
	SetFee(shardID uint32, fee int64) TxBuilder
	SetUTXOProvider(provider NewUTXOProvider) TxBuilder

	IntoTx(getFee FeeProviderFunc, kdb txscript.KeyDB) (*wire.MsgTx, error)
}

type txBuilder struct {
	err error
	net chaincfg.NetName

	txVersion       int32
	lockTime        uint32
	swapTx          bool
	defaultShardID  uint32
	redeemScripts   map[string]scriptData
	utxoProvider    NewUTXOProvider
	senderAddresses []string
	changeAddresses []string
	destinations    []destinationKey

	collectedOpts map[uint32]extraOpts
}

type extraOpts struct {
	fee      int64
	feeRate  int64
	change   int64
	needed   int64
	outs     int
	utxoRows txmodels.UTXOs
}

func (e extraOpts) calcFee(shardID uint32) int64 {
	return EstimateFee(e.utxoRows.Len(), e.outs, e.feeRate, true, shardID)
}

func NewTxBuilder(net chaincfg.NetName) TxBuilder {
	return &txBuilder{
		net:             net,
		txVersion:       wire.TxVerRegular,
		redeemScripts:   map[string]scriptData{},
		senderAddresses: []string{},
		changeAddresses: []string{},
		destinations:    []destinationKey{},
		collectedOpts:   map[uint32]extraOpts{},
	}
}

func (t *txBuilder) SetShardID(shardID uint32) TxBuilder {
	t.defaultShardID = shardID
	if _, ok := t.collectedOpts[shardID]; !ok {
		t.collectedOpts[shardID] = extraOpts{feeRate: 0, change: 0, utxoRows: txmodels.UTXORows{}}
	}

	return t
}

func (t *txBuilder) SetFee(shardID uint32, fee int64) TxBuilder {
	opts, ok := t.collectedOpts[shardID]
	if !ok {
		opts = extraOpts{feeRate: 0, change: 0, utxoRows: txmodels.UTXORows{}}
	}

	opts.fee = fee
	t.collectedOpts[shardID] = opts

	return t
}

func (t *txBuilder) SetType(txVersion int32) TxBuilder {
	t.txVersion = txVersion
	return t
}

func (t *txBuilder) SwapTx() TxBuilder {
	t.swapTx = true
	return t
}

func (t *txBuilder) AddRedeemScripts(redeemScripts ...string) TxBuilder {
	if t.err != nil {
		return t
	}

	if t.redeemScripts == nil {
		t.redeemScripts = make(map[string]scriptData, len(redeemScripts))
	}

	for _, redeemScript := range redeemScripts {
		rawScript, err := hex.DecodeString(redeemScript)
		if err != nil {
			t.err = errors.Wrap(err, "unable to decode hex script")
			return t
		}

		script, err := DecodeScript(rawScript, t.net.Params())
		if err != nil {
			t.err = errors.Wrap(err, "unable to parse script")
			return t
		}

		t.redeemScripts[script.P2sh] = scriptData{
			Type: script.Type,
			P2sh: script.P2sh,
			Hex:  redeemScript,
		}
	}

	return t
}

func (t *txBuilder) SetDestination(destination string, amount int64) TxBuilder {
	t.destinations = append(t.destinations, destinationKey{
		amount:      amount,
		shardID:     t.defaultShardID,
		destination: destination,
		utxo:        txmodels.UTXORows{},
	})

	opts := t.collectedOpts[t.defaultShardID]
	opts.feeRate = 0
	t.collectedOpts[t.defaultShardID] = opts
	return t
}

func (t *txBuilder) SetDestinationAtShard(destination string, amount int64, shardID uint32) TxBuilder {
	t.destinations = append(t.destinations, destinationKey{
		amount:      amount,
		shardID:     shardID,
		destination: destination,
		utxo:        txmodels.UTXORows{},
	})

	opts := t.collectedOpts[shardID]
	opts.feeRate = 0
	t.collectedOpts[t.defaultShardID] = opts
	return t
}

func (t *txBuilder) SetDestinationWithUTXO(destination string, amount int64, utxo txmodels.UTXOs) TxBuilder {
	shardID := t.defaultShardID
	for _, u := range utxo.List() {
		opts := t.collectedOpts[u.ShardID]
		opts.feeRate = 0
		t.collectedOpts[u.ShardID] = opts

		shardID = u.ShardID
		break
	}

	t.destinations = append(t.destinations, destinationKey{
		amount:      amount,
		shardID:     shardID,
		destination: destination,
		utxo:        utxo,
	})

	return t
}

func (t *txBuilder) SetSenders(addresses ...string) TxBuilder {
	t.senderAddresses = addresses
	return t
}

func (t *txBuilder) SetChangeDestination(changeAddresses ...string) TxBuilder {
	t.changeAddresses = changeAddresses
	return t
}

func (t *txBuilder) SetUTXOProvider(provider NewUTXOProvider) TxBuilder {
	t.utxoProvider = provider
	return t
}

func (t *txBuilder) IntoTx(getFee FeeProviderFunc, kdb txscript.KeyDB) (*wire.MsgTx, error) {
	if t.err != nil {
		return nil, t.err
	}

	if err := t.setFees(getFee); err != nil {
		return nil, err
	}

	if err := t.prepareUTXOs(); err != nil {
		return nil, err
	}

	if t.swapTx {
		return t.craftSwapTx(kdb)
	}

	return t.craftRegularTx(kdb)
}

func (t *txBuilder) craftSwapTx(kdb txscript.KeyDB) (*wire.MsgTx, error) {
	msgTx := wire.NewMsgTx(t.txVersion)
	msgTx.SetMark(wire.TxMarkShardSwap)
	msgTx.LockTime = t.lockTime

	for _, dest := range t.destinations {
		opts := t.collectedOpts[dest.shardID]

		rows := opts.utxoRows.List()
		for i := range rows {
			utxo := rows[i]
			utxoTxHash, err := chainhash.NewHashFromStr(utxo.TxHash)
			if err != nil {
				return nil, errors.Wrap(err, "can not decode TxHash")
			}

			outPoint := wire.NewOutPoint(utxoTxHash, utxo.OutIndex)
			txIn := wire.NewTxIn(outPoint, nil, nil)
			if t.lockTime != 0 {
				txIn.Sequence = blockchain.LockTimeToSequence(false, t.lockTime)
			}

			msgTx.AddTxIn(txIn)
		}

		draft := txmodels.DraftTx{Amount: dest.amount, NetworkFee: opts.fee}
		err := draft.SetPayToAddress(dest.destination, t.net.Params())
		if err != nil {
			return nil, errors.Wrap(err, "pay to address not set")
		}

		msgTx.AddTxOut(wire.NewTxOut(draft.Amount, draft.ReceiverScript))
		if msgTx, err = t.addChangeOut(msgTx, opts.change); err != nil {
			return nil, err
		}
	}

	if kdb == nil {
		return msgTx, nil
	}

	txInIndex := 0
	for _, dest := range t.destinations {
		opts := t.collectedOpts[dest.shardID]

		rows := opts.utxoRows.List()
		for i := range rows {
			utxo := rows[i].ToShort()
			_, err := t.signUTXOForTx(msgTx, utxo, txInIndex, kdb)
			if err != nil {
				return nil, errors.Wrap(err, "unable to sing utxo")
			}
			txInIndex += 1
		}
	}

	return msgTx, nil
}

func (t *txBuilder) craftRegularTx(kdb txscript.KeyDB) (*wire.MsgTx, error) {
	var (
		err            error
		amount         int64
		msgTx          = wire.NewMsgTx(t.txVersion)
		fee            = t.collectedOpts[t.defaultShardID].fee
		additionalUTXO = t.collectedOpts[t.defaultShardID].utxoRows
		totalSum       = additionalUTXO.GetSum()
	)

	for _, key := range t.destinations {
		receiverScript, err := txmodels.GetPayToAddressScript(key.destination, t.net.Params())
		if err != nil {
			return nil, err
		}
		amount += key.amount
		msgTx.AddTxOut(wire.NewTxOut(key.amount, receiverScript))
	}

	change := totalSum - amount - fee
	if msgTx, err = t.addChangeOut(msgTx, change); err != nil {
		return nil, err
	}

	au := additionalUTXO.List()
	for txInIndex := range au {
		utxo := au[txInIndex]
		utxoTxHash, err := chainhash.NewHashFromStr(utxo.TxHash)
		if err != nil {
			return nil, errors.Wrap(err, "can not decode TxHash")
		}

		outPoint := wire.NewOutPoint(utxoTxHash, utxo.OutIndex)
		txIn := wire.NewTxIn(outPoint, nil, nil)
		if t.lockTime != 0 {
			txIn.Sequence = blockchain.LockTimeToSequence(false, t.lockTime)
		}
		msgTx.AddTxIn(txIn)
	}

	if kdb == nil {
		return msgTx, nil
	}

	for i := range au {
		txInIndex := i
		utxo := au[txInIndex].ToShort()

		_, err := t.signUTXOForTx(msgTx, utxo, txInIndex, kdb)
		if err != nil {
			return nil, errors.Wrap(err, "unable to sing utxo")
		}
	}

	return msgTx, nil
}

func (t *txBuilder) addChangeOut(msgTx *wire.MsgTx, change int64) (*wire.MsgTx, error) {
	if change > 0 && len(t.changeAddresses) == 0 {
		return nil, errors.New("changeAddress not set")
	}

	if change > 0 {
		chunk := change / int64(len(t.changeAddresses))
		for _, address := range t.changeAddresses {
			changeRcvScript, err := txmodels.GetPayToAddressScript(address, t.net.Params())
			if err != nil {
				return nil, errors.Wrap(err, "unable to create P2A script for change")
			}
			msgTx.AddTxOut(wire.NewTxOut(chunk, changeRcvScript))
		}
	}
	return msgTx, nil
}

// signUTXOForTx performs signing of UTXO, adds this signature to redeemTx.
// Method also supports signing of multiSig UTXOs, so just provide existing signature as prevScript
// 	- redeemTx is a transaction that will be sent
// 	- utxo is output that will be spent
// 	- inIndex is an index, where placed this UTXO
// 	- prevScript is a SignatureScript made by one or more previous key in case of multiSig UTXO, otherwise it nil
// 	- postVerify say to check tx after signing
func (t *txBuilder) signUTXOForTx(msgTx *wire.MsgTx, utxo txmodels.ShortUTXO, inIndex int, kdb txscript.KeyDB) ([]byte, error) {
	pkScript, err := hex.DecodeString(utxo.PKScript)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode PK script")
	}

	scriptPubKey, err := DecodeScript(pkScript, t.net.Params())
	if err != nil {
		return nil, err
	}

	for _, address := range scriptPubKey.Addresses {
		if script, ok := t.redeemScripts[address]; ok {
			utxo.RedeemScript = script.Hex
			break
		}
	}

	var prevScript []byte = nil
	if msgTx.TxIn[inIndex].SignatureScript != nil {
		prevScript = msgTx.TxIn[inIndex].SignatureScript
	}

	var sig []byte
	sig, err = txscript.SignTxOutput(t.net.Params(), msgTx, inIndex, pkScript,
		txscript.SigHashAll, kdb, &utxo, prevScript)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign tx output")
	}

	msgTx.TxIn[inIndex].SignatureScript = sig

	return sig, nil
}

func (t *txBuilder) setFees(getFee FeeProviderFunc) error {
	for shardID := range t.collectedOpts {
		fee, feeRate, err := getFee(shardID)
		if err != nil {
			return errors.Wrapf(err, "unable to get feeRate for shard(%d)", shardID)
		}

		opts := t.collectedOpts[shardID]
		if opts.fee < 1 {
			opts.fee = fee
		}

		opts.feeRate = feeRate
		t.collectedOpts[shardID] = opts
	}

	return nil
}

func (t *txBuilder) prepareUTXOs() error {
	if len(t.destinations) == 0 {
		return errors.New("destinations are not set")
	}

	var (
		shardID *uint32
		shards  = map[uint32]struct{}{}
	)

	for _, key := range t.destinations {
		utxo := key.utxo
		if !t.swapTx && (shardID != nil && *shardID != key.shardID) {
			return errors.Errorf("destinations are not in the same shard (%d != %d)", *shardID, key.shardID)
		}

		shardID = &key.shardID

		opts := t.collectedOpts[key.shardID]
		opts.needed += key.amount
		opts.outs += 1
		if opts.utxoRows == nil {
			opts.utxoRows = utxo
		} else {
			opts.utxoRows = opts.utxoRows.Append(utxo.List()...)
		}

		t.collectedOpts[key.shardID] = opts
		shards[key.shardID] = struct{}{}
	}

	if shardID != nil && t.defaultShardID != *shardID {
		t.defaultShardID = *shardID
	}

	for shardID := range shards {
		opts := t.collectedOpts[shardID]
		hasCoins := opts.utxoRows.GetSum()
		if opts.fee < 1 {
			opts.fee = opts.calcFee(shardID)
		}

		needed := opts.needed + opts.fee
		if t.swapTx && hasCoins < needed {
			if t.utxoProvider == nil {
				return errors.New("utxoProvider not set")
			}
			row, err := t.utxoProvider.GetForAmount(needed-hasCoins, shardID, t.senderAddresses...)
			if err != nil {
				return err
			}

			if opts.utxoRows == nil {
				opts.utxoRows = txmodels.UTXORows{*row}
			} else {
				opts.utxoRows = opts.utxoRows.Append(*row)
			}
		}

		if !t.swapTx && hasCoins < needed {
			if t.utxoProvider == nil {
				return errors.New("utxoProvider not set")
			}
			rows, err := t.utxoProvider.SelectForAmount(needed-hasCoins, shardID, t.senderAddresses...)
			if err != nil {
				return err
			}

			if opts.utxoRows == nil {
				opts.utxoRows = rows
			} else {
				opts.utxoRows = opts.utxoRows.Append(rows...)
			}
		}

		opts.change = opts.utxoRows.GetSum() - needed
		t.collectedOpts[shardID] = opts
	}

	return nil
}

type scriptData struct {
	Type string
	P2sh string
	Hex  string
}

type destinationKey struct {
	amount      int64
	shardID     uint32
	destination string
	utxo        txmodels.UTXOs
}

// FeeProviderFunc should return non-zero value either fee or feeRate rate.
// Static fee has main priority, otherwise, the fee will be calculated using feeRate.
type FeeProviderFunc func(shardID uint32) (fee, feeRate int64, err error)
type GetTxOutFunc func(shardID uint32, txHash *chainhash.Hash, index uint32) (int64, error)
