package jaxjson

import (
	"encoding/json"
	"testing"

	jsoniter "github.com/json-iterator/go"
)

var (
	err           error
	marshalResult []byte

	chainStatsMarshalled = []byte(`
	{
		"time": 1639564816,
		"txcount": 12344,
		"txrate": "rate",
		"window_final_block_hash": "b985dc8f30ebc203c4b73a454b11304c67860d55255b75a9ac2b2f05614f6ade",
		"window_final_block_height": 55643, 
		"window_block_count": 123, 
		"window_tx_count": 1323, 
		"window_interval": 10
	}`)  // 238 B

	beaconBlockTemplateMarshalled = []byte(`
	{
    	"bits":"1d061b78",
    	"chainweight":"1728595899272192",
    	"curtime":1639565225,
    	"height":17587,
    	"previousblockhash":"1c6f5256eb4a28053563dfaa9541ddb32a6d838345999ecc51c22bb8e3798cbf",
    	"prevblocksmmrroot":"0a3ef600dab50d2262ea98556140dc79c1a63e0ee6159a95915a5268f15b3d1b",
    	"serialID":17835,
    	"prevSerialID":17834,
    	"sigoplimit":80000,
    	"sizelimit":786432,
    	"weightlimit":4000000,
    	"transactions":[
    	    
    	],
    	"version":1,
    	"shards":3,
    	"coinbaseaux":{
    	    "flags":"0e2f503253482f6a61786e6574642f"
    	},
    	"coinbasetxn":{
    	    "data":"01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1402b34400000e2f503253482f6a61786e6574642fffffffff0400000000000000001976a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac00943577000000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00e288c00600000023b303a08c00a06376a914b953dad0e79288eea918085c9b72c3ca5482349388ac676a6800000000000000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00000000",
    	    "hash":"1a9636c4aa0a2b2b0222eb158e2465ef3b17ea97e16c44f3b64b54c8c8b7f605",
    	    "depends":[
    	        
    	    ],
    	    "fee":0,
    	    "sigops":16,
    	    "weight":0
    	},
    	"coinbasevalue":31000000000,
    	"longpollid":"1c6f5256eb4a28053563dfaa9541ddb32a6d838345999ecc51c22bb8e3798cbf-1639424712",
    	"target":"000000061b780000000000000000000000000000000000000000000000000000",
    	"maxtime":1639572425,
    	"mintime":1639148691,
    	"mutable":[
    	    "time",
    	    "transactions/add",
    	    "prevblock",
    	    "coinbase/append"
    	],
    	"noncerange":"00000000ffffffff",
    	"capabilities":[
    	    "proposal"
    	], 
    	"btcAux":"0000000000000000000000000000000000000000000000000000000000000000000000007cdd0f0dd81233cf422215543925e23802dd2cdd20fd19244aff558785c23506a9c7b96132260e170000000001000000010000000000000000000000000000000000000000000000000000000000000000ffffffff484f080000000000000000066a61786e6574208b9ae4cdbc3dcf741c116a8fe261e4cf3c0a786cf447557879ca08052cadddb3066a61786e65740e2f503253482f6a61786e6574642fffffffff0300000000000000001976a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac40be4025000000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac00000000000000001976a914b953dad0e79288eea918085c9b72c3ca5482349388ac0000000000",
    	"k":1980760064,
    	"vote_k":1980760064
	}`)  // 1.95 kB
)

func BenchmarkJSONLibs(b *testing.B) {
	algos := []struct {
		name string
		f    func(b *testing.B)
	}{
		{name: "decode-easyjson-small", f: benchmarkDecodeSmallEasyJSON},
		{name: "decode-jsoniter-small", f: benchmarkDecodeSmallJSONIter},
		{name: "decode-jsonstd-small", f: benchmarkDecodeSmallJSONStd},
		{name: "decode-easyjson-med", f: benchmarkDecodeMediumEasyJSON},
		{name: "decode-jsoniter-med", f: benchmarkDecodeMediumJSONIter},
		{name: "decode-jsonstd-med", f: benchmarkDecodeMediumJSONStd},
		{name: "encode-easyjson-small", f: benchmarkEncodeSmallEasyJSON},
		{name: "encode-jsoniter-small", f: benchmarkEncodeSmallJSONIter},
		{name: "encode-jsonstd-small", f: benchmarkEncodeSmallJSONStd},
		{name: "encode-easyjson-med", f: benchmarkEncodeMediumEasyJSON},
		{name: "encode-jsoniter-med", f: benchmarkEncodeMediumJSONIter},
		{name: "encode-jsonstd-med", f: benchmarkEncodeMediumJSONStd},
	}
	for _, a := range algos {
		n := a.name
		b.Run(n, func(b *testing.B) {
			a.f(b)
		})
	}
}

