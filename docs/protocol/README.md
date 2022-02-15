# JAX.Network

Protocol and feature description

> THIS DOCUMENT IS NOT FINISHED YET
> 
> some data and information can be outdated

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
4. [Transactions & Standard Scripts](#Transactions):
    1. [Cross-shard Swap](#cross-shard-swap);
    2. [MultiSig Lock with Refund](#MultiSig-Lock-with-Refund);
    3. [EAD Registration](#ead-registration);
    4. [Hybrid Time-Lock Contract (HTLC)](#hybrid-time-lock-contract)

## Transactions

### Cross-shard Swap

SwapTx is a special tx for an atomic swap between chains. This is transaction contains marker `wire.TxMarkShardSwap`
in `MsgTx.Version`.

```text
TxMarkShardSwap int32 = 1 << 16  = 65536;

tx.Version = TxVerRegular + TxMarkShardSwap = 65537
tx.Version = TxVerTimeLock + TxMarkShardSwap = 65538
```

It can contain only TWO or FOUR inputs and TWO or FOUR outputs.
`wire.TxIn` and `wire.TxOut` are strictly associated with each other by an index.

One pair corresponds to the one chain. The second is for another. The order is not deterministic.

1. Scheme 4x4:
  - { TxIn_0, TxIn_1, TxOut_0, TxOut_1 } ∈ Shard_X
  - { TxIn_2, TxIn_3, TxOut_2, TxOut_3 } ∈ Shard_Y

| #   | []TxIn | --- | []TxOut | #   |                        |
|-----|--------|-----|---------|-----|------------------------|
| 0   | TxIn_0 | --> | TxOut_0 | 0   | spend or fee @ Shard X |
| 1   | TxIn_1 | --> | TxOut_1 | 1   | spend or fee @ Shard X |
| 2   | TxIn_2 | --> | TxOut_2 | 2   | spend or fee @ Shard Y |
| 3   | TxIn_3 | --> | TxOut_3 | 3   | spend or fee @ Shard Y |

2. Scheme 2x2:
  - { TxIn_0, TxOut_0 } ∈ Shard_X
  - { TxIn_1, TxOut_1 } ∈ Shard_Y

| #   | []TxIn | --- | []TxOut | #   |                        |
|-----|--------|-----|---------|-----|------------------------|
| 0   | TxIn_0 | --> | TxOut_0 | 0   | spend or fee @ Shard X |
| 1   | TxIn_1 | --> | TxOut_1 | 1   | spend or fee @ Shard Y |

3. Scheme 4x2:
  - { TxIn_0, TxIn_1, TxOut_0 } ∈ Shard_X
  - { TxIn_2, TxIn_3, TxOut_1 } ∈ Shard_Y

| #   | []TxIn | --- | []TxOut | #   |                        |
|-----|--------|-----|---------|-----|------------------------|
| 0   | TxIn_0 | --> | TxOut_0 | 0   | spend or fee @ Shard X |
| 1   | TxIn_1 | --- |         |     | spend or fee @ Shard X |
| 2   | TxIn_2 | --> | TxOut_1 | 1   | spend or fee @ Shard Y |
| 3   | TxIn_3 | --- |         |     | spend or fee @ Shard Y |


### MultiSig Lock with Refund

- `MultiSigLockTy` (multisig_lock) - new type of standard script.

- `OP_INPUTAGE` - new op_code, it pushes the value of `TxIn.Age` onto stack.

- `TxIn.Age int32` - is a new field, it is not stored in blockchain, it would not be serialized and/or hashed,
  Blockchain runtime sets the value of this field during validation.


MultiSigLockScript is a multi-signature lock script is of the form:

```text
OP_INPUTAGE <required_age_for_refund>  OP_LESSTHAN
OP_IF
   <numsigs> <pubkey> <pubkey> <pubkey>... <numpubkeys>
   OP_CHECKMULTISIG
OP_ELSE
    <refund_pubkey> OP_CHECKSIG OP_NIP
OP_ENDIF
```

Can be spent in two ways:

1. as a multi-signature script, if `tx.TxIn[i].Age` less than `<required_age_for_refund>`.
2. as pay-to-pubkey ("refund to owner") script, if `tx.TxIn[i].Age` greater than `<required_age_for_refund>`.

"Refund to owner" can be spend ONLY to address derived from the refund_pubkey: pay-to-pubkey or pay-to-pubkey-hash

In case of MultiSig activation the number of required signatures is the 4th item on the stack and the number of public
keys is the 5th to last item on the stack.

Otherwise, in case of "refund to owner" (pay-to-pubkey), requires one signature and pubkey is the second to last item on
the stack.


For both strategies' signature script starting from OP_FALSE. For that reason we need to have OP_NIP. It removes the last
value from stack (OP_FALSE) in case of refund.

#### Validation reference:

***TODO (mike): update test data!!***

```yaml
refund_key: 82610ee6f34f1d640a532335d362397fdeca997a84075f964774fa3de3779ccc
refund_pub_key: 0280dae123b8b57cbfb284011b6cc59fd25a9cb056c978c49345b4784724218c23

key_1: c6dc1d58c411053b6666107b714028e96765cd4245501ae4c3c9120373301924
pub_key_1: 03bbfb547f9b8ead386b8d54a1f1c154511b869dc8035df625a166e8dae314c55d

key_2: 1092a56f181ddcc8f3c835ba507d12f7b343a33285ec3461fcbfec7cac355ec3
pub_key_2: 03a914865a9652185758ec32d110182c225adcc5556bd390b31efd14cb65880156

lock_script_asm: OP_INPUTAGE 10 OP_LESSTHAN OP_IF 2 03bbfb547f9b8ead386b8d54a1f1c154511b869dc8035df625a166e8dae314c55d 03a914865a9652185758ec32d110182c225adcc5556bd390b31efd14cb65880156 2 OP_CHECKMULTISIG OP_ELSE 0280dae123b8b57cbfb284011b6cc59fd25a9cb056c978c49345b4784724218c23 OP_CHECKSIG OP_NIP OP_ENDIF
lock_script_hex: bc5a9f63522103bbfb547f9b8ead386b8d54a1f1c154511b869dc8035df625a166e8dae314c55d2103a914865a9652185758ec32d110182c225adcc5556bd390b31efd14cb6588015652ae67210280dae123b8b57cbfb284011b6cc59fd25a9cb056c978c49345b4784724218c23ac7768
lock_script_address: 2NBhie5eRwZtUNctUC2rRNkPeCLxnKFfKpu

pay_to_lock_script_asm: OP_HASH160 ca74ff0f31249b4c482f52416c468b928cd80221 OP_EQUAL
pay_to_lock_script_hex: a914ca74ff0f31249b4c482f52416c468b928cd8022187

partial_signature_asm: 0 30440220703339d977d73171a915a981a93ef70989a5832363424ed5336b4c6001fa3600022011987e9042d31190f62971d080a72950710a51e70f2bce9defb7220dc7996b6a01 bc5a9f63522103bbfb547f9b8ead386b8d54a1f1c154511b869dc8035df625a166e8dae314c55d2103a914865a9652185758ec32d110182c225adcc5556bd390b31efd14cb6588015652ae67210280dae123b8b57cbfb284011b6cc59fd25a9cb056c978c49345b4784724218c23ac7768
partial_signature_hex: 004730440220703339d977d73171a915a981a93ef70989a5832363424ed5336b4c6001fa3600022011987e9042d31190f62971d080a72950710a51e70f2bce9defb7220dc7996b6a014c71bc5a9f63522103bbfb547f9b8ead386b8d54a1f1c154511b869dc8035df625a166e8dae314c55d2103a914865a9652185758ec32d110182c225adcc5556bd390b31efd14cb6588015652ae67210280dae123b8b57cbfb284011b6cc59fd25a9cb056c978c49345b4784724218c23ac7768

full_signature_asm: 0 30440220703339d977d73171a915a981a93ef70989a5832363424ed5336b4c6001fa3600022011987e9042d31190f62971d080a72950710a51e70f2bce9defb7220dc7996b6a01 30440220778015f8ec683b0dc771f889cb2bacd2351bc014aa3f2166068b6560db2d39f50220630c72943bc5c58c59d885975f8d4b205aa29cc88ff9805d035d317e43b35cab01 bc5a9f63522103bbfb547f9b8ead386b8d54a1f1c154511b869dc8035df625a166e8dae314c55d2103a914865a9652185758ec32d110182c225adcc5556bd390b31efd14cb6588015652ae67210280dae123b8b57cbfb284011b6cc59fd25a9cb056c978c49345b4784724218c23ac7768
full_signature_hex: 004730440220703339d977d73171a915a981a93ef70989a5832363424ed5336b4c6001fa3600022011987e9042d31190f62971d080a72950710a51e70f2bce9defb7220dc7996b6a014730440220778015f8ec683b0dc771f889cb2bacd2351bc014aa3f2166068b6560db2d39f50220630c72943bc5c58c59d885975f8d4b205aa29cc88ff9805d035d317e43b35cab014c71bc5a9f63522103bbfb547f9b8ead386b8d54a1f1c154511b869dc8035df625a166e8dae314c55d2103a914865a9652185758ec32d110182c225adcc5556bd390b31efd14cb6588015652ae67210280dae123b8b57cbfb284011b6cc59fd25a9cb056c978c49345b4784724218c23ac7768

refund_signature_asm: 0 304502210084d9c9db59933da8d1fd47a685752bea9fb650cd50fa5700f3fa152d907697c7022015829b20a87d17d6e0f50fc361d71690673ab8b3bb71414b70adb7b20c2dcf4601 bc5a9f63522103bbfb547f9b8ead386b8d54a1f1c154511b869dc8035df625a166e8dae314c55d2103a914865a9652185758ec32d110182c225adcc5556bd390b31efd14cb6588015652ae67210280dae123b8b57cbfb284011b6cc59fd25a9cb056c978c49345b4784724218c23ac7768
refund_signature_hex: 0048304502210084d9c9db59933da8d1fd47a685752bea9fb650cd50fa5700f3fa152d907697c7022015829b20a87d17d6e0f50fc361d71690673ab8b3bb71414b70adb7b20c2dcf46014c71bc5a9f63522103bbfb547f9b8ead386b8d54a1f1c154511b869dc8035df625a166e8dae314c55d2103a914865a9652185758ec32d110182c225adcc5556bd390b31efd14cb6588015652ae67210280dae123b8b57cbfb284011b6cc59fd25a9cb056c978c49345b4784724218c23ac7768
```

### EAD Registration

Exchange Agents (EAs) can register their IPv4/IPv6 (and potentially in future DNS) addresses in the beacon chain, and
then would accept connections from clients in p2p manner on these interfaces and process the exchange operations in a
p2p manner.

| #   | []TxIn |     | []TxOut | #   |
|-----|--------|-----|---------|-----|
| 0   | TxIn_0 | --> | TxOut_0 | 0   |
| .   | ------ | --> | TxOut_1 | 1   |
| .   | ------ | --> | ....... | .   |
| .   | ------ | --> | TxOut_N | N   |

### Hybrid Time-Lock Contract
