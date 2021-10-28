// Copyright (c) 2013-2015 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
	"gitlab.com/jaxnet/jaxnetd/types/chaincfg"
	"gitlab.com/jaxnet/jaxnetd/types/jaxjson"
	"gopkg.in/yaml.v3"
)

const (
	// unusableFlags are the command usage flags which this utility are not
	// able to use.  In particular it doesn't support websockets and
	// consequently notifications.
	unusableFlags = jaxjson.UFWebsocketOnly | jaxjson.UFNotification
)

var (
	coreHomeDir           = jaxutil.AppDataDir("jaxnetd", false)
	ctlHomeDir            = jaxutil.AppDataDir("jaxctl", false)
	walletHomeDir         = jaxutil.AppDataDir("jaxwallet", false)
	defaultConfigFile     = "jaxctl.yaml"
	defaultRPCServer      = "localhost"
	defaultRPCCertFile    = filepath.Join(coreHomeDir, "rpc.cert")
	defaultWalletCertFile = filepath.Join(walletHomeDir, "rpc.cert")
)

// listCommands categorizes and lists all of the usable commands along with
// their one-line usage.
func listCommands() {
	const (
		categoryChain uint8 = iota
		categoryWallet
		numCategories
	)

	// Get a list of registered commands and categorize and filter them.
	cmdMethods := jaxjson.RegisteredCmdMethods()
	categorized := make([][]string, numCategories)
	for _, method := range cmdMethods {
		usageFlags, err := jaxjson.MethodUsageFlags(method)
		if err != nil {
			// This should never happen since the method was just
			// returned from the package, but be safe.
			continue
		}

		// Skip the commands that aren't usable from this utility.
		if usageFlags&unusableFlags != 0 {
			continue
		}

		usage, err := jaxjson.MethodUsageText(method)
		if err != nil {
			// This should never happen since the method was just
			// returned from the package, but be safe.
			continue
		}

		// Categorize the command based on the usage flags.
		category := categoryChain
		if usageFlags&jaxjson.UFWalletOnly != 0 {
			// category = categoryWallet
			continue
		}
		categorized[category] = append(categorized[category], usage)
	}

	// Display the command according to their categories.
	categoryTitles := make([]string, numCategories)
	categoryTitles[categoryChain] = "Chain Server Commands:"
	// wallet is not available now
	categoryTitles[categoryWallet] = "Wallet Server Commands (--wallet):"
	for category := uint8(0); category < numCategories; category++ {
		fmt.Println(categoryTitles[category])
		for _, usage := range categorized[category] {
			fmt.Println(usage)
		}
		fmt.Println()
	}
}

// config defines the configuration options for jaxctl.
//
// See loadConfig for details on the configuration load process.
type config struct {
	ConfigFile   string `yaml:"-" short:"C" long:"configfile" description:"Path to configuration file"`
	ListCommands bool   `yaml:"-" short:"l" long:"listcommands" description:"List all of the supported commands and exit"`
	ShowVersion  bool   `yaml:"-" short:"V" long:"version" description:"Display version information and exit"`
	ShardID      uint32 `yaml:"shard_id" short:"S" long:"shardid" description:"Send request to some shard, if not set request will be send to beacon"`

	Net         string `yaml:"net" description:"Network name"`
	RPCServer   string `yaml:"rpc_server" short:"s" long:"rpcserver" description:"RPC server to connect to"`
	RPCUser     string `yaml:"rpc_user" short:"u" long:"rpcuser" description:"RPC username"`
	RPCPassword string `yaml:"rpc_password" short:"P" long:"rpcpass" default-mask:"-" description:"RPC password"`
	RPCCert     string `yaml:"rpc_cert" short:"c" long:"rpccert" description:"RPC server certificate chain for validation"`

	Proxy     string `yaml:"proxy" long:"proxy" description:"Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)"`
	ProxyPass string `yaml:"proxy_pass" long:"proxypass" default-mask:"-" description:"Password for proxy server"`
	ProxyUser string `yaml:"proxy_user" long:"proxyuser" description:"Username for proxy server"`

	NoTLS         bool `yaml:"no_tls" long:"notls" description:"Disable TLS"`
	TLSSkipVerify bool `yaml:"tls_skip_verify" long:"skipverify" description:"Do not verify tls certificates (not recommended!)"`
	Wallet        bool `yaml:"wallet" long:"wallet" description:"Connect to wallet"`
}

