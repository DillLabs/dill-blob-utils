package main

import (
	"log"
	"os"

	das "github.com/DillLabs/dill-das"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:   "tx",
			Usage:  "send a blob transaction",
			Action: TxApp,
			Flags:  TxFlags,
		},
		{
			Name:   "batchTx",
			Usage:  "send a batch of transactions",
			Action: BatchTxApp,
			Flags:  BatchTxFlags,
		},
		{
			Name:   "transferTx",
			Usage:  "send a transfer transaction",
			Action: TransferTxApp,
			Flags:  TransferTxFlags,
		},
		{
			Name:   "batchTransferTx",
			Usage:  "send a batch of transfer transaction",
			Action: BatchTransferTxApp,
			Flags:  BatchTransferTxFlags,
		},
		{
			Name:   "download",
			Usage:  "download blobs from the beacon net",
			Action: DownloadApp,
			Flags:  DownloadFlags,
		},
		{
			Name:   "proof",
			Usage:  "generate kzg proof for any input point by using jth blob polynomial",
			Action: ProofApp,
			Flags:  ProofFlags,
		},
	}
	das.InitKZGContext()

	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("App failed: %v", err)
	}
}
