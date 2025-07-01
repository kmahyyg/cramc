package hardener

import (
	"cramc_go/common"
	"cramc_go/platform/windoge_utils"
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
		common.Logger.Infoln("remove file: ", filep, ", err: ", err)
		err = os.MkdirAll(filep, 0400)
		common.Logger.Infoln("replace with dir: ", filep, ", err: ", err)
		f_harden_SetRO(filep, "dir")
	default:
		common.Logger.Warnln("unsupported operation for type: ", aType)
	}
}

func f_harden_replaceFileSetRO(aType string, filep string) {
	switch aType {
	case "dir":
		err := os.RemoveAll(filep)
		common.Logger.Infoln("remove dir: ", filep, ", err: ", err)
		fd, err := os.Create(filep)
		common.Logger.Infoln("create file: ", filep, ", err: ", err)
		if err == nil {
			_ = fd.Close()
		}
		f_harden_SetRO(filep, "file")
	default:
		common.Logger.Warnln("unsupported operation for type: ", aType)
	}
}

func f_harden_CleanSetRO(aType string, filep string) {
	switch aType {
	case "file":
		fd, err := os.Create(filep)
		common.Logger.Infoln("Recreate File first, Err: ", err)
		if err == nil {
			_ = fd.Close()
		}
		common.Logger.Infoln("Truncate File Err: ", os.Truncate(filep, 0))
	case "dir":
		common.Logger.Infoln("Remove All, Err: ", os.RemoveAll(filep))
		common.Logger.Infoln("Recreate Dir, Err: ", os.Mkdir(filep, 0400))
	default:
		common.Logger.Warnln("Unsupported type: ", aType)
	}
	f_harden_SetRO(aType, filep)
}

func f_harden_SetRO(aType string, filep string) {
	switch aType {
	case "file":
		common.Logger.Infoln("Chmod 0400 (RO): ", os.Chmod(filep, 0400))
	case "dir":
		fStat, err := os.Stat(filep)
		if err != nil {
			common.Logger.Errorln(err)
			return
		}
		if fStat.IsDir() {
			err = filepath.Walk(filep, func(eachfPath string, eachInfo os.FileInfo, err error) error {
				err = os.Chmod(eachfPath, 0400)
				if err != nil {
					common.Logger.Errorln(err)
				}
				return nil
			})
			common.Logger.Infoln("Chmod 0400 (RO) on Dir Recursively: ", err)
		} else {
			common.Logger.Warnln("Mismatched Action Type: ", aType)
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
