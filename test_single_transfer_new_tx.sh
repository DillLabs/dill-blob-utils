#!/bin/bash

if [ $# -lt 1 ];then
    echo "Usage: $0 [priKeyFile]"
    exit
fi

priKeyFile=$1
priKey=$(cat $priKeyFile)

echo "Testing transfer with custom fees..."

echo ""
# 使用十进制格式的 maxFeePerGas 和 maxPriorityFeePerGas
./dill-blob-utils transferNewTx --rpc-url http://localhost:8545 \
--to 0x0fC1ba8D945d926003f18C1881F97d1E4043D9bB \
--value 0x5 \
--private-key $priKey \
--chain-id 71717 \
--gas-price 20000000000 \
--priority-gas-price 2000000000