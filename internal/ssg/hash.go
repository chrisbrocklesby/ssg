package ssg

import (
	"crypto/sha1"
	"encoding/hex"
)

func hashBytes(b []byte) string {
	h := sha1.Sum(b)
	return hex.EncodeToString(h[:])
}
