//go:build windows

package sanitizer_ole

import (
	"cramc_go/common"
	"cramc_go/platform/windoge_utils"
	"cramc_go/sanitizer_ole/pbrpc"
	"cramc_go/telemetry"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/sys/windows"
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
	rpcHelperExe             = "privhelper.exe"
	clientConnPipeAddr       = "winiopipe://cramcPriv"
)

func StartSanitizer() error {
	// kill all office processes, to avoid any potential file lock.
	_, _ = windoge_utils.KillAllOfficeProcesses()
	common.Logger.Info("Triggered M365 Office processes killer.")

	// client id generation
	clientID, err := uuid.NewUUID()
	if err != nil {
		return err
	}
	telemetry.CaptureMessage("info", "Sanitizer Client ID: "+clientID.String())
	common.Logger.Info("Sanitizer Client ID: " + clientID.String())

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
		// https://learn.microsoft.com/en-us/windows/console/creation-of-a-console
		// https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
		// /opt/homebrew/opt/go/libexec/src/os/exec/exec.go:703 go1.24.4
		rpcSProcAddr := &os.ProcAttr{
			// if you set token and leave `Env` empty, it will auto create.
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
			Sys: &syscall.SysProcAttr{
				HideWindow:    true,
				CreationFlags: windows.CREATE_NEW_PROCESS_GROUP, // detach from original proc group
				Token:         syscall.Token(impTkn),
			},
		}
		rpcProc, err2 = os.StartProcess(privHelperPath, nil, rpcSProcAddr)
		if err2 != nil {
			telemetry.CaptureException(err2, "Main.StartSanitizer.RPCServer.Impersonate")
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
			telemetry.CaptureException(err2, "Main.StartSanitizer.RPCServer.Normal")
			return err2
		}
	}
	// sleep 3 seconds for excel to startup
	time.Sleep(3 * time.Second)
	// Connect to RPC Client and ping
	rpCli, err := InitSimpleRPCClient(clientID.String(), clientConnPipeAddr)
	if err != nil {
		common.Logger.Error("Failed to initialize RPC Client: " + err.Error())
		return err
	}
	err = rpCli.Connect()
	if err != nil {
		telemetry.CaptureException(err, "Main.StartSanitizer.GRPCClient.Connect")
		common.Logger.Error("Failed to connect to RPC endpoint: " + err.Error())
		return err
	}
	common.Logger.Info("Sanitizer RPC Client Connected (ignore this message if using local named pipe).")
	// ping and check online
	err = rpCli.Ping()
	if err != nil {
		common.Logger.Error("Failed to ping RPC endpoint: " + err.Error())
		return err
		// in rare cases, if the RPC cannot connect correctly, privHelper cannot successfully exit.
	}
	common.Logger.Info("ping RPC endpoint succeeded.")

	// Create stream object
	sanReqCtrlC := make(chan struct{}, 1)
	biDStream, err := rpCli.CallDocSanitizeReqSender(sanReqCtrlC)
	if err != nil {
		common.Logger.Error("Failed to start streaming sanitize request: " + err.Error())
		return err
	}
	// iterate through workbooks
	for vObj := range common.SanitizeQueue {
		common.Logger.Debug("Sanitizer Queue Received a New File.")
		// get file from queue and send it out, waiting for response
		msgMeta := rpCli.PrepareMsgMeta()
		common.Logger.Info(fmt.Sprintf("New File Sanitize Request Prepared with MsgID: %d , FilePath: %s ", msgMeta.GetMessageID(), vObj.Path))
		sanReq := &pbrpc.SanitizeDocRequest{}
		sanReq.SetMeta(msgMeta)
		sanReq.SetAction(vObj.Action)
		sanReq.SetDestModule(vObj.DestModule)
		sanReq.SetDetectionName(vObj.DetectionName)
		sanReq.SetPath(vObj.Path)
		err = biDStream.Send(sanReq)
		if err != nil {
			common.Logger.Error("Failed to send sanitize request: " + err.Error())
			continue
		}
		common.Logger.Info(fmt.Sprintf("New File Sanitize Request Sent, MsgID: %d .", msgMeta.GetMessageID()))
	}

	// after all document had been sent, close stream
	_ = biDStream.CloseSend()

	// send a termination message and close connection
	_ = rpCli.SendQuit()
	_ = rpCli.Disconnect()

	// wait for termination of rpc server till timed out
	rpcSCOnce := &sync.Once{}
	rpcSC := make(chan struct{}, 1)
	go func() {
		// if terminating info sent, after 300 seconds, force kill process
		select {
		case <-time.After(300 * time.Second):
			common.Logger.Info("RPC Server Termination Timed out for 300 seconds. You may manually reboot your system or kill it.")
			telemetry.CaptureMessage("warn", "Privilege RPC Server Termination Timed Out.")
			// close chan and force terminate process
			rpcSCOnce.Do(func() {
				_ = rpcProc.Kill()
				common.Logger.Warn("RPC Server Force Terminated.")
				close(rpcSC)
			})
		case <-rpcSC:
			common.Logger.Info("Sanitizer RPC Server Gracefully shutdown - done.")
		}
	}()
	common.Logger.Info("Sanitizer RPC Server Termination Started, wait for 300 seconds.")
	_, _ = rpcProc.Wait()
	rpcSCOnce.Do(func() {
		rpcSC <- struct{}{}
		close(rpcSC)
	})
	common.Logger.Info("RPC Server terminated correctly.")
	return nil
}

func LiftVBAScriptingAccess(versionStr string, componentStr string) error {
	// this fix COM API via OLE returned null on VBProject access
	regK, openedExists, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Office\`+versionStr+`\`+componentStr+`\Security`, registry.ALL_ACCESS)
	if err != nil {
		common.Logger.Error("Failed to create registry key to lift VBOM restriction: " + err.Error())
		return err
	}
	if openedExists {
		common.Logger.Debug("Registry key already exists, opened existing one.")
	}
	common.Logger.Debug("Registry key Opened.")
	defer regK.Close()
	err = regK.SetDWordValue("AccessVBOM", (uint32)(1))
	if err != nil {
		common.Logger.Error("Failed to set registry value to lift VBOM restriction: " + err.Error())
		return err
	}
	common.Logger.Info("Registry value set to 1 for AccessVBOM.")
	return nil
}
