package windoge_utils

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	psutil "github.com/shirou/gopsutil/v4/process"
	"slices"
	"strings"
)

func KillAllOfficeProcesses() (bool, error) {
	coveredProcess := []string{"excel.exe"}
	procKilled := false
	if common.IsRunningOnWin {
		common.Logger.Infoln("Trying to kill office processes.")
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
