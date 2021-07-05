# jaxnetd Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


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
