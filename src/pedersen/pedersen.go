package pedersen

import (
	"crypto/rand"
	"log"
	"math/big"
)

const (
	p uint64 = 6661
	g uint64 = 666
	h uint64 = 426
)

func GetR() uint64 {
	r, err := rand.Int(rand.Reader, big.NewInt(int64(p-1)))
	if err != nil {
		log.Fatalf("Error: %s\n", err)
	}

	return r.Uint64()
}

func GetCommitment(m uint64, r uint64) uint64 {
	return (g ^ m) * (h ^ r) % p
}

func ValidateCommitment(c uint64, m uint64, r uint64) bool {
	return c == GetCommitment(m, r)
}
