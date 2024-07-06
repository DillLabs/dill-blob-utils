#!/bin/bash

if [ $# -lt 1 ];then
    echo "Usage: $0 [priKeyFile]"
    exit
fi

priKeyFile=$1
priKey=$(cat $priKeyFile)

#nohup ./dill-blob-utils batchTx --rpc-url http://localhost:8545 \
nohup ./dill-blob-utils batchTx --rpc-url http://localhost:8560 \
--to 0x0fC1ba8D945d926003f18C1881F97d1E4043D9bB \
--private-key $priKey \
--gas-limit 2100000 --chain-id 558329 \
--priority-gas-price 1000000000 --max-fee-per-blob-gas 30000000 --blob-size 262144 &>> test.log &
