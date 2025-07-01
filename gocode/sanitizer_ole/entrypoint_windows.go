//go:build windows

package sanitizer_ole

import (
	"context"
	"cramc_go/common"
	"cramc_go/platform/windoge_utils"
	"cramc_go/telemetry"
	"github.com/google/uuid"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	modOle32                 = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeSecurity = modOle32.NewProc("CoInitializeSecurity")
	nullptr                  = uintptr(0)
	rpcAddr                  = `\\.\cramcPriv`
)

func StartSanitizer() error {
	// kill all office processes, to avoid any potential file lock.
	_, _ = windoge_utils.KillAllOfficeProcesses()
	common.Logger.Infoln("Triggered M365 Office processes killer.")

	// client id generation
	clientID, err := uuid.NewUUID()
	if err != nil {
		return err
	}
	telemetry.CaptureMessage("info", "Sanitizer Client ID: "+clientID.String())
	common.Logger.Infoln("Sanitizer Client ID: " + clientID.String())

	// check if run as system
	runAsSystem, _ := windoge_utils.CheckRunningUnderSYSTEM()
	if runAsSystem {
		//todo: impersonate then start process, otherwise directly spawn
	} else {

	}

	// iterate through workbooks
	for vObj := range common.SanitizeQueue {
		common.Logger.Debugln("Sanitizer Queue Received a New File.")
		// todo: get file from queue and send it out

	}
	common.Logger.Infoln("Sanitizer Finished.")
	return nil
}

func LiftVBAScriptingAccess(versionStr string, componentStr string) error {
	// this fix COM API via OLE returned null on VBProject access
	regK, openedExists, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Office\`+versionStr+`\`+componentStr+`\Security`, registry.ALL_ACCESS)
	if err != nil {
		common.Logger.Errorln("Failed to create registry key to lift VBOM restriction:", err)
		return err
	}
	if openedExists {
		common.Logger.Debugln("Registry key already exists, opened existing one.")
	}
	common.Logger.Debugln("Registry key Opened.")
	defer regK.Close()
	err = regK.SetDWordValue("AccessVBOM", (uint32)(1))
	if err != nil {
		common.Logger.Errorln("Failed to set registry value to lift VBOM restriction:", err)
		return err
	}
	common.Logger.Infoln("Registry value set to 1 for AccessVBOM.")
	return nil
}

func SpawnRPCServer() error {
	//TODO
}
