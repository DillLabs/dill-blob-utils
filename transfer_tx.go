package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	ethereum "github.com/DillLabs/dill-execution"
	"github.com/DillLabs/dill-execution/accounts/abi/bind"
	"github.com/DillLabs/dill-execution/common"
	"github.com/DillLabs/dill-execution/crypto"
	"github.com/DillLabs/dill-execution/ethclient"
	"github.com/urfave/cli"
)

func TransferTxApp(cliCtx *cli.Context) error {
	addr := cliCtx.String(TxRPCURLFlag.Name)
	to := common.HexToAddress(cliCtx.String(TxToFlag.Name))
	receipient := common.HexToAddress(cliCtx.String(TxReceipientFlag.Name))
	value := cliCtx.Int64(TxValueFlag.Name)
	prv := cliCtx.String(TxPrivateKeyFlag.Name)
	nonce := cliCtx.Int64(TxNonceFlag.Name)
	chainID := cliCtx.Uint64(TxChainID.Name)

	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, addr)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(prv, "0x"))
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(0).SetUint64(chainID))
	chkErr(err)

	log.Printf("Before transfer")
	fromBalanceBefore, err := client.BalanceAt(ctx, auth.From, nil)
	chkErr(err)
	printBalanceMulUnit("from", auth.From, fromBalanceBefore)

	toBalanceBefore, err := client.BalanceAt(ctx, to, nil)
	chkErr(err)
	printBalanceMulUnit("to", to, toBalanceBefore)

	receipientBalanceBefore, err := client.BalanceAt(ctx, receipient, nil)
	chkErr(err)
	printBalanceMulUnit("receipient", receipient, receipientBalanceBefore)
	log.Printf("")

	transferAmount := big.NewInt(value)
	log.Printf("Transfer Amount: %v", transferAmount)
	key, err := crypto.HexToECDSA(prv)
	if err != nil {
		return fmt.Errorf("%w: invalid private key", err)
	}

	pendingNonce := uint64(0)
	if nonce == -1 {
		pendingNonce, err = client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
		if err != nil {
			log.Fatalf("Error getting nonce: %v", err)
		}
	} else {
		pendingNonce = uint64(nonce)
	}

	signedTx := ethTransfer(ctx, client, auth, to, transferAmount, &pendingNonce)
	fmt.Println("tx sent: ", signedTx.Hash().String())

	//var receipt *types.Receipt
	for {
		_, err = client.TransactionReceipt(context.Background(), signedTx.Hash())
		if err == ethereum.NotFound {
			time.Sleep(3 * time.Second)
		} else if err != nil {
			if _, ok := err.(*json.UnmarshalTypeError); ok {
				// TODO: ignore other errors for now. Some clients are treating the blobGasUsed as big.Int rather than uint64
				break
			}
		} else {
			break
		}
	}
	log.Printf("Transaction included. nonce=%d hash=%v", signedTx.Nonce(), signedTx.Hash())

	log.Printf("")
	log.Printf("After transfer")
	fromBalanceAfter, err := client.BalanceAt(ctx, auth.From, nil)
	chkErr(err)
	printBalanceMulUnit("from", auth.From, fromBalanceAfter)

	toBalanceAfter, err := client.BalanceAt(ctx, to, nil)
	chkErr(err)
	printBalanceMulUnit("to", to, toBalanceAfter)

	receipientBalanceAfter, err := client.BalanceAt(ctx, receipient, nil)
	chkErr(err)
	printBalanceMulUnit("receipient", receipient, receipientBalanceAfter)

	log.Printf("")

	printBalanceMulUnit("from delta", auth.From, new(big.Int).Sub(fromBalanceAfter, fromBalanceBefore))
	printBalanceMulUnit("to delta", to, new(big.Int).Sub(toBalanceAfter, toBalanceBefore))
	printBalanceMulUnit("receipient delta", receipient, new(big.Int).Sub(receipientBalanceAfter, receipientBalanceBefore))

	return nil
}

