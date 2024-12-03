package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	ethereum "github.com/DillLabs/dill-execution"
	"github.com/DillLabs/dill-execution/common"
	"github.com/DillLabs/dill-execution/core/types"
	"github.com/DillLabs/dill-execution/crypto"
	"github.com/DillLabs/dill-execution/ethclient"
	"github.com/holiman/uint256"
	"github.com/urfave/cli"
)

func BatchTxApp(cliCtx *cli.Context) {
	addrs := cliCtx.StringSlice(TxRPCURLSFlag.Name)
	to := common.HexToAddress(cliCtx.String(TxToFlag.Name))
	prv := cliCtx.String(TxPrivateKeyFlag.Name)
	blobSize := cliCtx.Uint64(TxBlobSizeFlag.Name)
	count := cliCtx.Uint64(TxConcurrenceFlag.Name)
	waitTime := time.Duration(cliCtx.Uint64(TxWaitingFlag.Name)) * time.Second
	value := cliCtx.String(TxValueFlag.Name)
	gasLimit := cliCtx.Uint64(TxGasLimitFlag.Name)
	gasPrice := cliCtx.String(TxGasPriceFlag.Name)
	priorityGasPrice := cliCtx.String(TxPriorityGasPrice.Name)
	maxFeePerBlobGas := cliCtx.String(TxMaxFeePerBlobGas.Name)
	chainID := cliCtx.String(TxChainID.Name)
	calldata := cliCtx.String(TxCalldata.Name)
	successSleepTime := cliCtx.Uint64(TxSleepSuccessFlag.Name)
	value256, err := uint256.FromHex(value)
	if err != nil {
		log.Fatalf("invalid value param: %v", err)
		return
	}
	calldataBytes, err := common.ParseHexOrString(calldata)
	if err != nil {
		log.Fatalf("failed to parse calldata: %v", err)
	}
	blobSize = blobSize - blobSize%32
	blobPerTx := cliCtx.Uint64(TxBlobCountFlag.Name)
	data := RandomFrData(int(blobSize * blobPerTx))
	blobs, commitments, proofs, _, versionedHashes, err := EncodeBlobs(data, true)
	if err != nil {
		log.Fatalf("failed to compute commitments: %v", err)
	}
	chainId, _ := new(big.Int).SetString(chainID, 0)
	ctx := context.Background()
	addr := addrs[0]
	client, err := ethclient.DialContext(ctx, addr)
	if err != nil {
		log.Panicf("Failed to connect to the Ethereum client: %v", err)
	}
	masterKey, err := crypto.HexToECDSA(prv)
	if err != nil {
		log.Panicf("%v: invalid private key", err)
	}
	keys := generatePrivateKeys(int(count))

	pendingNonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(masterKey.PublicKey))
	if err != nil {
		log.Panicf("Error getting nonce: %v", err)
	}
	masterNonce := int64(pendingNonce)

	var gasPrice256 *uint256.Int
	if gasPrice == "" {
		val, err := client.SuggestGasPrice(ctx)
		if err != nil {
			log.Panicf("Error getting suggested gas price: %v", err)
		}
		var nok bool
		gasPrice256, nok = uint256.FromBig(val)
		if nok {
			log.Panicf("gas price is too high! got %v", val.String())
		}
	} else {
		gasPrice256, err = DecodeUint256String(gasPrice)
		if err != nil {
			log.Panicf("%v: invalid gas price", err)
		}
	}
	transferTxs := map[int]*types.Transaction{}
	for i, subKey := range keys {
		pub := subKey.PublicKey
		addr := crypto.PubkeyToAddress(pub)
		tx, err := transferToken(client, addr.Hex(), 50, uint64(masterNonce), chainId.Int64(), gasPrice256.ToBig().Int64(), int64(gasLimit), masterKey)
		if err != nil {
			log.Panic(err)
		}
		masterNonce++
		transferTxs[i] = tx
	}
	for {
		for i, tx := range transferTxs {
			_, err = client.TransactionReceipt(context.Background(), tx.Hash())
			if err == nil {
				delete(transferTxs, i)
				continue
			}
			if err != ethereum.NotFound {
				log.Printf("transfer failed: %+v", err)
			}
		}
		if len(transferTxs) == 0 {
			break
		}
		time.Sleep(time.Second)
	}
	log.Printf("transfer tx done")
	for idx := range keys {
		go func(i int) {
			addr := addrs[i%len(addrs)]
			client, err := ethclient.DialContext(ctx, addr)
			if err != nil {
				log.Panicf("Failed to connect to the Ethereum client: %v", err)
			}
			key := keys[i]
			var gasPrice256 *uint256.Int
			if gasPrice == "" {
				val, err := client.SuggestGasPrice(ctx)
				if err != nil {
					log.Fatalf("Error getting suggested gas price: %v", err)
				}
				val = val.Mul(val, big.NewInt(2))
				var nok bool
				gasPrice256, nok = uint256.FromBig(val)
				if nok {
					log.Fatalf("gas price is too high! got %v", val.String())
				}
			} else {
				gasPrice256, err = DecodeUint256String(gasPrice)
				if err != nil {
					log.Fatalf("%v: invalid gas price", err)
				}
			}
			priorityGasPrice256 := gasPrice256
			if priorityGasPrice != "" {
				priorityGasPrice256, err = DecodeUint256String(priorityGasPrice)
				if err != nil {
					log.Fatalf("%v: invalid priority gas price", err)

				}
			}

			maxFeePerBlobGas256, err := DecodeUint256String(maxFeePerBlobGas)
			if err != nil {
				log.Fatalf("%v: invalid max_fee_per_blob_gas", err)
			}
			log.Printf("all preparation done for client %d, start loop sending transactions", i)
			for {
				pendingNonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
				if err != nil {
					log.Panicf("Error getting nonce: %v", err)
				}
				subNonuce := pendingNonce

				tx := types.NewTx(&types.BlobTx{
					ChainID:    uint256.MustFromBig(chainId),
					Nonce:      subNonuce,
					GasTipCap:  priorityGasPrice256,
					GasFeeCap:  gasPrice256,
					Gas:        gasLimit,
					To:         to,
					Value:      value256,
					Data:       calldataBytes,
					BlobFeeCap: maxFeePerBlobGas256,
					BlobHashes: versionedHashes,
					Sidecar: &types.BlobTxSidecar{
						Commitments: commitments,
						Proofs:      proofs,
						Blobs:       blobs,
					},
				})
				signedTx, err := types.SignTx(tx, types.NewCancunSigner(chainId), key)
				if err != nil {
					log.Fatalf("%v: sign tx failed", err)
				}
				log.Printf("Commitments: %v\n", fmt.Sprintf("%x", signedTx.BlobTxSidecar().Commitments))
				log.Printf("Extra proof count: %v\n", fmt.Sprintf("%d", len(signedTx.BlobTxSidecar().ExtraProofs)))
				log.Printf("GasTipCap: %v, BlobGasFeeCap: %v, GasFeeCap: %v\n",
					signedTx.GasTipCap(), signedTx.BlobGasFeeCap(), signedTx.GasFeeCap())
				err = client.SendTransaction(context.Background(), signedTx)
				if err != nil {
					log.Printf("failed to send transaction: %v", err)
					time.Sleep(waitTime)
				} else {
					log.Printf("successfully sent transaction. txhash=%v", signedTx.Hash())
					if successSleepTime > 0 {
						time.Sleep(time.Duration(successSleepTime) * time.Millisecond)
					}
				}
			}
		}(idx)
		time.Sleep(1 * time.Second)
	}

	pendingCh := make(chan struct{})
	<-pendingCh
}
