package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

var (
	rpcuserRegexp = regexp.MustCompile("(?m)^rpcuser=.+$")
	rpcpassRegexp = regexp.MustCompile("(?m)^rpcpass=.+$")
)

func TestCreateDefaultConfigFile(t *testing.T) {
	// find out where the sample config lives
	_, path, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("Failed finding config file path")
	}
	sampleConfigFile := filepath.Join(filepath.Dir(path), "sample-btcd.conf")

	// Setup a temporary directory
	tmpDir, err := ioutil.TempDir("", "btcd")
	if err != nil {
		t.Fatalf("Failed creating a temporary directory: %v", err)
	}
	testpath := filepath.Join(tmpDir, "test.conf")

	// copy config file to location of btcd binary
	data, err := ioutil.ReadFile(sampleConfigFile)
	if err != nil {
		t.Fatalf("Failed reading sample config file: %v", err)
	}
	appPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		t.Fatalf("Failed obtaining app path: %v", err)
	}
	tmpConfigFile := filepath.Join(appPath, "sample-btcd.conf")
	err = ioutil.WriteFile(tmpConfigFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed copying sample config file: %v", err)
	}

	// Clean-up
	defer func() {
		os.Remove(testpath)
		os.Remove(tmpConfigFile)
		os.Remove(tmpDir)
	}()

	err = createDefaultConfigFile(testpath)

	if err != nil {
		t.Fatalf("Failed to create a default config file: %v", err)
	}

	content, err := ioutil.ReadFile(testpath)
	if err != nil {
		t.Fatalf("Failed to read generated default config file: %v", err)
	}

	if !rpcuserRegexp.Match(content) {
		t.Error("Could not find rpcuser in generated default config file.")
	}

	if !rpcpassRegexp.Match(content) {
		t.Error("Could not find rpcpass in generated default config file.")
	}
}

func TestYAMLOverwrite(t *testing.T) {
	var testYAMLCfg = `
listeners:
  - "0.0.0.0:8333"

rpc_listeners:
  - ":18334"

rpc_pass: "rpc_pass"
rpc_user: "rpc_user"

simnet: true
`

	var cfg = config{
		AddPeers: []string{
			"127.0.0.0:18444",
			"127.0.0.0:28444",
			"127.0.0.0:38444",
		},
		DataDir:         defaultDataDirname,
		DbType:          defaultDbType,
		RPCPass:         "password",
		RPCUser:         "user",
		SigCacheMaxSize: 0,
		SimNet:          false,
	}

	err := yaml.Unmarshal([]byte(testYAMLCfg), &cfg)
	assert.NoError(t, err)
	assert.Equal(t, defaultDataDirname, cfg.DataDir)
	assert.Equal(t, defaultDbType, cfg.DbType)
	assert.Equal(t, "rpc_user", cfg.RPCUser)
	assert.Equal(t, "rpc_pass", cfg.RPCPass)
	assert.Equal(t, true, cfg.SimNet)
	assert.Equal(t, []string{"0.0.0.0:8333"}, cfg.Listeners)
	assert.Equal(t, []string{":18334"}, cfg.RPCListeners)
	assert.Equal(t, []string{
		"127.0.0.0:18444",
		"127.0.0.0:28444",
		"127.0.0.0:38444",
	}, cfg.AddPeers)
}
