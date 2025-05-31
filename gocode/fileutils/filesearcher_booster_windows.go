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
		log.Fatalln(err)
	}
	defer curProcTokenR.Close()
	if curProcTokenR.IsElevated() {
		return true, nil
	} else {
		return false, customerrs.ErrInsufficientPrivilege
	}
	return false, customerrs.ErrUnknownInternalError
}

func IsDriveFileSystemNTFS() (bool, error) {
	return false, customerrs.ErrUnknownInternalError
}

func ExtractAndParseMFT(actionPath []string, allowedExts []string, outputChan chan string) (bool, error) {
	return false, customerrs.ErrUnknownInternalError
}
