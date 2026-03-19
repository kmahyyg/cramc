//go:build windows

package fileutils

import (
	"cramc_go/customerrs"
	"golang.org/x/sys/windows"
	"os"
)

const (
	WINDOWS_FILE_ATTRIBUTE_UNPINNED = 0x00100000
)

func CheckFileOnDiskSize(fpath string) (exist bool, logicalSize int64, err error) {
	// check logical exists
	fInfo, err := os.Stat(fpath)
	if err != nil {
		return false, -1, err
	}

	fPathU16StrPtr := windows.StringToUTF16Ptr(fpath)
	fAttrs, err := windows.GetFileAttributes(fPathU16StrPtr)
	if err != nil {
		return false, -1, err
	}

	isOffline := fAttrs&windows.FILE_ATTRIBUTE_OFFLINE != 0
	isCloudOnly := fAttrs&windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS != 0
	isUnpinned := fAttrs&WINDOWS_FILE_ATTRIBUTE_UNPINNED != 0

	if isOffline || isCloudOnly || isUnpinned {
		return false, -1, customerrs.ErrFileExistsOnCloudOnly
	}

	return true, fInfo.Size(), nil
}
