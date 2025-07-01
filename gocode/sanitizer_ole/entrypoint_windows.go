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

	wg2 := &sync.WaitGroup{}
	// iterate through workbooks
	for vObj := range common.SanitizeQueue {
		common.Logger.Debugln("Sanitizer Queue Received a New File.")
		// todo: get file from queue and send it out
		switch vObj.Action {
		case "remediate":
			// parse and take action
			func(eWorker *ExcelWorker) {
				// 60 seconds should be sufficient for opening and sanitizing a single normal doc
				//
				// unfortunately, in some rare cases, it cost around 109 seconds for open.
				// in case of such a sucking document, have to change timeout to 180s
				ctx, cancelF := context.WithTimeout(context.TODO(), 180*time.Second)
				defer cancelF()
				// notice if finished earlier
				doneC := make(chan struct{}, 1)
				wg2.Add(1)
				common.Logger.Debugln("Sanitize workbook started, wg2 += 1")
				go func() {
					defer wg2.Done()
					// lock to ensure only single doc at a time
					eWorker.Lock()
					// must unlock whatever happened
					defer eWorker.Unlock()
					// open workbook
					common.Logger.Infoln("Opening workbook in sanitizer: ", fPathNonVariant)
					err3 := eWorker.OpenWorkbook(fPathNonVariant)
					if err3 != nil {
						common.Logger.Errorln("Failed to open workbook in sanitizer:", err3)
						doneC <- struct{}{}
						return
					}
					common.Logger.Debugln("Workbook opened: ", fPathNonVariant)
					defer func() {
						err4 := eWorker.SaveAndCloseWorkbook()
						if err != nil {
							common.Logger.Errorln("Failed to save and close workbook in defer Sanitizer:", err4)
						}
						time.Sleep(1 * time.Second)
						// rename file and save to clean state cache of cloud-storage provider
						err4 = renameFileAndSave(fPathNonVariant)
						if err4 != nil {
							common.Logger.Errorln("Rename file failed in sanitizer:", err4.Error())
						}
						common.Logger.Infoln("Workbook Sanitized: ", fPathNonVariant)
					}()
					// sanitize
					common.Logger.Debugln("Sanitize Workbook VBA Module now.")
					err3 = eWorker.SanitizeWorkbook(vObj.DestModule)
					if err3 != nil {
						common.Logger.Errorln("Failed to sanitize workbook:", err3)
						doneC <- struct{}{}
						return
					}
					common.Logger.Debugln("Sanitize Workbook VBA Module finished, doneC returned.")
					doneC <- struct{}{}
				}()
				select {
				case <-doneC:
					// properly remediated
					// go ahead
					common.Logger.Debugln("Sanitize workbook finished, doneC returned correctly.")
					return
				case <-ctx.Done():
					// timed out or error
					err5 := ctx.Err()
					if err5 != nil {
						telemetry.CaptureException(err5, "SanitizeWorkbookTimedOut")
						common.Logger.Errorln("Failed to sanitize workbook, timed out:", err5)
					}
					common.Logger.Infoln("Sanitize workbook timed out, ctx.Done() returned, go to force clean.")
					// for GC, cleanup and rebuild excel instance
					originalDbgStatus := eWorker.inDbg
					eWorker.Quit(true)
					// safely ignore errors as it's already built correctly before
					_ = eWorker.Init(originalDbgStatus)
					_ = eWorker.GetWorkbooks()
				}
			}(eWorker)
		default:
			common.Logger.Warnln("Unsupported action type: ", vObj.Action)
			continue
		}
	}
	wg2.Wait()
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

func SpawnRPCServer() {
	//TODO
}
