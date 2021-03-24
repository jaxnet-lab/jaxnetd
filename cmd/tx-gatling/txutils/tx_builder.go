/*
 * Copyright (c) 2020 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package txutils

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core/node/blockchain"
	"gitlab.com/jaxnet/core/shard.core/txscript"
	"gitlab.com/jaxnet/core/shard.core/types/chaincfg"
	"gitlab.com/jaxnet/core/shard.core/types/chainhash"
	"gitlab.com/jaxnet/core/shard.core/types/wire"
)

type TxBuilder interface {
	SetType(txVersion int32) TxBuilder
	SetShardID(shardID uint32) TxBuilder
	SwapTx() TxBuilder

	// utxo can be spent only after lock ended
	SetTimeLock(lockTime uint32) TxBuilder
	// utxo can be spent only before lock ended, after end money back
	SetTimeLockAllowance(lockTime uint32) TxBuilder

	AddRedeemScripts(redeemScripts ...string) TxBuilder

	SetDestination(destination string, amount int64) TxBuilder
	SetDestinationWithUTXO(destination string, amount int64, utxo txmodels.UTXORows) TxBuilder
	SetDestinationAtShard(destination string, amount int64, shardID uint32) TxBuilder

	SetSenders(addresses ...string) TxBuilder
	SetChangeDestination(changeAddresses ...string) TxBuilder

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
	change   int64
	needed   int64
	utxoRows txmodels.UTXORows
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
	t.collectedOpts[shardID] = extraOpts{fee: 0, change: 0, utxoRows: []txmodels.UTXO{}}
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

func (t *txBuilder) SetTimeLock(lockTime uint32) TxBuilder {
	t.lockTime = lockTime
	t.txVersion = wire.TxVerTimeLock
	return t
}

func (t *txBuilder) SetTimeLockAllowance(lockTime uint32) TxBuilder {
	t.lockTime = lockTime
	t.txVersion = wire.TxVerTimeLockAllowance
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
	opts.fee = 0
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
	opts.fee = 0
	t.collectedOpts[t.defaultShardID] = opts
	return t
}

func (t *txBuilder) SetDestinationWithUTXO(destination string, amount int64, utxo txmodels.UTXORows) TxBuilder {
	t.destinations = append(t.destinations, destinationKey{
		amount:      amount,
		shardID:     t.defaultShardID,
		destination: destination,
		utxo:        utxo,
	})

	for _, u := range utxo {
		opts := t.collectedOpts[u.ShardID]
		opts.fee = 0
		t.collectedOpts[t.defaultShardID] = opts
		break
	}

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

		for i := range opts.utxoRows {
			utxo := opts.utxoRows[i]
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

		change := opts.utxoRows.GetSum() - dest.amount - opts.fee
		if msgTx, err = t.addChangeOut(msgTx, change); err != nil {
			return nil, err
		}
	}

	txInIndex := 0
	for _, dest := range t.destinations {
		opts := t.collectedOpts[dest.shardID]

		for i := range opts.utxoRows {
			utxo := opts.utxoRows[i].ToShort()
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

	for txInIndex := range additionalUTXO {
		utxo := additionalUTXO[txInIndex]
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

	for i := range additionalUTXO {
		txInIndex := i
		utxo := additionalUTXO[txInIndex].ToShort()

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
		fee, err := getFee(shardID)
		if err != nil {
			return errors.Wrapf(err, "unable to get fee for shard(%d)", shardID)
		}

		opts := t.collectedOpts[shardID]
		opts.fee = fee
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
		opts.utxoRows = append(opts.utxoRows, utxo...)

		t.collectedOpts[key.shardID] = opts
		shards[key.shardID] = struct{}{}
	}

	if shardID != nil && t.defaultShardID != *shardID {
		t.defaultShardID = *shardID
	}

	for shardID := range shards {
		opts := t.collectedOpts[shardID]
		hasCoins := opts.utxoRows.GetSum()
		if hasCoins < opts.needed {
			rows, err := t.utxoProvider.SelectForAmount(opts.needed-hasCoins, shardID, t.senderAddresses...)
			if err != nil {
				return err
			}

			opts.utxoRows = append(opts.utxoRows, rows...)
		}

		change := opts.utxoRows.GetSum() - opts.needed
		opts.change += change

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
	utxo        txmodels.UTXORows
}

type FeeProviderFunc func(shardID uint32) (int64, error)
type GetTxOutFunc func(shardID uint32, txHash *chainhash.Hash, index uint32) (int64, error)