func benchmarkDecodeSmallEasyJSON(b *testing.B) {
	b.ReportAllocs()
	var errUnmarshal error
	for i := 0; i < b.N; i++ {
		var data GetChainStatsResult
		errUnmarshal = data.UnmarshalJSON(chainStatsMarshalled)
	}
	err = errUnmarshal
}

func benchmarkEncodeSmallEasyJSON(b *testing.B) {
	var (
		errUnmarshal error
		data         GetChainStatsResult
		marshalled   []byte
	)
	jsoniter.Unmarshal(chainStatsMarshalled, &data)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		marshalled, errUnmarshal = data.MarshalJSON()
	}
	err = errUnmarshal
	marshalResult = marshalled
}

func benchmarkDecodeMediumEasyJSON(b *testing.B) {
	b.ReportAllocs()
	var errUnmarshal error
	for i := 0; i < b.N; i++ {
		var data GetBlockTemplateResult
		errUnmarshal = data.UnmarshalJSON(beaconBlockTemplateMarshalled)
	}
	err = errUnmarshal
}

func benchmarkEncodeMediumEasyJSON(b *testing.B) {
	var (
		errUnmarshal error
		data         GetBlockTemplateResult
		marshalled   []byte
	)
	jsoniter.Unmarshal(beaconBlockTemplateMarshalled, &data)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		marshalled, errUnmarshal = data.MarshalJSON()
	}
	err = errUnmarshal
	marshalResult = marshalled
}

func benchmarkDecodeSmallJSONIter(b *testing.B) {
	b.ReportAllocs()
	var errUnmarshal error
	for i := 0; i < b.N; i++ {
		var data GetChainStatsResult
		errUnmarshal = jsoniter.Unmarshal(chainStatsMarshalled, &data)
	}
	err = errUnmarshal
}

func benchmarkEncodeSmallJSONIter(b *testing.B) {
	var (
		errUnmarshal error
		data         GetChainStatsResult
		marshalled   []byte
	)
	jsoniter.Unmarshal(chainStatsMarshalled, &data)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		marshalled, errUnmarshal = jsoniter.Marshal(data)
	}
	err = errUnmarshal
	marshalResult = marshalled
}

func benchmarkDecodeMediumJSONIter(b *testing.B) {
	b.ReportAllocs()
	var errUnmarshal error
	for i := 0; i < b.N; i++ {
		var data GetBlockTemplateResult
		errUnmarshal = jsoniter.Unmarshal(beaconBlockTemplateMarshalled, &data)
	}
	err = errUnmarshal
}

func benchmarkEncodeMediumJSONIter(b *testing.B) {
	var (
		errUnmarshal error
		data         GetBlockTemplateResult
		marshalled   []byte
	)
	jsoniter.Unmarshal(beaconBlockTemplateMarshalled, &data)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		marshalled, errUnmarshal = jsoniter.Marshal(data)
	}
	err = errUnmarshal
	marshalResult = marshalled
}

func benchmarkDecodeSmallJSONStd(b *testing.B) {
	b.ReportAllocs()
	var errUnmarshal error
	for i := 0; i < b.N; i++ {
		var data GetChainStatsResult
		errUnmarshal = json.Unmarshal(chainStatsMarshalled, &data)
	}
	err = errUnmarshal
}

func benchmarkEncodeSmallJSONStd(b *testing.B) {
	var (
		errUnmarshal error
		data         GetChainStatsResult
		marshalled   []byte
	)
	jsoniter.Unmarshal(chainStatsMarshalled, &data)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		marshalled, errUnmarshal = json.Marshal(data)
	}
	err = errUnmarshal
	marshalResult = marshalled
}

func benchmarkDecodeMediumJSONStd(b *testing.B) {
	b.ReportAllocs()
	var errUnmarshal error
	for i := 0; i < b.N; i++ {
		var data GetBlockTemplateResult
		errUnmarshal = json.Unmarshal(beaconBlockTemplateMarshalled, &data)
	}
	err = errUnmarshal
}

func benchmarkEncodeMediumJSONStd(b *testing.B) {
	var (
		errUnmarshal error
		data         GetBlockTemplateResult
		marshalled   []byte
	)
	jsoniter.Unmarshal(beaconBlockTemplateMarshalled, &data)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		marshalled, errUnmarshal = json.Marshal(data)
	}
	err = errUnmarshal
	marshalResult = marshalled
}
