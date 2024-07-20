package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"

	das "github.com/DillLabs/dill-das"
	"github.com/DillLabs/dill-execution/common"
	"github.com/DillLabs/dill-execution/core/types"
	"github.com/DillLabs/dill-execution/crypto"
	"github.com/DillLabs/dill-execution/crypto/kzg4844"
	"github.com/DillLabs/dill-execution/ethclient"
	"github.com/DillLabs/dill-execution/params"
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
	chainId int64, globalGasPrice int64,
	globalGasAmount int64,
	privateKey *ecdsa.PrivateKey) (*types.Transaction, error) {

	to := common.HexToAddress(toAddress)
	eip1559Tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(globalGasPrice),
		Gas:      uint64(globalGasAmount),
		To:       &to,
		Value:    new(big.Int).Mul(big.NewInt(etherAmount), big.NewInt(params.Ether)),
		Data:     nil,
	})
	signedTx, err := types.SignTx(eip1559Tx, types.LatestSignerForChainID(big.NewInt(chainId)), privateKey)
	if err != nil {
		return nil, err
	}
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return nil, err
	}

	return signedTx, nil
}
