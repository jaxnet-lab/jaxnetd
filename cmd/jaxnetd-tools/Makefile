GATLING_PKG=gitlab.com/jaxnet/jaxnetd/cmd/jaxnetd-tools

sync_miner:
	mkdir -p data || true
	go run $(GATLING_PKG) -f data/miner_utxo.csv sync -a $(MINER_ADDR)

sync_miner_for_shard:
	mkdir -p data || true
	go run $(GATLING_PKG) -f data/miner_utxo_1.csv --shard 1 sync -a $(MINER_ADDR)
	#go run $(GATLING_PKG) -f data/miner_utxo_2.csv --shard 2 sync -a $(MINER_ADDR)

sync_alice:
	go run $(GATLING_PKG) -f data/alice_utxo.csv sync -a $(ALICE_ADDR)

sync_bob:
	go run $(GATLING_PKG) -f data/bob_utxo.csv sync -a $(BOB_ADDR)

sync_eva:
	go run $(GATLING_PKG) -f data/eva_utxo.csv sync -a $(EVA_ADDR)

sync_all:
	go run $(GATLING_PKG) -f data/all_utxo.csv sync
	cat data/all_utxo.csv | grep -v pubkeyhash > data/filtered.csv

sync_all_for_shard:
	go run $(GATLING_PKG) --shard 2 -f data/all_utxo_2.csv sync
	cat data/all_utxo_2.csv | grep -v pubkeyhash > data/filtered.csv

sync: sync_all sync_miner sync_alice sync_bob sync_eva

send_tx:
	go run $(GATLING_PKG) -k $(MINER_SK) -f data/miner_utxo.csv send-tx

send_tx_for_shard:
	go run $(GATLING_PKG) -k $(MINER_SK) --shard 1 -f data/miner_utxo_1.csv send-tx

TX_HASH=8e8de99c0bf81f95b010e53f74bfd2c4d608227938f279954f062185be052cd6
spend_utxo:
	go run $(GATLING_PKG) -k $(MINER_SK)  spend-utxo -i 0 -x $(TX_HASH) -a $(ALICE_ADDR) -t



multisig_address:
	go run $(GATLING_PKG) multisig-address -f $(ALICE_PK) -s $(BOB_PK) -a 0


multisig_tx:
	go run $(GATLING_PKG) multisig-tx -f $(ALICE_PK) -s $(BOB_PK) -a 0


add_signature:
	go run $(GATLING_PKG) -k $(BOB_SK) add-signature -b $(NEW_TX) -t

help:
	go run $(GATLING_PKG) -h
