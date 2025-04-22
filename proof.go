package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/DillLabs/dill-blob-utils/hex"
	gethkzg4844 "github.com/DillLabs/dill-execution/crypto/kzg4844"
	"github.com/urfave/cli"
)

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
