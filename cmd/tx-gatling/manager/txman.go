package manager

import (
	"errors"
	"fmt"

	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/rpcclient"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
	"gitlab.com/jaxnet/core/shard.core.git/wire"
)

const (
	zeroHash       = "0000000000000000000000000000000000000000000000000000000000000000"
	OneCoin  int64 = 100_000_000
)

type TxMan struct {
	*rpcclient.Client

	cfg        ClientCfg
	key        *KeyData
	networkCfg *chaincfg.Params

	repo *storage.CSVStorage
}

func NewTxMan(cfg ClientCfg) (*TxMan, error) {
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
		Client:     rpcClient,
		cfg:        cfg,
		networkCfg: cfg.NetParams(),
	}

	client.key, err = NewKeyData(client.cfg.PrivateKey, client.networkCfg)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (client *TxMan) CollectUTXO(address string) error {
	maxHeight, err := client.GetBlockCount()
	if err != nil {
		return err
	}

	index := storage.NewUTXOIndex()

	for height := int64(1); height <= maxHeight; height++ {
		hash, err := client.GetBlockHash(height)
		if err != nil {
			return err
		}
		fmt.Printf("Got new hash %s for height %d\n", hash, height)

		block, err := client.GetBlock(hash)
		if err != nil {
			return err
		}

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
					return err
				}

				for _, s := range decodedScript.Addresses {
					if address == s {
						index.AddUTXO(storage.UTXO{
							Address:  address,
							Height:   height,
							TxHash:   msgTx.TxHash().String(),
							OutIndex: uint32(utxoID),
							Value:    out.Value,
							Used:     false,
						})
					}
				}
			}

		}
	}

	repo := storage.NewCSVStorage(client.cfg.DataFile, client.cfg.Append)
	defer repo.Shutdown()
	return repo.SaveRows(index.Rows())
}

func (client *TxMan) CreateTransaction(destination string, amount int64) (*wire.MsgTx, Transaction, error) {
	utxo, err := client.getUTXOForAmount(amount)
	if err != nil {
		return nil, Transaction{}, err
	}

	fee, err := client.EstimateFee(6)
	if err != nil {
		return nil, Transaction{}, err
	}

	draft := DraftTx{
		Amount:     amount,
		NetworkFee: int64(fee * float64(OneCoin)),
		UTXO:       utxo,
	}

	wireTx, err := client.signTx(draft)

	hashRes, err := client.SendRawTransaction(wireTx, true)
	if err != nil {
		return nil, Transaction{}, err
	}

	return wireTx, Transaction{
		TxId:        hashRes.String(),
		Source:      client.key.Address.String(),
		Destination: destination,
		Amount:      amount,
	}, nil
}

func (client *TxMan) Create2of2Transaction(fistSigner, secondSigner string, amount int64) (*wire.MsgTx, Transaction, error) {
	utxo, err := client.getUTXOForAmount(amount)
	if err != nil {
		return nil, Transaction{}, err
	}

	fee, err := client.EstimateFee(6)
	if err != nil {
		return nil, Transaction{}, err
	}

	draft := DraftTx{
		Amount:     amount,
		NetworkFee: int64(fee * float64(OneCoin)),
		UTXO:       utxo,
	}
	err = draft.SetMultiSig2of2(fistSigner, secondSigner, client.networkCfg)
	if err != nil {
		return nil, Transaction{}, err
	}

	wireTx, err := client.signTx(draft)
	if err != nil {
		return nil, Transaction{}, err
	}

	hashRes, err := client.SendRawTransaction(wireTx, true)
	if err != nil {
		return nil, Transaction{}, err
	}

	return wireTx, Transaction{
		TxId:        hashRes.String(),
		Source:      client.key.Address.String(),
		Destination: fistSigner,
		Amount:      amount,
	}, nil
}

func (client *TxMan) getUTXOForAmount(amount int64) (storage.UTXORows, error) {
	repo := storage.NewCSVStorage(client.cfg.DataFile, true)
	defer repo.Shutdown()

	rows, err := repo.FetchData()
	if err != nil {
		return nil, err
	}

	return rows.CollectForAmount(amount), nil
}

func (client *TxMan) signTx(data DraftTx) (*wire.MsgTx, error) {
	sum := data.UTXO.GetSum()
	change := sum - data.Amount - data.NetworkFee

	redeemTx := wire.NewMsgTx(wire.TxVersion)
	for _, u := range data.UTXO {
		var hash *chainhash.Hash
		hash, err := chainhash.NewHashFromStr(u.TxHash)
		if err != nil {
			return nil, err
		}

		outPoint := wire.NewOutPoint(hash, u.OutIndex)
		txIn := wire.NewTxIn(outPoint, nil, nil)
		redeemTx.AddTxIn(txIn)
	}

	txOut := wire.NewTxOut(data.Amount, data.ReceiverScript)
	redeemTx.AddTxOut(txOut)
	if change != 0 {
		var changeRcvScript []byte
		changeRcvScript, err := txscript.PayToAddrScript(client.key.Address)
		if err != nil {
			return nil, err
		}

		changeTxOut := wire.NewTxOut(change, changeRcvScript)
		redeemTx.AddTxOut(changeTxOut)
	}

	tempSum := data.Amount + change
	for i, utxo := range data.UTXO {
		inputAmount := utxo.Value
		if tempSum < inputAmount {
			inputAmount = tempSum
			// tempSum = 0
		}

		_, err := client.SignUTXOForTx(redeemTx, utxo, i, nil, true)
		if err != nil {
			return nil, err
		}

		tempSum -= inputAmount
	}

	return redeemTx, nil
}

// SignUTXOForTx performs signing of UTXO, adds this signature to redeemTx.
// Method also supports signing of multiSig UTXOs, so just provide existing signature as prevScript
func (client *TxMan) SignUTXOForTx(redeemTx *wire.MsgTx, utxo storage.UTXO,
	inIndex int, prevScript []byte, postVerify bool) ([]byte, error) {
	address, err := btcutil.DecodeAddress(utxo.Address, client.networkCfg)
	if err != nil {
		return nil, err
	}

	var pkScript []byte
	pkScript, err = txscript.PayToAddrScript(address)
	if err != nil {
		return nil, err
	}

	sig, err := txscript.SignTxOutput(client.networkCfg, redeemTx, inIndex, pkScript,
		txscript.SigHashAll, client.key, NewScriptsDB(utxo.Address, pkScript), prevScript)

	// var sig []byte
	// sig, err = txscript.SignatureScript(redeemTx, i, pkScript, txscript.SigHashAll, client.key.PrivateKey, false)
	if err != nil {
		return nil, err
	}

	redeemTx.TxIn[inIndex].SignatureScript = sig

	if postVerify {
		vm, err := txscript.NewEngine(pkScript, redeemTx, inIndex,
			txscript.StandardVerifyFlags, nil, nil, utxo.Value)
		if err != nil {
			return nil, err
		}

		if err = vm.Execute(); err != nil {
			return nil, err
		}
	}

	return sig, nil
}

func NewScriptsDB(scAddress string, script []byte) txscript.ScriptDB {
	return txscript.ScriptClosure(func(address btcutil.Address) ([]byte, error) {
		if scAddress != address.EncodeAddress() {
			return nil, errors.New("nope")
		}
		return script, nil
	})
}
