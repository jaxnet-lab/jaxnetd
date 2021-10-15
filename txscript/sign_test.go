// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/btcec"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

type addressToKey struct {
	key        *btcec.PrivateKey
	compressed bool
}

func mkGetKey(keys map[string]addressToKey) KeyDB {
	if keys == nil {
		return KeyClosure(func(addr jaxutil.Address) (*btcec.PrivateKey,
			bool, error) {
			return nil, false, errors.New("nope")
		})
	}
	return KeyClosure(func(addr jaxutil.Address) (*btcec.PrivateKey,
		bool, error) {
		a2k, ok := keys[addr.EncodeAddress()]
		if !ok {
			return nil, false, errors.New("nope")
		}
		return a2k.key, a2k.compressed, nil
	})
}

func mkGetScript(scripts map[string][]byte) ScriptDB {
	if scripts == nil {
		return ScriptClosure(func(addr jaxutil.Address) ([]byte, error) {
			return nil, errors.New("nope")
		})
	}
	return ScriptClosure(func(addr jaxutil.Address) ([]byte, error) {
		script, ok := scripts[addr.EncodeAddress()]
		if !ok {
			return nil, errors.New("nope")
		}
		return script, nil
	})
}

func checkScripts(msg string, tx *wire.MsgTx, idx int, inputAmt int64, sigScript, pkScript []byte) error {
	tx.TxIn[idx].SignatureScript = sigScript
	vm, err := NewEngine(pkScript, tx, idx, StandardVerifyFlags, nil, nil, inputAmt)
	if err != nil {
		return fmt.Errorf("failed to make script engine for %s: %v",
			msg, err)
	}

	err = vm.Execute()
	if err != nil {
		return fmt.Errorf("invalid script signature for %s: %v", msg,
			err)
	}

	return nil
}

func checkMultiSigLockScripts(msg string, tx *wire.MsgTx, idx int, inputAmt int64, sigScript, pkScript []byte) error {
	tx.TxIn[idx].SignatureScript = sigScript
	vm, err := NewEngine(pkScript, tx, idx, StandardVerifyFlags, nil, nil, inputAmt)
	if err != nil {
		return fmt.Errorf("failed to make script engine for %s: %v",
			msg, err)
	}

	err = vm.Execute()
	if err != nil {
		return fmt.Errorf("invalid script signature for %s: %v", msg, err)
	}

	return nil
}

func signAndCheck(msg string, tx *wire.MsgTx, idx int, inputAmt int64, pkScript []byte,
	hashType SigHashType, kdb KeyDB, sdb ScriptDB, previousScript []byte) error {

	sigScript, err := SignTxOutput(&chaincfg.TestNet3Params, tx, idx,
		pkScript, hashType, kdb, sdb, nil)
	if err != nil {
		return fmt.Errorf("failed to sign output %s: %v", msg, err)
	}

	return checkScripts(msg, tx, idx, inputAmt, sigScript, pkScript)
}

func genKeys(t *testing.T, msg string) (*btcec.PrivateKey, *jaxutil.AddressPubKey, error) {
	key1, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Errorf("failed to make privKey for %s: %v",
			msg, err)
		return nil, nil, err

	}

	pk1 := (*btcec.PublicKey)(&key1.PublicKey).
		SerializeCompressed()
	address1, err := jaxutil.NewAddressPubKey(pk1,
		&chaincfg.TestNet3Params)
	if err != nil {
		t.Errorf("failed to make address for %s: %v",
			msg, err)

		return nil, nil, err
	}
	return key1, address1, nil
}

