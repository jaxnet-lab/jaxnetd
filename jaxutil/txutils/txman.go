// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txutils

import (
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/jaxutil/txmodels"
	"gitlab.com/jaxnet/jaxnetd/network/rpcclient"
	"gitlab.com/jaxnet/jaxnetd/node/blockchain"
	"gitlab.com/jaxnet/jaxnetd/node/mempool"
	"gitlab.com/jaxnet/jaxnetd/txscript"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

const (
	zeroHash       = "0000000000000000000000000000000000000000000000000000000000000000"
	OneCoin  int64 = 100_000_000
)

type TxMan struct {
	cfg ManagerCfg
	key *KeyData

	NetParams *chaincfg.Params
	rpc       *rpcclient.Client
	lockTime  uint32
	txVersion int32
}

func NewTxMan(cfg ManagerCfg) (*TxMan, error) {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Params:       cfg.Net,
		Host:         cfg.RPC.Host,
		User:         cfg.RPC.User,
		Pass:         cfg.RPC.Pass,
		ShardID:      cfg.ShardID,
		HTTPPostMode: true,
		DisableTLS:   true,
	}

	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	rpcClient, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}

	client := &TxMan{
		cfg:       cfg,
		NetParams: cfg.NetParams(),
		rpc:       rpcClient,
	}

	if cfg.PrivateKey != "" {
		client.key, err = NewKeyData(client.cfg.PrivateKey, client.NetParams)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func (client *TxMan) SetKey(key *KeyData) {
	client.key = key
}

func (client *TxMan) WithKeys(key *KeyData) *TxMan {
	clone := new(TxMan)
	*clone = *client
	clone.key = key
	return clone
}

func (client *TxMan) ForShard(shardID uint32) *TxMan {
	clone := *client
	clone.cfg.ShardID = shardID

	clone.rpc, _ = rpcclient.New(&rpcclient.ConnConfig{
		Params:       clone.cfg.Net,
		Host:         clone.cfg.RPC.Host,
		User:         clone.cfg.RPC.User,
		Pass:         clone.cfg.RPC.Pass,
		ShardID:      shardID,
		HTTPPostMode: true,
		DisableTLS:   true,
	}, nil)

	return &clone
}

func (client *TxMan) RPC() *rpcclient.Client {
	return client.rpc
}

type UTXOCollectorOpts struct {
	Offset          int64
	OffsetByChain   map[uint32]int64
	AllChains       bool
	Shards          []uint32
	FilterAddresses []string
}

func (client *TxMan) CollectUTXO(address string, offset int64) (txmodels.UTXORows, int64, error) {
	filter := map[string]bool{}
	if address != "" {
		filter[address] = true
	}

	index, count, err := client.CollectUTXOIndex(client.cfg.ShardID, offset, filter, nil)
	if err != nil {
		return nil, 0, err
	}

	return index.Rows(), count, nil
}

func (client *TxMan) CollectUTXOs(opts UTXOCollectorOpts) (map[uint32]txmodels.UTXORows, int64, error) {
	addressFilter := make(map[string]bool, len(opts.FilterAddresses))
	for _, addr := range opts.FilterAddresses {
		addressFilter[addr] = true
	}

	var shards []uint32
	if opts.AllChains {
		// index beacon chain also
		shards = append(shards, 0)
		shardsInfo, err := client.rpc.ListShards()
		if err != nil {
			return nil, 0, err
		}
		for id := range shardsInfo.Shards {
			shards = append(shards, id)
		}
	} else if len(opts.Shards) > 0 {
		shards = append(shards, opts.Shards...)
	} else {
		shards = append(shards, client.cfg.ShardID)
	}

	sort.Slice(shards, func(i, j int) bool { return shards[i] < shards[j] })

	offset := opts.Offset
	result := make(map[uint32]txmodels.UTXORows, len(shards))
	var total int64

	for _, shardID := range shards {
		if o, ok := opts.OffsetByChain[shardID]; ok {
			offset = o
		}

		index, count, err := client.CollectUTXOIndex(shardID, offset, addressFilter, nil)
		if err != nil {
			return nil, 0, err
		}

		result[shardID] = index.Rows()
		total += count
	}

	return result, total, nil
}

func (client *TxMan) CollectUTXOIndex(shardID uint32, offset int64,
	filter map[string]bool, index *txmodels.UTXOIndex) (*txmodels.UTXOIndex, int64, error) {
	if offset == 0 {
		offset = 1
	}

	noAddressFilter := len(filter) == 0

	maxHeight, err := client.rpc.ForShard(shardID).GetBlockCount()
	if err != nil {
		return nil, 0, err
	}

	if index == nil {
		index = txmodels.NewUTXOIndex()
	}
	for height := offset; height <= maxHeight; height++ {
		hash, err := client.rpc.ForShard(shardID).GetBlockHash(height)
		if err != nil {
			return nil, 0, err
		}

		var block *rpcclient.BlockResult
		if shardID == 0 {
			block, err = client.rpc.GetBeaconBlock(hash)
		} else {
			block, err = client.rpc.ForShard(shardID).GetShardBlock(hash)
		}

		if err != nil {
			return nil, 0, err
		}

		for _, msgTx := range block.Block.Transactions {
			for _, in := range msgTx.TxIn {
				if in.PreviousOutPoint.Hash.String() == zeroHash {
					continue
				}

				index.MarkUsed(in.PreviousOutPoint.Hash.String(),
					in.PreviousOutPoint.Index,
					shardID,
				)
			}

			for utxoID, out := range msgTx.TxOut {
				decodedScript, err := client.DecodeScript(out.PkScript)
				if err != nil {
					return nil, 0, err
				}

				for _, skAddress := range decodedScript.Addresses {
					if noAddressFilter || filter[skAddress] {
						index.AddUTXO(txmodels.UTXO{
							ShardID:    shardID,
							Address:    skAddress,
							Height:     height,
							TxHash:     msgTx.TxHash().String(),
							OutIndex:   uint32(utxoID),
							Value:      out.Value,
							Used:       false,
							PKScript:   hex.EncodeToString(out.PkScript),
							ScriptType: decodedScript.Type,
						})
					}
				}
			}

		}
	}

	return index, maxHeight, nil
}

func (client *TxMan) NetworkFeeProvider(shardID uint32) (int64, int64, error) {
	rate, err := client.NetworkFee(shardID)
	return 0, rate, err
}

// NetworkFee returns Satoshi/bytes.
// To get size of the tx in bytes use chaindata.GetTransactionWeight(tx).
func (client *TxMan) NetworkFee(shardID uint32) (int64, error) {
	fee, err := client.rpc.ForShard(shardID).
		EstimateSmartFee(3, &jaxjson.EstimateModeEconomical)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get feeRate")
	}

	amount := int64(*fee.SatoshiPerB)
	if amount < mempool.DefaultMinRelayTxFeeSatoshiPerByte {
		return mempool.DefaultMinRelayTxFeeSatoshiPerByte, nil
	}

	return amount, nil
}

