//go:build windows

package sanitizer_ole

import (
	"github.com/klauspost/compress/zstd"
	"io"
	"os"
	"path/filepath"
)

func zstdBakFile(fPath string) error {
	bakFd, err := os.OpenFile(fPath+".zst.bak", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
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
	zstdWr, err := zstd.NewWriter(bakFd, zstd.WithEncoderCRC(true), zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		return err
	}
	_, err = io.Copy(zstdWr, originalFd)
	if err != nil {
		return err
	}
	defer zstdWr.Close()
	return nil
}

func renameFileAndSave(fPath string) error {
	fName := filepath.Base(fPath)
	fDir := filepath.Dir(fPath)
	newfPath := filepath.Join(fDir, "G-"+fName)
	return os.Rename(fPath, newfPath)
}
