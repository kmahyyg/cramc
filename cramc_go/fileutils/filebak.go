package fileutils

import (
	"bytes"
	"cramc_go/common"
	"cramc_go/cryptutils"
	"encoding/hex"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

func ZstdBakFile(fPath string) error {
	bakFd, err := os.OpenFile(fPath+".zst.ebak", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer bakFd.Close()
	defer bakFd.Sync()
	originalFd, err := os.OpenFile(fPath, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer originalFd.Close()

	// buffer for further encryption
	compressedBuf := bytes.NewBuffer(nil)
	zstdWr, err := zstd.NewWriter(compressedBuf, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		return err
	}
	defer zstdWr.Close()
	_, err = io.Copy(zstdWr, originalFd)
	if err != nil {
		return err
	}
	// from buffer, implement encryption
	keyBytes, err := hex.DecodeString(common.HexEncryptionPassword)
	if err != nil {
		return err
	}
	ctFull, err := cryptutils.XChacha20Encrypt(keyBytes, []byte(fPath), compressedBuf.Bytes())
	if err != nil {
		return err
	}
	_, err = bakFd.Write(ctFull)
	if err != nil {
		return err
	}
	compressedBuf.Reset()
	return nil
}