func (client *TxMan) NewEADRegistrationTx(amountToLock int64, utxoPrv UTXOProvider,
	destinationsScripts ...[]byte) (*txmodels.Transaction, error) {
	if client.key == nil {
		return nil, errors.New("keys not set")
	}

	msgTx := wire.NewMsgTx(wire.TxVerEADAction)
	for _, destination := range destinationsScripts {
		msgTx.AddTxOut(wire.NewTxOut(amountToLock, destination))
	}

	feeRate, err := client.NetworkFee(client.cfg.ShardID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get feeRate")
	}
	expectedInCount := 2

prepareUTXO:
	fee := EstimateFee(expectedInCount, len(destinationsScripts), feeRate, true, 0) // ead always works in beacon
	amountToSpend := (amountToLock * int64(len(destinationsScripts))) + fee

	utxo, err := utxoPrv.SelectForAmount(amountToSpend, 0, client.key.Address.EncodeAddress())
	if err != nil {
		return nil, err
	}
	if len(utxo) > expectedInCount {
		expectedInCount = len(utxo) + 1
		goto prepareUTXO
	}

	sum := utxo.GetSum()
	change := sum - amountToSpend - fee
	if change > 0 {
		changeRcvScript, err := txscript.PayToAddrScript(client.key.AddressPubKey.AddressPubKeyHash())
		if err != nil {
			return nil, errors.Wrap(err, "unable to create P2A script for change")
		}

		msgTx.AddTxOut(wire.NewTxOut(change, changeRcvScript))
	}

	msgTx, err = client.addInputsAndSign(msgTx, utxo, false)
	if err != nil {
		return nil, err
	}

	return &txmodels.Transaction{
		TxHash:   msgTx.TxHash().String(),
		Source:   client.key.Address.String(),
		SignedTx: EncodeTx(msgTx),
		RawTX:    msgTx,
	}, nil
}

