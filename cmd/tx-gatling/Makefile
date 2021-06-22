build:
	cd .. && go build -o ./tmp/jaxcore && go build -o ./tmp/jaxctl gitlab.com/jaxnet/jaxnetd/cmd/jaxctl


build_linux:
	cd .. && env GOOS=linux GOARCH=amd64 go build -o ./tmp/jaxcore && env GOOS=linux GOARCH=amd64 go build -o ./tmp/jaxctl gitlab.com/jaxnet/jaxnetd/cmd/btcctl

clean:
	rm -rf core_1/core/* && rm -rf core_2/core/* && rm -rf core_3/core/*

MINER_SK=3c83b4d5645075c9afac0626e8844007c70225f6625efaeac5999529eb8d791b
MINER_PK=0489f306c39ba355799d2f525e50372ca257b9a44e95a91cf9fc8677e1dc50dcfee1aa2e306310f415dc73e20aa0db04cfdd450d479db7937ea2fb9ca72651ad21
MINER_ADDR=mxQsksaTJb11i7vSxAUL6VBjoQnhP3bfFz


ALICE_SK=6443fb332e1cbfe456674aacf2be1327b6f9fc9c782061ee04ca35e17608d651
ALICE_PK=04e7a2fb1f4c1ff3b6da4f756c14e50c375dffb1d9bda1d90ae96e25847341d3ee36ba3fa698be0c07f67622febe229ea8e40e92d1b0b180672be47cd8d45b4299
ALICE_ADDR=myGdvt6vRNgrFZFtU5FNhW5gxwRqKBcLGv

BOB_SK=6bb4b4a9d5512c84f14bd38248dafb80c2424ae50a0495be8e4f657d734f1bd4
BOB_PK=04486596c443d5b00f198474f47a5ce5498968e546114687b9d067e94d2d7d074b3654af29677cec36ce468c84d9612609786b955d13c2265bc28eda1a2d093e61
BOB_ADDR=mz6Z8dFaxEAMmnR5ha4CVnKykeY6S3dUwi

EVA_SK=bdfb934f403bd6c3f74730f9690f6fc22863388f473860eb001a1e7f02261b79
EVA_PK=04229dea33b9b89b8b6d746bcf2df82c142f254b3acc6c5452525ef0774bf2d0a19ca45e6ace7a427756b4422dac084ca9b406835b33140641a5f6608c41434b8b
EVA_ADDR=mwnAejT1i6Fra7npajqEe6G3A22DFbU5aK

GATLING_PKG=gitlab.com/jaxnet/jaxnetd/cmd/tx-gatling

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
	go run $(GATLING_PKG) --shard 1 -f data/all_utxo_1.csv sync
	cat data/all_utxo_1.csv | grep -v pubkeyhash > data/filtered.csv

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


NEW_TX=0100000001dc8272047b12ee7fb032586956bd88dac73cc81745db1ebb1550d96dd7aea50c000000000100ffffffff010068c461080000001976a91464d5508377cd9f79ed7f99f0120c10b88eea2c8188ac00000000
add_signature:
	go run $(GATLING_PKG) -k $(BOB_SK) add-signature -b $(NEW_TX) -t

help:
	go run $(GATLING_PKG) -h
