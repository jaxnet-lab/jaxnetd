package main

import (
	"context"
	"encoding/csv"
	"os"
	"strconv"
)

type CSVRow interface {
	Header() []string
	CSV() []string
}

type Output struct {
	PkScript     string `csv:"pk_script"`   // hex encoded pkScript
	PkScriptType string `csv:"script_type"` // pkScript type
	Addresses    string `csv:"addresses"`   // list of addresses extracted from pkScript
	OutID        int    `csv:"out_id"`      // serial number of input in tx or output if !input
	Amount       int64  `csv:"amount"`      // amount of coins in satoshi
	TxHash       string `csv:"tx_hash"`     // hash of this tx
	TxIndex      int    `csv:"tx_index"`    // serial number of tx in block
	Coinbase     bool   `csv:"coinbase"`    // indicates is this coinbase tx
	BlockHash    string `csv:"block_hash"`
	BlockHeight  int64  `csv:"block_height"`
}

func (Output) Header() []string {
	return []string{
		"pk_script",
		"script_type",
		"addresses",
		"out_id",
		"amount",
		"tx_hash",
		"tx_index",
		"coinbase",
		"block_height",
		"block_hash",
	}
}

func (o Output) CSV() []string {
	coinbase := "false"
	if o.Coinbase {
		coinbase = "true"
	}
	return []string{
		o.PkScript,
		o.PkScriptType,
		o.Addresses,
		strconv.Itoa(o.OutID),
		strconv.Itoa(int(o.Amount)),
		o.TxHash,
		strconv.Itoa(o.TxIndex),
		coinbase,
		strconv.Itoa(int(o.BlockHeight)),
		o.BlockHash,
	}
}

type Input struct {
	SignatureScript string `csv:"sig_script"`     // hex encoded sigScript
	InID            int    `csv:"in_id"`          // serial number of input in tx
	TxHash          string `csv:"tx_hash"`        // hash of this tx
	TxIndex         int    `csv:"tx_index"`       // serial number of tx in block
	OriginTxHash    string `csv:"origin_tx_hash"` // tx hash of parent utxo
	OriginIdx       int    `csv:"origin_idx"`     // serial number of output in tx if input
	BlockHash       string `csv:"block_hash"`
	BlockHeight     int64  `csv:"block_height"`
}

func (Input) Header() []string {
	return []string{
		"sig_script",
		"in_id",
		"tx_hash",
		"tx_index",
		"origin_tx_hash",
		"origin_idx",
		"block_height",
		"block_hash",
	}
}

func (i Input) CSV() []string {
	return []string{
		i.SignatureScript,
		strconv.Itoa(i.InID),
		i.TxHash,
		strconv.Itoa(i.TxIndex),
		i.OriginTxHash,
		strconv.Itoa(i.OriginIdx),
		strconv.Itoa(int(i.BlockHeight)),
		i.BlockHash,
	}
}

type Block struct {
	BlockHeight int64  `csv:"blockHeight"`
	BlockHash   string `csv:"blockHash"`
	TxCount     int    `csv:"txCount"`
	InCount     int    `csv:"inCount"`
	OutCount    int    `csv:"outCount"`
	Weight      int    `csv:"weight"`
	Bits        uint32 `csv:"bits"`
	Timestamp   string `csv:"timestamp"`
}

func (Block) Header() []string {
	return []string{
		"height",
		"hash",
		"tx_count",
		"in_count",
		"out_count",
		"weight",
		"bits",
		"timestamp",
	}
}

func (b Block) CSV() []string {
	return []string{
		strconv.Itoa(int(b.BlockHeight)),
		b.BlockHash,
		strconv.Itoa(b.TxCount),
		strconv.Itoa(b.InCount),
		strconv.Itoa(b.OutCount),
		strconv.Itoa(b.Weight),
		strconv.Itoa(int(b.Bits)),
	}
}

// --------------------  //

type UTXO struct {
	Address  int
	TxHash   int
	OutID    int
	Amount   int64
	PkScript string
	State    int
}

func (UTXO) Header() []string {
	return []string{
		"address",
		"tx_hash",
		"tx_output",
		"amount",
		"pkscript",
		"state",
	}
}

