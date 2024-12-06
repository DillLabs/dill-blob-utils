package main

import (
	"context"
	"crypto/ecdsa"
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

func StressBlobTxApp(cliCtx *cli.Context) {
	addrs := cliCtx.StringSlice(TxRPCURLSFlag.Name)
	to := common.HexToAddress(cliCtx.String(TxToFlag.Name))
	prv := cliCtx.String(TxPrivateKeyFlag.Name)
	count := cliCtx.Uint64(TxConcurrenceFlag.Name)
	value := cliCtx.String(TxValueFlag.Name)
	gasLimit := cliCtx.Uint64(TxGasLimitFlag.Name)
	gasPrice := cliCtx.String(TxGasPriceFlag.Name)
	priorityGasPrice := cliCtx.String(TxPriorityGasPrice.Name)
	maxFeePerBlobGas := cliCtx.String(TxMaxFeePerBlobGas.Name)
	chainID := cliCtx.String(TxChainID.Name)
	calldata := cliCtx.String(TxCalldata.Name)
	blobPerTx := cliCtx.Uint64(TxBlobCountFlag.Name)

	value256, err := uint256.FromHex(value)
	if err != nil {
		log.Fatalf("invalid value param: %v", err)
		return
	}
	calldataBytes, err := common.ParseHexOrString(calldata)
	if err != nil {
		log.Fatalf("failed to parse calldata: %v", err)
	}

	chainId, _ := new(big.Int).SetString(chainID, 0)
	ctx := context.Background()
	addr := addrs[0]
	client, err := ethclient.DialContext(ctx, addr)
	if err != nil {
		log.Panicf("Failed to connect to the Ethereum client: %v", err)
	}
	var globalGasPrice256 *uint256.Int
	if gasPrice == "" {
		globalGasPrice256 = getSuggestedPrice(ctx, client)
	} else {
		globalGasPrice256, err = DecodeUint256String(gasPrice)
		if err != nil {
			log.Panicf("%v: invalid gas price", err)
		}
	}

	globalPriorityGasPrice256 := globalGasPrice256
	if priorityGasPrice != "" {
		globalPriorityGasPrice256, err = DecodeUint256String(priorityGasPrice)
		if err != nil {
			log.Fatalf("%v: invalid priority gas price", err)

		}
	}

	maxFeePerBlobGas256, err := DecodeUint256String(maxFeePerBlobGas)
	if err != nil {
		log.Fatalf("%v: invalid max_fee_per_blob_gas", err)
	}

	masterKey, err := crypto.HexToECDSA(prv)
	if err != nil {
		log.Panicf("%v: invalid private key", err)
	}
	keys := generatePrivateKeys(int(count))
	batchTransferToMultiAccounts(ctx, client, globalGasPrice256.ToBig(), gasLimit, masterKey, keys)
	log.Printf("transfer to multi accounts done: %+v", err)

	clients := make([]*ethclient.Client, 0, len(addrs))

	for i := range addrs {
		client, err := ethclient.DialContext(ctx, addrs[i])
		if err != nil {
			log.Panicf("Failed to connect to the Ethereum client with address: %s, err: %v", addrs[i], err)
		}
		clients = append(clients, client)
	}

	for idx := range keys {
		go func(i int) {
			client := clients[i%len(clients)]
			key := keys[i]
			var gasPrice256 *uint256.Int
			var priorityGasPrice256 *uint256.Int
			if gasPrice == "" {
				gasPrice256 = getSuggestedPrice(ctx, client)
			} else {
				gasPrice256 = globalGasPrice256
			}
			if priorityGasPrice != "" {
				priorityGasPrice256 = globalPriorityGasPrice256
			} else {
				priorityGasPrice256 = gasPrice256
			}

			log.Printf("all preparation done for client %d, start loop sending transactions", i)
			for {
				randBlobs := randomBlobs(int(blobPerTx))
				subNonuce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
				if err != nil {
					log.Panicf("Error getting nonce: %v", err)
				}
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
					BlobHashes: randBlobs.versionedHashes,
					Sidecar: &types.BlobTxSidecar{
						Commitments: randBlobs.comms,
						Proofs:      randBlobs.proofs,
						Blobs:       randBlobs.blobs,
					},
				})
				signedTx, err := types.SignTx(tx, types.NewCancunSigner(chainId), key)
				if err != nil {
					log.Fatalf("%v: sign tx failed", err)
				}
				err = sendTxAndWait(ctx, client, signedTx)
				if err != nil {
					log.Fatalf("%v: send tx and wait failed", err)
				}
			}
		}(idx)
		time.Sleep(1 * time.Second)
	}

	pendingCh := make(chan struct{})
	<-pendingCh
}

func batchTransferToMultiAccounts(ctx context.Context, client *ethclient.Client, gasPrice *big.Int, gasLimit uint64, key *ecdsa.PrivateKey, toKeys []*ecdsa.PrivateKey) {
	pendingNonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		log.Panicf("Error getting nonce: %v", err)
	}
	masterNonce := int64(pendingNonce)
	chainid, err := client.ChainID(ctx)
	if err != nil {
		log.Panicf("Error getting chain id: %v", err)
	}
	transferTxs := map[int]*types.Transaction{}
	for i, subKey := range toKeys {
		pub := subKey.PublicKey
		addr := crypto.PubkeyToAddress(pub)
		tx, err := transferToken(client, addr.Hex(), 50, uint64(masterNonce), chainid, gasPrice, gasLimit, key)
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
}

func getSuggestedPrice(ctx context.Context, client *ethclient.Client) *uint256.Int {
	val, err := client.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatalf("Error getting suggested gas price: %v", err)
	}
	var nok bool
	gasPrice256, nok := uint256.FromBig(val)
	if nok {
		log.Fatalf("gas price is too high! got %v", val.String())
	}
	return gasPrice256
}
