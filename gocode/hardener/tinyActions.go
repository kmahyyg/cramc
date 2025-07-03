package hardener

import (
	"cramc_go/common"
	"cramc_go/platform/windoge_utils"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func f_harden_rmReplaceDirSetRO(aType string, filep string) {
	switch aType {
	case "file":
		err := os.RemoveAll(filep)
		common.Logger.Info(fmt.Sprintf("remove file: %s, err: %v", filep, err))
		err = os.MkdirAll(filep, 0400)
		common.Logger.Info(fmt.Sprintf("replace with dir: %s, err: %v", filep, err))
		f_harden_SetRO(filep, "dir")
	default:
		common.Logger.Warn("unsupported operation for type: " + aType)
	}
}

func f_harden_replaceFileSetRO(aType string, filep string) {
	switch aType {
	case "dir":
		err := os.RemoveAll(filep)
		common.Logger.Info(fmt.Sprintf("remove dir: %s, err: %v ", filep, err))
		fd, err := os.Create(filep)
		common.Logger.Info(fmt.Sprintf("create file: %s, err: %v ", filep, err))
		if err == nil {
			_ = fd.Close()
		}
		f_harden_SetRO(filep, "file")
	default:
		common.Logger.Warn("unsupported operation for type: " + aType)
	}
}

func f_harden_CleanSetRO(aType string, filep string) {
	switch aType {
	case "file":
		fd, err := os.Create(filep)
		common.Logger.Info(fmt.Sprintf("Recreate File first, Err: %v", err))
		if err == nil {
			_ = fd.Close()
		}
		common.Logger.Info(fmt.Sprintf("Truncate File Err: %v", os.Truncate(filep, 0)))
	case "dir":
		common.Logger.Info(fmt.Sprintf("Remove All, Err: %v", os.RemoveAll(filep)))
		common.Logger.Info(fmt.Sprintf("Recreate Dir, Err: %v", os.Mkdir(filep, 0400)))
	default:
		common.Logger.Warn("Unsupported type: " + aType)
	}
	f_harden_SetRO(aType, filep)
}

func f_harden_SetRO(aType string, filep string) {
	switch aType {
	case "file":
		common.Logger.Info(fmt.Sprintf("Chmod 0400 (RO): %v", os.Chmod(filep, 0400)))
	case "dir":
		fStat, err := os.Stat(filep)
		if err != nil {
			common.Logger.Error(err.Error())
			return
		}
		if fStat.IsDir() {
			err = filepath.Walk(filep, func(eachfPath string, eachInfo os.FileInfo, err error) error {
				err = os.Chmod(eachfPath, 0400)
				if err != nil {
					common.Logger.Error(err.Error())
				}
				return nil
			})
			common.Logger.Info(fmt.Sprintf("Chmod 0400 (RO) on Dir Recursively: %v", err))
		} else {
			common.Logger.Warn("Mismatched Action Type: " + aType)
		}
	}
}

func applyTextTemplate(tmpl string) (string, error) {
	runAsSystem, err := windoge_utils.CheckRunningUnderSYSTEM()
	if err != nil {
		return "", err
	}
	cUser, err := user.Current()
	if err != nil {
		return "", err
	}
	var tRes string
	if runAsSystem {
		impToken, err := windoge_utils.GetLoggedInUserToken(windows.TokenPrimary)
		if err != nil {
			return "", err
		}
		intaUserToken := (windows.Token)(impToken)
		defer intaUserToken.Close()
		userProf, err := intaUserToken.GetUserProfileDirectory()
		if err != nil {
			return "", err
		}
		tRes = strings.ReplaceAll(tmpl, "${HOME}", userProf)
	} else {
		tRes = strings.ReplaceAll(tmpl, "${HOME}", cUser.HomeDir)
	}
	return tRes, nil
}
