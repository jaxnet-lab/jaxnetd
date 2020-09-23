package txutils

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"gitlab.com/jaxnet/core/shard.core.git/btcjson"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core.git/rpcclient"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
)

const (
	zeroHash       = "0000000000000000000000000000000000000000000000000000000000000000"
	OneCoin  int64 = 100_000_000
)

type TxMan struct {
	cfg ManagerCfg
	key *KeyData

	NetParams *chaincfg.Params
	RPC       *rpcclient.Client

	testMode bool
}

func NewTxMan(cfg ManagerCfg) (*TxMan, error) {
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Params:       cfg.Net,
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
		cfg:       cfg,
		NetParams: cfg.NetParams(),
		RPC:       rpcClient,
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

func (client *TxMan) CollectUTXO(address string, offset int64) (txmodels.UTXORows, int64, error) {
	maxHeight, err := client.RPC.GetBlockCount()
	if err != nil {
		return nil, 0, err
	}

	index := txmodels.NewUTXOIndex()

	if offset == 0 {
		offset = 1
	}

	for height := offset; height <= maxHeight; height++ {
		hash, err := client.RPC.GetBlockHash(height)
		if err != nil {
			return nil, 0, err
		}

		block, err := client.RPC.GetBlock(hash)
		if err != nil {
			return nil, 0, err
		}

		// fmt.Printf("\rProcess block #%d", height)

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
					return nil, 0, err
				}

				for _, skAddress := range decodedScript.Addresses {
					if address == "" || address == skAddress {
						index.AddUTXO(txmodels.UTXO{
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

	// fmt.Printf("\nFound %d UTXOs for %s in blocks[%d, %d]\n", len(index.Rows()), address, offset, maxHeight)
	return index.Rows(), maxHeight, nil
}

func (client *TxMan) NetworkFee() (int64, error) {
	fee, err := client.RPC.EstimateSmartFee(3, &btcjson.EstimateModeEconomical)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get fee")
	}

	amount, _ := btcutil.NewAmount(*fee.FeeRate)
	return int64(amount), nil

}

func (client *TxMan) NewTx(destination string, amount int64, utxoPrv UTXOProvider) (*txmodels.Transaction, error) {
	if client.key == nil {
		return nil, errors.New("keys not set")
	}

	fee, err := client.NetworkFee()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get fee")
	}

	draft := txmodels.DraftTx{
		Amount:     amount,
		NetworkFee: fee,
	}

	draft.UTXO, err = utxoPrv.SelectForAmount(amount + draft.NetworkFee)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get UTXO for amount")
	}

	err = draft.SetPayToAddress(destination, client.NetParams)
	if err != nil {
		return nil, errors.Wrap(err, "pay to address not set")
	}

	msgTx, err := client.DraftToSignedTx(draft, false)
	if err != nil {
		return nil, errors.Wrap(err, "tx not signed")
	}

	return &txmodels.Transaction{
		TxHash:      msgTx.TxHash().String(),
		Source:      client.key.Address.String(),
		Destination: draft.Destination(),
		Amount:      amount,
		SignedTx:    EncodeTx(msgTx),
		RawTX:       msgTx,
	}, nil
}

func (client *TxMan) _NewMultiSig2of2Tx(fistSigner, secondSigner *btcutil.AddressPubKey,
	amount int64, utxoPrv UTXOProvider) (txmodels.Transaction, error) {
	if client.key == nil {
		return txmodels.Transaction{}, errors.New("keys not set")
	}

	fee, err := client.NetworkFee()
	if err != nil {
		return txmodels.Transaction{}, errors.Wrap(err, "unable to get fee")
	}

	draft := txmodels.DraftTx{
		Amount:     amount,
		NetworkFee: fee,
	}

	draft.UTXO, err = utxoPrv.SelectForAmount(amount + draft.NetworkFee)
	if err != nil {
		return txmodels.Transaction{}, errors.Wrap(err, "unable to get UTXO for amount")
	}

	err = draft.SetMultiSig2of2(fistSigner, secondSigner, client.NetParams)
	if err != nil {
		return txmodels.Transaction{}, errors.Wrap(err, "multiSig 2of2 not set")
	}

	msgTx, err := client.DraftToSignedTx(draft, false)
	if err != nil {
		return txmodels.Transaction{}, errors.Wrap(err, "tx not signed")
	}

	return txmodels.Transaction{
		TxHash:      msgTx.TxHash().String(),
		Destination: draft.Destination(),
		Source:      client.key.Address.String(),
		Amount:      amount,
		SignedTx:    EncodeTx(msgTx),
		RawTX:       msgTx,
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
	if change != 0 {
		changeRcvScript, err := txscript.PayToAddrScript(client.key.AddressPubKey.AddressPubKeyHash())
		if err != nil {
			return nil, errors.Wrap(err, "unable to create P2A script for change")
		}

		msgTx.AddTxOut(wire.NewTxOut(change, changeRcvScript))
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
		msgTx.AddTxIn(txIn)
	}

	for i := range data.UTXO {
		txInIndex := i
		utxo := data.UTXO[txInIndex].ToShort()

		inputAmount := utxo.Value
		if tempSum < inputAmount {
			inputAmount = tempSum
		}

		_, err := client.SignUTXOForTx(msgTx, utxo, txInIndex, postVerify)
		if err != nil {
			return nil, errors.Wrap(err, "unable to sing utxo")
		}

		tempSum -= inputAmount
	}

	return msgTx, nil
}

func (client *TxMan) AddSignatureToTx(msgTx *wire.MsgTx, redeemScripts ...string) (*wire.MsgTx, error) {
	if client.key == nil {
		return nil, errors.New("keys not set")
	}
	type scriptData struct {
		Type string
		P2sh string
		Hex  string
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

		out, err := client.RPC.GetTxOut(&prevOut.Hash, prevOut.Index, false)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get utxo from node")
		}
		if out == nil {
			// todo(mike): validate correctness
			continue
		}

		value, _ := btcutil.NewAmount(out.Value)

		utxo := txmodels.ShortUTXO{
			Value:      int64(value),
			PKScript:   out.ScriptPubKey.Hex,
			ScriptType: out.ScriptPubKey.Type,
		}

		for _, address := range out.ScriptPubKey.Addresses {
			if script, ok := scripts[address]; ok {
				utxo.RedeemScript = script.Hex
				break
			}
		}

		_, err = client.SignUTXOForTx(msgTx, utxo, txInIndex, false)
		if err != nil {
			return nil, errors.Wrap(err, "unable to sing utxo")
		}
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
func (client *TxMan) SignUTXOForTx(msgTx *wire.MsgTx, utxo txmodels.ShortUTXO, inIndex int, postVerify bool) ([]byte, error) {
	if client.key == nil {
		return nil, errors.New("keys not set")
	}

	pkScript, err := hex.DecodeString(utxo.PKScript)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode PK script")
	}

	var prevScript []byte = nil
	if msgTx.TxIn[inIndex].SignatureScript != nil {
		prevScript = msgTx.TxIn[inIndex].SignatureScript
	}

	var sig []byte
	sig, err = txscript.SignTxOutput(client.NetParams, msgTx, inIndex, pkScript,
		txscript.SigHashAll, client.key, &utxo, prevScript)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign tx output")
	}

	msgTx.TxIn[inIndex].SignatureScript = sig

	if postVerify {
		vm, err := txscript.NewEngine(pkScript, msgTx, inIndex,
			txscript.ScriptBip16|txscript.ScriptVerifyDERSignatures, nil, nil, utxo.Value)
		if err != nil {
			return nil, errors.Wrap(err, "unable to init txScript engine")
		}

		if err = vm.Execute(); err != nil {
			return nil, errors.Wrap(err, "tx script exec failed")
		}
	}

	return sig, nil
}

func (client *TxMan) NewMultiSig2of2Address(firstPubKey, second string) (*MultiSigAddress, error) {
	return MakeMultiSigScript([]string{firstPubKey, second}, 2, client.NetParams)
}

func (client *TxMan) DecodeScript(script []byte) (*btcjson.DecodeScriptResult, error) {
	return DecodeScript(script, client.NetParams)
}