func (client *TxMan) NewTx(destination string, amount int64, utxoPrv UTXOProvider,
	redeemScripts ...string) (*txmodels.Transaction, error) {
	if client.key == nil {
		return nil, errors.New("keys not set")
	}

	feeRate, err := client.NetworkFee(client.cfg.ShardID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get feeRate")
	}
	expectedInCount := 1

prepareUTXO:
	fee := EstimateFee(expectedInCount, 1, feeRate, true, client.cfg.ShardID)
	fee = int64(0)

	draft := txmodels.DraftTx{
		Amount:     amount,
		NetworkFee: fee,
	}

	draft.UTXO, err = utxoPrv.SelectForAmount(amount+fee, client.cfg.ShardID,
		client.key.Address.EncodeAddress())
	if err != nil {
		return nil, errors.Wrap(err, "unable to select UTXO for amount")
	}

	if len(draft.UTXO) > expectedInCount {
		expectedInCount = len(draft.UTXO) + 1
		goto prepareUTXO
	}

	if draft.UTXO.GetSum() < amount+draft.NetworkFee {
		return nil, errors.New("not enough utxo value")
	}

	err = draft.SetPayToAddress(destination, client.NetParams)
	if err != nil {
		return nil, errors.Wrap(err, "pay to address not set")
	}

	msgTx, err := client.DraftToSignedTx(draft, false)
	if err != nil {
		return nil, errors.Wrap(err, "tx not signed")
	}

	if redeemScripts != nil {
		msgTx, err = client.AddSignatureToTx(msgTx, redeemScripts...)
		if err != nil {
			return nil, errors.Wrap(err, "unable to add signature to tx")
		}
	}

	return &txmodels.Transaction{
		TxHash:      msgTx.TxHash().String(),
		Source:      client.key.Address.String(),
		Destination: draft.Destination(),
		Amount:      amount,
		SignedTx:    EncodeTx(msgTx),
		ShardID:     client.cfg.ShardID,
		RawTX:       msgTx,
	}, nil
}

// DEPRECATED
// NewSwapTx creates new transaction with wire.TxVerCrossShardSwap version:
// 	- data is a map of <Destination Address> => <Source txmodels.UTXO>.
// 	- redeemScripts is optional, it allows to add proper signatures if source UTXO is a multisig address.
//
// SwapTx is a special tx for atomic swap between chains.
// It can contain only TWO or FOUR inputs and TWO or FOUR  outputs.
// wire.TxIn and wire.TxOut are strictly associated with each other by index.
// One pair corresponds to the one chain. The second is for another.
// | # | --- []TxIn ----- | --- | --- []TxOut ----- | # |
// | - | ---------------- | --- | ----------------- | - |
// | 0 | TxIn_0 ∈ Shard_X | --> | TxOut_0 ∈ Shard_X | 0 |
// | 1 | TxIn_1 ∈ Shard_Y | --> | TxOut_1 ∈ Shard_Y | 1 |
func (client *TxMan) NewSwapTx(spendingMap map[string]txmodels.UTXO, postVerify bool,
	redeemScripts ...string) (*txmodels.SwapTransaction, error) {
	if client.key == nil {
		return nil, errors.New("keys not set")
	}
	if len(spendingMap) != 2 && len(spendingMap) != 4 {
		return nil, errors.New("invalid size of spending map")
	}

	msgTx := wire.NewMsgTx(wire.TxVerCrossShardSwap)

	ind := 0
	outIndexes := map[string]int{}
	shards := make([]uint32, 0, len(spendingMap))
	for destination, utxo := range spendingMap {
		shards = append(shards, utxo.ShardID)
		fee, err := client.ForShard(utxo.ShardID).NetworkFee(utxo.ShardID)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get feeRate")
		}

		utxoTxHash, err := chainhash.NewHashFromStr(utxo.TxHash)
		if err != nil {
			return nil, errors.Wrap(err, "can not decode TxHash")
		}

		draft := txmodels.DraftTx{
			Amount:     utxo.Value - fee,
			NetworkFee: fee,
			UTXO:       []txmodels.UTXO{utxo},
		}

		err = draft.SetPayToAddress(destination, client.NetParams)
		if err != nil {
			return nil, errors.Wrap(err, "pay to address not set")
		}
		msgTx.AddTxOut(wire.NewTxOut(draft.Amount, draft.ReceiverScript))

		outPoint := wire.NewOutPoint(utxoTxHash, utxo.OutIndex)
		txIn := wire.NewTxIn(outPoint, nil, nil)

		if client.lockTime != 0 {
			msgTx.Version = wire.TxVerCrossShardSwap
			txIn.Sequence = blockchain.LockTimeToSequence(false, client.lockTime)
		}

		msgTx.AddTxIn(txIn)

		outIndexes[destination] = ind
		ind += 1
	}

	var err error
	for destination, utxo := range spendingMap {
		txInIndex := outIndexes[destination]
		utxo := utxo.ToShort()

		msgTx, _, err = client.SignUTXOForTx(msgTx, utxo, txInIndex, postVerify)
		if err != nil {
			return nil, errors.Wrap(err, "unable to sing utxo")
		}
	}

	if redeemScripts != nil {
		var err error
		msgTx, err = client.AddSignatureToSwapTx(msgTx, shards, redeemScripts...)
		if err != nil {
			return nil, errors.Wrap(err, "unable to add signature to tx")
		}
	}

	return &txmodels.SwapTransaction{
		TxHash:       msgTx.TxHash().String(),
		Source:       client.key.Address.String(),
		Destinations: outIndexes,
		SignedTx:     EncodeTx(msgTx),
		RawTX:        msgTx,
	}, nil
}

