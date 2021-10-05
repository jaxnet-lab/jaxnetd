package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"gitlab.com/jaxnet/jaxnetd/network/p2p"
	"gitlab.com/jaxnet/jaxnetd/network/rpc"
	"gitlab.com/jaxnet/jaxnetd/node"
	"gitlab.com/jaxnet/jaxnetd/node/cprovider"
	"gopkg.in/yaml.v2"
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
	sampleConfigFile := filepath.Join(filepath.Dir(path), "sample-jaxnetd.conf")

	// Setup a temporary directory
	tmpDir, err := ioutil.TempDir("", "jaxnetd")
	if err != nil {
		t.Fatalf("Failed creating a temporary directory: %v", err)
	}
	testpath := filepath.Join(tmpDir, "test.conf")

	// copy config file to location of jaxnetd binary
	data, err := ioutil.ReadFile(sampleConfigFile)
	if err != nil {
		t.Fatalf("Failed reading sample config file: %v", err)
	}
	appPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		t.Fatalf("Failed obtaining app path: %v", err)
	}
	tmpConfigFile := filepath.Join(appPath, "sample-jaxnetd.conf")
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
func TestLoadConfig(t *testing.T) {
	cfg := node.Config{
		ConfigFile:  "",
		ShowVersion: false,
		Node: node.InstanceConfig{
			BeaconChain: cprovider.ChainRuntimeConfig{
				SigCacheMaxSize:    0,
				AddCheckpoints:     nil,
				AddrIndex:          false,
				MaxPeers:           0,
				BlockMaxSize:       0,
				BlockMinSize:       0,
				BlockMaxWeight:     0,
				BlockMinWeight:     0,
				BlockPrioritySize:  0,
				TxIndex:            false,
				NoRelayPriority:    false,
				RejectReplacement:  false,
				RelayNonStd:        false,
				FreeTxRelayLimit:   0,
				MaxOrphanTxs:       0,
				MinRelayTxFee:      0,
				NoCFilters:         false,
				DisableCheckpoints: false,
				MiningAddresses:    nil,
				AutoExpand:         false,
			},
			RPC: rpc.Config{
				ListenerAddresses: nil,
				MaxClients:        0,
				User:              "",
				Password:          "",
				Disable:           false,
				LimitPass:         "",
				LimitUser:         "",
				MaxConcurrentReqs: 0,
				MaxWebsockets:     0,
				Listeners:         nil,
				WSEnable:          false,
			},
			P2P: p2p.Config{
				Peers:           nil,
				Listeners:       nil,
				AgentBlacklist:  nil,
				AgentWhitelist:  nil,
				DisableListen:   false,
				ExternalIPs:     nil,
				ConnectPeers:    nil,
				BanDuration:     0,
				BanThreshold:    0,
				DisableBanning:  false,
				BlocksOnly:      false,
				OnionProxy:      "",
				OnionProxyPass:  "",
				OnionProxyUser:  "",
				Proxy:           "",
				ProxyPass:       "",
				ProxyUser:       "",
				RejectNonStd:    false,
				TrickleInterval: 0,
				DisableDNSSeed:  false,
				NoOnion:         false,
				Upnp:            false,
				Oniondial:       nil,
				Dial:            nil,
				Lookup:          nil,
			},
			Shards: node.ShardConfig{
				Enable:  false,
				Autorun: false,
			},
			DbType:         "",
			Net:            "",
			EnableCPUMiner: false,
		},
		DataDir:       "",
		LogDir:        "",
		CPUProfile:    "",
		DebugLevel:    "",
		Profile:       "",
		DropAddrIndex: false,
		DropCfIndex:   false,
		DropTxIndex:   false,
		TorIsolation:  false,
		Whitelists:    nil,
	}
	data, _ := yaml.Marshal(cfg)
	println(string(data))
}