func TestSignTxOutput(t *testing.T) {
	t.Parallel()

	// make key
	// make script based on key.
	// sign with magic pixie dust.
	hashTypes := []SigHashType{
		// SigHashOld, // no longer used but should act like all
		SigHashAll,
		// SigHashNone,
		// SigHashSingle,
		// SigHashAll | SigHashAnyOneCanPay,
		// SigHashNone | SigHashAnyOneCanPay,
		// SigHashSingle | SigHashAnyOneCanPay,
	}
	inputAmounts := []int64{5, 10, 15}
	tx := &wire.MsgTx{
		Version: wire.TxVerRegular,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: 0},
				Sequence:         4294967295,
			},
			// {
			// 	PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: 1},
			// 	Sequence:         4294967295,
			// },
			// {
			// 	PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: 2},
			// 	Sequence:         4294967295,
			// },
		},
		TxOut: []*wire.TxOut{
			{Value: 1},
			{Value: 2},
			{Value: 3},
		},
		LockTime: 0,
	}
	//
	// // Pay to Pubkey TxHash (uncompressed)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		if err := signAndCheck(msg, tx, i, inputAmounts[i], pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(nil), nil); err != nil {
	// 			t.Error(err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to Pubkey TxHash (uncompressed) (merging with correct)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(nil), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg,
	// 				err)
	// 			break
	// 		}
	//
	// 		// by the above loop, this should be valid, now sign
	// 		// again and merge.
	// 		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(nil), sigScript)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s a "+
	// 				"second time: %v", msg, err)
	// 			break
	// 		}
	//
	// 		err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, pkScript)
	// 		if err != nil {
	// 			t.Errorf("twice signed script invalid for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to Pubkey TxHash (compressed)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		if err := signAndCheck(msg, tx, i, inputAmounts[i],
	// 			pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(nil), nil); err != nil {
	// 			t.Error(err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to Pubkey TxHash (compressed) with duplicate merge
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(nil), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg,
	// 				err)
	// 			break
	// 		}
	//
	// 		// by the above loop, this should be valid, now sign
	// 		// again and merge.
	// 		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(nil), sigScript)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s a "+
	// 				"second time: %v", msg, err)
	// 			break
	// 		}
	//
	// 		err = checkScripts(msg, tx, i, inputAmounts[i],
	// 			sigScript, pkScript)
	// 		if err != nil {
	// 			t.Errorf("twice signed script invalid for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to PubKey (uncompressed)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		if err := signAndCheck(msg, tx, i, inputAmounts[i],
	// 			pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(nil), nil); err != nil {
	// 			t.Error(err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to PubKey (uncompressed)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(nil), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg,
	// 				err)
	// 			break
	// 		}
	//
	// 		// by the above loop, this should be valid, now sign
	// 		// again and merge.
	// 		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(nil), sigScript)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s a "+
	// 				"second time: %v", msg, err)
	// 			break
	// 		}
	//
	// 		err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, pkScript)
	// 		if err != nil {
	// 			t.Errorf("twice signed script invalid for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to PubKey (compressed)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		if err := signAndCheck(msg, tx, i, inputAmounts[i],
	// 			pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(nil), nil); err != nil {
	// 			t.Error(err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to PubKey (compressed) with duplicate merge
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(nil), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg,
	// 				err)
	// 			break
	// 		}
	//
	// 		// by the above loop, this should be valid, now sign
	// 		// again and merge.
	// 		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, pkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(nil), sigScript)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s a "+
	// 				"second time: %v", msg, err)
	// 			break
	// 		}
	//
	// 		err = checkScripts(msg, tx, i, inputAmounts[i],
	// 			sigScript, pkScript)
	// 		if err != nil {
	// 			t.Errorf("twice signed script invalid for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // As before, but with p2sh now.
	// // Pay to Pubkey TxHash (uncompressed)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		scriptAddr, err := jaxutil.NewAddressScriptHash(
	// 			pkScript, &chaincfg.TestNet3Params)
	// 		if err != nil {
	// 			t.Errorf("failed to make p2sh addr for %s: %v",
	// 				msg, err)
	// 			break
	// 		}
	//
	// 		scriptPkScript, err := PayToAddrScript(
	// 			scriptAddr)
	// 		if err != nil {
	// 			t.Errorf("failed to make script pkscript for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		if err := signAndCheck(msg, tx, i, inputAmounts[i],
	// 			scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil); err != nil {
	// 			t.Error(err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to Pubkey TxHash (uncompressed) with duplicate merge
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		scriptAddr, err := jaxutil.NewAddressScriptHash(
	// 			pkScript, &chaincfg.TestNet3Params)
	// 		if err != nil {
	// 			t.Errorf("failed to make p2sh addr for %s: %v",
	// 				msg, err)
	// 			break
	// 		}
	//
	// 		scriptPkScript, err := PayToAddrScript(
	// 			scriptAddr)
	// 		if err != nil {
	// 			t.Errorf("failed to make script pkscript for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg,
	// 				err)
	// 			break
	// 		}
	//
	// 		// by the above loop, this should be valid, now sign
	// 		// again and merge.
	// 		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s a "+
	// 				"second time: %v", msg, err)
	// 			break
	// 		}
	//
	// 		err = checkScripts(msg, tx, i, inputAmounts[i],
	// 			sigScript, scriptPkScript)
	// 		if err != nil {
	// 			t.Errorf("twice signed script invalid for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to Pubkey TxHash (compressed)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		scriptAddr, err := jaxutil.NewAddressScriptHash(
	// 			pkScript, &chaincfg.TestNet3Params)
	// 		if err != nil {
	// 			t.Errorf("failed to make p2sh addr for %s: %v",
	// 				msg, err)
	// 			break
	// 		}
	//
	// 		scriptPkScript, err := PayToAddrScript(
	// 			scriptAddr)
	// 		if err != nil {
	// 			t.Errorf("failed to make script pkscript for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		if err := signAndCheck(msg, tx, i, inputAmounts[i],
	// 			scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil); err != nil {
	// 			t.Error(err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to Pubkey TxHash (compressed) with duplicate merge
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		scriptAddr, err := jaxutil.NewAddressScriptHash(
	// 			pkScript, &chaincfg.TestNet3Params)
	// 		if err != nil {
	// 			t.Errorf("failed to make p2sh addr for %s: %v",
	// 				msg, err)
	// 			break
	// 		}
	//
	// 		scriptPkScript, err := PayToAddrScript(
	// 			scriptAddr)
	// 		if err != nil {
	// 			t.Errorf("failed to make script pkscript for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg,
	// 				err)
	// 			break
	// 		}
	//
	// 		// by the above loop, this should be valid, now sign
	// 		// again and merge.
	// 		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s a "+
	// 				"second time: %v", msg, err)
	// 			break
	// 		}
	//
	// 		err = checkScripts(msg, tx, i, inputAmounts[i],
	// 			sigScript, scriptPkScript)
	// 		if err != nil {
	// 			t.Errorf("twice signed script invalid for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to PubKey (uncompressed)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		scriptAddr, err := jaxutil.NewAddressScriptHash(
	// 			pkScript, &chaincfg.TestNet3Params)
	// 		if err != nil {
	// 			t.Errorf("failed to make p2sh addr for %s: %v",
	// 				msg, err)
	// 			break
	// 		}
	//
	// 		scriptPkScript, err := PayToAddrScript(
	// 			scriptAddr)
	// 		if err != nil {
	// 			t.Errorf("failed to make script pkscript for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		if err := signAndCheck(msg, tx, i, inputAmounts[i],
	// 			scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil); err != nil {
	// 			t.Error(err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to PubKey (uncompressed) with duplicate merge
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		scriptAddr, err := jaxutil.NewAddressScriptHash(
	// 			pkScript, &chaincfg.TestNet3Params)
	// 		if err != nil {
	// 			t.Errorf("failed to make p2sh addr for %s: %v",
	// 				msg, err)
	// 			break
	// 		}
	//
	// 		scriptPkScript, err := PayToAddrScript(scriptAddr)
	// 		if err != nil {
	// 			t.Errorf("failed to make script pkscript for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg,
	// 				err)
	// 			break
	// 		}
	//
	// 		// by the above loop, this should be valid, now sign
	// 		// again and merge.
	// 		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, false},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s a "+
	// 				"second time: %v", msg, err)
	// 			break
	// 		}
	//
	// 		err = checkScripts(msg, tx, i, inputAmounts[i],
	// 			sigScript, scriptPkScript)
	// 		if err != nil {
	// 			t.Errorf("twice signed script invalid for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to PubKey (compressed)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	//
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		scriptAddr, err := jaxutil.NewAddressScriptHash(
	// 			pkScript, &chaincfg.TestNet3Params)
	// 		if err != nil {
	// 			t.Errorf("failed to make p2sh addr for %s: %v",
	// 				msg, err)
	// 			break
	// 		}
	//
	// 		scriptPkScript, err := PayToAddrScript(scriptAddr)
	// 		if err != nil {
	// 			t.Errorf("failed to make script pkscript for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		if err := signAndCheck(msg, tx, i, inputAmounts[i],
	// 			scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil); err != nil {
	// 			t.Error(err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Pay to PubKey (compressed)
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	//
	// 		key, address, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := PayToAddrScript(address)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		scriptAddr, err := jaxutil.NewAddressScriptHash(
	// 			pkScript, &chaincfg.TestNet3Params)
	// 		if err != nil {
	// 			t.Errorf("failed to make p2sh addr for %s: %v",
	// 				msg, err)
	// 			break
	// 		}
	//
	// 		scriptPkScript, err := PayToAddrScript(scriptAddr)
	// 		if err != nil {
	// 			t.Errorf("failed to make script pkscript for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg,
	// 				err)
	// 			break
	// 		}
	//
	// 		// by the above loop, this should be valid, now sign
	// 		// again and merge.
	// 		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address.EncodeAddress(): {key, true},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s a "+
	// 				"second time: %v", msg, err)
	// 			break
	// 		}
	//
	// 		err = checkScripts(msg, tx, i, inputAmounts[i],
	// 			sigScript, scriptPkScript)
	// 		if err != nil {
	// 			t.Errorf("twice signed script invalid for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Basic Multisig
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key1, address1, err := genKeys(t, msg)
			if err != nil {
				break
			}

			key2, address2, err := genKeys(t, msg)
			if err != nil {
				break
			}

			multiSigScript, err := MultiSigScript([]*jaxutil.AddressPubKey{address1, address2}, 2, false)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			pay2MultisigScriptAddr, err := jaxutil.NewAddressScriptHash(multiSigScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			pay2MultisigPkScript, err := PayToAddrScript(pay2MultisigScriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}
			activateTraceLogger()
			if err := signAndCheck(msg, tx, i, inputAmounts[i],
				pay2MultisigPkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address1.EncodeAddress(): {key1, true},
					address2.EncodeAddress(): {key2, true},
				}), mkGetScript(map[string][]byte{
					pay2MultisigScriptAddr.EncodeAddress(): multiSigScript,
				}), nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Two part multisig, sign with one key then the other.
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	//
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	//
	// 		key1, address1, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		key2, address2, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := MultiSigScript([]*jaxutil.AddressPubKey{address1, address2}, 2, false)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		scriptAddr, err := jaxutil.NewAddressScriptHash(
	// 			pkScript, &chaincfg.TestNet3Params)
	// 		if err != nil {
	// 			t.Errorf("failed to make p2sh addr for %s: %v",
	// 				msg, err)
	// 			break
	// 		}
	//
	// 		scriptPkScript, err := PayToAddrScript(scriptAddr)
	// 		if err != nil {
	// 			t.Errorf("failed to make script pkscript for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address1.EncodeAddress(): {key1, true},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg,
	// 				err)
	// 			break
	// 		}
	//
	// 		// Only 1 out of 2 signed, this *should* fail.
	// 		if checkScripts(msg, tx, i, inputAmounts[i], sigScript,
	// 			scriptPkScript) == nil {
	// 			t.Errorf("part signed script valid for %s", msg)
	// 			break
	// 		}
	//
	// 		// Sign with the other key and merge
	// 		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address2.EncodeAddress(): {key2, true},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), sigScript)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg, err)
	// 			break
	// 		}
	// 		activateTraceLogger()
	// 		asm, _ := DisasmString(sigScript)
	// 		fmt.Println("refund_signature_asm: ", asm)
	// 		err = checkScripts(msg, tx, i, inputAmounts[i], sigScript,
	// 			scriptPkScript)
	// 		if err != nil {
	// 			t.Errorf("fully signed script invalid for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	// 	}
	// }
	//
	// // Two part multisig, sign with one key then both, check key dedup
	// // correctly.
	// for _, hashType := range hashTypes {
	// 	for i := range tx.TxIn {
	// 		msg := fmt.Sprintf("%d:%d", hashType, i)
	//
	// 		key1, address1, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		key2, address2, err := genKeys(t, msg)
	// 		if err != nil {
	// 			break
	// 		}
	//
	// 		pkScript, err := MultiSigScript([]*jaxutil.AddressPubKey{address1, address2}, 2, false)
	// 		if err != nil {
	// 			t.Errorf("failed to make pkscript "+
	// 				"for %s: %v", msg, err)
	// 		}
	//
	// 		scriptAddr, err := jaxutil.NewAddressScriptHash(
	// 			pkScript, &chaincfg.TestNet3Params)
	// 		if err != nil {
	// 			t.Errorf("failed to make p2sh addr for %s: %v",
	// 				msg, err)
	// 			break
	// 		}
	//
	// 		scriptPkScript, err := PayToAddrScript(scriptAddr)
	// 		if err != nil {
	// 			t.Errorf("failed to make script pkscript for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address1.EncodeAddress(): {key1, true},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), nil)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg,
	// 				err)
	// 			break
	// 		}
	//
	// 		// Only 1 out of 2 signed, this *should* fail.
	// 		if checkScripts(msg, tx, i, inputAmounts[i], sigScript,
	// 			scriptPkScript) == nil {
	// 			t.Errorf("part signed script valid for %s", msg)
	// 			break
	// 		}
	//
	// 		// Sign with the other key and merge
	// 		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
	// 			tx, i, scriptPkScript, hashType,
	// 			mkGetKey(map[string]addressToKey{
	// 				address1.EncodeAddress(): {key1, true},
	// 				address2.EncodeAddress(): {key2, true},
	// 			}), mkGetScript(map[string][]byte{
	// 				scriptAddr.EncodeAddress(): pkScript,
	// 			}), sigScript)
	// 		if err != nil {
	// 			t.Errorf("failed to sign output %s: %v", msg, err)
	// 			break
	// 		}
	//
	// 		// Now we should pass.
	// 		err = checkScripts(msg, tx, i, inputAmounts[i],
	// 			sigScript, scriptPkScript)
	// 		if err != nil {
	// 			t.Errorf("fully signed script invalid for "+
	// 				"%s: %v", msg, err)
	// 			break
	// 		}
	// 	}
	// }
}

func parseKeys(t *testing.T, secret, msg string) (*btcec.PrivateKey, *jaxutil.AddressPubKey, error) {
	raw, err := hex.DecodeString(secret)
	if err != nil {
		t.Errorf("failed to make privKey for %s: %v", msg, err)
		return nil, nil, err

	}

	key1, _ := btcec.PrivKeyFromBytes(btcec.S256(), raw)

	pk1 := (*btcec.PublicKey)(&key1.PublicKey).SerializeCompressed()
	address1, err := jaxutil.NewAddressPubKey(pk1, &chaincfg.TestNet3Params)
	if err != nil {
		t.Errorf("failed to make address for %s: %v", msg, err)
		return nil, nil, err
	}

	return key1, address1, nil
}

func TestSignTxOutput_multiSigLock(t *testing.T) {
	// t.Parallel()

	// make key
	// make script based on key.
	// sign with magic pixie dust.
	hashTypes := []SigHashType{
		// SigHashOld, // no longer used but should act like all
		SigHashAll,
		SigHashNone,
		SigHashSingle,
		SigHashAll | SigHashAnyOneCanPay,
		SigHashNone | SigHashAnyOneCanPay,
		SigHashSingle | SigHashAnyOneCanPay,
	}
	inputAmounts := []int64{5, 10, 15}
	tx := &wire.MsgTx{
		Version: wire.TxVerRegular,
		TxIn: []*wire.TxIn{
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: 0}, Sequence: 4294967295}},
		TxOut:    []*wire.TxOut{{Value: 1}, {Value: 2}, {Value: 3}},
		LockTime: 0,
	}

	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			if !testMultiSigLockTx(t, tx.Copy(), inputAmounts, hashType, i) {
				break
			}
		}
	}
}

