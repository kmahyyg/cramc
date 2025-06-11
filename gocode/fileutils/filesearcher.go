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
		if err != nil {
			// safely ignore errors as you can't access these file under current privilege
			// neither virus nor you can access
			common.Logger.Warningln(err.Error())
			return nil
		}
		if d.IsDir() {
			return nil
		}
		var matchF = func(fullPath string) bool {
			fExt := path.Ext(fullPath)
			if slices.Contains(allowedExts, fExt) || strings.Contains(fullPath, "AppData\\Roaming\\Microsoft\\Excel\\XLSTART") {
				return true
			}
			return false
		}
		if matchF(curPath) {
			counter += 1
			outputChan <- fsRootDir + "/" + curPath
		}
		return nil
	}

	err := fs.WalkDir(fsRoot, ".", walkFn)
	if err != nil {
		return -1, err
	}
	return counter, nil
}
