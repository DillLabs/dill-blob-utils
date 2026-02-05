package main

import (
	"context"
	"log"

	"github.com/DillLabs/dill-execution/common"
	"github.com/DillLabs/dill-execution/ethclient"
	"github.com/urfave/cli"
)

// BalanceApp queries and prints the balance of the given address.
func BalanceApp(cliCtx *cli.Context) error {
	addr := cliCtx.String(TxRPCURLFlag.Name)
	account := common.HexToAddress(cliCtx.String(BalanceAddressFlag.Name))

	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, addr)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	balance, err := client.BalanceAt(ctx, account, nil)
	if err != nil {
		log.Fatalf("Failed to get balance: %v", err)
	}

	printBalanceMulUnit("address", account, balance)
	return nil
}
