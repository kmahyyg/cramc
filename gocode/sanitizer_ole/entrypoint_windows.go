//go:build windows

package sanitizer_ole

import (
	"context"
	"cramc_go/common"
	"cramc_go/platform/windoge_utils"
	"cramc_go/telemetry"
	ole "github.com/go-ole/go-ole"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	cleanupComment = `' Sanitized by CRAMC
Private Sub CRAMCPlaceholder()
    ' This ensures the comment above persists
End Sub
`
)

func StartSanitizer() error {
	// enable scripting access to VBAObject Model
	err := LiftVBAScriptingAccess("16.0", "Excel")
	if err != nil {
		return err
	}
	// kill all office processes, to avoid any potential file lock.
	_, _ = windoge_utils.KillAllOfficeProcesses()
	common.Logger.Infoln("Triggered M365 Office processes killer.")

	// prepare to call ole
	err = ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		return err
	}
	defer ole.CoUninitialize()

	// new approach: bundled
	inDebugging := false
	if data, ok := os.LookupEnv("RunEnv"); ok {
		if data == "DEBUG" {
			inDebugging = true
		}
	}
	eWorker := &ExcelWorker{}
	err = eWorker.Init(inDebugging)
	if err != nil {
		common.Logger.Errorln("Failed to initialize excel worker:", err)
		return err
	}
	defer eWorker.Quit(false)
	err = eWorker.GetWorkbooks()
	if err != nil {
		common.Logger.Errorln("Failed to get workbooks:", err)
		return err
	}
	common.Logger.Infoln("Excel.Application worker initialized.")

	wg2 := &sync.WaitGroup{}
	// iterate through workbooks
	for vObj := range common.SanitizeQueue {
		common.Logger.Debugln("Sanitizer Queue Received a New File.")
		// change path separator, make sure consistent in os-level
		fPathNonVariant, err2 := filepath.Abs(vObj.Path)
		if err2 != nil {
			common.Logger.Errorln("Failed to get absolute path:", err2)
			continue
		}
		// backup file
		err = gzBakFile(fPathNonVariant)
		if err != nil {
			common.Logger.Errorln("Backup file failed:", err.Error())
		}
		common.Logger.Infoln("Original file backup succeeded: ", vObj.Path)
		// sleep 1 second to leave space for saving
		time.Sleep(1 * time.Second)
		switch vObj.Action {
		case "sanitize":
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
					err := eWorker.OpenWorkbook(fPathNonVariant)
					if err != nil {
						common.Logger.Errorln("Failed to open workbook in sanitizer:", err)
						doneC <- struct{}{}
						return
					}
					common.Logger.Debugln("Workbook opened: ", fPathNonVariant)
					defer func() {
						err = eWorker.SaveAndCloseWorkbook()
						if err != nil {
							common.Logger.Errorln("Failed to save and close workbook in defer Sanitizer:", err)
						}
						time.Sleep(1 * time.Second)
						// rename file and save to clean state cache of cloud-storage provider
						err = renameFileAndSave(fPathNonVariant)
						if err != nil {
							common.Logger.Errorln("Rename file failed in sanitizer:", err.Error())
						}
						common.Logger.Infoln("Workbook Sanitized: ", fPathNonVariant)
					}()
					// sanitize
					common.Logger.Debugln("Sanitize Workbook VBA Module now.")
					err = eWorker.SanitizeWorkbook(vObj.DestModule)
					if err != nil {
						common.Logger.Errorln("Failed to sanitize workbook:", err)
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
					err := ctx.Err()
					if err != nil {
						telemetry.CaptureException(err, "SanitizeWorkbookTimedOut")
						common.Logger.Errorln("Failed to sanitize workbook, timed out:", err)
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

type MsoAutomationSecurity int

const (
	MsoAutomationSecurityLow = 1 + iota
	MsoAutomationSecurityByUI
	MsoAutomationSecurityForceDisable
)

type XlCalculation int

const (
	XlCalculationManual        = -4135
	XlCalculationAutomatic     = -4105
	XlCalculationSemiautomatic = 2
)

type XlUpdateLinks int

const (
	XlUpdateLinksUserSetting = 1 + iota
	XlUpdateLinksNever
	XlUpdateLinksAlways
)
