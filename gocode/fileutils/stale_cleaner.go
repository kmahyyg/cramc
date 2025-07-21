package fileutils

import (
	"cramc_go/common"
	"os"
	"path/filepath"
	"strings"
)

func checkAndCleanStaleFiles(fPath string) (bool, error) {
	// if return false, unsuccessful cleanup, forward to another func
	// if return true, delete successfully, no need to go ahead
	//
	// check if filepath match
	if strings.Contains(fPath, "AppData/Roaming/Microsoft/Excel") || strings.Contains(fPath, "AppData/Local/Microsoft/Windows/INetCache") {
		// start removal
		fAbsPath, err := filepath.Abs(fPath)
		if err != nil {
			return false, err
		}
		err = os.RemoveAll(fAbsPath)
		if err != nil {
			return false, err
		}
		common.Logger.Info("Removed stale file under ExcelAppData: " + fAbsPath)
		return true, nil
	}
	return false, nil
}
