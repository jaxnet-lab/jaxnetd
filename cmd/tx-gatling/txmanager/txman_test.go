package txmanager

import (
	"encoding/hex"
	"log"

	"gitlab.com/jaxnet/core/shard.core.git/chaincfg"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmanager/models"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
)

func Example() {
	// bobSecret is hex-encoded private key
	bobSecret := "9e3f8b9b5e3698fa5a8b4b927eecec88fc36a91c14963d89e41747d1acf89dfb"
	aliceSecret := "6e38943e714b5b3ead017ab003939366e798b07dff5f9e12de913d7388aa572c"
	// bob :="825e97f8c0d485bef4f39292f6b7152657fc7972e5bf5a23d07eef54924ed88c"

	// address is btcutil.Address in format according to current network
	address := "mg6co2k1wBug21jvCVbm8YoFfSmDQxMwZW"

	// create new instance of tx manager,
	txManager, err := NewTxMan(ManagerCfg{
		Net: "testnet",
		RPC: NodeRPC{
			Host: "127.0.0.1:18334",
			User: "somerpc",
			Pass: "somerpc",
		},
		PrivateKey: bobSecret,
	})
	if err != nil {
		log.Fatal("unable to init txManager:", err)
	}

	utxo, err := txManager.CollectUTXO(address, 0)
	if err != nil {
		log.Fatal("unable to collect utxo:", err)
	}

	bobKP, err := NewKeyData(bobSecret, &chaincfg.TestNet3Params)
	if err != nil {
		log.Fatal("unable to create bobKP:", err)
	}

	aliceKP, err := NewKeyData(aliceSecret, &chaincfg.TestNet3Params)
	if err != nil {
		log.Fatal("unable to create aliceKP:", err)
	}

	sentTx, err := txManager.NewMultiSig2of2Tx(
		bobKP.AddressPubKey,
		aliceKP.AddressPubKey,
		100*OneCoin,
		UTXOFromRows(utxo))
	if err != nil {
		log.Fatal("unable to collect utxo:", err)
	}

	hashRes, err := txManager.RPC.SendRawTransaction(sentTx.RawTX, true)
	if err != nil {
		log.Fatal("tx not sent:", err)
	}

	log.Println("Sent new Tx:", hashRes.String())

	// tx, err  := txManager.RPC.GetTransaction(hashRes)

	// here is multiSig UTXO from tx, that we send
	multisigTxOut := sentTx.RawTX.TxOut[0]

	script, err := txManager.DecodeScript(multisigTxOut.PkScript)
	if err != nil {
		log.Fatal("can not decod script:", err)
	}

	// a way to check that it is correct
	{
		if script.Type != txscript.MultiSigTy.String() {
			log.Fatal("wrong script")
		}

		if len(script.Addresses) != 2 {
			log.Fatal("wrong script")
		}

		for _, addr := range script.Addresses {
			if addr != bobKP.AddressPubKey.String() && addr != aliceKP.AddressPubKey.String() {
				log.Fatal("unknown script")
			}

		}
	}

	newUtxo := models.UTXO{
		Address:    "",
		TxHash:     sentTx.RawTX.TxHash().String(),
		OutIndex:   0,
		Value:      multisigTxOut.Value,
		PKScript:   hex.EncodeToString(multisigTxOut.PkScript),
		ScriptType: script.Type,
	}

	{
		// sing tx by one key
		newUtxo.Address = aliceKP.Address.String()

		draft := models.DraftTx{
			Amount:     100 * OneCoin,
			NetworkFee: 0,
			UTXO:       []models.UTXO{newUtxo},
		}
		err = draft.SetPayToAddress(aliceKP.Address.String(), &chaincfg.TestNet3Params)
		if err != nil {
			log.Fatal("can not to SetPayToAddress:", err)
		}

		txManager.SetKey(bobKP)
		txSignerByBob, err := txManager.DraftToSignedTx(draft)
		if err != nil {
			log.Fatal("can not to DraftToSignedTx:", err)
		}
		bobSignature := txSignerByBob.TxIn[0].SignatureScript
		// txSignerByBob and bobSignature new to pass to Alice for signing and submitting

		txManager.SetKey(aliceKP)
		aliceAndBobSignature, err := txManager.SignUTXOForTx(txSignerByBob, newUtxo, 0, bobSignature, false)
		if err != nil {
			log.Fatal("can not to SignUTXOForTx:", err)
		}

		// this step is not necessary, because it already set by signing func
		txSignerByBob.TxIn[0].SignatureScript = aliceAndBobSignature

		hashRes, err := txManager.RPC.SendRawTransaction(sentTx.RawTX, true)
		if err != nil {
			log.Fatal("tx not sent:", err)
		}

		log.Println("Sent new Tx:", hashRes.String())
	}

}
