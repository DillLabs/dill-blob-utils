package main

import (
	"testing"
)

func makeBlob(siz int) []byte {
	b := make([]byte, siz)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func TestBlobCodec(t *testing.T) {

}

/*
func TestBlobsCodec(t *testing.T) {
	blob := makeBlob(params.FieldElementsPerBlob*32 + 10)
	encB := EncodeBlobs(blob)
	if len(encB) != 2 {
		t.Fatal("expected 2 blobs, got", len(encB))
	}
	dec1 := DecodeBlob(encB[0][:])
	dec2 := DecodeBlob(encB[1][:])
	dec := append(dec1, dec2...)
	if len(dec) != len(blob) {
		t.Fatalf("mismatched lengths: expected %d, got %d", len(blob), len(dec))
	}
	if !bytes.Equal(blob, dec) {
		t.Fatalf("expected %x, got %x", blob, dec)
	}
}
*/
