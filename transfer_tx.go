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

	balance, err := client.BalanceAt(ctx, auth.From, nil)
	chkErr(err)
	log.Printf("ETH Balance for %v: %v", auth.From, balance)

	// Valid ETH Transfer
	balance, err = client.BalanceAt(ctx, auth.From, nil)
	log.Printf("ETH Balance for %v: %v", auth.From, balance)
	chkErr(err)

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
	//fmt.Println("tx sent: ", signedTx.Hash().String())

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
	log.Printf("Transaction included. nonce=%d hash=%v", nonce, signedTx.Hash())

	return nil
}
