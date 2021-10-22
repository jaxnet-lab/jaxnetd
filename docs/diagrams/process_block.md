# Process Block

- [index & db] check is block exist:
    - in index
    - in cold database
    - in orphan index

- check block sanity (CheckBlockSanityWF) :
    - header sanity (checkBlockHeaderSanity) - proof of work & timestamp
    - serialized size & tx merkle tree
    - CheckTransactionSanity (coinbase, amount, script sizes, swapTx template)
    - check Block Sig Ops count

- compare target & time with last known checkpoint (if present)

- [index & db] look up for parent block

- maybe accept block
    - [index] load parent block
    - validate Jax Aux Rules
    - check block header context (difficulty, median_time, version, shards, k/voteK)
    - check deployments (CSV, Segwit for txs)
    - [db] save block to the database
    - [index] add block node to the index
    - try to connect block as new best state
        - compare with best state
        - check connect block

        - process and store all transactions
        - do chain reorganization


- process orphans


- [index & db] process orphans (children of this block) 
