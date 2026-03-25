package vba

import (
	"crypto/sha1"
	"encoding/hex"
)

func XORCipher(data []byte, key []byte) []byte {
	if len(key) == 0 {
		return append([]byte(nil), data...)
	}
	out := make([]byte, len(data))
	for idx := range data {
		out[idx] = data[idx] ^ key[idx%len(key)]
	}
	return out
}

func ContentHashSHA1(data []byte) [20]byte {
	return sha1.Sum(data)
}

func ContentHashHex(data []byte) string {
	sum := ContentHashSHA1(data)
	return hex.EncodeToString(sum[:])
}
