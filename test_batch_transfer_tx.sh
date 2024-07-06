#!/bin/bash

if [ $# -lt 1 ];then
    echo "Usage: $0 [priKeyFile]"
    exit
fi

priKeyFile=$1
priKey=$(cat $priKeyFile)

nohup ./dill-blob-utils batchTransferTx --rpc-url http://localhost:8560 \
--to 0x123463a4B065722E99115D6c222f267d9cABb524 \
--private-key $priKey \
--value 1 \
--chain-id 558329 \
--delta-sleep-time 10 &>> test2.log &
