//go:build !windows

package fileutils

import (
	"cramc_go/customerrs"
	"os"
)

func CheckFileOnDiskSize(fpath string) (exist bool, logicalSize int64, err error) {
	fInfo, err := os.Stat(fpath)
	if err != nil {
		return false, -1, err
	}
	if fInfo.IsDir() {
		return false, -1, customerrs.ErrInvalidInput
	}
	return true, fInfo.Size(), nil
}
