package fileutils

import (
	"cramc_go/common"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
)

func GeneralWalkthroughSearch(actionPath string, allowedExts []string, outputChan chan string) (int, error) {
	defer close(outputChan)
	fsRoot := os.DirFS(actionPath)
	fsRootDir := strings.ReplaceAll(actionPath, "\\", "/")

	counter := 0
	walkFn := func(curPath string, d fs.DirEntry, err error) error {
		// filter1: prefix path already in place
		// filter2: allowedExts
		// filter3: must remove regardless what happened
		if err != nil {
			// safely ignore errors as you can't access these file under current privilege
			// neither virus nor you can access
			common.Logger.Warn(err.Error())
			return nil
		}
		if d.IsDir() {
			return nil
		}
		var matchF = func(fullPath string) bool {
			fExt := path.Ext(fullPath)
			// issue #26: allow to remove all stale files under "AppData/Roaming/Microsoft/Excel"
			if strings.Contains(fullPath, "AppData/Roaming/Microsoft/Excel") || strings.Contains(fullPath, "AppData/Local/Microsoft/Windows/INetCache") {
				return true
			}
			// filter known exts
			if slices.Contains(allowedExts, fExt) {
				return true
			}
			return false
		}
		if matchF(curPath) {
			// issue #26: remove all stale files under "AppData/Roaming/Microsoft/Excel"
			finalOpt := fsRootDir + "/" + curPath
			fJudge, err := checkAndCleanStaleFiles(finalOpt)
			if err != nil {
				common.Logger.Error(err.Error())
				return nil
			}
			if !fJudge {
				counter += 1
				outputChan <- finalOpt
			}
		}
		return nil
	}

	err := fs.WalkDir(fsRoot, ".", walkFn)
	if err != nil {
		return -1, err
	}
	return counter, nil
}

func CheckFileLogicalExists(filename string) bool {
	if len(filename) == 0 {
		return false
	}
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
