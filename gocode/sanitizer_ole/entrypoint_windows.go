//go:build windows

package sanitizer_ole

import (
	"cramc_go/common"
	"cramc_go/platform/windoge_utils"
	"cramc_go/telemetry"
	"github.com/google/uuid"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"syscall"
)

var (
	modOle32                 = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeSecurity = modOle32.NewProc("CoInitializeSecurity")
	nullptr                  = uintptr(0)
	rpcAddr                  = `\\.\cramcPriv`
	rpcHelperExe             = "privhelper.exe"
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

	// get privhelper executable path
	exePath, _ := os.Executable()
	privHelperPath := filepath.Join(filepath.Dir(exePath), rpcHelperExe)

	var rpcProc *os.Process
	// check if run as system
	runAsSystem, _ := windoge_utils.CheckRunningUnderSYSTEM()
	if runAsSystem {
		// impersonate then start process, otherwise directly spawn
		userTkn, err2 := windoge_utils.GetLoggedInUserToken(windows.TokenPrimary)
		if err2 != nil {
			return err2
		}
		impTkn := (windows.Token)(userTkn)
		defer impTkn.Close()
		procEnvBlk, err2 := impTkn.Environ(false)
		if err2 != nil {
			return err2
		}
		rpcSProcAddr := &os.ProcAttr{
			Env: procEnvBlk,
			Sys: &syscall.SysProcAttr{
				HideWindow:    true,
				CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
				Token:         syscall.Token(impTkn),
			},
		}
		rpcProc, err2 = os.StartProcess(privHelperPath, nil, rpcSProcAddr)
		if err2 != nil {
			return err2
		}
	} else {
		var err2 error
		rpcSProcAddr := &os.ProcAttr{
			Sys: &syscall.SysProcAttr{
				HideWindow:    true,
				CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
			},
		}
		rpcProc, err2 = os.StartProcess(privHelperPath, nil, rpcSProcAddr)
		if err2 != nil {
			return err2
		}
	}
	// iterate through workbooks
	for vObj := range common.SanitizeQueue {
		common.Logger.Debugln("Sanitizer Queue Received a New File.")
		// todo: get file from queue and send it out

	}
	common.Logger.Infoln("Sanitizer Finished, now sending control message to terminate RPC server.")
	// todo: send control msg
	_, _ = rpcProc.Wait()
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
