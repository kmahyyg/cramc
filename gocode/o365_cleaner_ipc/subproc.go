package o365_cleaner_ipc

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
)

func SpawnSubprocessCleaner() error {
	var targetProcName string
	if runtime.GOOS == "windows" {
		targetProcName = ".\\cramc_o365_cleaner.exe"
	} else {
		targetProcName = "./cramc_o365_cleaner"
		_ = os.Chmod(targetProcName, 0755)
	}
	// golang 1.19 security update - won't check current working directory, had to use lookPath before start
	actualTarget, err := exec.LookPath(targetProcName)
	if err != nil && !errors.Is(err, exec.ErrDot) {
		return err
	}
	// verify hash of current version cleaner
	isLegit, err := validateHashOfSubproc(actualTarget)
	if err != nil {
		return err
	}
	if !isLegit {
		return customerrs.ErrSubprocessModified
	}
	// run proc
	rCleaner := exec.Command(actualTarget, common.RPCServerListen, common.RPCServerSecret)
	if err = rCleaner.Start(); err != nil {
		return err
	}
	go func() {
		_ = rCleaner.Wait()
	}()
	return nil
}

func validateHashOfSubproc(targetPath string) (bool, error) {
	if data, ok := os.LookupEnv("RunEnv"); ok {
		if data == "DEBUG" {
			return true, nil
		}
	}
	h := sha256.New()
	subP, err := os.OpenFile(targetPath, os.O_RDONLY, 0644)
	defer subP.Close()
	if err != nil {
		return false, err
	}
	_, err = io.Copy(h, subP)
	finalH := hex.EncodeToString(h.Sum(nil))
	common.Logger.Infoln("Subprocess hash: ", finalH)
	if finalH != common.RPCCleanerHash {
		return false, customerrs.ErrSubprocessModified
	} else {
		return true, nil
	}
}
