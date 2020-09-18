package txutils

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/chaincfg/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
)

func TestAddSignatureToTx(ot *testing.T) {
	t := (*T)(ot)

	cfg := ManagerCfg{Net: "testnet"}
	inputTxHashStr := "02cf5a871a3000c01ef41416c453473842311288d950b3ba2a609d33cea83d37"
	data, _ := hex.DecodeString(inputTxHashStr)
	inputTxHash, _ := chainhash.NewHash(data)
	inIndex := uint32(0)
	amount := 9 * OneCoin

	aliceSk := "6443fb332e1cbfe456674aacf2be1327b6f9fc9c782061ee04ca35e17608d651"
	aliceKP, err := NewKeyData(aliceSk, cfg.NetParams())
	assert.NoError(t, err)

	bobSk := "6bb4b4a9d5512c84f14bd38248dafb80c2424ae50a0495be8e4f657d734f1bd4"
	bobKP, err := NewKeyData(bobSk, cfg.NetParams())
	assert.NoError(t, err)

	signers := []string{
		aliceKP.AddressPubKey.String(),
		bobKP.AddressPubKey.String(),
	}

	multiSigScript, err := MakeMultiSigScript(signers, len(signers), cfg.NetParams())
	assert.NoError(t, err)
	t.Log(multiSigScript.Address)

	scriptAddr, err := btcutil.NewAddressScriptHash(
		multiSigScript.RawRedeemScript, &chaincfg.TestNet3Params)
	assert.NoError(t, err)

	scriptPkScript, err := txscript.PayToAddrScript(scriptAddr)
	assert.NoError(t, err)

	msgTx := wire.NewMsgTx(wire.TxVersion)
	outPoint := wire.NewOutPoint(inputTxHash, inIndex)
	txIn := wire.NewTxIn(outPoint, nil, nil)
	msgTx.AddTxIn(txIn)

	evaDest, _ := hex.DecodeString("76a914b2629111cf79c2f1cd025a7aebc403fc9bb5d48b88ac")
	txOut := wire.NewTxOut(amount, evaDest)
	msgTx.AddTxOut(txOut)

	msgTx.TxIn[0].SignatureScript = nil

	var sig []byte
	sig, err = txscript.SignTxOutput(cfg.NetParams(), msgTx, int(inIndex), scriptPkScript,
		txscript.SigHashAll, aliceKP, txscript.ScriptClosure(func(address btcutil.Address) ([]byte, error) {
			return multiSigScript.RawRedeemScript, nil
		}), nil)
	assert.NoError(t, err)

	msgTx.TxIn[inIndex].SignatureScript = sig

	sig, err = txscript.SignTxOutput(cfg.NetParams(), msgTx, int(inIndex), scriptPkScript,
		txscript.SigHashAll, bobKP, txscript.ScriptClosure(func(address btcutil.Address) ([]byte, error) {
			return multiSigScript.RawRedeemScript, nil
		}), sig)
	assert.NoError(t, err)
	msgTx.TxIn[inIndex].SignatureScript = sig

	// class, addresses, count, err := txscript.ExtractPkScriptAddrs(sig, cfg.ChainParams())
	// assert.NoError(t, err)
	// assert.Equal(t, count, 2)
	// assert.Equal(t, len(addresses), 2)
	// fmt.Println(class)

	fmt.Println(EncodeTx(msgTx))
	vm, err := txscript.NewEngine(multiSigScript.RawRedeemScript, msgTx, int(inIndex),
		txscript.StandardVerifyFlags, nil, nil, amount)
	assert.NoError(t, err)

	err = vm.Execute()
	assert.NoError(t, err)

}