func testMultiSigLockTx(t *testing.T, tx *wire.MsgTx, inputAmounts []int64, hashType SigHashType, i int) bool {
	msg := fmt.Sprintf("%d:%d", hashType, i)
	refundKey, refundAddress, err := genKeys(t, msg)
	if err != nil {
		return false
	}

	key1, address1, err := genKeys(t, msg)
	if err != nil {
		return false
	}

	key2, address2, err := genKeys(t, msg)
	if err != nil {
		return false
	}

	_, _ = key1, key2

	refundDeferringPeriod := int32(240)

	multiSigLockScript, err := MultiSigLockScript([]*jaxutil.AddressPubKey{address1, address2}, 2, refundAddress, refundDeferringPeriod, false)
	if err != nil {
		t.Errorf("failed to make pkscript for %s: %v", msg, err)
	}

	t.Logf("multisig lock: %s", hex.EncodeToString(multiSigLockScript))

	scriptAddr, err := jaxutil.NewAddressScriptHash(multiSigLockScript, &chaincfg.TestNet3Params)
	if err != nil {
		t.Errorf("failed to make p2sh addr for %s: %v", msg, err)
		return false
	}

	scriptPkScript, err := PayToAddrScript(scriptAddr)
	if err != nil {
		t.Errorf("failed to make script pkscript for %s: %v", msg, err)
		return false
	}
	// fmt.Println("refund_key: ", hex.EncodeToString(refundKey.Serialize()))
	// fmt.Println("refund_pub_key: ", refundAddress.String())
	//
	// fmt.Println("key_1: ", hex.EncodeToString(key1.Serialize()))
	// fmt.Println("pub_key_1: ", address1.String())
	//
	// fmt.Println("key_2: ", hex.EncodeToString(key2.Serialize()))
	// fmt.Println("pub_key_2: ", address2.String())
	// asm, _ := DisasmString(multiSigLockScript)
	// fmt.Println("lock_script_asm: ", asm)
	// fmt.Println("lock_script_hex: ", hex.EncodeToString(multiSigLockScript))
	// fmt.Println("lock_script_address: ", scriptAddr.EncodeAddress())
	// asm, _ = DisasmString(scriptPkScript)
	// fmt.Println("pay_to_lock_script_asm: ", asm)
	// fmt.Println("pay_to_lock_script_hex: ", hex.EncodeToString(scriptPkScript))
	parentTx := &wire.MsgTx{
		Version: wire.TxVerRegular,
		TxIn: []*wire.TxIn{
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32}, Sequence: 4294967295}},
		TxOut:    []*wire.TxOut{{Value: 100000, PkScript: scriptPkScript}},
		LockTime: 0,
	}
	buf := bytes.NewBuffer(nil)
	parentTx.Serialize(buf)
	t.Logf("parent tx[%d]: %s", i, hex.EncodeToString(buf.Bytes()))
	tx.TxIn[i].PreviousOutPoint.Hash = parentTx.TxHash()
	tx.TxIn[i].PreviousOutPoint.Index = 0

	buf = bytes.NewBuffer(nil)
	tx.Serialize(buf)
	t.Logf("tx[%d] before signing: %s", i, hex.EncodeToString(buf.Bytes()))
	{ // check the strategy of the multi sig spend
		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
			tx, i, scriptPkScript, hashType,
			mkGetKey(map[string]addressToKey{
				address1.EncodeAddress(): {key1, true},
			}), mkGetScript(map[string][]byte{
				scriptAddr.EncodeAddress(): multiSigLockScript,
			}), nil)
		if err != nil {
			t.Errorf("failed to sign output %s: %v", msg, err)
			return false
		}

		tx.TxIn[i].SignatureScript = sigScript
		buf = bytes.NewBuffer(nil)
		tx.Serialize(buf)
		t.Logf("tx[%d] with 1 sig: %s", i, hex.EncodeToString(buf.Bytes()))

		// asm, _ = DisasmString(sigScript)
		// fmt.Println("partial_signature_asm: ", asm)
		// fmt.Println("partial_signature_hex: ", hex.EncodeToString(sigScript))
		// Only 1 out of 2 signed, this *should* fail.
		if checkScripts(msg, tx, i, inputAmounts[i], sigScript, scriptPkScript) == nil {
			t.Errorf("part signed script valid for %s", msg)
			return false
		}

		// Sign with the other key and merge
		sigScript, err = SignTxOutput(&chaincfg.TestNet3Params,
			tx, i, scriptPkScript, hashType,
			mkGetKey(map[string]addressToKey{
				address2.EncodeAddress(): {key2, true},
			}), mkGetScript(map[string][]byte{
				scriptAddr.EncodeAddress(): multiSigLockScript,
			}), sigScript)
		if err != nil {
			t.Errorf("failed to sign output %s: %v", msg, err)
			return false
		}

		tx.TxIn[i].SignatureScript = sigScript
		buf = bytes.NewBuffer(nil)
		tx.Serialize(buf)
		t.Logf("tx[%d] with 2 sig: %s", i, hex.EncodeToString(buf.Bytes()))

		// Now we should pass.
		err = checkMultiSigLockScripts(msg, tx, i, inputAmounts[i], sigScript, scriptPkScript)
		if err != nil {
			t.Errorf("fully signed script invalid for %s: %v\n", msg, err)
			return false
		}

		// asm, _ = DisasmString(sigScript)
		// fmt.Println("full_signature_asm: ", asm)
		// fmt.Println("full_signature_hex: ", hex.EncodeToString(sigScript))
	}

	{ // check the strategy of the refund spend
		sigScript, err := SignTxOutput(&chaincfg.TestNet3Params,
			tx, i, scriptPkScript, hashType,
			mkGetKey(map[string]addressToKey{
				refundAddress.EncodeAddress(): {refundKey, true},
			}), mkGetScript(map[string][]byte{
				scriptAddr.EncodeAddress(): multiSigLockScript,
			}), nil)
		if err != nil {
			t.Errorf("failed to sign output %s: %v", msg, err)
			return false
		}

		in := tx.TxIn[i]
		in.Age = refundDeferringPeriod + 5
		in.SignatureScript = nil
		tx.TxIn[i] = in
		// activateTraceLogger()
		if err = checkMultiSigLockScripts(msg, tx, i, inputAmounts[i], sigScript, scriptPkScript); err != nil {
			t.Errorf("fully signed refund script invalid for %s: %v\n", msg, err)
			return false
		}
		// asm, _ = DisasmString(sigScript)
		// fmt.Println("refund_signature_asm: ", asm)
		// fmt.Println("refund_signature_hex: ", hex.EncodeToString(sigScript))
	}
	return true
}

