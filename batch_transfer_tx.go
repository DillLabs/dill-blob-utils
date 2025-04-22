package main

import (
	"context"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/DillLabs/dill-execution/accounts/abi/bind"
	"github.com/DillLabs/dill-execution/common"
	"github.com/DillLabs/dill-execution/crypto"
	"github.com/DillLabs/dill-execution/ethclient"
	"github.com/urfave/cli"
)

func BatchTransferTxApp(cliCtx *cli.Context) {
	addr := cliCtx.String(TxRPCURLFlag.Name)
	to := common.HexToAddress(cliCtx.String(TxToFlag.Name))
	value := cliCtx.Int64(TxValueFlag.Name)
	prv := cliCtx.String(TxPrivateKeyFlag.Name)
	nonce := cliCtx.Int64(TxNonceFlag.Name)
	chainID := cliCtx.Uint64(TxChainID.Name)
	deltaNonce := cliCtx.Int64(TxDeltaNonceFlag.Name)
	deltaSleep := cliCtx.Int64(TxDeltaSleepTimeFlag.Name)

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

	for {
		balance, err := client.BalanceAt(ctx, auth.From, nil)
		chkErr(err)
		log.Printf("ETH Balance for %v: %v", auth.From, balance)

		transferAmount := big.NewInt(value)
		log.Printf("Transfer Amount: %v", transferAmount)
		key, err := crypto.HexToECDSA(prv)
		if err != nil {
			log.Printf("%s: invalid private key", err.Error())
			return
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
		log.Printf("tx sent: %s", signedTx.Hash().String())

		nonce = int64(pendingNonce) + 1
		if nonce%int64(deltaNonce) == 0 {
			log.Printf("nonce %d, deltaNonce: %d, sleep %d seconds", nonce, deltaNonce, deltaSleep)
			time.Sleep(time.Duration(deltaSleep) * time.Second)
		}
	}
}
