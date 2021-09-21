# jaxnetd Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

- Removed TxMark, now CSTX (CrossShard Swap Tx aka SwapTx) has own version - `wire.TxVerCrossShardSwap` = 4.
- Keep only one timestamp filed for all block headers.
- Removed redundant fields and Copy operations from `blocknode.BeaconBlockNode` and `blocknode.ShardBlockNode`
- Enabled hash-sorting.


## [0.3.12-14]

- Removed usage of the `IShardsMergedMiningTree`, because it is broken, useless and part of abandoned feature.
- Extend response of the `GetBlockTxOps`
- Exposed internal RPC types to provide possibility compose compatible RPC server.
- Allow the common BTC coinbase tx format if no burning.
- Added toml annotations for config.
- Tweak PoW limits, initial target and other PoW params.

## [0.3.11]

- Added support of the `toml` config format, fixed json-mode for log output;
- Fixed comparing of EADAddresses and record update of EADAddress when connecting tx;
- Added optional sorting of the pubkeys in Multisig and MultisigLock scripts. 

## [0.3.10]

- Changed format of EADScript - need to clean stack;
- Increased lock period for the CSTL tx;
- Added possibility to spend EADAddress UTXO using txbuilder and txutils;

## [0.3.9]

- Upgraded format of the EADAddress and script: now is possible to set OR `IP`, OR `URL` of the Agent.


## [0.3.8]

- Added `estimateSwapLockTime` and `getTxMethods` RPC calls
- Added db exporter (don't support shards for now)

## [0.3.7]

- Added `estimateLockTime` and `getMempoolUTXOs` RPC calls
- Normalize fee for the shard chains;
- Fix the `getTxOutsStatus` method;
- Added the network diagram to docs.

## [0.3.7]

- Added `estimateLockTime` and `getMempoolUTXOs` RPC calls
- Normalize fee for the shard chains;
- Fix the `getTxOutsStatus` method;
- Added the network diagram to docs.

## [0.3.5] - 2021-06-30: Initial testnet release

- Implemented basic Jax.Net protocol:
    1. Multi-chain network - Beacon and Shards;
    2. Merge-mining of Shard Chains;
    3. Merge-mining with Bitcoin;
    4. Cross-Shard Swap Transaction;
    5. Registration of the Exchange Agents in Beacon;

- Improve and extend RPC Api;
- Added `jaxutils/txmodels` and `jaxutils/txutils` packages for building transactions;
- Introduces `zerolog` for structured logging and `prometheus` for monitoring.
