<br/>


![JAX.Network Logo](https://jax.network/wp-content/uploads/2020/10/logo.png)  

# jaxnetd

jaxnetd is a full node JAX.Network implementation written in Go (golang).


## Requirements

[Go](http://golang.org) 1.17 or newer.

## Installation

#### Pre-compiled binary

Fetch archive with latest release binary from [release page](https://gitlab.com/jaxnet/jaxnetd/-/releases)

#### Linux/BSD/MacOSX/POSIX - Build from Source

- Install Go according to the installation instructions here:
  http://golang.org/doc/install

- Ensure Go was installed properly and is a supported version:

```bash
$ go version
$ go env GOROOT GOPATH
```

NOTE: The `GOROOT` and `GOPATH` above must not be the same path. It is
recommended that `GOPATH` is set to a directory in your home directory such as
`~/goprojects` to avoid write permission issues.  It is also recommended adding
`$GOPATH/bin` to your `PATH` at this point.

- Run the following commands to obtain **jaxnetd**, all dependencies, and install it:

```bash
$ cd $GOPATH/src/gitlab.com/jaxnet/jaxnetd
$ GO111MODULE=on go install -v . ./cmd/...
```

- **jaxnetd** (and utilities) will now be installed in ```$GOPATH/bin```.  If you did
  not already add the bin directory to your system path during Go installation,
  we recommend you do so now.

## Updating

#### Linux/BSD/MacOSX/POSIX - Build from Source

- Run the following commands to update btcd, all dependencies, and install it:

```bash
$ cd $GOPATH/src/gitlab.com/jaxnet/jaxnetd
$ git pull
$ GO111MODULE=on go install -v . ./cmd/...
```

## Getting Started

jaxnetd has several configuration options available to tweak how it runs, but all the basic operations described in the intro section work with zero
configuration.

### Configuration

Edit the at least the `data_dir`, `mining_addresses`, `user`, `password` in [jaxnetd.mainnet.toml](./jaxnetd.mainnet.toml) 


#### Linux/BSD/POSIX/Source

```bash
$ go build .
$ ./jaxnetd -C jaxnetd.mainnet.toml
```


## Issue Tracker

The [integrated github issue tracker](https://gitlab.com/jaxnet/jaxnetd/issues)
is used for this project.

## Documentation

The documentation is a work-in-progress.  It is located in the [docs](https://gitlab.com/jaxnet/jaxnetd/tree/master/docs) folder.

## Release Verification

Please see our [documentation on the current build/verification
process](https://github.com/btcsuite/btcd/tree/master/release) for all our
releases for information on how to verify the integrity of published releases
using our reproducible build system.

## License

**jaxnetd** is licensed under the [copyfree](http://copyfree.org) ISC License.
