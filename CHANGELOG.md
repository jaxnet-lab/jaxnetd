# jaxnetd Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## [0.4.8]

- **[PROTOCOL FIX]** - fixed validation of the jax vanity prefix.
- **[PROTOCOL FIX]** - it is allowed to set 3 or 4 outputs for TYPE_A coinbase transaction.
- **[RPC]** - Added new getters `ShardID() uint32` and `Config() *ConnConfig` to `rpcclient.Client`. 
- Passthrough the request ID into the handler's context.
- Added witness UTXO spending support for `txbuilder`
- Added witness address support in `txutils` package
- 

## [0.4.7]

- Repaired WebSocket handlers

## [0.4.6]

- Configured packaging and publishing
- Calculate a new difficulty before the coinbase creation and pass difficulty as `CalcBlockSubsidy` argument.

## [0.4.3]

- Replaced standard `sha256` by  [sha256-simd](https://github.com/minio/sha256-simd).
- Improved speed of the catching up of the chain by saving of the index of the best chain.
- Added the ability to batch initialize the MMR tree.
- Removed prometheus metrics, use standalone [monitoring daemon](https://gitlab.com/jaxnet/core/jaxnetd-monitor)
  instead.
- Added saving the index of the best chain to the database. This speeds up the startup of the node.
- Dropped support of the `shard.json`. Now this data is housed in the beacon database.
- Added the automining mode for the CPU-miner.
- Allowed mining of the outdated testnet chain.
- Configuration: made `mining_address` optional.
- Configuration: added optional `enabled_shards` list configuration.
- Added simple db-inspector to check the db sanity.
- Replaced standard `encoding/json` in RPC client to [easyjson](https://github.com/mailru/easyjson)
- Extended ListShard response with shard genesis hash.
- **[RPC BREAKING CHANGES]** - The `GetShardBlockTemplate` and `GetBeaconBlockTemplate` methods are no longer supported,
  use `GetBlockTemplate`.
- **[RPC BREAKING CHANGES]** - The responses of methods `GetBeaconBlockHeaderVerbose`,  `GetShardBlockHeaderVerbose`
  ,  `GetBeaconBlockVerbose`, `GetShardBlockVerbose`  `GetBeaconBlockVerboseTx`, `GetShardBlockVerboseTx` have been
  extended.

## [0.4.2]

- Changed default jaxBurnScript and validation of vanity prefixes
- Finalized Mainnet Genesis with BTC Block 707634
- Fix vanity check for BCH
- Small clean-up
- Updated MMT validation
- Updated coinbase validation

## [0.4.1]

- Fixed shard genesis generation.
- Re-implemented block-serial-id, now it works as expected.
- Added last serialID to best state.
- Added saving mmr-roots to the db.
- Added Height & ChainWeight in block header
- Added magic byte as first bytes in serialized header and blocks. This allows you to automatically handle what type of
  data is BeaconHeader or ShardHeader.
- Replaced CmdBlock by CmdBlockBox. This allows to push Actual MMR Root for the Block during sync.

## [0.4.0]

- Removed TxMark, now CSTX (CrossShard Swap Tx aka SwapTx) has own version - `wire.TxVerCrossShardSwap` = 4.
- Keep only one timestamp filed for all block headers.
- Removed redundant fields and Copy operations at the `blocknode.BeaconBlockNode` and `blocknode.ShardBlockNode` impl.
- Enabled hash-sorting.
- Added `ListBeaconBlocksBySerialNumber`, `ListShardBlocksBySerialNumber` RPC calls.
- Changed precision of JAX to 4 digit.
- Fix the Tx Mempool Relay.
- Added HTLC transaction.
- Added special markers in coinbase SignatureScript of BTC for the safe merge-mining.
- Changed validation rules of CoinbaseAux (Proof of Burn) at the beacon and shard chains.
- Introduced the Merkle Mountain Range for the chains. Replaced PrevBlockHash by the BlocksMMRRoot.
- Removed of some BTC historical&outdated features.
- Proper implementation of K and VoteK function.

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

[0.4.8]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.4.8

[0.4.7]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.4.7

[0.4.6]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.4.6

[0.4.4]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.4.4

[0.4.3]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.4.3

[0.4.2]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.4.2

[0.4.1]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.4.1

[0.4.0]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.4.0

[0.3.12-14]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.3.14

[0.3.11]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.3.11

[0.3.10]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.3.10

[0.3.9]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.3.9

[0.3.8]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.3.8

[0.3.7]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.3.7

[0.3.5]: https://gitlab.com/jaxnet/jaxnetd/-/releases/v0.3.5
