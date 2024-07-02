#!/bin/bash

nohup ./dill-blob-utils batchTx --rpc-url http://localhost:8545 \
--to 0xC8D369B164361A8961286CFbaB3bc10F962185a8 \
--private-key f213ad28eee53c2ae6262a704472c972aca45ebc3f9a628c877e0bb13e2255b3 \
--gas-limit 2100000 --chain-id 558329 \
--priority-gas-price 1000000000 --max-fee-per-blob-gas 30000000 --blob-size 262144 &>> test.log &
