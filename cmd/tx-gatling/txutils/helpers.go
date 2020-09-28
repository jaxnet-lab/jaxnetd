package txutils

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"gitlab.com/jaxnet/core/shard.core.git/blockchain"
	"gitlab.com/jaxnet/core/shard.core.git/btcjson"
	"gitlab.com/jaxnet/core/shard.core.git/btcutil"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/storage"
	"gitlab.com/jaxnet/core/shard.core.git/cmd/tx-gatling/txmodels"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chaincore"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
	"gitlab.com/jaxnet/core/shard.core.git/txscript"
)

// UTXOProvider is provider that cat give list of txmodels.UTXO for provided amount.
type UTXOProvider interface {
	SelectForAmount(amount int64) (txmodels.UTXORows, error)
}

// UTXOFromCSV is an implementation of UTXOProvider that collects txmodels.UTXORows from CSV file,
// value of UTXOFromCSV is a path to file.
type UTXOFromCSV string

func (path UTXOFromCSV) SelectForAmount(amount int64) (txmodels.UTXORows, error) {
	rows, err := storage.NewCSVStorage(string(path)).FetchData()
	if err != nil {
		return nil, err
	}
	collected := rows.CollectForAmount(amount)
	sum := collected.GetSum()
	if sum < amount {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, sum)
	}
	return collected, nil
}

// UTXOFromRows is a wrapper for txmodels.UTXORows to implement the UTXOProvider.
type UTXOFromRows txmodels.UTXORows

func (rows UTXOFromRows) SelectForAmount(amount int64) (txmodels.UTXORows, error) {
	collected := txmodels.UTXORows(rows).CollectForAmount(amount)
	sum := collected.GetSum()
	if sum < amount {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, sum)
	}
	return collected, nil
}

// UTXOFromRows is a wrapper for txmodels.UTXO to implement the UTXOProvider.
type SingleUTXO txmodels.UTXO

func (row SingleUTXO) SelectForAmount(amount int64) (txmodels.UTXORows, error) {
	if row.Value < amount {
		return nil, fmt.Errorf("not enough coins (need %d; has %d)", amount, row.Value)
	}

	return []txmodels.UTXO{txmodels.UTXO(row)}, nil
}

// EncodeTx serializes and encodes to hex wire.MsgTx.
func EncodeTx(tx *wire.MsgTx) string {
	buf := bytes.NewBuffer([]byte{})
	_ = tx.Serialize(buf)
	return hex.EncodeToString(buf.Bytes())
}

// EncodeTx decodes hex-encoded wire.MsgTx.
func DecodeTx(hexTx string) (*wire.MsgTx, error) {
	raw, err := hex.DecodeString(hexTx)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(raw)

	tx := &wire.MsgTx{}
	err = tx.Deserialize(buf)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func TxToJson(mtx *wire.MsgTx, chainParams *chaincore.Params) btcjson.TxRawDecodeResult {
	return btcjson.TxRawDecodeResult{
		Txid:     mtx.TxHash().String(),
		Version:  mtx.Version,
		Locktime: mtx.LockTime,
		Vin:      createVinList(mtx),
		Vout:     createVoutList(mtx, chainParams, nil),
	}
}

// witnessToHex formats the passed witness stack as a slice of hex-encoded
// strings to be used in a JSON response.
func witnessToHex(witness wire.TxWitness) []string {
	// Ensure nil is returned when there are no entries versus an empty
	// slice so it can properly be omitted as necessary.
	if len(witness) == 0 {
		return nil
	}

	result := make([]string, 0, len(witness))
	for _, wit := range witness {
		result = append(result, hex.EncodeToString(wit))
	}

	return result
}

// createVinList returns a slice of JSON objects for the inputs of the passed
// transaction.
func createVinList(mtx *wire.MsgTx) []btcjson.Vin {
	// Coinbase transactions only have a single txin by definition.
	vinList := make([]btcjson.Vin, len(mtx.TxIn))
	if blockchain.IsCoinBaseTx(mtx) {
		txIn := mtx.TxIn[0]
		vinList[0].Coinbase = hex.EncodeToString(txIn.SignatureScript)
		vinList[0].Sequence = txIn.Sequence
		vinList[0].Witness = witnessToHex(txIn.Witness)
		return vinList
	}

	for i, txIn := range mtx.TxIn {
		// The disassembled string will contain [error] inline
		// if the script doesn't fully parse, so ignore the
		// error here.
		disbuf, _ := txscript.DisasmString(txIn.SignatureScript)

		vinEntry := &vinList[i]
		vinEntry.Txid = txIn.PreviousOutPoint.Hash.String()
		vinEntry.Vout = txIn.PreviousOutPoint.Index
		vinEntry.Sequence = txIn.Sequence
		vinEntry.ScriptSig = &btcjson.ScriptSig{
			Asm: disbuf,
			Hex: hex.EncodeToString(txIn.SignatureScript),
		}

		if mtx.HasWitness() {
			vinEntry.Witness = witnessToHex(txIn.Witness)
		}
	}

	return vinList
}

// createVoutList returns a slice of JSON objects for the outputs of the passed
// transaction.
func createVoutList(mtx *wire.MsgTx, chainParams *chaincore.Params, filterAddrMap map[string]struct{}) []btcjson.Vout {
	voutList := make([]btcjson.Vout, 0, len(mtx.TxOut))
	for i, v := range mtx.TxOut {
		// The disassembled string will contain [error] inline if the
		// script doesn't fully parse, so ignore the error here.
		disbuf, _ := txscript.DisasmString(v.PkScript)

		// Ignore the error here since an error means the script
		// couldn't parse and there is no additional information about
		// it anyways.
		scriptClass, addrs, reqSigs, _ := txscript.ExtractPkScriptAddrs(
			v.PkScript, chainParams)

		// Encode the addresses while checking if the address passes the
		// filter when needed.
		passesFilter := len(filterAddrMap) == 0
		encodedAddrs := make([]string, len(addrs))
		for j, addr := range addrs {
			encodedAddr := addr.EncodeAddress()
			encodedAddrs[j] = encodedAddr

			// No need to check the map again if the filter already
			// passes.
			if passesFilter {
				continue
			}
			if _, exists := filterAddrMap[encodedAddr]; exists {
				passesFilter = true
			}
		}

		if !passesFilter {
			continue
		}

		var vout btcjson.Vout
		vout.N = uint32(i)
		vout.Value = btcutil.Amount(v.Value).ToBTC()
		vout.ScriptPubKey.Addresses = encodedAddrs
		vout.ScriptPubKey.Asm = disbuf
		vout.ScriptPubKey.Hex = hex.EncodeToString(v.PkScript)
		vout.ScriptPubKey.Type = scriptClass.String()
		vout.ScriptPubKey.ReqSigs = int32(reqSigs)

		voutList = append(voutList, vout)
	}

	return voutList
}
