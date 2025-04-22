package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/DillLabs/dill-blob-utils/hex"
	das "github.com/DillLabs/dill-das"
	ethereum "github.com/DillLabs/dill-execution"
	"github.com/DillLabs/dill-execution/accounts/abi/bind"
	"github.com/DillLabs/dill-execution/common"
	"github.com/DillLabs/dill-execution/core/types"
	"github.com/DillLabs/dill-execution/crypto"
	"github.com/DillLabs/dill-execution/crypto/kzg4844"
	"github.com/DillLabs/dill-execution/ethclient"
	"github.com/DillLabs/dill-execution/params"
	"github.com/crate-crypto/go-ipa/bandersnatch/fr"
	"github.com/holiman/uint256"
)

func encodeBlobs(data []byte) []kzg4844.Blob {
	blobs := []kzg4844.Blob{{}}
	blobIndex := 0
	fieldIndex := -1
	for i := 0; i < len(data); i += 31 {
		fieldIndex++
		if fieldIndex == params.BlobTxFieldElementsPerBlob {
			blobs = append(blobs, kzg4844.Blob{})
			blobIndex++
			fieldIndex = 0
		}
		max := i + 31
		if max > len(data) {
			max = len(data)
		}
		copy(blobs[blobIndex][fieldIndex*32:], data[i:max])
	}
	return blobs
}

func EncodeBlobs(data []byte, canonical ...bool) ([]kzg4844.Blob, []kzg4844.Commitment, []kzg4844.Proof, []kzg4844.Proof, []common.Hash, error) {
	var (
		blobs           []kzg4844.Blob
		commits         []kzg4844.Commitment
		proofs          []kzg4844.Proof
		extraProofs     []kzg4844.Proof
		versionedHashes []common.Hash
	)

	if len(canonical) != 0 && canonical[0] {
		blobSize := 4096 * 32
		for i := 0; i < len(data)/blobSize; i++ {
			blobs = append(blobs, kzg4844.Blob(data[i*blobSize:(i+1)*blobSize]))
		}
	} else {
		blobs = encodeBlobs(data)
	}
	for _, blob := range blobs {
		commit, err := kzg4844.BlobToCommitment(blob)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		commits = append(commits, commit)

		proof, err := kzg4844.ComputeBlobProof(blob, commit)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		proofs = append(proofs, proof)
		ep, err := das.BlobToSegmentsProofOnly(blob[:])
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		for _, p := range ep {
			extraProofs = append(extraProofs, kzg4844.Proof(das.MarshalProof(&p)))
		}
		versionedHashes = append(versionedHashes, kZGToVersionedHash(commit))
	}
	return blobs, commits, proofs, extraProofs, versionedHashes, nil
}

var blobCommitmentVersionKZG uint8 = 0x01

// kZGToVersionedHash implements kzg_to_versioned_hash from EIP-4844
func kZGToVersionedHash(kzg kzg4844.Commitment) common.Hash {
	h := sha256.Sum256(kzg[:])
	h[0] = blobCommitmentVersionKZG

	return h
}

func DecodeBlob(blob []byte) []byte {
	if len(blob) != params.BlobTxFieldElementsPerBlob*32 {
		panic("invalid blob encoding")
	}
	var data []byte

	// XXX: the following removes trailing 0s in each field element (see EncodeBlobs), which could be unexpected for certain blobs
	j := 0
	for i := 0; i < params.BlobTxFieldElementsPerBlob; i++ {
		data = append(data, blob[j:j+31]...)
		j += 32
	}

	i := len(data) - 1
	for ; i >= 0; i-- {
		if data[i] != 0x00 {
			break
		}
	}
	data = data[:i+1]
	return data
}

func DecodeUint256String(hexOrDecimal string) (*uint256.Int, error) {
	var base = 10
	if strings.HasPrefix(hexOrDecimal, "0x") {
		base = 16
	}
	b, ok := new(big.Int).SetString(hexOrDecimal, base)
	if !ok {
		return nil, fmt.Errorf("invalid value")
	}
	val256, nok := uint256.FromBig(b)
	if nok {
		return nil, fmt.Errorf("value is too big")
	}
	return val256, nil
}

func generatePrivateKeys(count int) []*ecdsa.PrivateKey {
	keys := []*ecdsa.PrivateKey{}
	for range count {
		key, _ := crypto.GenerateKey()
		keys = append(keys, key)
	}
	return keys
}

func transferToken(client *ethclient.Client, toAddress string,
	etherAmount int64, nonce uint64,
	chainId *big.Int, globalGasPrice *big.Int,
	globalGasAmount uint64,
	privateKey *ecdsa.PrivateKey) (*types.Transaction, error) {

	to := common.HexToAddress(toAddress)
	eip1559Tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: globalGasPrice,
		Gas:      globalGasAmount,
		To:       &to,
		Value:    new(big.Int).Mul(big.NewInt(etherAmount), big.NewInt(params.Ether)),
		Data:     nil,
	})
	signedTx, err := types.SignTx(eip1559Tx, types.LatestSignerForChainID(chainId), privateKey)
	if err != nil {
		return nil, err
	}
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return nil, err
	}

	return signedTx, nil
}

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

func sendTxAndWait(ctx context.Context, client *ethclient.Client, tx *types.Transaction) error {
	start := time.Now()
	log.Printf("Commitments: %v\n", fmt.Sprintf("%x", tx.BlobTxSidecar().Commitments))
	log.Printf("GasTipCap: %v, BlobGasFeeCap: %v, GasFeeCap: %v\n",
		tx.GasTipCap(), tx.BlobGasFeeCap(), tx.GasFeeCap())
	err := client.SendTransaction(ctx, tx)
	if err != nil {
		return err
	}
	for {
		_, err := client.TransactionReceipt(context.Background(), tx.Hash())
		if err == ethereum.NotFound {
			time.Sleep(4 * time.Second)
			continue
		}
		if err != nil {
			return err
		}
		break
	}
	log.Printf("tx %s included, time used %fs", tx.Hash().String(), time.Since(start).Seconds())
	return nil
}

type blobsStruct struct {
	blobs           []kzg4844.Blob
	comms           []kzg4844.Commitment
	proofs          []kzg4844.Proof
	versionedHashes []common.Hash
}

func randomBlobs(cnt int) blobsStruct {
	data := RandomFrData(4096 * 32 * cnt)
	blobs, commitments, proofs, _, versionedHashes, err := EncodeBlobs(data, true)
	if err != nil {
		log.Fatalf("failed to compute commitments: %v", err)
	}
	return blobsStruct{
		blobs:           blobs,
		comms:           commitments,
		proofs:          proofs,
		versionedHashes: versionedHashes,
	}
}
