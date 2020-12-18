# JAX.Network - core difference and feature description

## Table of Content

1. Configuration;
2. Project structure;
3. Multi-chain:
    1. Beacon;
    2. Shards;
    3. P2P;
    4. RPC;
4. Mining:
    1. Blocks Headers;
    2. Block Version, Network Expansion;
    3. Merge-Mining;
    4. Difficulty Adjustment;
    5. Timestamp Adjustment;
    6. Block Reward;
4. [Transactions](#Transactions):
    1. [Cross-shard Swap](#cross-shard-swap);
    2. [Time-lock Allowance](#time-lock-allowance);
    3. [EAD Registration](#ead-registration);

## Transactions

### Cross-shard Swap

SwapTx is a special tx for an atomic swap between chains. This is transaction contains marker `wire.TxMarkShardSwap`
in `MsgTx.Version`.

It can contain only TWO inputs and TWO outputs.
`wire.TxIn` and `wire.TxOut` are strictly associated with each other by an index.

One pair corresponds to the one chain. The second is for another. The order is not deterministic.

| # | []TxIn | __ | []TxOut | # |
|---|--------|----|---------|---|
| 0 | TxIn_0 ∈ Shard_X | --> | TxOut_0 ∈ Shard_X | 0 |
| 1 | TxIn_1 ∈ Shard_Y | --> | TxOut_1 ∈ Shard_Y | 1 |

### Time-lock Allowance

### EAD Registration

Exchange Agents (EAs) can register their IPv4/IPv6 (and potentially in future DNS) addresses in the beacon chain, and
then would accept connections from clients in p2p manner on these interfaces and process the exchange operations in a p2p
manner.



| # | []TxIn | __ | []TxOut | # |
|---|--------|----|---------|---|
| 0 | TxIn_0 | --> | TxOut_0 | 0 |
| 1 | ------ | --> | TxOut_1 | 1 |
| . | ------ | --> | ....... | . |
| N | ------ | --> | TxOut_N | N |