type tstInput struct {
	txout              *wire.TxOut
	sigscriptGenerates bool
	inputValidates     bool
	indexOutOfRange    bool
}

type tstSigScript struct {
	name               string
	inputs             []tstInput
	hashType           SigHashType
	compress           bool
	scriptAtWrongIndex bool
}

var coinbaseOutPoint = &wire.OutPoint{
	Index: (1 << 32) - 1,
}

// Pregenerated private key, with associated public key and pkScripts
// for the uncompressed and compressed hash160.
var (
	privKeyD = []byte{0x6b, 0x0f, 0xd8, 0xda, 0x54, 0x22, 0xd0, 0xb7,
		0xb4, 0xfc, 0x4e, 0x55, 0xd4, 0x88, 0x42, 0xb3, 0xa1, 0x65,
		0xac, 0x70, 0x7f, 0x3d, 0xa4, 0x39, 0x5e, 0xcb, 0x3b, 0xb0,
		0xd6, 0x0e, 0x06, 0x92}
	pubkeyX = []byte{0xb2, 0x52, 0xf0, 0x49, 0x85, 0x78, 0x03, 0x03, 0xc8,
		0x7d, 0xce, 0x51, 0x7f, 0xa8, 0x69, 0x0b, 0x91, 0x95, 0xf4,
		0xf3, 0x5c, 0x26, 0x73, 0x05, 0x05, 0xa2, 0xee, 0xbc, 0x09,
		0x38, 0x34, 0x3a}
	pubkeyY = []byte{0xb7, 0xc6, 0x7d, 0xb2, 0xe1, 0xff, 0xc8, 0x43, 0x1f,
		0x63, 0x32, 0x62, 0xaa, 0x60, 0xc6, 0x83, 0x30, 0xbd, 0x24,
		0x7e, 0xef, 0xdb, 0x6f, 0x2e, 0x8d, 0x56, 0xf0, 0x3c, 0x9f,
		0x6d, 0xb6, 0xf8}
	uncompressedPkScript = []byte{0x76, 0xa9, 0x14, 0xd1, 0x7c, 0xb5,
		0xeb, 0xa4, 0x02, 0xcb, 0x68, 0xe0, 0x69, 0x56, 0xbf, 0x32,
		0x53, 0x90, 0x0e, 0x0a, 0x86, 0xc9, 0xfa, 0x88, 0xac}
	compressedPkScript = []byte{0x76, 0xa9, 0x14, 0x27, 0x4d, 0x9f, 0x7f,
		0x61, 0x7e, 0x7c, 0x7a, 0x1c, 0x1f, 0xb2, 0x75, 0x79, 0x10,
		0x43, 0x65, 0x68, 0x27, 0x9d, 0x86, 0x88, 0xac}
	shortPkScript = []byte{0x76, 0xa9, 0x14, 0xd1, 0x7c, 0xb5,
		0xeb, 0xa4, 0x02, 0xcb, 0x68, 0xe0, 0x69, 0x56, 0xbf, 0x32,
		0x53, 0x90, 0x0e, 0x0a, 0x88, 0xac}
	uncompressedAddrStr = "1L6fd93zGmtzkK6CsZFVVoCwzZV3MUtJ4F"
	compressedAddrStr   = "14apLppt9zTq6cNw8SDfiJhk9PhkZrQtYZ"
)

