package txmanager

import (
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core.git/btcjson"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmanager/models"
	"gitlab.com/jaxnet/core/shard.core.git/rpcclient"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
	"gitlab.com/jaxnet/core/shard.core.git/wire"
)

const (
	zeroHash       = "0000000000000000000000000000000000000000000000000000000000000000"
	OneCoin  int64 = 100_000_000
)

type TxMan struct {
	cfg        ManagerCfg
	key        *KeyData
	networkCfg *chaincfg.Params

	RPC *rpcclient.Client
}

func NewTxMan(cfg ManagerCfg) (*TxMan, error) {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.RPC.Host,
		User:         cfg.RPC.User,
		Pass:         cfg.RPC.Pass,
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}

	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	rpcClient, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}

	client := &TxMan{
		cfg:        cfg,
		networkCfg: cfg.NetParams(),
		RPC:        rpcClient,
	}

	client.key, err = NewKeyData(client.cfg.PrivateKey, client.networkCfg)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (client *TxMan) SetKeys(key *KeyData) {
	client.key = key
}

func (client *TxMan) CollectUTXO(address string, offset int64) (models.UTXORows, error) {
	maxHeight, err := client.RPC.GetBlockCount()
	if err != nil {
		return nil, err
	}

	index := models.NewUTXOIndex()

	if offset == 0 {
		offset = 1
	}

	fmt.Printf("Start processing...")
	for height := offset; height <= maxHeight; height++ {
		hash, err := client.RPC.GetBlockHash(height)
		if err != nil {
			return nil, err
		}

		block, err := client.RPC.GetBlock(hash)
		if err != nil {
			return nil, err
		}

		fmt.Printf("\rProcess block #%d", height)

		for _, msgTx := range block.Transactions {
			for _, in := range msgTx.TxIn {
				if in.PreviousOutPoint.Hash.String() == zeroHash {
					continue
				}

				index.MarkUsed(in.PreviousOutPoint.Hash.String(), in.PreviousOutPoint.Index)
			}

			for utxoID, out := range msgTx.TxOut {
				decodedScript, err := client.DecodeScript(out.PkScript)
				if err != nil {
					return nil, err
				}

				for _, skAddress := range decodedScript.Addresses {
					if address == skAddress {
						index.AddUTXO(models.UTXO{
							Address:    address,
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

	fmt.Printf("\nFound %d UTXOs for %s in blocks[%d, %d]\n", len(index.Rows()), address, offset, maxHeight)
	return index.Rows(), nil
}

func (client *TxMan) NewTx(destination string, amount int64, utxoPrv UTXOProvider) (models.Transaction, error) {
	utxo, err := utxoPrv.SelectForAmount(amount)
	if err != nil {
		return models.Transaction{}, errors.Wrap(err, "unable to get UTXO for amount")
	}

	fee, err := client.RPC.EstimateFee(6)
	if err != nil {
		return models.Transaction{}, errors.Wrap(err, "unable to get fee")
	}

	draft := models.DraftTx{
		Amount:     amount,
		NetworkFee: int64(fee * float64(OneCoin)),
		UTXO:       utxo,
	}
	err = draft.SetPayToAddress(destination, client.networkCfg)
	if err != nil {
		return models.Transaction{}, errors.Wrap(err, "pay to address not set")
	}

	wireTx, err := client.DraftToSignedTx(draft)
	if err != nil {
		return models.Transaction{}, errors.Wrap(err, "tx not signed")
	}

	return models.Transaction{
		TxHash:      wireTx.TxHash().String(),
		Source:      client.key.Address.String(),
		Destination: draft.Destination(),
		Amount:      amount,
		RawTX:       wireTx,
	}, nil
}

func (client *TxMan) NewMultiSig2of2Tx(fistSigner, secondSigner *btcutil.AddressPubKey, amount int64, utxoPrv UTXOProvider) (models.Transaction, error) {
	utxo, err := utxoPrv.SelectForAmount(amount)
	if err != nil {
		return models.Transaction{}, errors.Wrap(err, "unable to get UTXO for amount")
	}

	fee, err := client.RPC.EstimateFee(6)
	if err != nil {
		return models.Transaction{}, errors.Wrap(err, "unable to get fee")
	}

	draft := models.DraftTx{
		Amount:     amount,
		NetworkFee: int64(fee * float64(OneCoin)),
		UTXO:       utxo,
	}
	err = draft.SetMultiSig2of2(fistSigner, secondSigner, client.networkCfg)
	if err != nil {
		return models.Transaction{}, errors.Wrap(err, "multiSig 2of2 not set")
	}

	wireTx, err := client.DraftToSignedTx(draft)
	if err != nil {
		return models.Transaction{}, errors.Wrap(err, "tx not signed")
	}

	return models.Transaction{
		TxHash:      wireTx.TxHash().String(),
		Destination: draft.Destination(),
		Source:      client.key.Address.String(),
		Amount:      amount,
		RawTX:       wireTx,
	}, nil
}

func (client *TxMan) DraftToSignedTx(data models.DraftTx) (*wire.MsgTx, error) {
	redeemTx := wire.NewMsgTx(wire.TxVersion)
	redeemTx.AddTxOut(wire.NewTxOut(data.Amount, data.ReceiverScript))

	sum := data.UTXO.GetSum()
	change := sum - data.Amount - data.NetworkFee
	if change != 0 {
		changeRcvScript, err := txscript.PayToAddrScript(client.key.AddressPubKey.AddressPubKeyHash())
		if err != nil {
			return nil, errors.Wrap(err, "unable to create P2A script for change")
		}

		redeemTx.AddTxOut(wire.NewTxOut(change, changeRcvScript))
	}

	tempSum := data.Amount + change
	for i := range data.UTXO {
		txInIndex := i
		utxo := data.UTXO[txInIndex]
		utxoTxHash, err := chainhash.NewHashFromStr(utxo.TxHash)
		if err != nil {
			return nil, errors.Wrap(err, "can not decode TxHash")
		}

		outPoint := wire.NewOutPoint(utxoTxHash, utxo.OutIndex)
		txIn := wire.NewTxIn(outPoint, nil, nil)
		redeemTx.AddTxIn(txIn)
	}

	for i := range data.UTXO {
		txInIndex := i
		utxo := data.UTXO[txInIndex]

		inputAmount := utxo.Value
		if tempSum < inputAmount {
			inputAmount = tempSum
		}

		_, err := client.SignUTXOForTx(redeemTx, utxo, txInIndex, nil, true)
		if err != nil {
			return nil, errors.Wrap(err, "unable to sing utxo")
		}

		tempSum -= inputAmount
	}

	return redeemTx, nil
}

// SignUTXOForTx performs signing of UTXO, adds this signature to redeemTx.
// Method also supports signing of multiSig UTXOs, so just provide existing signature as prevScript
// 	- redeemTx is a transaction that will be sent
// 	- utxo is output that will be spent
// 	- inIndex is an index, where placed this UTXO
// 	- prevScript is a SignatureScript made by one or more previous key in case of multiSig UTXO, otherwise it nil
// 	- postVerify say to check tx after signing
func (client *TxMan) SignUTXOForTx(redeemTx *wire.MsgTx, utxo models.UTXO, inIndex int, prevScript []byte, postVerify bool) ([]byte, error) {
	pkScript, err := hex.DecodeString(utxo.PKScript)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create P2A script")
	}

	var sig []byte
	sig, err = txscript.SignTxOutput(client.networkCfg, redeemTx, inIndex, pkScript,
		txscript.SigHashSingle, client.key, &utxo, prevScript)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign tx output")
	}
	redeemTx.TxIn[inIndex].SignatureScript = sig

	if postVerify {
		vm, err := txscript.NewEngine(pkScript, redeemTx, inIndex,
			txscript.StandardVerifyFlags, nil, nil, utxo.Value)
		if err != nil {
			return nil, errors.Wrap(err, "unable to init txScript engine")
		}

		if err = vm.Execute(); err != nil {
			return nil, errors.Wrap(err, "tx script exec failed")
		}
	}

	return sig, nil
}

func (client *TxMan) DecodeScript(script []byte) (*btcjson.DecodeScriptResult, error) {
	// The disassembled string will contain [error] inline if the script
	// doesn't fully parse, so ignore the error here.
	asm, _ := txscript.DisasmString(script)

	// Get information about the script.
	// Ignore the error here since an error means the script couldn't parse
	// and there is no additional information about it anyways.
	scriptClass, address, reqSigns, _ := txscript.ExtractPkScriptAddrs(script, client.networkCfg)
	addresses := make([]string, len(address))
	for i, addr := range address {
		addresses[i] = addr.EncodeAddress()
	}

	// Convert the script itself to a pay-to-script-hash address.
	p2sh, err := btcutil.NewAddressScriptHash(script, client.networkCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert script to pay-to-script-hash")
	}

	// Generate and return the reply.
	reply := &btcjson.DecodeScriptResult{
		Asm:       asm,
		ReqSigs:   int32(reqSigns),
		Type:      scriptClass.String(),
		Addresses: addresses,
	}
	if scriptClass != txscript.ScriptHashTy {
		reply.P2sh = p2sh.EncodeAddress()
	}

	return reply, nil
}
