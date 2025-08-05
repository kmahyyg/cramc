package cryptutils

import (
	"bytes"
	"cramc_go/common"
	"cramc_go/customerrs"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	AddiMsgVersionInAssoData = 1

	addiMsgLengthRecord  = 4
	addiMsgVersionLength = 1
)

func KCRC32(pt []byte) []byte {
	kTable := crc32.MakeTable(crc32.Koopman)
	hKCRC32 := crc32.Checksum(pt, kTable)
	res := make([]byte, 4)
	binary.LittleEndian.PutUint32(res, hKCRC32)
	return res
}

func XChacha20Encrypt(key []byte, amad []byte, pt []byte) (ct []byte, err error) {
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

	// handling additional messages in associated data
	amadLen := len(amad)
	amadLenInBytes := binary.LittleEndian.AppendUint32([]byte{}, uint32(amadLen))

	// calc checksum
	crcInAssoData := KCRC32(pt)

	// arrange them in correct place
	assoData := bytes.NewBuffer(nil)
	assoData.Write(crcInAssoData)                      // 4 bytes
	assoData.WriteByte(byte(AddiMsgVersionInAssoData)) // 1 byte
	assoData.Write(amadLenInBytes)                     // 4 bytes
	assoData.Write(amad)                               // X bytes

	fAssoData := assoData.Bytes()

	// combined ciphertext = nonce (iv) + associatedData (kcrc32 of pt+length of am+am version+am) + ciphertext (pt) + tag (overhead)
	ct = make([]byte, chacha20poly1305.NonceSizeX+len(fAssoData)+len(pt)+chacha20poly1305.Overhead)
	copy(ct, iv)
	// put asso data in place
	copy(ct[chacha20poly1305.NonceSizeX:], fAssoData)
	ctFinal := ciph.Seal(nil, iv, pt, fAssoData)
	copy(ct[chacha20poly1305.NonceSizeX+len(fAssoData):], ctFinal)

	return ct, nil
}

func XChacha20Decrypt(key []byte, mixedct []byte) (addimsg []byte, pt []byte, err error) {
	// figure IV out
	iv := make([]byte, chacha20poly1305.NonceSizeX)
	copy(iv, mixedct[:chacha20poly1305.NonceSizeX])

	ciph, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, nil, err
	}

	// extract assodata
	fAssoData, amVer, kcrc, _, amad, err := GetAssoData(mixedct)
	if amVer != AddiMsgVersionInAssoData {
		return nil, nil, customerrs.ErrEncSchemaMismatch
	}

	// plaintext length = all - IV size - AEAD Tag size - AEAD associated data size
	pt = make([]byte, len(mixedct)-chacha20poly1305.NonceSizeX-chacha20poly1305.Overhead-len(fAssoData))
	ptFinal, err := ciph.Open(nil, iv, mixedct[chacha20poly1305.NonceSizeX+len(fAssoData):], fAssoData)
	if err != nil {
		return nil, nil, err
	}
	copy(pt, ptFinal)

	// verify checksum
	expectedCSum := KCRC32(pt)
	if !bytes.Equal(expectedCSum, kcrc) {
		return nil, nil, customerrs.ErrDecryptionFailed
	}

	return amad, pt, nil
}

func GetAssoData(mixedct []byte) (fassoData []byte, msgVer byte, kcrc []byte, amadLen uint32, amad []byte, err error) {
	msgVer = mixedct[chacha20poly1305.NonceSizeX+crc32.Size]
	amadLen = binary.LittleEndian.Uint32(mixedct[chacha20poly1305.NonceSizeX+crc32.Size+addiMsgVersionLength : chacha20poly1305.NonceSizeX+crc32.Size+addiMsgVersionLength+addiMsgLengthRecord])
	amad = mixedct[chacha20poly1305.NonceSizeX+crc32.Size+addiMsgVersionLength+addiMsgLengthRecord : chacha20poly1305.NonceSizeX+crc32.Size+addiMsgVersionLength+addiMsgLengthRecord+amadLen]
	fassoData = mixedct[chacha20poly1305.NonceSizeX : chacha20poly1305.NonceSizeX+crc32.Size+addiMsgVersionLength+addiMsgLengthRecord+amadLen]
	kcrc = mixedct[chacha20poly1305.NonceSizeX : chacha20poly1305.NonceSizeX+crc32.Size]
	err = nil
	return
}

func InterpreteAssoData(fassoData []byte, msgVer byte, kcrc []byte, amadLen uint32, amad []byte) (err error) {
	common.Logger.Info(fmt.Sprintf("Associated Data Length: %d, Additional Message Version: %d - Length: %d, KCRC32: %s, Additional Message in Base64: %s, Additional Message in String: %s", len(fassoData),
		msgVer, amadLen, hex.EncodeToString(kcrc), base64.RawURLEncoding.EncodeToString(amad), string(amad)))
	return nil
}
