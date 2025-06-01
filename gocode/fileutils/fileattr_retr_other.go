//go:build !windows

package fileutils

import "os"

func CheckFileOnDiskSize(fpath string) (exist bool, logicalSize int64, err error) {
	fInfo, err := os.Stat(fpath)
	if err != nil {
		return false, -1, err
	}
	return true, fInfo.Size(), nil
}