// Pretend output amounts.
const coinbaseVal = 2500000000
const fee = 5000000

var sigScriptTests = []tstSigScript{
	{
		name: "one input uncompressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "two inputs uncompressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "one input compressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, compressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           true,
		scriptAtWrongIndex: false,
	},
	{
		name: "two inputs compressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, compressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, compressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           true,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType SigHashNone",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashNone,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType SigHashSingle",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashSingle,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType SigHashAnyoneCanPay",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAnyOneCanPay,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType non-standard",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           0x04,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "invalid compression",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     false,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           true,
		scriptAtWrongIndex: false,
	},
	{
		name: "short PkScript",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, shortPkScript),
				sigscriptGenerates: false,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "valid script at wrong index",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           false,
		scriptAtWrongIndex: true,
	},
	{
		name: "index out of range",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           SigHashAll,
		compress:           false,
		scriptAtWrongIndex: true,
	},
}

// Test the sigscript generation for valid and invalid inputs, all
// hashTypes, and with and without compression.  This test creates
// sigscripts to spend fake coinbase inputs, as sigscripts cannot be
// created for the MsgTxs in txTests, since they come from the blockchain
// and we don't have the private keys.
func TestSignatureScript(t *testing.T) {
	t.Parallel()

	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyD)

