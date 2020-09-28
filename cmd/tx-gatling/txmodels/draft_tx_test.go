package txmodels

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/jaxnet/core/shard.core.git/shards/chain/chainhash"
	"gitlab.com/jaxnet/core/shard.core.git/shards/network/wire"
)

func TestTransaction_UnmarshalBinary(t *testing.T) {
	amount := 9 * 10000_0000
	inputTxHashStr := "02cf5a871a3000c01ef41416c453473842311288d950b3ba2a609d33cea83d37"

	data, err := hex.DecodeString(inputTxHashStr)
	assert.NoError(t, err)
	inputTxHash, err := chainhash.NewHash(data)
	assert.NoError(t, err)

	outPoint := wire.NewOutPoint(inputTxHash, 0)
	txIn := wire.NewTxIn(outPoint, nil, nil)

	evaPkScript := "76a914b2629111cf79c2f1cd025a7aebc403fc9bb5d48b88ac"
	evaDest, err := hex.DecodeString(evaPkScript)
	assert.NoError(t, err)
	txOut := wire.NewTxOut(int64(amount), evaDest)

	msgTx := wire.NewMsgTx(wire.TxVersion)
	msgTx.AddTxIn(txIn)
	msgTx.AddTxOut(txOut)

	buf := bytes.NewBuffer(nil)
	err = msgTx.Serialize(buf)
	assert.NoError(t, err)

	tx := Transaction{
		TxHash:      msgTx.TxHash().String(),
		Source:      "myGdvt6vRNgrFZFtU5FNhW5gxwRqKBcLGv",
		Destination: "mwnAejT1i6Fra7npajqEe6G3A22DFbU5aK",
		Amount:      int64(amount),
		SignedTx:    hex.EncodeToString(buf.Bytes()),
		RawTX:       msgTx,
	}

	binaryTx, err := tx.MarshalBinary()
	assert.NoError(t, err)
	t.Log(hex.EncodeToString(binaryTx))

	newTx := Transaction{}

	err = newTx.UnmarshalBinary(binaryTx)
	assert.NoError(t, err)

	tx.RawTX = nil
	newTx.RawTX = nil
	assert.Equal(t, tx, newTx)
}
