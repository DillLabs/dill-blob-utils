package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/DillLabs/dill-blob-utils/hex"
	ethereum "github.com/DillLabs/dill-execution"

	"github.com/DillLabs/dill-execution/accounts/abi/bind"
	"github.com/DillLabs/dill-execution/common"
	"github.com/DillLabs/dill-execution/core/types"
	"github.com/DillLabs/dill-execution/crypto"
	gethkzg4844 "github.com/DillLabs/dill-execution/crypto/kzg4844"
	"github.com/DillLabs/dill-execution/ethclient"
	"github.com/holiman/uint256"

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

	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("App failed: %v", err)
	}
}

func TxApp(cliCtx *cli.Context) error {
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

	blobs, commitments, proofs, versionedHashes, err := EncodeBlobs(data)
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
	addr := cliCtx.String(TxRPCURLFlag.Name)
	to := common.HexToAddress(cliCtx.String(TxToFlag.Name))
	prv := cliCtx.String(TxPrivateKeyFlag.Name)
	blobSize := cliCtx.Uint64(TxBlobSizeFlag.Name)
	nonce := cliCtx.Int64(TxNonceFlag.Name)
	deltaNonce := cliCtx.Int64(TxDeltaNonceFlag.Name)
	deltaSleep := cliCtx.Int64(TxDeltaSleepTimeFlag.Name)
	value := cliCtx.String(TxValueFlag.Name)
	gasLimit := cliCtx.Uint64(TxGasLimitFlag.Name)
	gasPrice := cliCtx.String(TxGasPriceFlag.Name)
	priorityGasPrice := cliCtx.String(TxPriorityGasPrice.Name)
	maxFeePerBlobGas := cliCtx.String(TxMaxFeePerBlobGas.Name)
	chainID := cliCtx.String(TxChainID.Name)
	calldata := cliCtx.String(TxCalldata.Name)
	value256, err := uint256.FromHex(value)
	if err != nil {
		log.Fatalf("invalid value param: %v", err)
		return
	}

	for {
		blobSize = blobSize - blobSize%32
		data := RandomFrData(int(blobSize))

		chainId, _ := new(big.Int).SetString(chainID, 0)

		ctx := context.Background()
		client, err := ethclient.DialContext(ctx, addr)
		if err != nil {
			log.Printf("Failed to connect to the Ethereum client: %v", err)
			continue
		}

		key, err := crypto.HexToECDSA(prv)
		if err != nil {
			log.Printf("%v: invalid private key", err)
			continue
		}

		if nonce == -1 || nonce%int64(deltaNonce) == 0 {
			pendingNonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
			if err != nil {
				log.Printf("Error getting nonce: %v", err)
				continue
			}
			nonce = int64(pendingNonce)
		}

		var gasPrice256 *uint256.Int
		if gasPrice == "" {
			val, err := client.SuggestGasPrice(ctx)
			if err != nil {
				log.Printf("Error getting suggested gas price: %v", err)
				continue
			}
			var nok bool
			gasPrice256, nok = uint256.FromBig(val)
			if nok {
				log.Printf("gas price is too high! got %v", val.String())
				continue
			}
		} else {
			gasPrice256, err = DecodeUint256String(gasPrice)
			if err != nil {
				log.Printf("%v: invalid gas price", err)
				continue
			}
		}

		priorityGasPrice256 := gasPrice256
		if priorityGasPrice != "" {
			priorityGasPrice256, err = DecodeUint256String(priorityGasPrice)
			if err != nil {
				log.Printf("%v: invalid priority gas price", err)
				continue
			}
		}

		maxFeePerBlobGas256, err := DecodeUint256String(maxFeePerBlobGas)
		if err != nil {
			log.Printf("%v: invalid max_fee_per_blob_gas", err)
			continue
		}

		blobs, commitments, proofs, versionedHashes, err := EncodeBlobs(data, true)
		if err != nil {
			log.Printf("failed to compute commitments: %v", err)
			continue
		}

		calldataBytes, err := common.ParseHexOrString(calldata)
		if err != nil {
			log.Printf("failed to parse calldata: %v", err)
			continue
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
			log.Printf("failed to send transaction: %v", err)
			continue
		} else {
			log.Printf("successfully sent transaction. txhash=%v", signedTx.Hash())
			nonce += 1
			if nonce%int64(deltaNonce) == 0 {
				time.Sleep(time.Duration(deltaSleep) * time.Second)
			}
		}
	}
}

func ethTransfer(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, to common.Address, amount *big.Int, nonce *uint64) *types.Transaction {
	if nonce == nil {
		log.Printf("reading nonce for account: %v", auth.From.Hex())
		var err error
		n, err := client.NonceAt(ctx, auth.From, nil)
		log.Printf("nonce: %v", n)
		chkErr(err)
		nonce = &n
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	chkErr(err)

	gasLimit, err := client.EstimateGas(context.Background(), ethereum.CallMsg{To: &to})
	chkErr(err)

	tx := types.NewTransaction(*nonce, to, amount, gasLimit, gasPrice, nil)

	signedTx, err := auth.Signer(auth.From, tx)
	chkErr(err)

	//log.Printf("sending transfer tx")
	err = client.SendTransaction(ctx, signedTx)
	chkErr(err)
	log.Printf("tx sent: %v", signedTx.Hash().Hex())

	rlp, err := signedTx.MarshalBinary()
	chkErr(err)

	log.Printf("tx rlp: %v", hex.EncodeToHex(rlp))

	return signedTx
}

func chkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

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

func ProofApp(cliCtx *cli.Context) error {
	file := cliCtx.String(ProofBlobFileFlag.Name)
	blobIndex := cliCtx.Uint64(ProofBlobIndexFlag.Name)
	inputPoint := cliCtx.String(ProofInputPointFlag.Name)

	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("error reading blob file: %v", err)
	}
	blobs, commitments, _, versionedHashes, err := EncodeBlobs(data)
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