nexttest:
	for i := range sigScriptTests {
		tx := wire.NewMsgTx(wire.TxVersion)

		output := wire.NewTxOut(500, []byte{OP_RETURN})
		tx.AddTxOut(output)

		for range sigScriptTests[i].inputs {
			txin := wire.NewTxIn(coinbaseOutPoint, nil, nil)
			tx.AddTxIn(txin)
		}

		var script []byte
		var err error
		for j := range tx.TxIn {
			var idx int
			if sigScriptTests[i].inputs[j].indexOutOfRange {
				t.Errorf("at test %v", sigScriptTests[i].name)
				idx = len(sigScriptTests[i].inputs)
			} else {
				idx = j
			}
			script, err = SignatureScript(tx, idx,
				sigScriptTests[i].inputs[j].txout.PkScript,
				sigScriptTests[i].hashType, privKey,
				sigScriptTests[i].compress)

			if (err == nil) != sigScriptTests[i].inputs[j].sigscriptGenerates {
				if err == nil {
					t.Errorf("passed test '%v' incorrectly",
						sigScriptTests[i].name)
				} else {
					t.Errorf("failed test '%v': %v",
						sigScriptTests[i].name, err)
				}
				continue nexttest
			}
			if !sigScriptTests[i].inputs[j].sigscriptGenerates {
				// done with this test
				continue nexttest
			}

			tx.TxIn[j].SignatureScript = script
		}

		// If testing using a correct sigscript but for an incorrect
		// index, use last input script for first input.  Requires > 0
		// inputs for test.
		if sigScriptTests[i].scriptAtWrongIndex {
			tx.TxIn[0].SignatureScript = script
			sigScriptTests[i].inputs[0].inputValidates = false
		}

		// Validate tx input scripts
		scriptFlags := ScriptBip16 | ScriptVerifyDERSignatures
		for j := range tx.TxIn {
			vm, err := NewEngine(sigScriptTests[i].
				inputs[j].txout.PkScript, tx, j, scriptFlags, nil, nil, 0)
			if err != nil {
				t.Errorf("cannot create script vm for test %v: %v",
					sigScriptTests[i].name, err)
				continue nexttest
			}
			err = vm.Execute()
			if (err == nil) != sigScriptTests[i].inputs[j].inputValidates {
				if err == nil {
					t.Errorf("passed test '%v' validation incorrectly: %v",
						sigScriptTests[i].name, err)
				} else {
					t.Errorf("failed test '%v' validation: %v",
						sigScriptTests[i].name, err)
				}
				continue nexttest
			}
		}
	}
}

