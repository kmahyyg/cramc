//go:build windows

package fileutils

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	"golang.org/x/sys/windows"
	"os/user"
)

func CheckProcessElevated() (bool, error) {
	u, err := user.Current()
	if err != nil {
		return false, err
	}
	common.Logger.Infof("Current running as: %s (%s) ", u.Name, u.Username)
	var curProcTokenR windows.Token
	err = windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &curProcTokenR)
	if err != nil {
		common.Logger.Fatalln(err)
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

	return fileSystemName == "NTFS", customerrs.ErrUnknownInternalError
}

func ExtractAndParseMFT(actionPath string, allowedExts []string, outputChan chan string) (int64, error) {
	return -1, customerrs.ErrUnknownInternalError
}
