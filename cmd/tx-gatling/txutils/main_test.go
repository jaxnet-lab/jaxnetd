// + build integration_test

package txutils

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
)

type T testing.T

func (t *T) Errorf(format string, args ...interface{}) { t.Fatalf(format, args...) }

// func TestMain(m *testing.M) {
//
// }

func TestOperator_SpendUTXO(t *testing.T) {
	cfg := ManagerCfg{
		Net: "testnet",
		RPC: NodeRPC{
			Host: "127.0.0.1:28334",
			User: "somerpc",
			Pass: "somerpc",
		},
		PrivateKey: "",
	}
	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	aliceAddr := "myGdvt6vRNgrFZFtU5FNhW5gxwRqKBcLGv"
	txHash := "8e8de99c0bf81f95b010e53f74bfd2c4d608227938f279954f062185be052cd6"
	kd, err := NewKeyData("3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b", cfg.NetParams())
	assert.NoError(t, err)

	tx, err := op.SpendUTXO(*kd, txHash, 0, aliceAddr, 1_0000_0000)
	assert.NoError(t, err)
	assert.NotNil(t, tx)
}

func TestMakeMultiSigScript(ot *testing.T) {
	t := (*T)(ot)

	aliceSk := "6443fb332e1cbfe456674aacf2be1327b6f9fc9c782061ee04ca35e17608d651"
	bobSk := "6bb4b4a9d5512c84f14bd38248dafb80c2424ae50a0495be8e4f657d734f1bd4"
	evaAddress := "mwnAejT1i6Fra7npajqEe6G3A22DFbU5aK"
	utxoSearchOffset := 0
	amount := int64(1000000000) - OneCoin
	// -----------------------------------------------------------------------------------------
	// ---/---- PREPARE ----\----
	cfg := ManagerCfg{
		Net: "testnet",
		RPC: NodeRPC{
			Host: "116.202.107.209:18334",
			User: "somerpc",
			Pass: "somerpc",
		},
		PrivateKey: "",
	}

	aliceKP, err := NewKeyData(aliceSk, cfg.NetParams())
	assert.NoError(t, err)

	bobKP, err := NewKeyData(bobSk, cfg.NetParams())
	assert.NoError(t, err)

	op, err := NewOperator(cfg)
	assert.NoError(t, err)

	aliceUTXO, _, err := op.TxMan.CollectUTXO(aliceKP.Address.EncodeAddress(), int64(utxoSearchOffset))
	assert.NoError(t, err)
	// -----------------------------------------------------------------------------------------

	// -----------------------------------------------------------------------------------------
	// ---/---- CREATE MULTISIG ADDRESS ----\----
	signers := []string{
		aliceKP.AddressPubKey.String(),
		bobKP.AddressPubKey.String(),
	}

	multiSigScript, err := MakeMultiSigScript(signers, len(signers), cfg.NetParams())
	assert.NoError(t, err)
	t.Log(multiSigScript.Address)
	// -----------------------------------------------------------------------------------------

	// -----------------------------------------------------------------------------------------
	// ---/---- SEND COINS TO MULTISIG ADDRESS  ----\----
	toMultiSigAddrTx, err := op.TxMan.WithKeys(aliceKP).NewTx(multiSigScript.Address, amount, UTXOFromRows(aliceUTXO))
	assert.NoError(t, err)

	// publish created transaction
	txHash, err := op.TxMan.RPC.SendRawTransaction(toMultiSigAddrTx.RawTX, true)
	assert.NoError(t, err)

waitLoop:
	for {
		// wait for the transaction to be added to the block
		out, err := op.TxMan.RPC.GetTxOut(txHash, 0, false)
		assert.NoError(t, err)
		if out != nil && out.Confirmations > 1 {
			fmt.Println("tx mined into block")
			break waitLoop
		}

		time.Sleep(time.Second)
	}
	// -----------------------------------------------------------------------------------------

	// -----------------------------------------------------------------------------------------
	// ---/---- CREATE TX TO SPEND MULTISIG UTXO  ----\----
	multiSigOutTxHash := toMultiSigAddrTx.TxHash
	multiSigOutIndex := uint32(0)
	for i, out := range toMultiSigAddrTx.RawTX.TxOut {
		decoded, _ := DecodeScript(out.PkScript, cfg.NetParams())
		for _, address := range decoded.Addresses {
			if address == multiSigScript.Address {
				multiSigOutIndex = uint32(i)
			}
		}
	}
	amount = amount - OneCoin
	multiSigSpendTx, err := op.NewMultiSigSpendTx(*aliceKP, multiSigOutTxHash,
		multiSigScript.RedeemScript, multiSigOutIndex, evaAddress, amount)
	assert.NoError(t, err)
	// -----------------------------------------------------------------------------------------

	// -----------------------------------------------------------------------------------------
	// ---/---- ADD SECOND SIGNATURE TO SPEND MULTISIG UTXO TX ----\----
	multiSigSpendTx, err = op.AddSignatureToTx(*aliceKP, multiSigSpendTx.SignedTx, multiSigScript.RedeemScript)
	assert.NoError(t, err)

	multiSigSpendTx, err = op.AddSignatureToTx(*bobKP, multiSigSpendTx.SignedTx, multiSigScript.RedeemScript)
	assert.NoError(t, err)
	// -----------------------------------------------------------------------------------------

	fmt.Println(EncodeTx(multiSigSpendTx.RawTX))
	vm, err := txscript.NewEngine(multiSigScript.RawRedeemScript, multiSigSpendTx.RawTX, 0,
		txscript.StandardVerifyFlags, nil, nil, amount)
	assert.NoError(t, err)

	err = vm.Execute()
	// assert.NoError(t, err)
	// -----------------------------------------------------------------------------------------

	// ---/---- SUBMIT MULTI SIG UTXO TX ----\----
	// publish created transaction
	txHash, err = op.TxMan.RPC.SendRawTransaction(multiSigSpendTx.RawTX, true)
	assert.NoError(t, err)

	for {
		// wait for the transaction to be added to the block
		out, err := op.TxMan.RPC.GetTxOut(txHash, 0, false)
		assert.NoError(t, err)
		if out != nil && out.Confirmations > 2 {
			println("tx mined into block")
			break
		}

		time.Sleep(time.Second)
	}
	// -----------------------------------------------------------------------------------------
}
