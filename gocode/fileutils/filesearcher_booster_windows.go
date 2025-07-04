//go:build windows

package fileutils

import (
	"context"
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/logging"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/user"
	"path"
	"regexp"
	"slices"
	"strings"
	ntfs "www.velocidex.com/golang/go-ntfs/parser"
)

func CheckProcessElevated() (bool, error) {
	u, err := user.Current()
	if err != nil {
		return false, err
	}
	common.Logger.Info(fmt.Sprintf("Current running as: %s (%s) ", u.Name, u.Username))
	var curProcTokenR windows.Token
	err = windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &curProcTokenR)
	if err != nil {
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(5)
	}
	defer curProcTokenR.Close()
	if curProcTokenR.IsElevated() {
		return true, nil
	} else {
		return false, customerrs.ErrInsufficientPrivilege
	}
}

func IsDriveFileSystemNTFS(actionPath string) (bool, error) {
	// Extract drive letter from the first character
	driveLetter := actionPath[0]

	// Construct root path (e.g., "C:\")
	rootPath := string(driveLetter) + ":\\"

	// Convert to UTF16 for Windows API
	rootPathPtr, err := windows.UTF16PtrFromString(rootPath)
	if err != nil {
		return false, err
	}

	// Buffer for filesystem name (32 characters should be enough for filesystem names)
	fileSystemNameBuffer := make([]uint16, 32)

	// Call GetVolumeInformation to get filesystem name
	err = windows.GetVolumeInformation(
		rootPathPtr,                       // lpRootPathName
		nil,                               // lpVolumeNameBuffer (we don't need volume name)
		0,                                 // nVolumeNameSize
		nil,                               // lpVolumeSerialNumber
		nil,                               // lpMaximumComponentLength
		nil,                               // lpFileSystemFlags
		&fileSystemNameBuffer[0],          // lpFileSystemNameBuffer
		uint32(len(fileSystemNameBuffer)), // nFileSystemNameSize
	)

	if err != nil {
		return false, err
	}

	// Convert UTF16 buffer to string
	fileSystemName := windows.UTF16ToString(fileSystemNameBuffer)
	res := fileSystemName == "NTFS"
	if res {
		return res, nil
	} else {
		return res, customerrs.ErrFallbackToCompatibleSolution
	}
}

func ExtractAndParseMFTThenSearch(actionPath string, allowedExts []string, outputChan chan string) (int, error) {
	defer close(outputChan)
	// Extract drive letter from the first character
	volDiskLetter := actionPath[0]

	common.Logger.Debug("Check Drive Letter.")
	// check user input
	var IsDiskLetter = regexp.MustCompile(`^[a-zA-Z]$`).MatchString
	if !IsDiskLetter(string(volDiskLetter)) {
		return -1, customerrs.ErrInvalidInput
	}

	common.Logger.Debug("Open Raw Device Handle.")
	// use UNC path to access raw device to bypass limitation of file lock, e.g. \\.\C:
	volFd, err := os.Open("\\\\.\\" + string(volDiskLetter) + ":")
	if err != nil {
		common.Logger.Error("Triggered Fallback Error: " + customerrs.ErrDeviceInaccessible.Error())
		return -1, customerrs.ErrFallbackToCompatibleSolution
	}
	defer volFd.Close()

	common.Logger.Debug("Create PagedReader with page 4096, cache size 65536.")
	// build a pagedReader for raw device to feed the NTFSContext initializor
	ntfsPagedReader, err := ntfs.NewPagedReader(volFd, 0x1000, 0x10000)
	if err != nil {
		common.Logger.Error("Triggered Fallback Error: " + err.Error())
		return -1, customerrs.ErrFallbackToCompatibleSolution
	}

	common.Logger.Debug("Create NTFSContext.")
	// build NTFS context for root device
	ntfsVolCtx, err := ntfs.GetNTFSContext(ntfsPagedReader, 0)
	if err != nil {
		common.Logger.Error("Triggered Fallback Error: " + err.Error())
		return -1, customerrs.ErrFallbackToCompatibleSolution
	}

	common.Logger.Debug("Try to get $MFT $DATA stream.")
	volMFTEntry, err := ntfsVolCtx.GetMFT(0)
	if err != nil {
		common.Logger.Error("Triggered Fallback Error: " + err.Error())
		return -1, customerrs.ErrFallbackToCompatibleSolution
	}

	// open $DATA attr of $MFT, https://github.com/Velocidex/go-ntfs/blob/master/bin/mft.go
	mftReader, err := ntfs.OpenStream(ntfsVolCtx, volMFTEntry, uint64(128), ntfs.WILDCARD_STREAM_ID, ntfs.WILDCARD_STREAM_NAME)
	if err != nil {
		common.Logger.Error("Triggered Fallback Error: " + err.Error())
		return -1, customerrs.ErrFallbackToCompatibleSolution
	}
	common.Logger.Debug("Successfully opened $MFT:$DATA.")

	// check if prefix matched the actionPath
	residentialPathDir := strings.Split(strings.ReplaceAll(actionPath, "\\", "/"), ":")
	if len(residentialPathDir) != 2 {
		// windows filename doesn't allow ':' char
		// result should be: []string{"C", "/Users"}
		common.Logger.Warn("actionPath contains invalid char, " + customerrs.ErrInvalidInput.Error())
		return -1, customerrs.ErrInvalidInput
	}

	// start iterating and filter
	counter := 0
	for item := range ntfs.ParseMFTFile(context.Background(), mftReader, ntfs.RangeSize(mftReader),
		ntfsVolCtx.ClusterSize, ntfsVolCtx.RecordSize) {
		// filter files here
		// only include current on-disk ones
		if !item.InUse {
			continue
		}
		if item.IsDir {
			continue
		}
		fPath := item.FullPath()
		// fPath example: /Users/<username>/<dir>/file.bin
		// if hasPrefix && intended extensions, all good.
		var matchF = func(fullPath string) bool {
			fExt := path.Ext(fullPath)
			if strings.HasPrefix(fullPath, residentialPathDir[1]) && (slices.Contains(allowedExts, fExt) || strings.Contains(fullPath, "AppData/Roaming/Microsoft/Excel/XLSTART")) {
				return true
			}
			return false
		}
		// add matchFunc for XLSTART
		if matchF(fPath) {
			counter += 1
			outputChan <- string(volDiskLetter) + ":" + fPath
		}
	}
	return counter, nil
}