// normalizeAddress returns addr with the passed default port appended if
// there is not already a port specified.
func normalizeAddress(addr string, chain *chaincfg.Params, useWallet bool) (string, error) {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		var defaultPort string
		switch chain {
		case &chaincfg.TestNet3Params:
			if useWallet {
				defaultPort = "18332"
			} else {
				defaultPort = "18333"
			}
		case &chaincfg.SimNetParams:
			if useWallet {
				defaultPort = "18554"
			} else {
				defaultPort = "18556"
			}
		default:
			if useWallet {
				defaultPort = "8332"
			} else {
				defaultPort = "18333"
			}
		}

		return net.JoinHostPort(addr, defaultPort), nil
	}
	return addr, nil
}

// cleanAndExpandPath expands environement variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(ctlHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// loadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
// 	1) Start with a default config with sane settings
// 	2) Pre-parse the command line to check for an alternative config file
// 	3) Load configuration file overwriting defaults with any specified options
// 	4) Parse CLI options and overwrite/add any specified options
//
// The above results in functioning properly without any config settings
// while still allowing the user to override settings with config files and
// command line options.  Command line options always take precedence.
func loadConfig() (*config, []string, error) {
	// Default config.
	cfg := config{
		ConfigFile: defaultConfigFile,
		RPCServer:  defaultRPCServer,
		RPCCert:    defaultRPCCertFile,
	}

	// Pre-parse the command line options to see if an alternative config
	// file, the version flag, or the list commands flag was specified.  Any
	// errors aside from the help message error can be ignored here since
	// they will be caught by the final parse below.
	preCfg := cfg
	preParser := flags.NewParser(&preCfg, flags.HelpFlag)
	_, err := preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "The special parameter `-` "+
				"indicates that a parameter should be read "+
				"from the\nnext unread line from standard "+
				"input.")
			return nil, nil, err
		}
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show options", appName)
	if preCfg.ShowVersion {
		fmt.Println(appName, "version", version())
		os.Exit(0)
	}

	// Show the available commands and exit if the associated flag was
	// specified.
	if preCfg.ListCommands {
		listCommands()
		os.Exit(0)
	}

	stats, err := os.Stat(preCfg.ConfigFile)
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error open a config file: %v\n", err)
		return nil, nil, err
	}

	// Load additional config from file.
	parser := flags.NewParser(&cfg, flags.Default)
	cfgFile, err := os.OpenFile(preCfg.ConfigFile, os.O_RDONLY, 0644)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing config file: %v\n", err)
		_, _ = fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	ext := filepath.Ext(stats.Name())
	switch ext {
	case ".yaml":
		err = yaml.NewDecoder(cfgFile).Decode(&cfg)
		if err != nil {
			return nil, nil, err
		}
	default:
		_, _ = fmt.Fprintln(os.Stderr, "Invalid file extension:", ext)
		return nil, nil, errors.New("Invalid file extension: " + ext)
	}

	// Parse command line options again to ensure they take precedence.
	remainingArgs, err := parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			fmt.Fprintln(os.Stderr, usageMessage)
		}
		return nil, nil, err
	}

	network := chaincfg.NetName(cfg.Net).Params()
	if network == nil {
		str := "%s: Unknown network name - %s"
		err := fmt.Errorf(str, "loadConfig", cfg.Net)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	// Override the RPC certificate if the --wallet flag was specified and
	// the user did not specify one.
	if cfg.Wallet && cfg.RPCCert == defaultRPCCertFile {
		cfg.RPCCert = defaultWalletCertFile
	}

	// Handle environment variable expansion in the RPC certificate path.
	cfg.RPCCert = cleanAndExpandPath(cfg.RPCCert)

	// Add default port to RPC server based on --testnet and --wallet flags
	// if needed.
	cfg.RPCServer, err = normalizeAddress(cfg.RPCServer, network, cfg.Wallet)
	if err != nil {
		return nil, nil, err
	}

	return &cfg, remainingArgs, nil
}