func TransferNewTxApp(cliCtx *cli.Context) error {
	addr := cliCtx.String(TxRPCURLFlag.Name)
	to := common.HexToAddress(cliCtx.String(TxToFlag.Name))
	receipient := common.HexToAddress(cliCtx.String(TxReceipientFlag.Name))
	value := cliCtx.Int64(TxValueFlag.Name)
	prv := cliCtx.String(TxPrivateKeyFlag.Name)
	nonce := cliCtx.Int64(TxNonceFlag.Name)
	chainID := cliCtx.Uint64(TxChainID.Name)

	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, addr)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(prv, "0x"))
	if err != nil {
		log.Fatalf("Failed to parse private key: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(0).SetUint64(chainID))
	chkErr(err)

	log.Printf("Before transfer")
	fromBalanceBefore, err := client.BalanceAt(ctx, auth.From, nil)
	chkErr(err)
	printBalanceMulUnit("from", auth.From, fromBalanceBefore)

	toBalanceBefore, err := client.BalanceAt(ctx, to, nil)
	chkErr(err)
	printBalanceMulUnit("to", to, toBalanceBefore)

	receipientBalanceBefore, err := client.BalanceAt(ctx, receipient, nil)
	chkErr(err)
	printBalanceMulUnit("receipient", receipient, receipientBalanceBefore)
	log.Printf("")

	transferAmount := big.NewInt(value)
	log.Printf("Transfer Amount: %v", transferAmount)
	key, err := crypto.HexToECDSA(prv)
	if err != nil {
		return fmt.Errorf("%w: invalid private key", err)
	}

	pendingNonce := uint64(0)
	if nonce == -1 {
		pendingNonce, err = client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
		if err != nil {
			log.Fatalf("Error getting nonce: %v", err)
		}
	} else {
		pendingNonce = uint64(nonce)
	}

	// 解析 maxFeePerGas 和 maxPriorityFeePerGas 参数
	var maxFeePerGas, maxPriorityFeePerGas *big.Int

	// 解析 maxFeePerGas (支持十六进制和十进制格式)
	if gasPriceStr := cliCtx.String(TxGasPriceFlag.Name); gasPriceStr != "" {
		gasPriceUint256, err := DecodeUint256String(gasPriceStr)
		if err != nil {
			log.Fatalf("Failed to parse maxFeePerGas '%s': %v", gasPriceStr, err)
		}
		maxFeePerGas = gasPriceUint256.ToBig()
		log.Printf("Using custom maxFeePerGas: %v wei (%v gwei, %v ETH) (0x%x)", maxFeePerGas, new(big.Int).Div(maxFeePerGas, big.NewInt(1e9)), new(big.Float).SetInt(maxFeePerGas).Quo(new(big.Float).SetInt(maxFeePerGas), big.NewFloat(1e18)), maxFeePerGas)
	}

	// 解析 maxPriorityFeePerGas (支持十六进制和十进制格式)
	if priorityGasPriceStr := cliCtx.String(TxPriorityGasPrice.Name); priorityGasPriceStr != "" {
		priorityGasPriceUint256, err := DecodeUint256String(priorityGasPriceStr)
		if err != nil {
			log.Fatalf("Failed to parse maxPriorityFeePerGas '%s': %v", priorityGasPriceStr, err)
		}
		maxPriorityFeePerGas = priorityGasPriceUint256.ToBig()
		log.Printf("Using custom maxPriorityFeePerGas: %v wei (%v gwei, %v ETH) (0x%x)", maxPriorityFeePerGas, new(big.Int).Div(maxPriorityFeePerGas, big.NewInt(1e9)), new(big.Float).SetInt(maxPriorityFeePerGas).Quo(new(big.Float).SetInt(maxPriorityFeePerGas), big.NewFloat(1e18)), maxPriorityFeePerGas)
	}

	// 使用自定义费用的函数
	signedTx := newTx(ctx, client, auth, to, transferAmount, &pendingNonce, maxFeePerGas, maxPriorityFeePerGas)
	fmt.Println("tx sent: ", signedTx.Hash().String())

	//var receipt *types.Receipt
	for {
		_, err = client.TransactionReceipt(context.Background(), signedTx.Hash())
		if err == ethereum.NotFound {
			time.Sleep(3 * time.Second)
		} else if err != nil {
			if _, ok := err.(*json.UnmarshalTypeError); ok {
				// TODO: ignore other errors for now. Some clients are treating the blobGasUsed as big.Int rather than uint64
				break
			}
		} else {
			break
		}
	}
	log.Printf("Transaction included. nonce=%d hash=%v", signedTx.Nonce(), signedTx.Hash())

	log.Printf("")
	log.Printf("After transfer")
	fromBalanceAfter, err := client.BalanceAt(ctx, auth.From, nil)
	chkErr(err)
	printBalanceMulUnit("from", auth.From, fromBalanceAfter)

	toBalanceAfter, err := client.BalanceAt(ctx, to, nil)
	chkErr(err)
	printBalanceMulUnit("to", to, toBalanceAfter)

	receipientBalanceAfter, err := client.BalanceAt(ctx, receipient, nil)
	chkErr(err)
	printBalanceMulUnit("receipient", receipient, receipientBalanceAfter)

	log.Printf("")

	printBalanceMulUnit("from delta", auth.From, new(big.Int).Sub(fromBalanceAfter, fromBalanceBefore))
	printBalanceMulUnit("to delta", to, new(big.Int).Sub(toBalanceAfter, toBalanceBefore))
	printBalanceMulUnit("receipient delta", receipient, new(big.Int).Sub(receipientBalanceAfter, receipientBalanceBefore))

	return nil
}

func printBalanceMulUnit(name string, addr common.Address, w *big.Int) {
	log.Printf("Balance for %s %v: %v wei (%v gwei, %v ETH)", name, addr, w, new(big.Int).Div(w, big.NewInt(1e9)), new(big.Float).SetInt(w).Quo(new(big.Float).SetInt(w), big.NewFloat(1e18)))
	return
}
