package main

import (
	"github.com/urfave/cli"
)

var (
	TxRPCURLFlag = cli.StringFlag{
		Name:  "rpc-url",
		Usage: "Address of execution node JSON-RPC endpoint",
		Value: "http://127.0.0.1:8545",
	}
	TxBlobFileFlag = cli.StringFlag{
		Name:  "blob-file",
		Usage: "Blob file data",
	}
	TxBlobSizeFlag = cli.Uint64Flag{
		Name:  "blob-size",
		Usage: "Blob file size",
		Value: 131072,
	}
	TxToFlag = cli.StringFlag{
		Name:     "to",
		Usage:    "tx to address",
		Required: true,
	}
	TxValueFlag = cli.StringFlag{
		Name:  "value",
		Usage: "tx value (wei deonominated)",
		Value: "0x0",
	}
	TxPrivateKeyFlag = cli.StringFlag{
		Name:     "private-key",
		Usage:    "tx private key",
		Required: true,
	}
	TxNonceFlag = cli.Int64Flag{
		Name:  "nonce",
		Usage: "tx nonce",
		Value: -1,
	}
	TxGasLimitFlag = cli.Uint64Flag{
		Name:  "gas-limit",
		Usage: "tx gas limit",
		Value: 210000,
	}
	TxGasPriceFlag = cli.StringFlag{
		Name:  "gas-price",
		Usage: "sets the tx max_fee_per_gas",
	}
	TxPriorityGasPrice = cli.StringFlag{
		Name:  "priority-gas-price",
		Usage: "Sets the priority fee per gas",
		Value: "2000000000",
	}
	TxMaxFeePerBlobGas = cli.StringFlag{
		Name:  "max-fee-per-blob-gas",
		Usage: "Sets the max_fee_per_blob_gas",
		Value: "30000000000",
	}
	TxChainID = cli.StringFlag{
		Name:  "chain-id",
		Usage: "chain-id of the transaction",
		Value: "1332",
	}
	TxCalldata = cli.StringFlag{
		Name:  "calldata",
		Usage: "calldata of the transaction",
		Value: "0x",
	}
	TxDeltaNonceFlag = cli.Int64Flag{
		Name:  "delta-nonce",
		Usage: "tx delta nonce",
		Value: 10,
	}
	TxDeltaSleepTimeFlag = cli.Uint64Flag{
		Name:  "delta-sleep-time",
		Usage: "delta sleep time",
		Value: 60,
	}

	DownloadBeaconP2PAddr = cli.StringFlag{
		Name:  "beacon-p2p-addr",
		Usage: "P2P multiaddr of the beacon node",
		Value: "/ip4/127.0.0.1/tcp/13000",
	}
	DownloadSlotFlag = cli.Int64Flag{
		Name:     "slot",
		Usage:    "Slot to download blob from",
		Required: true,
	}

	ProofBlobFileFlag = cli.StringFlag{
		Name:     "blob-file",
		Usage:    "Blob file data",
		Required: true,
	}
	ProofBlobIndexFlag = cli.StringFlag{
		Name:     "blob-index",
		Usage:    "Blob index",
		Required: true,
	}
	ProofInputPointFlag = cli.StringFlag{
		Name:     "input-point",
		Usage:    "Input point of the proof",
		Required: true,
	}
	TxRPCURLSFlag = cli.StringSliceFlag{
		Name:  "rpc-urls",
		Usage: "Addresses of execution node JSON-RPC endpoint",
		Value: &cli.StringSlice{"http://127.0.0.1:8545"},
	}
	TxConcurrenceFlag = cli.Uint64Flag{
		Name:  "tx-concurrence",
		Usage: "accounts sending tx in parallel",
		Value: 4,
	}
	TxSleepSuccessFlag = cli.Uint64Flag{
		Name:  "tx-sleep-success",
		Usage: "sleep millisecond if tx sending success",
		Value: 0,
	}
	TxWaitingFlag = cli.Uint64Flag{
		Name:  "tx-waiting",
		Usage: "waiting time when tx pool overloaded",
		Value: 15,
	}
	TxBlobCountFlag = cli.Uint64Flag{
		Name:  "tx-blob-count",
		Usage: "blob counts in a single tx",
		Value: 2,
	}
	TxBlobWaitInclusionFlag = cli.BoolTFlag{
		Name:  "tx-wait-inclusion",
		Usage: "if wait for tx inclusion",
	}
)

var TxFlags = []cli.Flag{
	TxRPCURLFlag,
	TxBlobFileFlag,
	TxToFlag,
	TxValueFlag,
	TxPrivateKeyFlag,
	TxNonceFlag,
	TxGasLimitFlag,
	TxGasPriceFlag,
	TxPriorityGasPrice,
	TxMaxFeePerBlobGas,
	TxChainID,
	TxCalldata,
	TxBlobCountFlag,
}

var StressBlobTxFlags = []cli.Flag{
	TxRPCURLSFlag,
	TxBlobSizeFlag,
	TxWaitingFlag,
	TxBlobCountFlag,
	TxConcurrenceFlag,
	TxToFlag,
	TxValueFlag,
	TxPrivateKeyFlag,
	TxNonceFlag,
	TxGasLimitFlag,
	TxGasPriceFlag,
	TxPriorityGasPrice,
	TxMaxFeePerBlobGas,
	TxChainID,
	TxCalldata,
	TxSleepSuccessFlag,
}

var TransferTxFlags = []cli.Flag{
	TxRPCURLFlag,
	TxToFlag,
	TxValueFlag,
	TxPrivateKeyFlag,
	TxNonceFlag,
	TxChainID,
}

var BatchTransferTxFlags = []cli.Flag{
	TxRPCURLFlag,
	TxToFlag,
	TxValueFlag,
	TxPrivateKeyFlag,
	TxNonceFlag,
	TxChainID,
	TxDeltaNonceFlag,
	TxDeltaSleepTimeFlag,
}

var DownloadFlags = []cli.Flag{
	DownloadBeaconP2PAddr,
	DownloadSlotFlag,
}

var ProofFlags = []cli.Flag{
	ProofBlobFileFlag,
	ProofBlobIndexFlag,
	ProofInputPointFlag,
}
