package main

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfig(t *testing.T) {
	cfg := config{
		ConfigFile:    "",
		ListCommands:  false,
		NoTLS:         false,
		Proxy:         "",
		ProxyPass:     "",
		ProxyUser:     "",
		RPCCert:       "",
		RPCPassword:   "",
		RPCServer:     "",
		RPCUser:       "",
		TLSSkipVerify: false,
		ShowVersion:   false,
		Wallet:        false,
	}

	data, _ := yaml.Marshal(cfg)
	println(string(data))
}
