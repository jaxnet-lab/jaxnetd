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

	txVersion        int32
	lockTime         uint32
	swapTx           bool
	defaultShardID   uint32
	redeemScripts    map[string]scriptData
	utxoProvider     NewUTXOProvider
	senderAddresses  []string
	changeAddresses  []string
	destinations     []destinationKey
	destinationsUtxo txmodels.UTXORows

	fee    int64
	change int64

	feeByShard    map[uint32]int64 // this will be used only for wire.TxMarkShardSwap
	changeByShard map[uint32]int64 // this will be used only for wire.TxMarkShardSwap

}

func NewTxBuilder(net chaincfg.NetName) TxBuilder {
	return &txBuilder{
		net:             net,
		txVersion:       wire.TxVerRegular,
		redeemScripts:   map[string]scriptData{},
		senderAddresses: []string{},
		changeAddresses: []string{},
		destinations:    []destinationKey{},
		feeByShard:      map[uint32]int64{},
		changeByShard:   map[uint32]int64{},
	}

}
func (t *txBuilder) SetShardID(shardID uint32) TxBuilder {
	t.defaultShardID = shardID
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

	t.feeByShard[t.defaultShardID] = 0
	return t
}

func (t *txBuilder) SetDestinationAtShard(destination string, amount int64, shardID uint32) TxBuilder {
	t.destinations = append(t.destinations, destinationKey{
		amount:      amount,
		shardID:     shardID,
		destination: destination,
		utxo:        txmodels.UTXORows{},
	})

	t.feeByShard[shardID] = 0
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
		t.feeByShard[u.ShardID] = 0
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

	ind := 0
	outIndexes := map[string]int{}

	for _, dest := range t.destinations {
		utxo := dest.utxo[0]
		utxoTxHash, err := chainhash.NewHashFromStr(utxo.TxHash)
		if err != nil {
			return nil, errors.Wrap(err, "can not decode TxHash")
		}

		draft := txmodels.DraftTx{
			Amount:     utxo.Value - t.feeByShard[dest.shardID],
			NetworkFee: t.feeByShard[dest.shardID],
			UTXO:       []txmodels.UTXO{utxo},
		}

		err = draft.SetPayToAddress(dest.destination, t.net.Params())
		if err != nil {
			return nil, errors.Wrap(err, "pay to address not set")
		}
		msgTx.AddTxOut(wire.NewTxOut(draft.Amount, draft.ReceiverScript))

		outPoint := wire.NewOutPoint(utxoTxHash, utxo.OutIndex)
		txIn := wire.NewTxIn(outPoint, nil, nil)
		if t.lockTime != 0 {
			txIn.Sequence = blockchain.LockTimeToSequence(false, t.lockTime)
		}

		msgTx.AddTxIn(txIn)

		outIndexes[dest.destination] = ind
		ind += 1
	}

	for _, dest := range t.destinations {
		txInIndex := outIndexes[dest.destination]
		utxo := dest.utxo[0].ToShort()

		_, err := t.signUTXOForTx(msgTx, utxo, txInIndex, kdb)
		if err != nil {
			return nil, errors.Wrap(err, "unable to sing utxo")
		}
	}

	return msgTx, nil
}

func (t *txBuilder) craftRegularTx(kdb txscript.KeyDB) (*wire.MsgTx, error) {
	msgTx := wire.NewMsgTx(t.txVersion)

	var amount int64
	totalSum := t.destinationsUtxo.GetSum()

	for _, key := range t.destinations {
		receiverScript, err := txmodels.GetPayToAddressScript(key.destination, t.net.Params())
		if err != nil {
			return nil, err
		}
		amount += key.amount
		msgTx.AddTxOut(wire.NewTxOut(key.amount, receiverScript))
	}

	change := totalSum - amount - t.fee
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

	for i := range t.destinationsUtxo {
		txInIndex := i
		utxo := t.destinationsUtxo[txInIndex]
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

	for i := range t.destinationsUtxo {
		txInIndex := i
		utxo := t.destinationsUtxo[txInIndex].ToShort()

		_, err := t.signUTXOForTx(msgTx, utxo, txInIndex, kdb)
		if err != nil {
			return nil, errors.Wrap(err, "unable to sing utxo")
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
	if !t.swapTx {
		t.fee, t.err = getFee(t.defaultShardID)
		if t.err != nil {
			return errors.Wrapf(t.err, "unable to get fee for shard(%d)", t.defaultShardID)
		}
		return nil
	}

	for shardID := range t.feeByShard {
		if _, ok := t.feeByShard[shardID]; ok {
			continue
		}

		fee, err := getFee(shardID)
		if err != nil {
			return errors.Wrapf(err, "unable to get fee for shard(%d)", shardID)
		}

		t.feeByShard[shardID] = fee
	}

	return nil
}

func (t *txBuilder) prepareUTXOs() error {
	t.change = 0
	t.changeByShard = map[uint32]int64{}

	// todo: include fe properly
	// amountByShard := map[uint32]int64{}
	// for key := range t.destinations {
	//
	// }

	if t.swapTx {
		return nil
	}

	var hasCoins, needed int64
	var shardID *uint32

	for _, key := range t.destinations {
		utxo := key.utxo
		if shardID != nil && *shardID != key.shardID {
			return errors.Errorf("destinations are not in the same shard (%d != %d)", *shardID, key.shardID)
		}

		shardID = &key.shardID

		hasCoins += utxo.GetSum()
		needed += key.amount
		t.destinationsUtxo = append(t.destinationsUtxo, utxo...)
		// needed := t.feeByShard[key.shardID] + key.amount
	}

	if len(t.destinations) == 0 || shardID == nil {
		return errors.New("destinations are not set")
	}

	if hasCoins < needed {
		rows, err := t.utxoProvider.SelectForAmount(needed-hasCoins, *shardID, t.senderAddresses...)
		if err != nil {
			return err
		}

		t.destinationsUtxo = append(t.destinationsUtxo, rows...)
	}

	change := t.destinationsUtxo.GetSum() - needed
	t.change += change
	t.changeByShard[*shardID] += change

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