func TestEADScript(t *testing.T) {
	key, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		t.Errorf("failed to make privKey: %v", err)
		return
	}

	pk := (*btcec.PublicKey)(&key.PublicKey).SerializeCompressed()
	address, err := jaxutil.NewAddressPubKey(pk, &chaincfg.TestNet3Params)
	if err != nil {
		t.Errorf("failed to make address for: %v", err)
		return
	}

	ipV4 := net.IPv4(77, 244, 36, 161)
	expTime := int64(1608157135)
	port := int64(80)
	shardID := uint32(1)

	script, err := EADAddressScript(EADScriptData{
		ShardID: shardID,
		// IP:      ipV4,
		URL:            fmt.Sprintf("https://%s.jax.net", ipV4.String()),
		Port:           port,
		ExpirationDate: expTime,
		Owner:          address,
	})
	if err != nil {
		t.Errorf("failed to craft ead address script: %v", err)
		return
	}
	scriptAsm, err := DisasmString(script)
	if err != nil {
		t.Errorf("failed to disasm ead address script: %v", err)
		return
	}

	t.Logf("scriptAsm: %s\n", scriptAsm)
	data, err := PushedData(script)
	if err != nil {
		t.Errorf("failed to disasm ead address script: %v", err)
		return
	}

	t.Log(data)

	class, addressees, sigReq, err := ExtractPkScriptAddrs(script, &chaincfg.TestNet3Params)
	if err != nil {
		t.Errorf("failed to parse the ead address script: %v", err)
		return
	}

	if class != EADAddressTy {
		t.Errorf("got invalid script type: %v", class)
		return
	}
	if len(addressees) != 1 || (len(addressees) == 1 && addressees[0].String() != address.String()) {
		t.Errorf("exctracted invalid address: %v", addressees)
		return
	}

	if sigReq != 1 {
		t.Errorf("expect 1 requirement of the signature, got %v", sigReq)
		return
	}

	scriptData, err := EADAddressScriptData(script)
	if err != nil {
		t.Errorf("failed to parse the ead address script: %v", err)
		return
	}
	t.Logf("%+v", scriptData)

	// if scriptData.IP.String() != ipV4.String() {
	// 	t.Errorf("ip mismatch: %v", scriptData.IP.String())
	// 	return
	// }

	if scriptData.Port != port {
		t.Errorf("newPort mismatch: %v", scriptData.Port)
		return
	}
	if scriptData.ExpirationDate != expTime {
		t.Errorf("newExpTime mismatch: %v", scriptData.ExpirationDate)
		return
	}

	if scriptData.ShardID != shardID {
		t.Errorf("shardID mismatch: %v", scriptData.ShardID)
		return
	}

	// t.Parallel()

	// make key
	// make script based on key.
	// sign with magic pixie dust.
	hashTypes := []SigHashType{
		// SigHashOld, // no longer used but should act like all
		SigHashAll,
		// SigHashNone,
		// SigHashSingle,
		// SigHashAll | SigHashAnyOneCanPay,
		// SigHashNone | SigHashAnyOneCanPay,
		// SigHashSingle | SigHashAnyOneCanPay,
	}
	inputAmounts := []int64{5, 10, 15}
	tx := &wire.MsgTx{
		Version: wire.TxVerEADAction,
		TxIn: []*wire.TxIn{
			{PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: 0}, Sequence: 4294967295}},
		TxOut:    []*wire.TxOut{{Value: 1}, {Value: 2}, {Value: 3}},
		LockTime: 0,
	}

	for _, hashType := range hashTypes {
		for i := range tx.TxIn {

			msg := fmt.Sprintf("%d:%d", hashType, i)
			activateTraceLogger()

			if err := signAndCheck(msg, tx, i, inputAmounts[i], script, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil); err != nil {
				t.Error(err)
				break
			}

		}
	}

}

func TestSmallInt(t *testing.T) {
	script, err := NewScriptBuilder().
		AddData(scriptNum(2 + 16).Bytes()).
		Script()
	if err != nil {
		t.Error(err)
		return
	}
	parsed, err := parseScript(script)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(parsed)
	//
	data, err := PushedData(script)
	if err != nil {
		t.Error(err)
		return
	}
	var rawTime scriptNum
	rawTime, err = makeScriptNum(data[0], false, 5)
	if err != nil {
		return
	}
	fmt.Println(rawTime.Int32() - 16)
}
