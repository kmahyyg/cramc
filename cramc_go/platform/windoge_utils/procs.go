package windoge_utils

import (
	"context"
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/logging"

	psutil "github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"

	"fmt"
	"os"
	"os/user"
	"slices"
	"strings"
)

func KillAllOfficeProcesses() (bool, error) {
	if os.Getenv("RunEnv") != "" {
		common.Logger.Info("RunEnv is set, return true, no operation.")
		return true, nil
	}
	coveredProcess := []string{"excel.exe"}
	procKilled := false
	if common.IsRunningOnWin {
		common.Logger.Info("Trying to kill office processes.")
		if common.DryRunOnly {
			common.Logger.Info("DryRun set, return true, no operation.")
			return true, nil
		} else {
			procs, err := psutil.Processes()
			if err != nil {
				return false, err
			}
			for _, p := range procs {
				pName, err := p.Name()
				if err != nil {
					continue
				}
				pNameInvariant := strings.ToLower(pName)
				if slices.Contains(coveredProcess, pNameInvariant) {
					_ = p.Terminate() // on windows, this library only supports terminating, SIGKILL is not working on non-UNIX system.
					procKilled = true
				}
			}
			return procKilled, nil
		}
	}
	return false, customerrs.ErrUnsupportedPlatform
}

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
