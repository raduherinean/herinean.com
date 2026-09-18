package images

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash8 is the first 8 hex chars of SHA-256: enough to make a URL change whenever the bytes do.
func Hash8(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])[:8]
}
