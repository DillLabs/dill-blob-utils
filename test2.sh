#!/bin/bash

nohup ./blob-utils batchTransferTx --rpc-url http://localhost:8545 \
--to 0x123463a4B065722E99115D6c222f267d9cABb524 \
--private-key 7d706043227ee1f26095e724d24c47ed8e9d04852cae42e4e7313c4d6fa64fa8 \
--chain-id 32382 --value 1 \
--delta-sleep-time 10 &>> test2.log &