func (b UTXO) CSV() []string {
	return []string{
		strconv.Itoa(b.Address),
		strconv.Itoa(b.TxHash),
		strconv.Itoa(b.OutID),
		strconv.Itoa(int(b.Amount)),
		b.PkScript,
		strconv.Itoa(b.State),
	}
}

type ShortInput struct {
	TxHash      int
	InputTxHash int
	InputTxIDx  int
}

func (ShortInput) Header() []string {
	return []string{
		"tx_hash",
		"input_tx_hash",
		"input_tx_idx",
	}
}

func (b ShortInput) CSV() []string {
	return []string{
		strconv.Itoa(b.TxHash),
		strconv.Itoa(b.InputTxHash),
		strconv.Itoa(b.InputTxIDx),
	}
}

type AddressTx struct {
	Address   int
	TxHash    int
	OutID     int
	Direction bool // Direction bool -- true = incoming; false = outgoing

}

func (AddressTx) Header() []string {
	return []string{
		"address",
		"tx_hash",
		"output_number",
		"direction",
	}
}

func (b AddressTx) CSV() []string {
	return []string{
		strconv.Itoa(b.Address),
		strconv.Itoa(b.TxHash),
		strconv.Itoa(b.OutID),
		boolToStr(b.Direction),
	}
}

type TxOperation struct {
	TxHash      string // hash of this tx
	TxIndex     int    // serial number of tx in block
	Address     string // list of addresses extracted from pkScript
	Amount      int64  // amount of coins in satoshi
	BlockNumber int
	IsInput     bool   // indicates is this tx input or output
	PkScript    string // hex encoded pkScript
	DateTime    string
}

func (TxOperation) Header() []string {
	return []string{
		"tx_hash",
		"tx_index",
		"address",
		"amount",
		"is_input",
		"block_number",
		"pkscript",
		"datetime",
	}
}

func (b TxOperation) CSV() []string {
	return []string{
		b.TxHash,
		strconv.Itoa(b.TxIndex),
		b.Address,
		strconv.Itoa(int(b.Amount)),
		strconv.Itoa(b.BlockNumber),
		boolToStr(b.IsInput),
		b.PkScript,
		b.DateTime,
	}
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

type Hash struct {
	ID   int
	Hash string
}

func (Hash) Header() []string {
	return []string{
		"id",
		"hash",
	}
}

func (b Hash) CSV() []string {
	return []string{
		strconv.Itoa(b.ID),
		b.Hash,
	}
}

type Address struct {
	ID      int
	Address string
}

func (Address) Header() []string {
	return []string{
		"id",
		"address",
	}
}

func (b Address) CSV() []string {
	return []string{
		strconv.Itoa(b.ID),
		b.Address,
	}
}

type CSVStorage struct {
	path            string
	file            *os.File
	wrt             *csv.Writer
	firstRowWritten bool
}

func NewCSVStorage(path string) (*CSVStorage, error) {
	s := &CSVStorage{path: path}
	err := s.open(false, true)
	if err != nil {
		return nil, err
	}
	return s, err
}

func (storage *CSVStorage) open(readOnly, truncate bool) error {
	mode := os.O_RDWR | os.O_CREATE
	if truncate {
		mode |= os.O_TRUNC
	}

	if readOnly {
		mode = os.O_RDONLY
	}

	file, err := os.OpenFile(storage.path, mode, 0644)
	if os.IsPermission(err) {
		file, err = os.Create(storage.path)
	}

	storage.file = file

	storage.wrt = csv.NewWriter(storage.file)
	return err
}

func (storage *CSVStorage) Close() error {
	if storage.wrt != nil {
		storage.wrt.Flush()
	}

	if storage.file != nil {
		return storage.file.Close()
	}
	return nil
}

type row struct {
	flush bool
	data  CSVRow
}

func (storage *CSVStorage) WriteData(ctx context.Context, input <-chan row) error {
	log.Infof("Start writer: file=%s", storage.path)

	for {
		select {
		case <-ctx.Done():
			log.Infof("Close writer: file=%s", storage.path)
			return storage.Close()
		case r := <-input:
			if !storage.firstRowWritten {
				err := storage.wrt.Write(r.data.Header())
				if err != nil {
					log.Error(err)
				}
				storage.firstRowWritten = true
			}

			err := storage.wrt.Write(r.data.CSV())
			if err != nil {
				log.Error(err)
			}
			if r.flush {
				storage.wrt.Flush()
			}
		}
	}
}
