package cryptutils

import (
	"bytes"
	"cramc_go/customerrs"
	crand "crypto/rand"
	"encoding/binary"
	"hash/crc32"

	"golang.org/x/crypto/chacha20poly1305"
)

func KCRC32(pt []byte) []byte {
	kTable := crc32.MakeTable(crc32.Koopman)
	hKCRC32 := crc32.Checksum(pt, kTable)
	res := make([]byte, 4)
	binary.LittleEndian.PutUint32(res, hKCRC32)
	return res
}

func XChacha20Encrypt(key []byte, pt []byte) (ct []byte, err error) {
	// authenticated data used to encrypt config
	iv := make([]byte, chacha20poly1305.NonceSizeX)
	_, err = crand.Read(iv)
	if err != nil {
		return nil, err
	}

	ciph, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	assoData := KCRC32(pt)
	// combined ciphertext = nonce (iv) + associatedData (crc32) + ciphertext (pt) + tag (overhead)
	ct = make([]byte, len(pt)+chacha20poly1305.NonceSizeX+chacha20poly1305.Overhead+crc32.Size)
	copy(ct, iv)
	copy(ct[len(iv):], assoData)
	ciph.Seal(ct[chacha20poly1305.NonceSizeX+crc32.Size:], iv, pt, assoData)

	return ct, nil
}

func XChacha20Decrypt(key []byte, mixedct []byte) (pt []byte, err error) {
	iv := make([]byte, chacha20poly1305.NonceSizeX)
	assoData := make([]byte, crc32.Size)
	copy(iv, mixedct)
	copy(assoData[chacha20poly1305.NonceSizeX:chacha20poly1305.NonceSizeX+crc32.Size], mixedct)

	ciph, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}

	pt = make([]byte, len(mixedct)-chacha20poly1305.NonceSizeX-crc32.Size)
	_, err = ciph.Open(pt, iv, mixedct[chacha20poly1305.NonceSizeX+crc32.Size:], assoData)
	if err != nil {
		return nil, err
	}

	expectedAssoData := KCRC32(pt)
	if !bytes.Equal(expectedAssoData, assoData) {
		return nil, customerrs.ErrDecryptionFailed
	}

	return pt, nil
}