func (client *TxMan) DraftToSignedTx(data txmodels.DraftTx, postVerify bool) (*wire.MsgTx, error) {
	if client.key == nil {
		return nil, errors.New("keys not set")
	}

	msgTx := wire.NewMsgTx(wire.TxVersion)
	msgTx.AddTxOut(wire.NewTxOut(data.Amount, data.ReceiverScript))

	sum := data.UTXO.GetSum()
	change := sum - data.Amount - data.NetworkFee
	if change > 0 {
		changeRcvScript, err := txscript.PayToAddrScript(client.key.AddressPubKey.AddressPubKeyHash())
		if err != nil {
			return nil, errors.Wrap(err, "unable to create P2A script for change")
		}

		msgTx.AddTxOut(wire.NewTxOut(change, changeRcvScript))
	}

	return client.addInputsAndSign(msgTx, data.UTXO, postVerify)
}

func (client *TxMan) addInputsAndSign(msgTx *wire.MsgTx, data txmodels.UTXORows, postVerify bool) (*wire.MsgTx, error) {
	// msgTx := tx.Copy()

	for i := range data {
		txInIndex := i
		utxo := data[txInIndex]
		utxoTxHash, err := chainhash.NewHashFromStr(utxo.TxHash)
		if err != nil {
			return nil, errors.Wrap(err, "can not decode TxHash")
		}

		outPoint := wire.NewOutPoint(utxoTxHash, utxo.OutIndex)
		txIn := wire.NewTxIn(outPoint, nil, nil)
		if client.lockTime != 0 {
			msgTx.Version = client.txVersion
			txIn.Sequence = blockchain.LockTimeToSequence(false, client.lockTime)
		}
		msgTx.AddTxIn(txIn)
	}

	var err error
	for i := range data {
		txInIndex := i
		utxo := data[txInIndex].ToShort()

		msgTx, _, err = client.SignUTXOForTx(msgTx, utxo, txInIndex, postVerify)
		if err != nil {
			return nil, errors.Wrap(err, "unable to sing utxo")
		}
	}

	return msgTx, nil
}

func (client *TxMan) AddSignatureToSwapTx(msgTx *wire.MsgTx, shards []uint32,
	redeemScripts ...string) (*wire.MsgTx, error) {
	if client.key == nil {
		return nil, errors.New("keys not set")
	}

	scripts := make(map[string]scriptData, len(redeemScripts))
	for _, redeemScript := range redeemScripts {
		rawScript, err := hex.DecodeString(redeemScript)
		if err != nil {
			return nil, errors.Wrap(err, "unable to decode hex script")
		}

		script, err := client.DecodeScript(rawScript)
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse script")
		}

		scripts[script.P2sh] = scriptData{
			Type: script.Type,
			P2sh: script.P2sh,
			Hex:  redeemScript,
		}
	}

	for i := range msgTx.TxIn {
		txInIndex := i
		prevOut := msgTx.TxIn[i].PreviousOutPoint
		var txOut *jaxjson.GetTxOutResult
	shardsLoop:
		for _, shardID := range shards {
			out, _ := client.rpc.ForShard(shardID).GetTxOut(&prevOut.Hash, prevOut.Index, false, false)
			if out == nil {
				continue
			}
			txOut = out
			break shardsLoop
		}

		if txOut == nil {
			return nil, errors.New("unable to get utxo from node")
		}

		sType := txOut.ScriptPubKey.Type
		if sType != txscript.ScriptHashTy.String() &&
			sType != txscript.MultiSigTy.String() &&
			sType != txscript.MultiSigLockTy.String() {
			continue
		}

		value, _ := jaxutil.NewAmount(txOut.Value)
		utxo := txmodels.ShortUTXO{
			Value:    int64(value),
			PKScript: txOut.ScriptPubKey.Hex,
		}

		for _, address := range txOut.ScriptPubKey.Addresses {
			if script, ok := scripts[address]; ok {
				utxo.RedeemScript = script.Hex
				break
			}
		}
		var err error
		_, _, err = client.SignUTXOForTx(msgTx, utxo, txInIndex, false)
		if err != nil {
			return nil, errors.Wrap(err, "unable to sing utxo")
		}
	}

	return msgTx, nil
}

