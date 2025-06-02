package windoge_utils

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	psutil "github.com/shirou/gopsutil/v4/process"
	"runtime"
	"slices"
)

func KillAllOfficeProcesses() (bool, error) {
	coveredProcess := []string{"excel.exe"}
	procKilled := false
	if runtime.GOOS == "windows" {
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
					common.Logger.Errorln("Loop Process Name:", err)
					continue
				}
				if slices.Contains(coveredProcess, pName) {
					_ = p.Kill()
					procKilled = true
				}
			}
			return procKilled, nil
		}
	}
	return false, customerrs.ErrUnsupportedPlatform
}
