#!/bin/bash

if [ $# -lt 1 ];then
    echo "Usage: $0 [priKeyFile]"
    exit
fi

priKeyFile=$1
priKey=$(cat $priKeyFile)

./dill-blob-utils transferTx --rpc-url http://localhost:8560 \
--to 0x0fC1ba8D945d926003f18C1881F97d1E4043D9bB \
--value 5 \
--private-key $priKey \
--chain-id 558329
