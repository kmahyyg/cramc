package hardener

import (
	"cramc_go/common"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func f_harden_CleanSetRO(aType string, filep string) {
	switch aType {
	case "file":
		fd, err := os.Create(filep)
		common.Logger.Infoln("Recreate File first, Err: ", err)
		_ = fd.Close()
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
	cUser, err := user.Current()
	if err != nil {
		return "", err
	}
	tRes := strings.ReplaceAll(tmpl, "${USERNAME}", cUser.Username)
	return tRes, nil
}
