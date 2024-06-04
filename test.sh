#!/bin/bash

nohup ./blob-utils batchTx --rpc-url http://localhost:8545 --to 0xC8D369B164361A8961286CFbaB3bc10F962185a8 \
--private-key 2e0834786285daccd064ca17f1654f67b4aef298acbb82cef9ec422fb4975622 \
--gas-limit 2100000 --chain-id 32382 \
--priority-gas-price 1000000000 --max-fee-per-blob-gas 30000000 --blob-size 32 &>> test.log & 
