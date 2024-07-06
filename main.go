package main

import (
	"bytes"
	"context"
	"encoding/hex"
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
	gethkzg4844 "github.com/DillLabs/dill-execution/crypto/kzg4844"
	"github.com/DillLabs/dill-execution/ethclient"
	"github.com/holiman/uint256"

	"strings"

	das "github.com/DillLabs/dill-das"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
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

	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("App failed: %v", err)
	}
}

func TxApp(cliCtx *cli.Context) error {
	addr := cliCtx.StringSlice(TxRPCURLSFlag.Name)[0]
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

	value256, err := uint256.FromHex(value)
	if err != nil {
		return fmt.Errorf("invalid value param: %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("error reading blob file: %v", err)
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

	blobs, commitments, proofs, _, versionedHashes, err := EncodeBlobs(data)
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

// 生成随机字符串
// const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
//const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
//
//func randomString(n int) string {
//	b := make([]byte, n)
//	for i := range b {
//		b[i] = charset[rand.Intn(len(charset))]
//	}
//	return string(b)
//}

func RandomFrData(n int) []byte {
	data := make([]byte, n)
	ele := fr.Element{}
	for i := 0; i < n/32; i++ {
		ele.SetRandom()
		eleBytes := ele.Bytes()
		copy(data[i*32:], eleBytes[:])
	}
	return data
}

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
	blobs, commitments, _, extra, versionedHashes, err := EncodeBlobs(data, true)
	if err != nil {
		log.Fatalf("failed to compute commitments: %v", err)
	}
	if len(extra) != len(blobs)*128 {
		log.Fatal("extra number wrong")
	}
	handle := das.New()
	var segments [][]byte
	for _, b := range blobs {
		blobSegs, err := handle.BlobToSegmentNoProof(b[:])
		if err != nil {
			log.Fatal("calculat blob ec failed")
		}
		for _, seg := range blobSegs {
			segments = append(segments, seg.Marshal())
		}
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
			pendingNonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
			if err != nil {
				log.Panicf("Error getting nonce: %v", err)
			}
			subNonuce := pendingNonce
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
						ExtraProofs: extra,
						Segments:    segments,
					},
				})
				signedTx, _ := types.SignTx(tx, types.NewCancunSigner(chainId), key)
				log.Printf("Commitments: %v\n", fmt.Sprintf("%x", signedTx.BlobTxSidecar().Commitments))
				log.Printf("Extra proof count: %v\n", fmt.Sprintf("%d", len(signedTx.BlobTxSidecar().ExtraProofs)))
				log.Printf("GasTipCap: %v, BlobGasFeeCap: %v, GasFeeCap: %v\n",
					signedTx.GasTipCap(), signedTx.BlobGasFeeCap(), signedTx.GasFeeCap())
				err = client.SendTransaction(context.Background(), signedTx)
				if err != nil {
					log.Printf("failed to send transaction: %v", err)
					if strings.Contains(err.Error(), "nonce too high") {
						pendingNonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
						if err != nil {
							log.Panicf("Error getting nonce: %v", err)
						}
						subNonuce = pendingNonce
					} else {
						time.Sleep(waitTime)
					}
				} else {
					log.Printf("successfully sent transaction. txhash=%v", signedTx.Hash())
					subNonuce += 1
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

func ProofApp(cliCtx *cli.Context) error {
	file := cliCtx.String(ProofBlobFileFlag.Name)
	blobIndex := cliCtx.Uint64(ProofBlobIndexFlag.Name)
	inputPoint := cliCtx.String(ProofInputPointFlag.Name)

	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("error reading blob file: %v", err)
	}
	blobs, commitments, _, _, versionedHashes, err := EncodeBlobs(data)
	if err != nil {
		log.Fatalf("failed to compute commitments: %v", err)
	}

	if blobIndex >= uint64(len(blobs)) {
		return fmt.Errorf("error reading %d blob", blobIndex)
	}

	if len(inputPoint) != 64 {
		return fmt.Errorf("wrong input point, len is %d", len(inputPoint))
	}

	var x gethkzg4844.Point
	ip, _ := hex.DecodeString(inputPoint)
	copy(x[:], ip)
	proof, claimedValue, err := gethkzg4844.ComputeProof(gethkzg4844.Blob(blobs[blobIndex]), x)
	if err != nil {
		log.Fatalf("failed to compute proofs: %v", err)
	}

	pointEvalInput := bytes.Join(
		[][]byte{
			versionedHashes[blobIndex][:],
			x[:],
			claimedValue[:],
			commitments[blobIndex][:],
			proof[:],
		},
		[]byte{},
	)
	log.Printf(
		"\nversionedHash %x \n"+"x %x \n"+"y %x \n"+"commitment %x \n"+"proof %x \n"+"pointEvalInput %x",
		versionedHashes[blobIndex][:], x[:], claimedValue[:], commitments[blobIndex][:], proof[:], pointEvalInput[:])
	return nil
}
