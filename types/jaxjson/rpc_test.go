package jaxjson

import (
	"encoding/json"
	"fmt"
	"testing"
)

type Request2 struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	//Params  *json.RawMessage `json:"params"`
	ID interface{} `json:"id"`
}

func TestIsValidIDType(t *testing.T) {
	rpc := "{\"id\":\"1592982359788\",\"method\":\"getchaintxstats\"}"
	req := Request2{}
	err := json.Unmarshal([]byte(rpc), &req)
	fmt.Println(err)
	fmt.Println(req)
	if err != nil {
		t.Error(err)
	}
}
