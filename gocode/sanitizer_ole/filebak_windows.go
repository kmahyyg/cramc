//go:build windows

package sanitizer_ole

import (
	"github.com/klauspost/compress/gzip"
	"io"
	"os"
	"path/filepath"
)

func gzBakFile(fPath string) error {
	bakFd, err := os.OpenFile(fPath+".gz.bak", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
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
	gzWr, err := gzip.NewWriterLevel(bakFd, gzip.BestSpeed)
	if err != nil {
		return err
	}
	_, err = io.Copy(gzWr, originalFd)
	if err != nil {
		return err
	}
	defer gzWr.Close()
	defer gzWr.Flush()
	return nil
}

func renameFileAndSave(fPath string) error {
	fName := filepath.Base(fPath)
	fDir := filepath.Dir(fPath)
	newfPath := filepath.Join(fDir, "G-"+fName)
	return os.Rename(fPath, newfPath)
}
