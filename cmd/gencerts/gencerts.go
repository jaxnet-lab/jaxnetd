// Copyright (c) 2013-2014 The btcsuite developers
// Copyright (c) 2020 The JaxNetwork developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	flags "github.com/jessevdk/go-flags"
	"gitlab.com/jaxnet/jaxnetd/jaxutil"
)

const defaultTimeCertValidYears = 10

type config struct {
	Directory    string   `short:"d" long:"directory" description:"Directory to write certificate pair"`
	Force        bool     `short:"f" long:"force" description:"Force overwriting of any old certs and keys"`
	ExtraHosts   []string `short:"H" long:"host" description:"Additional hosts/IPs to create certificate for"`
	Organization string   `short:"o" long:"org" description:"Organization in certificate"`
	Years        int      `short:"y" long:"years" description:"How many years a certificate is valid for"`
}

// nolint: gomnd
func main() {
	cfg := config{
		Years:        defaultTimeCertValidYears,
		Organization: "gencerts",
	}
	parser := flags.NewParser(&cfg, flags.Default)
	_, err := parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		}
		return
	}

	if cfg.Directory == "" {
		var err error
		cfg.Directory, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "no directory specified and cannot get working directory\n")
			os.Exit(1)
		}
	}
	cfg.Directory = cleanAndExpandPath(cfg.Directory)
	certFile := filepath.Join(cfg.Directory, "rpc.cert")
	keyFile := filepath.Join(cfg.Directory, "rpc.key")

	if !cfg.Force {
		if fileExists(certFile) || fileExists(keyFile) {
			fmt.Fprintf(os.Stderr, "%v: certificate and/or key files exist; use -f to force\n", cfg.Directory)
			os.Exit(1)
		}
	}

	validUntil := time.Now().Add(time.Duration(cfg.Years) * 365 * 24 * time.Hour)
	cert, key, err := jaxutil.NewTLSCertPair(cfg.Organization, validUntil, cfg.ExtraHosts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot generate certificate pair: %v\n", err)
		os.Exit(1)
	}

	// Write cert and key files.
	// nolint: gosec
	if err = ioutil.WriteFile(certFile, cert, 0o666); err != nil {
		fmt.Fprintf(os.Stderr, "cannot write cert: %v\n", err)
		os.Exit(1)
	}
	if err = ioutil.WriteFile(keyFile, key, 0o600); err != nil {
		os.Remove(certFile)
		fmt.Fprintf(os.Stderr, "cannot write key: %v\n", err)
		os.Exit(1)
	}
}

// cleanAndExpandPath expands environement variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		appHomeDir := jaxutil.AppDataDir("gencerts", false)
		homeDir := filepath.Dir(appHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// filesExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
