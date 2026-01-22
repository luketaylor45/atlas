package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomString generates a random hex string of given length
func RandomString(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}
