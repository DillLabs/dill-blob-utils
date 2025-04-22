package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	ethereum "github.com/DillLabs/dill-execution"
	"github.com/DillLabs/dill-execution/common"
	"github.com/DillLabs/dill-execution/core/types"
	"github.com/DillLabs/dill-execution/crypto"
	"github.com/DillLabs/dill-execution/ethclient"
	"github.com/holiman/uint256"
	"github.com/urfave/cli"
)

func BlobTxApp(cliCtx *cli.Context) error {
	addr := cliCtx.String(TxRPCURLFlag.Name)
	to := common.HexToAddress(cliCtx.String(TxToFlag.Name))
	prv := cliCtx.String(TxPrivateKeyFlag.Name)
	file := cliCtx.String(TxBlobFileFlag.Name)
	nonce := cliCtx.Int64(TxNonceFlag.Name)
	value := cliCtx.String(TxValueFlag.Name)
	gasLimit := cliCtx.Uint64(TxGasLimitFlag.Name)
	gasPrice := cliCtx.String(TxGasPriceFlag.Name)
	priorityGasPrice := cliCtx.String(TxPriorityGasPrice.Name)
	maxFeePerBlobGas := cliCtx.String(TxMaxFeePerBlobGas.Name)
	chainID := cliCtx.String(TxChainID.Name)
	calldata := cliCtx.String(TxCalldata.Name)
	blobCnt := cliCtx.Uint64(TxBlobCountFlag.Name)

	value256, err := uint256.FromHex(value)
	if err != nil {
		return fmt.Errorf("invalid value param: %v", err)
	}
	var data []byte
	if file == "" {
		data = RandomFrData(4096 * 32 * int(blobCnt))
	} else {
		data, err = os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("error reading blob file: %v", err)
		}
	}
	log.Printf("file size: %d\n", len(data))

	chainId, _ := new(big.Int).SetString(chainID, 0)

	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, addr)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	key, err := crypto.HexToECDSA(prv)
	if err != nil {
		return fmt.Errorf("%w: invalid private key", err)
	}

	if nonce == -1 {
		pendingNonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
		if err != nil {
			log.Fatalf("Error getting nonce: %v", err)
		}
		nonce = int64(pendingNonce)
	}

	var gasPrice256 *uint256.Int
	if gasPrice == "" {
		val, err := client.SuggestGasPrice(ctx)
		if err != nil {
			log.Fatalf("Error getting suggested gas price: %v", err)
		}
		var nok bool
		gasPrice256, nok = uint256.FromBig(val)
		if nok {
			log.Fatalf("gas price is too high! got %v", val.String())
		}
	} else {
		gasPrice256, err = DecodeUint256String(gasPrice)
		if err != nil {
			return fmt.Errorf("%w: invalid gas price", err)
		}
	}

	priorityGasPrice256 := gasPrice256
	if priorityGasPrice != "" {
		priorityGasPrice256, err = DecodeUint256String(priorityGasPrice)
		if err != nil {
			return fmt.Errorf("%w: invalid priority gas price", err)
		}
	}

	maxFeePerBlobGas256, err := DecodeUint256String(maxFeePerBlobGas)
	if err != nil {
		return fmt.Errorf("%w: invalid max_fee_per_blob_gas", err)
	}

	blobs, commitments, proofs, _, versionedHashes, err := EncodeBlobs(data, file == "")
	if err != nil {
		log.Fatalf("failed to compute commitments: %v", err)
	}

	calldataBytes, err := common.ParseHexOrString(calldata)
	if err != nil {
		log.Fatalf("failed to parse calldata: %v", err)
	}

	tx := types.NewTx(&types.BlobTx{
		ChainID:    uint256.MustFromBig(chainId),
		Nonce:      uint64(nonce),
		GasTipCap:  priorityGasPrice256,
		GasFeeCap:  gasPrice256,
		Gas:        gasLimit,
		To:         to,
		Value:      value256,
		Data:       calldataBytes,
		BlobFeeCap: maxFeePerBlobGas256,
		BlobHashes: versionedHashes,
		Sidecar:    &types.BlobTxSidecar{Blobs: blobs, Commitments: commitments, Proofs: proofs},
	})
	signedTx, _ := types.SignTx(tx, types.NewCancunSigner(chainId), key)

	log.Printf("Commitments: %v\n", fmt.Sprintf("0x%x", signedTx.BlobTxSidecar().Commitments))

	log.Printf("GasTipCap: %v, BlobGasFeeCap: %v, GasFeeCap: %v\n",
		signedTx.GasTipCap(), signedTx.BlobGasFeeCap(), signedTx.GasFeeCap())

	err = client.SendTransaction(context.Background(), signedTx)

	if err != nil {
		log.Fatalf("failed to send transaction: %v", err)
	} else {
		log.Printf("successfully sent transaction. txhash=%v", signedTx.Hash())
	}

	//var receipt *types.Receipt
	for {
		_, err = client.TransactionReceipt(context.Background(), signedTx.Hash())
		if err == ethereum.NotFound {
			time.Sleep(1 * time.Second)
		} else if err != nil {
			if _, ok := err.(*json.UnmarshalTypeError); ok {
				// TODO: ignore other errors for now. Some clients are treating the blobGasUsed as big.Int rather than uint64
				break
			}
		} else {
			break
		}
	}

	log.Printf("Transaction included. nonce=%d hash=%v", nonce, tx.Hash())
	//log.Printf("Transaction included. nonce=%d hash=%v, block=%d", nonce, tx.Hash(), receipt.BlockNumber.Int64())
	return nil
}