// AddSignatureToTx adds signature(s) to wire.TxIn in wire.MsgTx that spend coins with the txscript.MultiSigTy address.
// NOTE: this method don't work with SwapTx, use the AddSignatureToSwapTx.
func (client *TxMan) AddSignatureToTx(msgTx *wire.MsgTx, redeemScripts ...string) (*wire.MsgTx, error) {
	if client.key == nil {
		return nil, errors.New("keys not set")
	}
	type scriptData struct {
		Type string
		P2sh string
		Hex  string
	}

	// msgTx := tx

	scripts := make(map[string]scriptData, len(redeemScripts))
	for _, redeemScript := range redeemScripts {
		rawScript, err := hex.DecodeString(redeemScript)
		if err != nil {
			return nil, errors.Wrap(err, "unable to decode hex script")
		}

		script, err := client.DecodeScript(rawScript)
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse script")
		}

		scripts[script.P2sh] = scriptData{
			Type: script.Type,
			P2sh: script.P2sh,
			Hex:  redeemScript,
		}
	}

	skipped := 0
	for i := range msgTx.TxIn {
		txInIndex := i
		prevOut := msgTx.TxIn[i].PreviousOutPoint

		out, err := client.rpc.ForShard(client.cfg.ShardID).
			GetTxOut(&prevOut.Hash, prevOut.Index, false, false)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get utxo from node")
		}
		if out == nil {
			skipped += 1
			continue
		}

		value, _ := jaxutil.NewAmount(out.Value)

		utxo := txmodels.ShortUTXO{
			Value:    int64(value),
			PKScript: out.ScriptPubKey.Hex,
		}

		for _, address := range out.ScriptPubKey.Addresses {
			if script, ok := scripts[address]; ok {
				utxo.RedeemScript = script.Hex
				break
			}
		}

		_, _, err = client.SignUTXOForTx(msgTx, utxo, txInIndex, false)
		if err != nil {
			return nil, errors.Wrap(err, "unable to sing utxo")
		}
	}
	if skipped > 0 {
		return msgTx, fmt.Errorf("some inputs not signed, count(%d)", skipped)
	}

	return msgTx, nil
}

// SignUTXOForTx performs signing of UTXO, adds this signature to redeemTx.
// Method also supports signing of multiSig UTXOs, so just provide existing signature as prevScript
// 	- redeemTx is a transaction that will be sent
// 	- utxo is output that will be spent
// 	- inIndex is an index, where placed this UTXO
// 	- prevScript is a SignatureScript made by one or more previous key in case of multiSig UTXO, otherwise it nil
// 	- postVerify say to check tx after signing
func (client *TxMan) SignUTXOForTx(msgTx *wire.MsgTx, utxo txmodels.ShortUTXO,
	inIndex int, postVerify bool) (*wire.MsgTx, []byte, error) {
	if client.key == nil {
		return nil, nil, errors.New("keys not set")
	}

	pkScript, err := hex.DecodeString(utxo.PKScript)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to decode PK script")
	}

	var prevScript []byte = nil
	if msgTx.TxIn[inIndex].SignatureScript != nil {
		prevScript = msgTx.TxIn[inIndex].SignatureScript
	}

	var sig []byte
	sig, err = txscript.SignTxOutput(client.NetParams, msgTx, inIndex, pkScript,
		txscript.SigHashAll, client.key, &utxo, prevScript)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to sign tx output")
	}

	msgTx.TxIn[inIndex].SignatureScript = sig

	if postVerify {
		vm, err := txscript.NewEngine(pkScript, msgTx, inIndex,
			txscript.StandardVerifyFlags, nil, nil, utxo.Value)
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to init txScript engine")
		}

		if err = vm.Execute(); err != nil {
			return nil, nil, errors.Wrap(err, "tx script exec failed")
		}
	}

	return msgTx, sig, nil
}

func (client *TxMan) NewMultiSig2of2Address(firstPubKey, second string) (*MultiSigAddress, error) {
	return MakeMultiSigScript([]string{firstPubKey, second}, 2, client.NetParams)
}

func (client *TxMan) DecodeScript(script []byte) (*jaxjson.DecodeScriptResult, error) {
	return DecodeScript(script, client.NetParams)
}
