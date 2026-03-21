//go:build windows

package windoge_utils

import (
	"context"
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/logging"
	"fmt"
	"os"
	"os/user"

	"golang.org/x/sys/windows"
)

const (
	WELLKNOWN_SYSTEM_SID = "S-1-5-18"
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

func CheckRunningBySYSTEM() (bool, error) {
	curU, err := user.Current()
	if err != nil {
		common.Logger.Error(err.Error())
		return false, err
	}
	if curU.Uid == WELLKNOWN_SYSTEM_SID {
		return true, nil
	}
	return false, nil
}
