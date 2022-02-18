# BTC Coinbase Tx

## Table of Content

1. [Requirements for Inputs](#Requirements-for-Inputs)
2. [Requirements for Outputs](#Requirements-for-Outputs)
    1. [TYPE_A](#TYPE_A)
    2. [TYPE_B](#TYPE_B)
    3. [TYPE_C](#TYPE_C)
    4. [Jaxnet Marker Out](#Jaxnet-Marker-Out)
3. [Jaxnet Burn Address](#Jaxnet-Burn-Address)
4. [Vanity Address](#Vanity-Address)
5. [Serialization format](#Serialization-format)
    1. [Transaction](#transaction)
    2. [signature script](#signature-script)
6. [Examples of Coinbase Transaction](#Examples-of-Coinbase-Transaction)
    1. [TYPE_A Example](#TYPE_A-Example)
    2. [TYPE_B Example](#TYPE_B-Example)
    3. [TYPE_C Example](#TYPE_C-Example)

> - JAX.Network has two kind of chain: **Beacon Chain** and **Shard Chain**.
> - **JAXNET** is name of the **Beacon Chain** coins.
> - **JAX** is name of the **Shard chains** coins.
> - **Dime**  is common name for fractional part of coins in both types of chains.
> - **HaberStornetta**  is name for the dimes in one Beacon Chain coin. 1 **JAXNET** == 10^8 **HaberStornetta**
> - **Juro**  is name for the dimes in one **Shard Chain** coin. 1 **JAX** == 10^4 **Juro**

This document defines requirements for Bitcoin Coinbase Transaction, that can be a valid **BTC Coinbase AUX** in Beacon.
Chain and Shard Chain blocks in the **JAX.Network**.

## Requirements for Inputs

The protocol requires to be present _special markers_ and **Proof of Inclusion** in the signature script of the first tx
input.

- **Signature script marker** is `6a61786e6574` ("jaxnet" in ASCII hex format).

- **Proof of Inclusion** is an Exclusive Hash of JAX.Net Beacon Block.

They can be at any position of the signature script, but they must follow each other in a
row. `<marker> <beacon_hash> <marker>`

According to rules of the [**_Bitcoin Script Serialization_**](#signature-script) in the raw format before each part
must be present its length.

```
<marker> <beacon_hash> <marker>

marker: 6a61786e6574
hash: 03e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414

// part of script
...066a61786e65742003e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414066a61786e6574...

06 // 0x06 = 6 -- 6 bytes
6a 61 78 6e 65 74 // marker
20 // 0x20 = 32 -- 32 bytes
03 e8 18 e6 64 51 5a 15 8c 4e b0 b9 97 11 ca e6 ce e3 30 72 4f cc 76 b6 4f 59 a1 78 9e ab f4 14 // hash
06 // 0x06 = 6 -- 6 bytes
6a 61 78 6e 65 74 // marker

```

Example:

```
marker: 6a61786e6574
hash: 03e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414

signature_script_hex: 03e1c720080000000000000000066a61786e65742003e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414066a61786e65740e2f503253482f6a61786e6574642f
signature_script_asm: e1c720 0000000000000000 6a61786e6574 03e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414 6a61786e6574 2f503253482f6a61786e6574642f
```

## Requirements for Outputs

There is 3 way to craft a Bitcoin Coinbase Transaction as valid **BTC Coinbase AUX** for the JAX.Network. Difference
between them only in the set of outputs.

| type   | output count    | allows to do *Proof Of Burn |
|--------|-----------------|-----------------------------|
| TYPE_A | 1, 2, 5, 6 or 7 | NO                          |
| TYPE_B | 3 or 4          | NO                          |
| TYPE_C | 3 or 4          | YES                         |

ONLY a transaction matching one of these can be used as a **BTC Coinbase AUX** to create a valid **BTC AUX** and mine
the
_JAX.Network block_. Valid blocks for Beacon Chain and Shard Chains can be made using these kinds.

If [TYPE_A](#type_a) and [TYPE_B](#type_b) used as **BTC Coinbase AUX**, only BTC and JAXNET block reward will be
issued, and JAX reward will be burned.

If [TYPE_C](#type_c) used as **BTC Coinbase AUX**, BTC and JAXNET block reward will be burned.

### TYPE_A

Bitcoin coinbase transaction WITHOUT [`jaxnet marker out`](#jaxnet-marker-out). This type of coinbase can't cause *Proof
of Burn* and issuance of the JAX coins (in shards). It can have _1, 2, 5, 6, or 7_ outputs.

There is only one requirement:

- address of the at least one out MUST have a vanity prefix [`1JAX`](#vanity-address).
  Example: `1JAXmGDsiE2CyK31dYZsMamM18pPebRDAk`

Example of such kind of transaction - [TYPE_A](#type_a-example)

### TYPE_B

This kind is a more strict format of Bitcoin coinbase transactions. This type of coinbase **can't** cause
*Proof of Burn* and issuance of the JAX coins (in shards). It can have only _3 or 4_ outputs.

There are requirements for outputs:

| Out N | Value                                             | Address                                  | Description                                          |
|-------|---------------------------------------------------|------------------------------------------|------------------------------------------------------|
| 0     | 0                                                 | [JAX BURN ADDRESS](#jaxnet-burn-address) | This must be [Jaxnet Marker Out](#jaxnet-marker-out) |
| 1     | block_reward <= 6.25 BTC                          | any valid btc address                    | This is output handles all block reward              |
| 2     | block_fee <= 0.5 BTC; if block_reward < 6.25 BTC  | any valid btc address                    | This is output handles fee of all transaction        |
| 3*    | 0                                                 | witness commitment                       | Optional output with witness commitment              |

> NOTE: if the value of the second output is LESS than **6.25 BTC** (current BTC Block reward),
> protocol will check that block fee is LESS OR EQUAL than **0.5 BTC**

Example of such kind of transaction - [TYPE_B](#type_b-example)

### TYPE_C

This type of coinbase contains *Proof of Burn* and **CAUSE** issuance of the JAX coins (in shards). It can have only
_3 or 4_ outputs.

There are requirements for outputs:

| Out N | Value                                            | Address                                  | Description                                           |
|-------|--------------------------------------------------|------------------------------------------|-------------------------------------------------------|
| 0     | 0                                                | [JAX BURN ADDRESS](#jaxnet-burn-address) | This must be [Jaxnet Marker Out](#jaxnet-marker-out)  |
| 1     | block_reward <= 6.25 BTC                         | [JAX BURN ADDRESS](#jaxnet-burn-address) | This is output "burns" all block reward               |
| 2     | block_fee <= 0.5 BTC; if block_reward < 6.25 BTC | any valid btc address                    | This is output handles fee of all transaction         |
| 3*    | 0                                                | witness commitment                       | Optional output with witness commitment               |

> NOTE: if the value of the second output is LESS than **6.25 BTC** (current BTC Block reward),
> protocol will check that block fee is LESS OR EQUAL than **0.5 BTC**

Example of such kind of transaction - [TYPE_C](#type_c-example)

### Jaxnet Marker Out

`jaxnet marker out` is a strict special predefined output and matches with next requirements:

1. This is **first** _Tx Out_.
2. Value of this out equal `0`.
3. This out is to the [JAX BURN ADDRESS](#jaxnet-burn-address).
4. Address in mainnet is equal to `1JAXNETJAXNETJAXNETJAXNETJAXW3bkUN` (testnet
   address `mxgUfHYGyYoVEQe95oRfzSaZKHmEPHD7Kr`).
5. PKScript of this out equal to `76a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac`.

Here it is:

```
tx_out: [
  0: {
    value: 0
    pk_script_hex: 76a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac
    pk_script_asm: OP_DUP OP_HASH160 bc473af4c71c45d5aa3278adc99701ded3740a54 OP_EQUALVERIFY OP_CHECKSIG
    pk_script_address: [ 1JAXNETJAXNETJAXNETJAXNETJAXW3bkUN ]
    script_class: pubkeyhash
  },
  ...
]
```

## Jaxnet Burn Address

**jaxnet burn address** is the predefined unspendable address.

| type            | output count                                                                          |
|-----------------|---------------------------------------------------------------------------------------|
| mainnet address | `1JAXNETJAXNETJAXNETJAXNETJAXW3bkUN`                                                  |
| testnet address | `mxgUfHYGyYoVEQe95oRfzSaZKHmEPHD7Kr`                                                  |
| pk_script       | `76a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac`                                  |
| script asm      | OP_DUP OP_HASH160 bc473af4c71c45d5aa3278adc99701ded3740a54 OP_EQUALVERIFY OP_CHECKSIG |
| script class    | pay-to-pubkeyhash                                                                     |

## Vanity Address

**Vanity address** is a normal bitcoin PublicKeyHash address that starts with the "JAX" string. For
example: `1JAXmGDsiE2CyK31dYZsMamM18pPebRDAk`.

## Serialization format

### transaction

- Serialized transaction:

```
010000000001010000000000000000000000000000000000000000000000000000000000000000ffffffff4b03e1c720080000000000000000066a61786e65742003e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414066a61786e65740e2f503253482f6a61786e6574642fffffffff0400000000000000001976a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac7c814a00000000001976a914bc478fcf931d1a0074fc5f8014d6d9c69641db3a88ac38440100000000001976a914bc478fcf931d1a0074fc5f8014d6d9c69641db3a88ac0000000000000000266a24aa21a9edd960d12fcd5bae3184513d03828259cd6b3dd50bf7153bf56d7498560d22edcb0120000000000000000000000000000000000000000000000000000000000000000000000000
```

- Explanation of serialization format:

| data                                                                         |                          |
|------------------------------------------------------------------------------|--------------------------|
| 01 00 00 00                                                                  | version                  |
| 00 01                                                                        | witness marker bytes     |
| 01                                                                           | number of inputs         |
| 0000000000000000000000000000000000000000000000000000000000000000             | prev_out_hash            |
| ff ff ff ff                                                                  | prev_out_id              |
| 4b                                                                           | len of signature script  |
| 03e1c720080000000000000000066a61786e657420<br />03e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf41406<br />6a61786e65740e2f503253482f6a61786e6574642f| signature script         |
| ff ff ff ff                                                                  | sequence                 |
| 04                                                                           | number of outputs        |
| 00 00 00 00 00 00 00 00                                                      | value == 0x00            |
| 19                                                                           | len of pk_script == 0x19 |
| 76a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac                           | pk script                |
| 7c 81 4a 00 00 00 00 00                                                      | value == 0x4a817c        |
| 19                                                                           | len of pk_script == 0x19 |
| 76a914bc478fcf931d1a0074fc5f8014d6d9c69641db3a88ac                           | pk script                |
| 38 44 01 00 00 00 00 00                                                      | value == 0x14438         |
| 19                                                                           | len of pk_script == 0x19 |
| 76a914bc478fcf931d1a0074fc5f8014d6d9c69641db3a88ac                           | pk script                |
| 00 00 00 00 00 00 00 00                                                      | value == 0x00            |
| 26                                                                           | len of pk_script == 0x26 |
| 6a24aa21a9edd960d12fcd5bae3184513d03828259cd6b3dd50bf7153bf56d7498560d22edcb | pk script                |
| 01                                                                           | number of witness hashes |
| 20                                                                           | len of witness hash      |
| 000000000000000000000000000000000000000000000000000000000000000000000000     | witness hash             |

### signature script

- Serialized signature script:

```
03e1c720080000000000000000066a61786e65742003e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414066a61786e65740e2f503253482f6a61786e6574642f

```

- Explanation of serialization format:

| data                                                             |                         |
|------------------------------------------------------------------|-------------------------|
| 03                                                               | size of varint          |
| e1 c7 20                                                         | varint; next height     |
| 08                                                               | size of extranonce      |
| 00 00 00 00 00 00 00 00                                          | extranonce              |
| 06                                                               | size of marker          |
| 6a61786e6574                                                     | "jaxnet" marker         |
| 20                                                               | size of exclusive hash  |
| 03e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414 | exclusive hash          |
| 06                                                               | size of marker          |
| 6a61786e6574                                                     | "jaxnet" marker         |
| 0e                                                               | size of some extra data |
| 2f503253482f6a61786e6574642f                                     | some extra data         |

## Examples of Coinbase Transaction

### TYPE_A Example

Decoded and pretty-printed transaction data:

```
version: 1
locktime: 0

tx_in: [
  0: {
    prev_out_hash: 00000000000000000000000000000000000000000000000c00000000000000000
    prev_out_id: ffffffff
    signature_script_hex: 03e1c720080000000000000000066a61786e65742003e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414066a61786e65740e2f503253482f6a61786e6574642f
    signature_script: e1c720 0000000000000000 6a61786e6574 03e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414 6a61786e6574 2f503253482f6a61786e6574642f
    sequence: ffffffff
  }
]

tx_out: [
  0: {
    value: 4bc5b4
    pk_script_hex: 76a914bc478fcf931d1a0074fc5f8014d6d9c69641db3a88ac
    pk_script: OP_DUP OP_HASH160 bc478fcf931d1a0074fc5f8014d6d9c69641db3a OP_EQUALVERIFY OP_CHECKSIG
    pk_script_address: [ 1JAXmGDsiE2CyK31dYZsMamM18pPebRDAk ]
    script_class: pubkeyhash
  }
  1: {
    value: 0
    pk_script_hex: 6a24aa21a9edd960d12fcd5bae3184513d03828259cd6b3dd50bf7153bf56d7498560d22edcb
    pk_script: OP_RETURN aa21a9edd960d12fcd5bae3184513d03828259cd6b3dd50bf7153bf56d7498560d22edcb
    pk_script_address: []
    script_class: nulldata
  }
]

tx_witness: [ 0000000000000000000000000000000000000000000000000000000000000000 ]
```

### TYPE_B Example

Decoded and pretty-printed transaction data:

```
version: 1
locktime: 0

tx_in: [
  0: {
    prev_out_hash: 00000000000000000000000000000000000000000000000c00000000000000000
    prev_out_id: ffffffff
    signature_script_hex: 03e1c720080000000000000000066a61786e65742003e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414066a61786e65740e2f503253482f6a61786e6574642f
    signature_script: e1c720 0000000000000000 6a61786e6574 03e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414 6a61786e6574 2f503253482f6a61786e6574642f
    sequence: ffffffff
  }
]

tx_out: [
  0: {
    value: 0
    pk_script_hex: 76a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac
    pk_script: OP_DUP OP_HASH160 bc473af4c71c45d5aa3278adc99701ded3740a54 OP_EQUALVERIFY OP_CHECKSIG
    pk_script_address: [1JAXNETJAXNETJAXNETJAXNETJAXW3bkUN ]
    script_class: pubkeyhash
  }
  1: {
    value: 4a817c
    pk_script_hex: 76a914bc478fcf931d1a0074fc5f8014d6d9c69641db3a88ac
    pk_script: OP_DUP OP_HASH160 bc478fcf931d1a0074fc5f8014d6d9c69641db3a OP_EQUALVERIFY OP_CHECKSIG
    pk_script_address: [1JAXmGDsiE2CyK31dYZsMamM18pPebRDAk ]
    script_class: pubkeyhash
  }
  2: {
    value: 14438
    pk_script_hex: 76a914bc478fcf931d1a0074fc5f8014d6d9c69641db3a88ac
    pk_script: OP_DUP OP_HASH160 bc478fcf931d1a0074fc5f8014d6d9c69641db3a OP_EQUALVERIFY OP_CHECKSIG
    pk_script_address: [1JAXmGDsiE2CyK31dYZsMamM18pPebRDAk ]
    script_class: pubkeyhash
  }
  3: {
    value: 0
    pk_script_hex: 6a24aa21a9edd960d12fcd5bae3184513d03828259cd6b3dd50bf7153bf56d7498560d22edcb
    pk_script: OP_RETURN aa21a9edd960d12fcd5bae3184513d03828259cd6b3dd50bf7153bf56d7498560d22edcb
    pk_script_address: []
    script_class: nulldata
  }
]

tx_witness: [ 0000000000000000000000000000000000000000000000000000000000000000 ]
```

### TYPE_C Example

Decoded and pretty-printed transaction data:

```
version: 1
locktime: 0

tx_in: [
  0: {
    prev_out_hash: 00000000000000000000000000000000000000000000000c00000000000000000
    prev_out_id: ffffffff
    signature_script_hex: 03e1c720080000000000000000066a61786e65742003e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414066a61786e65740e2f503253482f6a61786e6574642f
    signature_script: e1c720 0000000000000000 6a61786e6574 03e818e664515a158c4eb0b99711cae6cee330724fcc76b64f59a1789eabf414 6a61786e6574 2f503253482f6a61786e6574642f
    sequence: ffffffff
  }
]

tx_out: [
  0: {
    value: 0
    pk_script_hex: 76a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac
    pk_script: OP_DUP OP_HASH160 bc473af4c71c45d5aa3278adc99701ded3740a54 OP_EQUALVERIFY OP_CHECKSIG
    pk_script_address: [ 1JAXNETJAXNETJAXNETJAXNETJAXW3bkUN ]
    script_class: pubkeyhash
  }
  1: {
    value: 4a817c
    pk_script_hex: 76a914bc473af4c71c45d5aa3278adc99701ded3740a5488ac
    pk_script: OP_DUP OP_HASH160 bc473af4c71c45d5aa3278adc99701ded3740a54 OP_EQUALVERIFY OP_CHECKSIG
    pk_script_address: [ 1JAXNETJAXNETJAXNETJAXNETJAXW3bkUN ]
    script_class: pubkeyhash
  }
  2: {
    value: 14438
    pk_script_hex: 76a914bc478fcf931d1a0074fc5f8014d6d9c69641db3a88ac
    pk_script: OP_DUP OP_HASH160 bc478fcf931d1a0074fc5f8014d6d9c69641db3a OP_EQUALVERIFY OP_CHECKSIG
    pk_script_address: [ 1JAXmGDsiE2CyK31dYZsMamM18pPebRDAk ]
    script_class: pubkeyhash
  }
  3: {
    value: 0
    pk_script_hex: 6a24aa21a9edd960d12fcd5bae3184513d03828259cd6b3dd50bf7153bf56d7498560d22edcb
    pk_script: OP_RETURN aa21a9edd960d12fcd5bae3184513d03828259cd6b3dd50bf7153bf56d7498560d22edcb
    pk_script_address: []
    script_class: nulldata
  }
]

tx_witness: [ 0000000000000000000000000000000000000000000000000000000000000000 ]
```
